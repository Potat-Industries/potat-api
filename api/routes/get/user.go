package get

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"potat-api/api"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/utils"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type PotatoInfo struct {
	JoinedAt      string  `json:"joinedAt"`
	Count         int     `json:"count"`
	Prestige      int     `json:"prestige"`
	Rank          int     `json:"rank"`
	TaxMultiplier int     `json:"taxMultiplier"`
	Verbose       bool    `json:"verbose"`
	Potato        Potato  `json:"potato"`
	Cdr           CDR     `json:"cdr"`
	Trample       Trample `json:"trample"`
	Steal         Steal   `json:"steal"`
	Eat           Eat     `json:"eat"`
	Quiz          Quiz    `json:"quiz"`
	Gamble        Gamble  `json:"gamble"`
	Duel          Duel    `json:"duel"`
}

type Potato struct {
	ReadyAt          int `json:"readyAt"`
	Ready            bool   `json:"ready"`
	Usage            int    `json:"usage"`
	AverageResponse  string `json:"averageResponse"`
}

type CDR struct {
	ReadyAt int `json:"readyAt"`
	Ready   bool   `json:"ready"`
}

type Trample struct {
	ReadyAt       int  `json:"readyAt"`
	Ready         bool    `json:"ready"`
	TrampleCount  int     `json:"trampleCount"`
	TrampledCount int     `json:"trampledCount"`
	TrampledBy    *string `json:"trampledBy"`
}

type Steal struct {
	ReadyAt       int  `json:"readyAt"`
	Ready         bool    `json:"ready"`
	StolenCount   int     `json:"stolenCount"`
	TheftCount    int     `json:"theftCount"`
	StoleBy       *string `json:"stoleBy"`
	StolenAmount  *int `json:"stolenAmount"`
}

type Eat struct {
	ReadyAt int `json:"readyAt"`
	Ready   bool    `json:"ready"`
}

type Quiz struct {
	ReadyAt   int `json:"readyAt"`
	Ready     bool    `json:"ready"`
	Attempted int     `json:"attempted"`
	Completed int     `json:"completed"`
}

type Gamble struct {
	WinCount    int `json:"winCount"`
	LoseCount   int `json:"loseCount"`
	TotalWins   int `json:"totalWins"`
	TotalLosses int `json:"totalLosses"`
}

type Duel struct {
	WinCount     int `json:"winCount"`
	LoseCount    int `json:"loseCount"`
	TotalWins    int `json:"totalWins"`
	TotalLosses  int `json:"totalLosses"`
	CaughtLosses int `json:"caughtLosses"`
}

type ErrorMessage struct {
	Message string `json:"message"`
}

type UsersResponse struct {
	Data   *[]UserInfo   `json:"data"`
	Errors *ErrorMessage `json:"errors,omitempty"`
}

type UserInfo struct {
	User     *common.User       `json:"user"`
	Channel  *common.Channel    `json:"channel"`
	Potatoes *PotatoInfo       `json:"potatoes"`
}

func init() {
	api.SetRoute(api.Route{
		Path:    "/users/{username}",
		Method:  "GET",
		Handler: getUsers,
	})
}

func response(
	r http.ResponseWriter, 
	code int, 
	response UsersResponse,
	start time.Time,
) {
	r.WriteHeader(code)
	r.Header().Set("Content-Type", "application/json")
	r.Header().Set("X-Potat-Request-Duration", time.Since(start).String())
	
	json.NewEncoder(r).Encode(response)
}

func getQuizReady(lastQuiz int) bool {
	return time.Now().UnixMilli() > int64(lastQuiz)
}

func tidyPotatoInfo(
	data *common.PotatoData,
	lastPotato,
	lastCDR,
	lastTrample,
	lastSteal,
	lastEat,
	lastQuiz int,
) *PotatoInfo {
	if data == nil {
		return nil
	}

	return &PotatoInfo{
		JoinedAt:      data.FirstSeen,
		Count:         data.PotatoCount,
		Prestige:      data.PotatoPrestige,
		Rank:          data.PotatoRank,
		TaxMultiplier: data.TaxMultiplier,
		Verbose:       data.NotVerbose,
		Potato: Potato{
			ReadyAt:          lastPotato,
			Ready:            time.Now().UnixMilli() > int64(lastPotato),
			Usage:            data.HarvestCount,
			AverageResponse:  data.AverageResponseTime,
		},
		Cdr: CDR{
			ReadyAt: lastCDR,
			Ready:   time.Now().UnixMilli() - int64(lastCDR) > 30 * 60 * 1000,
		},
		Trample: Trample{
			ReadyAt:       lastTrample,
			Ready:         time.Now().UnixMilli() > int64(lastTrample),
			TrampleCount:  data.TrampleCount,
			TrampledCount: data.TrampledCount,
			TrampledBy:    data.TrampledBy,
		},
		Steal: Steal{
			ReadyAt:       lastSteal,
			Ready:         time.Now().UnixMilli() > int64(lastSteal),
			StolenCount:   data.StolenCount,
			TheftCount:    data.TheftCount,
			StoleBy:       data.StoleFrom,
			StolenAmount:  data.StoleAmount,
		},
		Eat: Eat{
			ReadyAt: lastEat,
			Ready:   time.Now().UnixMilli() > int64(lastEat),
		},
		Quiz: Quiz{
			ReadyAt:   lastQuiz,
			Ready:     getQuizReady(lastQuiz),
			Attempted: data.QuizCount,
			Completed: data.QuizCompleteCount,
		},
		Gamble: Gamble{
			WinCount:    data.GambleWinCount,
			LoseCount:   data.GambleLossCount,
			TotalWins:   data.GambleWinsTotal,
			TotalLosses: data.GambleLossesTotal,
		},
		Duel: Duel{
			WinCount:     data.DuelWinCount,
			LoseCount:    data.DuelLossCount,
			TotalWins:    data.DuelWinsAmount,
			TotalLosses:  data.DuelLossesAmount,
			CaughtLosses: data.DuelCaughtLosses,
		},
	}
}

func loadUser(ctx context.Context, user string) UserInfo {
	var wg sync.WaitGroup

	wg.Add(9)
	var userData *common.User
	var channelData *common.Channel
	var potatData *common.PotatoData
	var lastPotato int
	var lastCDR int
	var lastTrample int
	var lastSteal int
	var lastEat int
	var lastQuiz int

	go func() {
		defer wg.Done()
		data, err := db.Postgres.GetUserByName(ctx, user)
		if err != nil && err != db.PostgresNoRows {
			utils.Warn.Println("Error fetching user data: ", err)
		} else {
			userData = data
		}
	}()

	go func() {
		defer wg.Done()
		data, err := db.Postgres.GetChannelByName(ctx, user)
		if err != nil && err != db.PostgresNoRows {
			utils.Warn.Println("Error fetching channel data: ", err)
		} else {
			channelData = data
		}
	}()

	go func() {
		defer wg.Done()
		data, err := db.Postgres.GetPotatoData(ctx, user)
		if err != nil && err != db.PostgresNoRows {
			utils.Warn.Println("Error fetching potato data: ", err)
		} else {
			potatData = data
		}
	}()

	go func() {
		defer wg.Done()
		data, err := db.Redis.Get(ctx, fmt.Sprintf("potato:%s", user)).Int()
		if err != nil && err != db.RedisErrNil {
			utils.Warn.Println("Error fetching last potato: ", err)
		} else {
			lastPotato = data
		}
	}()

	go func() {
		defer wg.Done()
		data, err := db.Redis.Get(ctx, fmt.Sprintf("cdr:%s", user)).Int()
		if err != nil && err != db.RedisErrNil {
			utils.Warn.Println("Error fetching last cdr: ", err)
		} else {
			lastCDR = data
		}
	}()

	go func() {
		defer wg.Done()
		data, err := db.Redis.Get(ctx, fmt.Sprintf("trample:%s", user)).Int()
		if err != nil && err != db.RedisErrNil {
			utils.Warn.Println("Error fetching last trample: ", err)
		} else {
			lastTrample = data
		}
	}()

	go func() {
		defer wg.Done()
		data, err := db.Redis.Get(ctx, fmt.Sprintf("steal:%s", user)).Int()
		if err != nil && err != db.RedisErrNil {
			utils.Warn.Println("Error fetching last steal: ", err)
		} else {
			lastSteal = data
		}
	}()

	go func() {
		defer wg.Done()
		data, err := db.Redis.Get(ctx, fmt.Sprintf("eat:%s", user)).Int()
		if err != nil && err != db.RedisErrNil {
			utils.Warn.Println("Error fetching last eat: ", err)
		} else {
			lastEat = data
		}
	}()

	go func() {
		defer wg.Done()
		data, err := db.Redis.Get(ctx, fmt.Sprintf("quiz:%s", user)).Int()
		if err != nil && err != db.RedisErrNil {
			utils.Warn.Println("Error fetching last quiz: ", err)
		} else {
			lastQuiz = data
		}
	}()

	wg.Wait()

	potatoes := tidyPotatoInfo(
		potatData, 
		lastPotato,
		lastCDR,
		lastTrample,
		lastSteal,
		lastEat,
		lastQuiz,
	)

	return UserInfo{
		User: userData,
		Channel: channelData,
		Potatoes: potatoes,
	}
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	params := mux.Vars(r)
	word := params["username"]
	if word == "" {
		err := "No username provided"

		res := UsersResponse{
			Data: &[]UserInfo{},
			Errors: &ErrorMessage{Message: err},
		}

		response(w, http.StatusBadRequest, res, start)
		return
	}

	userArray := strings.Split(word, ",")
	if len(userArray) > 25 {
		err := fmt.Sprintf("Too many users provided. Expected 1-25, found %d", len(userArray))

		res := UsersResponse{
			Data: &[]UserInfo{},
			Errors: &ErrorMessage{Message: err},
		}

		response(w, http.StatusBadRequest, res, start)
		return
	}

	dataChan := make(chan UserInfo, len(userArray))

	var dataArray []UserInfo
	var wg sync.WaitGroup

	wg.Add(len(userArray))
	for _, user := range userArray {
		go func() {	
			defer wg.Done()
			info := loadUser(r.Context(), user)
			dataChan <- info
		}()
	}

	go func() {
		wg.Wait()
		close(dataChan)
	}()

	for userData := range dataChan {
		dataArray = append(dataArray, userData)
	}

	if dataArray[0].User == nil {
		err := "User not found"

		res := UsersResponse{
			Data: &dataArray,
			Errors: &ErrorMessage{Message: err},
		}

		response(w, http.StatusNotFound, res, start)
		return
	}

	res := UsersResponse{
		Data: &dataArray,
	}

	response(w, http.StatusOK, res, start)
}