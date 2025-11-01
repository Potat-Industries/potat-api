// Package db provides database clients and functions to retrieve or update data.
package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/Potat-Industries/potat-api/common/utils"
	"github.com/robfig/cron/v3"
)

var (
	errNoRows              = fmt.Errorf("no rows returned for database size query")
	errMissingRefreshToken = errors.New("missing refresh token")
)

const dumpPath = "./dump"

// StartLoops initializes schedules and loops for various tasks.
func StartLoops(
	ctx context.Context,
	config common.Config,
	natsClient *utils.NatsClient,
	postgres *PostgresClient,
	clickhouse *ClickhouseClient,
	redis *RedisClient,
) {
	if !config.Loops.Enabled {
		return
	}
	cronManager := cron.New()

	var err error
	_, err = cronManager.AddFunc("@hourly", func() {
		go updateHourlyUsage(ctx, postgres)
		go validateTokens(ctx, config, postgres)
	})
	if err != nil {
		logger.Error.Println("Failed initializing cron updateHourlyUsage", err)

		return
	}
	_, err = cronManager.AddFunc("@daily", func() {
		updateDailyUsage(ctx, postgres)
	})
	if err != nil {
		logger.Error.Println("Failed initializing cron updateDailyUsage", err)

		return
	}
	_, err = cronManager.AddFunc("@weekly", func() {
		updateWeeklyUsage(ctx, postgres)
	})
	if err != nil {
		logger.Error.Println("Failed initializing cron updateWeeklyUsage", err)

		return
	}
	_, err = cronManager.AddFunc("0 */2 * * *", func() {
		refreshAllHelixTokens(ctx, config, postgres)
	})
	if err != nil {
		logger.Error.Println("Failed initializing cron refreshAllHelixTokens", err)

		return
	}
	_, err = cronManager.AddFunc("*/15 * * * *", func() {
		updateColorView(ctx, clickhouse)
		updateActiveBadgeView(ctx, clickhouse)
		updateOwnedBadgeView(ctx, clickhouse)
		updateUserOwnedBadgeView(ctx, clickhouse)
	})
	if err != nil {
		logger.Error.Println("Failed initializing cron clickhouse views", err)

		return
	}
	_, err = cronManager.AddFunc("0 */12 * * *", func() {
		go backupPostgres(ctx, postgres, natsClient, config)
		// go optimizeClickhouse(ctx, config, clickhouse)
	})
	if err != nil {
		logger.Error.Println("Failed initializing cron backupPostgres", err)

		return
	}

	cronManager.Start()

	go decrementDuels(ctx, redis)
	go deleteOldUploads(ctx, postgres)
	go updateAggregateTable(ctx, postgres)
}

func decrementDuels(ctx context.Context, redis *RedisClient) {
	for {
		time.Sleep(30 * time.Minute)
		logger.Info.Println("Decrementing duels")

		keys, err := redis.Scan(ctx, "duelUse:*", 100, 0)
		if err != nil {
			logger.Error.Println("Failed scanning keys for duels", err)

			return
		}

		if len(keys) == 0 {
			return
		}

		luaScript := `
		  local decrementedKeys = 0
			for _, key in ipairs(KEYS) do
				local num = redis.call("DECR", key)
				decrementedKeys = decrementedKeys + 1
				if num <= 0 then
					redis.call("DEL", key)
				end
			end

			return decrementedKeys
		`

		value, err := redis.Eval(ctx, luaScript, keys).Result()
		if err != nil {
			logger.Error.Println("Failed decrementing duels", err)
		}

		logger.Info.Printf("Decremented %d duel keys", value)
	}
}

func deleteOldUploads(ctx context.Context, postgres *PostgresClient) {
	for {
		time.Sleep(24 * time.Hour)
		logger.Info.Println("Deleting old uploads")

		query := `
			DELETE FROM file_store
			WHERE (created_at < NOW() - INTERVAL '30 days' AND expires_at IS NULL)
			OR (expires_at IS NOT NULL AND expires_at < NOW());
		`

		_, err := postgres.Exec(ctx, query)
		if err != nil {
			logger.Error.Println("Error deleting old uploads ", err)
		}

		logger.Debug.Println("Deleted old uploads")
	}
}

func updateAggregateTable(ctx context.Context, postgres *PostgresClient) {
	for {
		time.Sleep(5 * time.Minute)

		query := `
			INSERT INTO channel_command_usage (channel_id, channel_usage)
			SELECT channel_id, SUM(channel_usage) AS channel_usage
			FROM command_settings
			GROUP BY channel_id
			ON CONFLICT (channel_id) DO UPDATE
			SET channel_usage = EXCLUDED.channel_usage;
		`

		_, err := postgres.Exec(ctx, query)
		if err != nil {
			logger.Error.Println("Error updating aggregate table", err)
		}

		logger.Info.Println("Updated aggregate table")
	}
}

func updateHourlyUsage(ctx context.Context, postgres *PostgresClient) {
	logger.Info.Println("Updating hourly usage")
	query := `UPDATE gpt_usage SET hourly_usage = 0;`

	_, err := postgres.Exec(ctx, query)
	if err != nil {
		logger.Error.Println("Error updating hourly usage", err)
	}

	logger.Info.Println("Updated hourly usage")
}

func updateDailyUsage(ctx context.Context, postgres *PostgresClient) {
	logger.Info.Println("Updating daily usage")
	query := `UPDATE gpt_usage SET daily_usage = 0`

	_, err := postgres.Exec(ctx, query)
	if err != nil {
		logger.Error.Println("Error updating daily usage", err)
	}

	logger.Info.Println("Updated daily usage")
}

func updateWeeklyUsage(ctx context.Context, postgres *PostgresClient) {
	logger.Info.Println("Updating weekly usage")
	query := `UPDATE gpt_usage SET weekly_usage = 0`

	_, err := postgres.Exec(ctx, query)
	if err != nil {
		logger.Error.Println("Error updating weekly usage", err)
	}

	logger.Info.Println("Updated weekly usage")
}

func updateColorView(ctx context.Context, clickhouse *ClickhouseClient) {
	logger.Info.Println("Updating color view")

	query := `
		INSERT INTO potatbotat.twitch_color_stats
		SELECT
			color,
			COUNT(DISTINCT user_id) AS user_count,
			(COUNT(DISTINCT user_id) * 100.0) / (
				SELECT COUNT(user_id)
				FROM potatbotat.twitch_colors
			) AS percentage,
			ROW_NUMBER() OVER (ORDER BY COUNT(DISTINCT user_id) DESC) AS rank
		FROM potatbotat.twitch_colors FINAL
		GROUP BY color;
	`

	err := clickhouse.Exec(ctx, query)
	if err != nil {
		logger.Error.Println("Error updating color view ", err)
	}
}

func updateActiveBadgeView(ctx context.Context, clickhouse *ClickhouseClient) {
	logger.Info.Println("Updating active badge view")

	err := clickhouse.Exec(ctx, `TRUNCATE TABLE potatbotat.twitch_active_badge_stats;`)
	if err != nil {
		logger.Error.Println("Error truncating badge stats table ", err)

		return
	}

	query := `
	  INSERT INTO potatbotat.twitch_active_badge_stats
		SELECT
			badge,
			count(user_id) AS user_count,
			version
		FROM potatbotat.twitch_badges
		WHERE badge NOT IN ('', 'NOBADGE')
		GROUP BY (badge, version);
	`

	err = clickhouse.Exec(ctx, query)
	if err != nil {
		logger.Error.Println("Error updating badge view ", err)
	}
}

func updateOwnedBadgeView(ctx context.Context, clickhouse *ClickhouseClient) {
	logger.Info.Println("Updating owned badge view")

	// Insert owned badges from active table first
	prepare := `
	  INSERT INTO potatbotat.twitch_owned_badges
		SELECT
			badge,
			user_id,
			version
		FROM potatbotat.twitch_badges
		WHERE badge NOT IN ('', 'NOBADGE')
	`

	err := clickhouse.Exec(ctx, prepare)
	if err != nil {
		logger.Error.Println("Error preparing badge view ", err)
	}

	err = clickhouse.Exec(ctx, `TRUNCATE TABLE potatbotat.twitch_owned_badge_stats;`)
	if err != nil {
		logger.Error.Println("Error truncating badge stats table ", err)

		return
	}

	query := `
	  INSERT INTO potatbotat.twitch_owned_badge_stats
		SELECT
			badge,
			count(user_id) AS user_count,
			version
		FROM potatbotat.twitch_owned_badges
		WHERE badge NOT IN ('', 'NOBADGE')
		GROUP BY (badge, version);
	`

	err = clickhouse.Exec(ctx, query)
	if err != nil {
		logger.Error.Println("Error updating badge view ", err)
	}
}

func updateUserOwnedBadgeView(ctx context.Context, clickhouse *ClickhouseClient) {
	logger.Info.Println("Updating user owned badge view")

	err := clickhouse.Exec(ctx, `TRUNCATE TABLE potatbotat.twitch_owned_badge_user_stats;`)
	if err != nil {
		logger.Error.Println("Error truncating badge stats table ", err)

		return
	}

	query := `
	  INSERT INTO potatbotat.twitch_owned_badge_user_stats
		SELECT
			user_id,
			count(badge) AS badge_count,
  	  groupArrayDistinct(badge) AS badges
		FROM potatbotat.twitch_owned_badges FINAL
		WHERE badge NOT IN ('', 'NOBADGE')
		GROUP BY user_id
		HAVING uniqExact(badge) >= 5
		ORDER BY badge_count DESC;
	`

	err = clickhouse.Exec(ctx, query)
	if err != nil {
		logger.Error.Println("Error updating user owned badge view ", err)
	}
}

func upsertOAuthToken(
	ctx context.Context,
	postgres *PostgresClient,
	oauth *common.GenericOAUTHResponse,
	con common.PlatformOauth,
) error {
	query := `
		INSERT INTO connection_oauth (
			platform_id,
			access_token,
			refresh_token,
			scope,
			expires_in,
			added_at,
			platform
      )
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (platform_id, platform)
		DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			scope = EXCLUDED.scope,
			expires_in = EXCLUDED.expires_in,
			added_at = EXCLUDED.added_at;
	`

	_, err := postgres.Exec(
		ctx,
		query,
		con.PlatformID,
		oauth.AccessToken,
		oauth.RefreshToken,
		oauth.Scope,
		oauth.ExpiresIn,
		time.Now(),
		common.TWITCH,
	)

	return err
}

func refreshOrDelete(
	ctx context.Context,
	config common.Config,
	postgres *PostgresClient,
	con common.PlatformOauth,
) (bool, error) {
	var err error
	if con.RefreshToken == "" {
		return false, errMissingRefreshToken
	}

	refreshResult, err := utils.RefreshHelixToken(ctx, config, con.RefreshToken)
	if err != nil || refreshResult == nil {
		return false, err
	}

	err = upsertOAuthToken(ctx, postgres, refreshResult, con)
	if err != nil {
		logger.Error.Println(
			"Error updating token for user_id", con.PlatformID, ":", err,
		)

		return false, err
	}

	return true, nil
}

func validateTokens(ctx context.Context, config common.Config, postgres *PostgresClient) {
	logger.Info.Println("Validating Twitch tokens ")

	query := `
		SELECT
			access_token,
			platform_id,
			refresh_token
		FROM connection_oauth
		WHERE platform = 'TWITCH';
	`

	rows, err := postgres.Query(ctx, query)
	if err != nil {
		logger.Error.Println("Error getting tokens ", err)

		return
	}
	defer rows.Close()

	validated, deleted := 0, 0
	for rows.Next() {
		var con common.PlatformOauth

		err := rows.Scan(&con.AccessToken, &con.PlatformID, &con.RefreshToken)
		if err != nil {
			logger.Error.Println("Error scanning token: ", err)

			continue
		}

		valid, _, err := utils.ValidateHelixToken(ctx, con.AccessToken, false)
		if err != nil {
			logger.Error.Println("Error validating token ", err)

			continue
		}

		if !valid {
			ok, err := refreshOrDelete(ctx, config, postgres, con)
			if err != nil {
				logger.Error.Println("Error refreshing token ", err)
				deleted++

				continue
			}

			if ok {
				validated++
			} else {
				deleted++
			}

			continue
		}
		validated++

		time.Sleep(200 * time.Millisecond)
	}

	logger.Info.Printf(
		"Validated %d helix tokens, and deleted %d expired tokens",
		validated,
		deleted,
	)
}

func refreshAllHelixTokens(ctx context.Context, config common.Config, postgres *PostgresClient) {
	logger.Info.Println("Refreshing all Twitch tokens")

	query := `
		SELECT
	  	platform,
			platform_id,
			access_token,
			refresh_token,
			expires_in,
			added_at,
			scope
		FROM connection_oauth
		WHERE platform = 'TWITCH';
	`

	rows, err := postgres.Query(ctx, query)
	if err != nil {
		logger.Error.Println("Error getting tokens ", err)

		return
	}
	defer rows.Close()

	refreshed, failed := 0, 0
	for rows.Next() {
		var con common.PlatformOauth

		err := rows.Scan(
			&con.Platform,
			&con.PlatformID,
			&con.AccessToken,
			&con.RefreshToken,
			&con.ExpiresIn,
			&con.AddedAt,
			&con.Scope,
		)
		if err != nil {
			logger.Error.Println("Error scanning token: ", err)

			continue
		}

		ok, err := refreshOrDelete(ctx, config, postgres, con)
		if err != nil {
			logger.Error.Println("Error refreshing token ", err)
			failed++

			continue
		}

		if ok {
			refreshed++
		} else {
			failed++
		}

		time.Sleep(200 * time.Millisecond)

		continue
	}

	logger.Info.Printf(
		"Refreshed %d helix tokens, %d failed and were expunged",
		refreshed,
		failed,
	)
}

func sortFiles(files []string) func(i, j int) bool {
	return func(i, j int) bool {
		fileI, errI := os.Stat(filepath.Join(dumpPath, files[i]))
		if errI != nil {
			return false
		}

		fileJ, errJ := os.Stat(filepath.Join(dumpPath, files[j]))
		if errJ != nil {
			return true
		}

		return fileI.ModTime().Before(fileJ.ModTime())
	}
}

func deleteOldDumps(files []string, maxSize int) {
	logger.Debug.Printf("Checking for old dump files, current count: %d, max: %d", len(files), maxSize)
	if len(files) <= maxSize {
		return
	}

	sort.Slice(files, sortFiles(files))

	filesToDelete := files[:len(files)-maxSize+1]
	for _, file := range filesToDelete {
		err := os.Remove(file)
		if err != nil {
			logger.Error.Println("Failed deleting dump file ", err)
		}
	}

	logger.Info.Printf("Deleted %d old dump files", len(filesToDelete))
}

func backupPostgres(
	ctx context.Context,
	postgres *PostgresClient,
	natsClient *utils.NatsClient,
	config common.Config,
) {
	logger.Debug.Println("Backing up Postgres")

	if err := os.MkdirAll(dumpPath, 0o750); err != nil {
		logger.Error.Println("Failed to create backup folder:", err)

		return
	}

	files, err := filepath.Glob(filepath.Join(dumpPath, "*.sql.zst"))
	if err != nil {
		logger.Error.Println("Failed to list dump files:", err)

		return
	}

	deleteOldDumps(files, 10)

	filePath := filepath.Join(
		dumpPath,
		fmt.Sprintf("data_%d.sql.zst", time.Now().Unix()),
	)

	//nolint:gosec
	cmd := exec.Command("sh", "-c", fmt.Sprintf(
		"PGPASSWORD=%s pg_dump -d %s -U %s -h %s | zstd -3 --threads=%d > %s",
		config.Postgres.Password,
		config.Postgres.Database,
		config.Postgres.User,
		config.Postgres.Host,
		runtime.NumCPU(),
		filePath,
	))

	defer func() {
		if err = cmd.Process.Release(); err != nil {
			logger.Error.Println("Failed to release pg_dump process:", err)

			if err = cmd.Process.Kill(); err != nil {
				logger.Error.Fatalln("Failed to kill pg_dump process:", err)
			}
		}
	}()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	if err = cmd.Run(); err != nil {
		logger.Error.Println("Failed to execute pg_dump:", err, stderr.String())

		return
	}
	duration := time.Since(start)

	stat, err := os.Stat(filePath)
	if err != nil {
		logger.Error.Println("Failed to get backup file size:", err)

		return
	}

	backupSize := float64(stat.Size()) / (1024 * 1024 * 1024)

	dbSize, err := getDatabaseSize(ctx, postgres, config.Postgres.Database)
	if err != nil {
		logger.Error.Println("Failed to get database size:", err)

		return
	}

	message := fmt.Sprintf(
		"Database back-up successful in %s - DB size: %s - Backup size: %.2f GB",
		utils.Humanize(duration, 2),
		dbSize,
		backupSize,
	)

	jsonMessage, err := json.Marshal(message)
	if err != nil {
		logger.Error.Println("Failed to JSON stringify message:", err)
		logger.Info.Println(message)

		return
	}

	err = natsClient.Publish("github.com/Potat-Industries/potat-api.postgres-backup", jsonMessage)
	if err != nil {
		logger.Error.Println("Failed to publish to queue:", err)

		return
	}

	logger.Info.Println(message)
}

func getDatabaseSize(ctx context.Context, postgres *PostgresClient, dbName string) (string, error) {
	query := `SELECT pg_size_pretty(pg_database_size($1)) AS size`
	rows, err := postgres.Query(ctx, query, dbName)
	if err != nil {
		return "", err
	}

	defer rows.Close()

	if rows.Next() {
		var size string
		if err := rows.Scan(&size); err != nil {
			return "", err
		}

		return size, nil
	}

	return "", errNoRows
}

// func optimizeClickhouse(ctx context.Context, config common.Config, clickhouse *ClickhouseClient) {
// 	// offset any concurrent crons
// 	time.Sleep(5 * time.Minute)

// 	logger.Info.Println("Optimizing Clickhouse tables")

// 	if config.Clickhouse.Database == "" {
// 		logger.Error.Println("Clickhouse database is not configured")

// 		return
// 	}

// 	query := `SELECT table FROM system.tables WHERE database = ?`

// 	rows, err := clickhouse.Query(ctx, query, config.Clickhouse.Database)
// 	if err != nil {
// 		logger.Error.Println("Failed to query Clickhouse tables:", err)

// 		return
// 	}

// 	for rows.Next() {
// 		var table string
// 		if err := rows.Scan(&table); err != nil {
// 			logger.Error.Println("Failed to scan Clickhouse table:", err)

// 			continue
// 		}

// 		query := fmt.Sprintf("OPTIMIZE TABLE %s.%s FINAL", config.Clickhouse.Database, table)
// 		if err := clickhouse.Exec(ctx, query); err != nil {
// 			logger.Error.Println("Failed to optimize Clickhouse table:", err)
// 		}

// 		logger.Info.Printf(
// 			"Optimized Clickhouse table %s.%s",
// 			config.Clickhouse.Database,
// 			table,
// 		)

// 		time.Sleep(5 * time.Second)
// 	}
// }
