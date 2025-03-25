package get

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"potat-api/api"
	"potat-api/api/middleware"
	"potat-api/common"
	"potat-api/common/db"
	"potat-api/common/logger"
	"potat-api/common/utils"
)

type HelpResponse = common.GenericResponse[common.Command]

func init() {
	api.SetRoute(api.Route{
		Path:    "/help",
		Method:  http.MethodGet,
		Handler: getCommandsHandler,
		UseAuth: false,
	})
}

func setCache(ctx context.Context, key string, data interface{}) {
	redis, ok := ctx.Value(middleware.RedisKey).(*db.RedisClient)
	if !ok {
		logger.Error.Println("Redis client not found in context")

		return
	}

	err := redis.SetEx(ctx, key, data, time.Hour).Err()
	if err != nil {
		logger.Error.Printf("Error caching commands: %v", err)
	}
}

func getCache(ctx context.Context, key string) (*[]common.Command, error) {
	redis, ok := ctx.Value(middleware.RedisKey).(*db.RedisClient)
	if !ok {
		logger.Error.Println("Redis client not found in context")

		return nil, middleware.ErrMissingContext
	}

	data, err := redis.Get(ctx, key).Bytes()
	if (err != nil && !errors.Is(err, db.RedisErrNil)) || data == nil {
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

func getCommands(ctx context.Context, writer http.ResponseWriter, start time.Time, data []byte) {
	var commandsJson []common.Command
	err := json.Unmarshal(data, &commandsJson)
	if err != nil {
		logger.Error.Printf("Error unmarshalling commands: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, HelpResponse{
			Data:   &[]common.Command{},
			Errors: &[]common.ErrorMessage{{Message: "Error unmarshalling commands"}},
		}, start)

		return
	}
	filteredCommands := filterCommands(commandsJson)

	if len(filteredCommands) > 0 {
		go setCache(ctx, "website:commands", data)
	}

	api.GenericResponse(writer, http.StatusOK, HelpResponse{
		Data: &filteredCommands,
	}, start)
}

func getCommandsHandler(writer http.ResponseWriter, request *http.Request) {
	start := time.Now()

	cache, err := getCache(request.Context(), "website:commands")
	if err == nil && cache != nil {
		writer.Header().Set("X-Cache-Hit", "HIT")
		api.GenericResponse(writer, http.StatusOK, HelpResponse{
			Data: cache,
		}, start)

		return
	} else {
		writer.Header().Set("X-Cache-Hit", "MISS")
	}

	response, err := utils.BridgeRequest(
		request.Context(),
		5*time.Second,
		"get-commands",
	)
	if err != nil {
		logger.Error.Printf("Error getting commands: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, HelpResponse{
			Data:   &[]common.Command{},
			Errors: &[]common.ErrorMessage{{Message: "Error getting commands"}},
		}, start)

		return
	}

	getCommands(request.Context(), writer, start, response)
}
