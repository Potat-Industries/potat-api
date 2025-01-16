package get

import (
	"context"
	"encoding/json"
	"net/http"
	"potat-api/api"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/utils"
	"time"
)

type HelpResponse = common.GenericResponse[common.Command]

func init() {
	api.SetRoute(api.Route{
		Path:    "/help",
		Method:  http.MethodGet,
		Handler: getCommandsHandler,
	}, false)
}

func setCache(key string, data interface{}) {
	err := db.Redis.SetEx(context.Background(), key, data, time.Hour).Err()
	if err != nil {
		utils.Error.Printf("Error caching commands: %v", err)
	}
}

func getCache(ctx context.Context, key string) (*[]common.Command, error) {
	data, err := db.Redis.Get(ctx, key).Bytes()
	if (err != nil && err != db.RedisErrNil) || data == nil {
		return &[]common.Command{}, err
	}

	var commands []common.Command
	err = json.Unmarshal([]byte(data), &commands)
	if err != nil {
		return &[]common.Command{}, err
	}

	return &commands, nil
}

func filterCommands(commands []common.Command) []common.Command {
	filteredCommands := make([]common.Command, 0)
	for _, command := range commands {
		if command.Category == "unlisted" {
			continue
		}
		filteredCommands = append(filteredCommands, command)
	}
	return filteredCommands
}

func getCommandsCallback(w http.ResponseWriter, start time.Time) func ([]byte) {
	return func(data []byte) {
		var commandsJson []common.Command
		err := json.Unmarshal(data, &commandsJson)
		if err != nil {
			utils.Error.Printf("Error unmarshalling commands: %v", err)
			api.GenericResponse(w, http.StatusInternalServerError, HelpResponse{
				Data: &[]common.Command{},
				Errors: &[]common.ErrorMessage{{Message: "Error unmarshalling commands"}},
			}, start)
			return
		}
		filteredCommands := filterCommands(commandsJson)

		if (len(filteredCommands) > 0) {
			go setCache("website:commands", data)
		}

		api.GenericResponse(w, http.StatusOK, HelpResponse{
			Data: &filteredCommands,
		}, start)
		return
	}
}

func getCommandsHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	cache, err := getCache(r.Context(), "website:commands")
	if err == nil && cache != nil {
		w.Header().Set("X-Cache-Hit", "HIT")
		api.GenericResponse(w, http.StatusOK, HelpResponse{
			Data: cache,
		}, start)
		return
	} else {
		w.Header().Set("X-Cache-Hit", "MISS")
	}

	err = utils.RequestManager(
		r.Context(),
		5 * time.Second,
		"get-commands",
		getCommandsCallback(w, start),
	)
	if err != nil {
		utils.Error.Printf("Error getting commands: %v", err)
		api.GenericResponse(w, http.StatusInternalServerError, HelpResponse{
			Data: &[]common.Command{},
			Errors: &[]common.ErrorMessage{{Message: "Error getting commands"}},
		}, start)
		return
	}
}
