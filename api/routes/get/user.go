// Package get contains routes for http.MethodGet requests.
package get

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Potat-Industries/potat-api/api"
	"github.com/Potat-Industries/potat-api/api/middleware"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/gorilla/mux"
)

type potatoInfo struct {
	JoinedAt      string  `json:"joinedAt"`
	Steal         steal   `json:"steal"`
	Trample       trample `json:"trample"`
	Potato        potato  `json:"potato"`
	Duel          duel    `json:"duel"`
	Gamble        gamble  `json:"gamble"`
	Quiz          quiz    `json:"quiz"`
	Cdr           cdr     `json:"cdr"`
	Eat           eat     `json:"eat"`
	Rank          int     `json:"rank"`
	TaxMultiplier int     `json:"taxMultiplier"`
	Prestige      int     `json:"prestige"`
	Count         int     `json:"count"`
	Verbose       bool    `json:"verbose"`
}

type potato struct {
	AverageResponse string `json:"averageResponse"`
	ReadyAt         int    `json:"readyAt"`
	Usage           int    `json:"usage"`
	Ready           bool   `json:"ready"`
}

type cdr struct {
	ReadyAt int  `json:"readyAt"`
	Ready   bool `json:"ready"`
}

type trample struct {
	TrampledBy    *string `json:"trampledBy"`
	ReadyAt       int     `json:"readyAt"`
	TrampleCount  int     `json:"trampleCount"`
	TrampledCount int     `json:"trampledCount"`
	Ready         bool    `json:"ready"`
}

type steal struct {
	StoleBy      *string `json:"stoleBy"`
	StolenAmount *int    `json:"stolenAmount"`
	ReadyAt      int     `json:"readyAt"`
	StolenCount  int     `json:"stolenCount"`
	TheftCount   int     `json:"theftCount"`
	Ready        bool    `json:"ready"`
}

type eat struct {
	ReadyAt int  `json:"readyAt"`
	Ready   bool `json:"ready"`
}

type quiz struct {
	ReadyAt   int  `json:"readyAt"`
	Ready     bool `json:"ready"`
	Attempted int  `json:"attempted"`
	Completed int  `json:"completed"`
}

type gamble struct {
	WinCount    int `json:"winCount"`
	LoseCount   int `json:"loseCount"`
	TotalWins   int `json:"totalWins"`
	TotalLosses int `json:"totalLosses"`
}

type duel struct {
	WinCount     int `json:"winCount"`
	LoseCount    int `json:"loseCount"`
	TotalWins    int `json:"totalWins"`
	TotalLosses  int `json:"totalLosses"`
	CaughtLosses int `json:"caughtLosses"`
}

// UsersResponse is the response type for the /users/{username} endpoint.
type UsersResponse = common.GenericResponse[UserInfo]

// UserInfo is the structure for UsersResponse.
type UserInfo struct {
	User     *common.User    `json:"user"`
	Channel  *common.Channel `json:"channel"`
	Potatoes *potatoInfo     `json:"potatoes"`
}

func init() {
	api.SetRoute(api.Route{
		Path:    "/users/{username}",
		Method:  http.MethodGet,
		Handler: getUsers,
		UseAuth: false,
	})
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
) *potatoInfo {
	if data == nil {
		return nil
	}

	return &potatoInfo{
		JoinedAt:      data.FirstSeen,
		Count:         data.PotatoCount,
		Prestige:      data.PotatoPrestige,
		Rank:          data.PotatoRank,
		TaxMultiplier: data.TaxMultiplier,
		Verbose:       data.NotVerbose,
		Potato: potato{
			ReadyAt:         lastPotato,
			Ready:           time.Now().UnixMilli() > int64(lastPotato),
			Usage:           data.HarvestCount,
			AverageResponse: data.AverageResponseTime,
		},
		Cdr: cdr{
			ReadyAt: lastCDR,
			Ready:   time.Now().UnixMilli()-int64(lastCDR) > 30*60*1000,
		},
		Trample: trample{
			ReadyAt:       lastTrample,
			Ready:         time.Now().UnixMilli() > int64(lastTrample),
			TrampleCount:  data.TrampleCount,
			TrampledCount: data.TrampledCount,
			TrampledBy:    data.TrampledBy,
		},
		Steal: steal{
			ReadyAt:      lastSteal,
			Ready:        time.Now().UnixMilli() > int64(lastSteal),
			StolenCount:  data.StolenCount,
			TheftCount:   data.TheftCount,
			StoleBy:      data.StoleFrom,
			StolenAmount: data.StoleAmount,
		},
		Eat: eat{
			ReadyAt: lastEat,
			Ready:   time.Now().UnixMilli() > int64(lastEat),
		},
		Quiz: quiz{
			ReadyAt:   lastQuiz,
			Ready:     getQuizReady(lastQuiz),
			Attempted: data.QuizCount,
			Completed: data.QuizCompleteCount,
		},
		Gamble: gamble{
			WinCount:    data.GambleWinCount,
			LoseCount:   data.GambleLossCount,
			TotalWins:   data.GambleWinsTotal,
			TotalLosses: data.GambleLossesTotal,
		},
		Duel: duel{
			WinCount:     data.DuelWinCount,
			LoseCount:    data.DuelLossCount,
			TotalWins:    data.DuelWinsAmount,
			TotalLosses:  data.DuelLossesAmount,
			CaughtLosses: data.DuelCaughtLosses,
		},
	}
}

func loadUser(ctx context.Context, user string) UserInfo { //nolint:gocognit,cyclop
	postgres, ok := ctx.Value(middleware.PostgresKey).(*db.PostgresClient)
	if !ok {
		logger.Error.Println("Postgres client not found in context")

		return UserInfo{}
	}

	redis, ok := ctx.Value(middleware.RedisKey).(*db.RedisClient)
	if !ok {
		logger.Error.Println("Redis client not found in context")

		return UserInfo{}
	}

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

		data, err := postgres.GetUserByName(ctx, user)
		if err != nil && !errors.Is(err, db.ErrPostgresNoRows) {
			logger.Warn.Println("Error fetching user data: ", err)
		} else {
			userData = data
		}
	}()

	go func() {
		defer wg.Done()

		data, err := postgres.GetChannelByName(ctx, user, common.TWITCH)
		if err != nil && !errors.Is(err, db.ErrPostgresNoRows) {
			logger.Warn.Println("Error fetching channel data: ", err)
		} else {
			channelData = data
		}
	}()

	go func() {
		defer wg.Done()

		data, err := postgres.GetPotatoData(ctx, user)
		if err != nil && !errors.Is(err, db.ErrPostgresNoRows) {
			logger.Warn.Println("Error fetching potato data: ", err)
		} else {
			potatData = data
		}
	}()

	go func() {
		defer wg.Done()

		data, err := redis.Get(ctx, fmt.Sprintf("potato:%s", user)).Int()
		if err != nil && !errors.Is(err, db.ErrRedisNil) {
			logger.Warn.Println("Error fetching last potato: ", err)
		} else {
			lastPotato = data
		}
	}()

	go func() {
		defer wg.Done()

		data, err := redis.Get(ctx, fmt.Sprintf("cdr:%s", user)).Int()
		if err != nil && !errors.Is(err, db.ErrRedisNil) {
			logger.Warn.Println("Error fetching last cdr: ", err)
		} else {
			lastCDR = data
		}
	}()

	go func() {
		defer wg.Done()

		data, err := redis.Get(ctx, fmt.Sprintf("trample:%s", user)).Int()
		if err != nil && !errors.Is(err, db.ErrRedisNil) {
			logger.Warn.Println("Error fetching last trample: ", err)
		} else {
			lastTrample = data
		}
	}()

	go func() {
		defer wg.Done()

		data, err := redis.Get(ctx, fmt.Sprintf("steal:%s", user)).Int()
		if err != nil && !errors.Is(err, db.ErrRedisNil) {
			logger.Warn.Println("Error fetching last steal: ", err)
		} else {
			lastSteal = data
		}
	}()

	go func() {
		defer wg.Done()

		data, err := redis.Get(ctx, fmt.Sprintf("eat:%s", user)).Int()
		if err != nil && !errors.Is(err, db.ErrRedisNil) {
			logger.Warn.Println("Error fetching last eat: ", err)
		} else {
			lastEat = data
		}
	}()

	go func() {
		defer wg.Done()

		data, err := redis.Get(ctx, fmt.Sprintf("quiz:%s", user)).Int()
		if err != nil && !errors.Is(err, db.ErrRedisNil) {
			logger.Warn.Println("Error fetching last quiz: ", err)
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
		User:     userData,
		Channel:  channelData,
		Potatoes: potatoes,
	}
}

func getUsers(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	params := mux.Vars(request)
	word := params["username"]
	if word == "" {
		err := "No username provided"

		res := UsersResponse{
			Data:   &[]UserInfo{},
			Errors: &[]common.ErrorMessage{{Message: err}},
		}

		api.GenericResponse(writer, http.StatusBadRequest, res, start)

		return
	}

	userArray := strings.Split(word, ",")
	if len(userArray) > 25 {
		err := fmt.Sprintf("Too many users provided. Expected 1-25, found %d", len(userArray))

		res := UsersResponse{
			Data:   &[]UserInfo{},
			Errors: &[]common.ErrorMessage{{Message: err}},
		}

		api.GenericResponse(writer, http.StatusBadRequest, res, start)

		return
	}

	dataChan := make(chan UserInfo, len(userArray))

	var dataArray []UserInfo
	var wg sync.WaitGroup

	wg.Add(len(userArray))
	for _, user := range userArray {
		go func(ctx context.Context) {
			defer wg.Done()
			info := loadUser(ctx, user)
			dataChan <- info
		}(request.Context())
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
			Data:   &dataArray,
			Errors: &[]common.ErrorMessage{{Message: err}},
		}

		api.GenericResponse(writer, http.StatusNotFound, res, start)

		return
	}

	res := UsersResponse{
		Data: &dataArray,
	}

	api.GenericResponse(writer, http.StatusOK, res, start)
}
