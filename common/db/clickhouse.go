// Package db provides database clients and functions to retrieve or update data.
package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/logger"
)

// ClickhouseClient is a wrapper around the ClickHouse driver.Conn to provide a custom client.
type ClickhouseClient struct {
	driver.Conn
}

// InitClickhouse initializes a ClickHouse connection using the provided configuration.
func InitClickhouse(config common.Config) (*ClickhouseClient, error) {
	host := config.Clickhouse.Host
	if host == "" {
		host = "localhost" //nolint:goconst
	}

	port := config.Clickhouse.Port
	if port == "" {
		port = "9000"
	}

	user := config.Clickhouse.User
	if user == "" {
		user = "default"
	}

	options := &clickhouse.Options{
		Addr:   []string{fmt.Sprintf("%s:%s", host, port)},
		Auth:   clickhouse.Auth{Username: user, Password: config.Clickhouse.Password},
		Debugf: logger.Debug.Printf,
	}

	conn, err := clickhouse.Open(options)
	if err != nil {
		return nil, err
	}

	return &ClickhouseClient{conn}, nil
}

// EmoteStatsOptions holds the query filters for GetEmoteStats.
type EmoteStatsOptions struct {
	// ChannelID filters results to a specific channel (uses emote_usage table when UserID is empty).
	ChannelID string
	// UserID filters results to a specific user (uses user_emote_usage table).
	UserID string
	// Order is "ASC" or "DESC".
	Order string
	// Providers is the normalised list of provider enum values (e.g. "STV", "FFZ"). Empty = all.
	Providers []string
	// PeriodHours is the time window in hours. 0 means no time filter ("all").
	PeriodHours int
	// Limit is the maximum number of rows to return.
	Limit int
	// Offset is the number of rows to skip (cursor-based pagination).
	Offset int
}

// GetEmoteStats queries aggregated emote usage from Clickhouse with cursor-based pagination.
func (db *ClickhouseClient) GetEmoteStats( //nolint:cyclop
	ctx context.Context,
	opts EmoteStatsOptions,
) ([]common.EmoteStat, error) {
	table := "potatbotat.emote_usage"
	if opts.UserID != "" {
		table = "potatbotat.user_emote_usage"
	}

	var sb strings.Builder
	args := make([]any, 0, 8)

	fmt.Fprintf(&sb,
		"SELECT emote_id, emote_name, emote_alias, provider, sum(count) AS count FROM %s FINAL WHERE 1=1",
		table,
	)

	if opts.ChannelID != "" {
		args = append(args, opts.ChannelID)
		fmt.Fprintf(&sb, " AND channel_id = $%d", len(args))
	}

	if opts.UserID != "" {
		args = append(args, opts.UserID)
		fmt.Fprintf(&sb, " AND user_id = $%d", len(args))
	}

	if opts.PeriodHours > 0 {
		cutoff := time.Now().Add(-time.Duration(opts.PeriodHours) * time.Hour)
		args = append(args, cutoff)
		fmt.Fprintf(&sb, " AND used_at >= $%d", len(args))
	}

	if len(opts.Providers) > 0 {
		placeholders := make([]string, len(opts.Providers))
		for i, p := range opts.Providers {
			args = append(args, p)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		fmt.Fprintf(&sb, " AND provider IN (%s)", strings.Join(placeholders, ","))
	}

	sb.WriteString(" GROUP BY emote_id, emote_name, emote_alias, provider")

	order := "DESC"
	if strings.EqualFold(opts.Order, "asc") {
		order = "ASC"
	}
	fmt.Fprintf(&sb, " ORDER BY count %s", order)

	limit := opts.Limit
	if limit <= 0 || limit > 300 {
		limit = 100
	}
	fmt.Fprintf(&sb, " LIMIT %d", limit)

	if opts.Offset > 0 {
		fmt.Fprintf(&sb, " OFFSET %d", opts.Offset)
	}

	rows, err := db.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var stats []common.EmoteStat
	for rows.Next() {
		var s common.EmoteStat
		if err := rows.Scan(&s.EmoteID, &s.EmoteName, &s.EmoteAlias, &s.Provider, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// GetEmoteHistory queries the most recent per-user emote usage records from Clickhouse.
func (db *ClickhouseClient) GetEmoteHistory(
	ctx context.Context,
	userID string,
	channelID string,
	limit int,
) ([]common.EmoteHistoryEntry, error) {
	if limit <= 0 || limit > 300 {
		limit = 100
	}

	var sb strings.Builder
	args := make([]any, 0, 3)

	sb.WriteString(`
		SELECT emote_id, emote_name, emote_alias, provider, channel_id, user_id, sum(count) AS count, max(used_at) AS used_at
		FROM potatbotat.user_emote_usage FINAL
		WHERE 1=1
	`)

	if userID != "" {
		args = append(args, userID)
		fmt.Fprintf(&sb, " AND user_id = $%d", len(args))
	}

	if channelID != "" {
		args = append(args, channelID)
		fmt.Fprintf(&sb, " AND channel_id = $%d", len(args))
	}

	sb.WriteString(" GROUP BY emote_id, emote_name, emote_alias, provider, channel_id, user_id")
	sb.WriteString(" ORDER BY used_at DESC")
	fmt.Fprintf(&sb, " LIMIT %d", limit)

	rows, err := db.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var entries []common.EmoteHistoryEntry
	for rows.Next() {
		var e common.EmoteHistoryEntry
		if err := rows.Scan(
			&e.EmoteID, &e.EmoteName, &e.EmoteAlias, &e.Provider,
			&e.ChannelID, &e.UserID, &e.Count, &e.UsedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	return entries, rows.Err()
}
