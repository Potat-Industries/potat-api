// Package get contains routes for http.MethodGet requests.
package get

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Potat-Industries/potat-api/api"
	"github.com/Potat-Industries/potat-api/api/middleware"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/db"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/gorilla/mux"
)

const (
	defaultEmoteLimit = 100
	maxEmoteLimit     = 300
)

func init() {
	api.SetRoute(api.Route{
		Path:    "/emotes/stats",
		Method:  http.MethodGet,
		Handler: getEmoteStats,
		UseAuth: false,
	})
	api.SetRoute(api.Route{
		Path:    "/emotes/history/{login}",
		Method:  http.MethodGet,
		Handler: getEmoteHistory,
		UseAuth: false,
	})
}

// periodToHours converts a period string to a number of hours. 0 means no time filter.
func periodToHours(period string) int {
	switch strings.ToLower(period) {
	case "hour":
		return 1
	case "day":
		return 24
	case "week":
		return 168
	case "month":
		return 720
	default:
		return 0
	}
}

// normaliseProvider maps user-facing provider names to Clickhouse enum values.
func normaliseProvider(provider string) []string {
	switch strings.ToUpper(provider) {
	case "7TV", "STV":
		return []string{"STV"}
	case "FFZ":
		return []string{"FFZ"}
	case "BTTV":
		return []string{"BTTV"}
	case "TWITCH":
		return []string{"TWITCH"}
	case "EMOJI":
		return []string{"EMOJI"}
	case "ALL", "":
		return []string{}
	default:
		return []string{"STV"}
	}
}

func encodeCursor(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodeCursor(cursor string) (int, error) {
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(b))
}

func getEmoteStats(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop
	start := time.Now()

	clickhouse, ok := request.Context().Value(middleware.ClickhouseKey).(*db.ClickhouseClient)
	if !ok {
		logger.Error.Println("Clickhouse client not found in context")
		writeEmoteError(writer, http.StatusInternalServerError, start)

		return
	}

	query := request.URL.Query()

	channelID := query.Get("id")
	login := query.Get("login")

	// If login was provided, resolve to a channel ID via Postgres
	if channelID == "" && login != "" {
		postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
		if !pgOK {
			writeEmoteError(writer, http.StatusInternalServerError, start)

			return
		}

		ch, err := postgres.GetChannelByName(request.Context(), login, common.TWITCH)
		if err != nil {
			writeEmoteError(writer, http.StatusNotFound, start)

			return
		}

		channelID = ch.ChannelID
	}

	if channelID == "" {
		writeEmoteError(writer, http.StatusBadRequest, start)

		return
	}

	limit := defaultEmoteLimit
	if v := query.Get("limit"); v == "" {
		if v = query.Get("first"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxEmoteLimit {
				limit = n
			}
		}
	} else if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxEmoteLimit {
		limit = n
	}

	offset := 0
	if cursor := query.Get("after"); cursor != "" {
		if o, err := decodeCursor(cursor); err == nil {
			offset = o
		}
	}

	opts := db.EmoteStatsOptions{
		ChannelID:   channelID,
		PeriodHours: periodToHours(query.Get("period")),
		Providers:   normaliseProvider(query.Get("provider")),
		Order:       query.Get("order"),
		Limit:       limit + 1, // fetch one extra to determine hasNextPage
		Offset:      offset,
	}

	stats, err := clickhouse.GetEmoteStats(request.Context(), opts)
	if err != nil {
		logger.Error.Printf("Error fetching emote stats: %v", err)
		writeEmoteError(writer, http.StatusInternalServerError, start)

		return
	}

	if stats == nil {
		stats = []common.EmoteStat{}
	}

	hasNextPage := len(stats) > limit
	if hasNextPage {
		stats = stats[:limit]
	}

	var nextCursor string
	if hasNextPage {
		nextCursor = encodeCursor(offset + limit)
	}

	elapsed := time.Since(start).Seconds()
	api.GenericResponse(writer, http.StatusOK, common.EmoteStatsResponse{
		Data: &stats,
		Pagination: common.PageInfo{
			HasNextPage: hasNextPage,
			Cursor:      nextCursor,
		},
		StatusCode: http.StatusOK,
		Duration:   elapsed,
	}, start)
}

func getEmoteHistory(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop
	start := time.Now()

	login := mux.Vars(request)["login"]
	if login == "" {
		api.GenericResponse(writer, http.StatusBadRequest, common.GenericResponse[common.EmoteHistoryEntry]{
			Errors: &[]common.ErrorMessage{{Message: "login is required"}},
		}, start)

		return
	}

	clickhouse, ok := request.Context().Value(middleware.ClickhouseKey).(*db.ClickhouseClient)
	if !ok {
		logger.Error.Println("Clickhouse client not found in context")
		api.GenericResponse(writer, http.StatusInternalServerError, common.GenericResponse[common.EmoteHistoryEntry]{
			Errors: &[]common.ErrorMessage{{Message: "Internal Server Error"}},
		}, start)

		return
	}

	// Resolve login → channel ID
	postgres, pgOK := request.Context().Value(middleware.PostgresKey).(*db.PostgresClient)
	if !pgOK {
		api.GenericResponse(writer, http.StatusInternalServerError, common.GenericResponse[common.EmoteHistoryEntry]{
			Errors: &[]common.ErrorMessage{{Message: "Internal Server Error"}},
		}, start)

		return
	}

	ch, err := postgres.GetChannelByName(request.Context(), login, common.TWITCH)
	if err != nil {
		api.GenericResponse(writer, http.StatusNotFound, common.GenericResponse[common.EmoteHistoryEntry]{
			Errors: &[]common.ErrorMessage{{Message: "Channel not found"}},
		}, start)

		return
	}

	limit := defaultEmoteLimit
	if v := request.URL.Query().Get("limit"); v != "" {
		if n, parseErr := strconv.Atoi(v); parseErr == nil && n > 0 && n <= maxEmoteLimit {
			limit = n
		}
	}

	entries, err := clickhouse.GetEmoteHistory(request.Context(), "", ch.ChannelID, limit)
	if err != nil {
		logger.Error.Printf("Error fetching emote history: %v", err)
		api.GenericResponse(writer, http.StatusInternalServerError, common.GenericResponse[common.EmoteHistoryEntry]{
			Errors: &[]common.ErrorMessage{{Message: "Failed to fetch emote history"}},
		}, start)

		return
	}

	if entries == nil {
		entries = []common.EmoteHistoryEntry{}
	}

	api.GenericResponse(writer, http.StatusOK, common.GenericResponse[common.EmoteHistoryEntry]{
		Data: &entries,
	}, start)
}

func writeEmoteError(writer http.ResponseWriter, code int, start time.Time) {
	api.GenericResponse(writer, code, common.EmoteStatsResponse{
		Data:       &[]common.EmoteStat{},
		StatusCode: code,
		Duration:   time.Since(start).Seconds(),
	}, start)
}
