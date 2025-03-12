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

	"potat-api/common"
	"potat-api/common/utils"

	"github.com/robfig/cron/v3"
)

var (
	maxFiles   = 10
	dumpPath   = "./dump"
	dbName     = ""
	dbUser     = ""
	dbHost     = ""
	pgPassword = ""
)

func StartLoops(config common.Config, natsClient *utils.NatsClient) {
	dbName = config.Postgres.Database
	dbUser = config.Postgres.User
	dbHost = config.Postgres.Host
	pgPassword = config.Postgres.Password

	c := cron.New()

	var err error
	_, err = c.AddFunc("@hourly", updateHourlyUsage)
	if err != nil {
		utils.Error.Println("Failed initializing cron updateHourlyUsage", err)
		return
	}
	_, err = c.AddFunc("@daily", updateDailyUsage)
	if err != nil {
		utils.Error.Println("Failed initializing cron updateDailyUsage", err)
		return
	}
	_, err = c.AddFunc("@weekly", updateWeeklyUsage)
	if err != nil {
		utils.Error.Println("Failed initializing cron updateWeeklyUsage", err)
		return
	}
	_, err = c.AddFunc("@hourly", validateTokens)
	if err != nil {
		utils.Error.Println("Failed initializing cron validateTokens", err)
		return
	}
	_, err = c.AddFunc("0 */2 * * *", refreshAllHelixTokens)
	if err != nil {
		utils.Error.Println("Failed initializing cron refreshAllHelixTokens", err)
		return
	}
	_, err = c.AddFunc("*/5 * * * *", updateColorView)
	if err != nil {
		utils.Error.Println("Failed initializing cron updateColorView", err)
		return
	}
	_, err = c.AddFunc("*/5 * * * *", updateBadgeView)
	if err != nil {
		utils.Error.Println("Failed initializing cron updateBadgeView", err)
		return
	}
	_, err = c.AddFunc("0 */12 * * *", backupPostgres(natsClient))
	if err != nil {
		utils.Error.Println("Failed initializing cron backupPostgres", err)
		return
	}
	_, err = c.AddFunc("0 */12 * * *", optimizeClickhouse)
	if err != nil {
		utils.Error.Println("Failed initializing cron optimizeClickhouse", err)
		return
	}

	c.Start()

	go decrementDuels()
	go deleteOldUploads()
	go updateAggregateTable()
}

func decrementDuels() {
	for {
		time.Sleep(30 * time.Minute)
		utils.Info.Println("Decrementing duels")

		keys, err := Scan(context.Background(), "duelUse:*", 100, 0)
		if err != nil {
			utils.Error.Println("Failed scanning keys for duels", err)
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

		value, err := Redis.Eval(context.Background(), luaScript, keys).Result()
		if err != nil {
			utils.Error.Println("Failed decrementing duels", err)
		}

		utils.Info.Printf("Decremented %d duel keys", value)
	}
}

func deleteOldUploads() {
	for {
		time.Sleep(24 * time.Hour)
		utils.Info.Println("Deleting old uploads")

		query := `
			DELETE FROM file_store
			WHERE (created_at < NOW() - INTERVAL '30 days' AND expires_at IS NULL)
			OR (expires_at IS NOT NULL AND expires_at < NOW());
		`

		_, err := Postgres.Pool.Exec(context.Background(), query)
		if err != nil {
			utils.Error.Println("Error deleting old uploads ", err)
		}

		utils.Debug.Println("Deleted old uploads")
	}
}

func updateAggregateTable() {
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

		_, err := Postgres.Pool.Exec(context.Background(), query)
		if err != nil {
			utils.Error.Println("Error updating aggregate table", err)
		}

		utils.Info.Println("Updated aggregate table")
	}
}

func updateHourlyUsage() {
	utils.Info.Println("Updating hourly usage")
	query := `UPDATE gpt_usage SET hourly_usage = 0;`

	_, err := Postgres.Pool.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating hourly usage", err)
	}

	utils.Info.Println("Updated hourly usage")
}

func updateDailyUsage() {
	utils.Info.Println("Updating daily usage")
	query := `UPDATE gpt_usage SET daily_usage = 0`

	_, err := Postgres.Pool.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating daily usage", err)
	}

	utils.Info.Println("Updated daily usage")
}

func updateWeeklyUsage() {
	utils.Info.Println("Updating weekly usage")
	query := `UPDATE gpt_usage SET weekly_usage = 0`

	_, err := Postgres.Pool.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating weekly usage", err)
	}

	utils.Info.Println("Updated weekly usage")
}

func updateColorView() {
	utils.Info.Println("Updating color view")

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

	err := Clickhouse.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating color view ", err)
	}
}

func updateBadgeView() {
	utils.Info.Println("Updating badge view")

	query := `
		INSERT INTO potatbotat.twitch_badge_stats
		SELECT
			badge,
			COUNT(DISTINCT user_id) AS user_count,
			(user_count * 100.) / (
				SELECT COUNT(user_id)
				FROM potatbotat.twitch_badges
			) AS percentage,
			ROW_NUMBER() OVER (ORDER BY COUNT(DISTINCT user_id) DESC) AS rank
		FROM potatbotat.twitch_badges FINAL
		GROUP BY badge;
	`

	err := Clickhouse.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating badge view ", err)
	}
}

func upsertOAuthToken(
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

	_, err := Postgres.Pool.Exec(
		context.Background(),
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

func refreshOrDelete(con common.PlatformOauth) (bool, error) {
	var err error
	if con.RefreshToken == "" {
		return false, errors.New("missing refresh token")
	}

	refreshResult, err := utils.RefreshHelixToken(con.RefreshToken)
	if err != nil || refreshResult == nil {
		return false, err
	}

	err = upsertOAuthToken(refreshResult, con)
	if err != nil {
		utils.Error.Println(
			"Error updating token for user_id", con.PlatformID, ":", err,
		)
		return false, err
	}

	return true, nil
}

func validateTokens() {
	utils.Info.Println("Validating Twitch tokens ")

	query := `
		SELECT
			access_token,
			platform_id,
			refresh_token
		FROM connection_oauth
		WHERE platform = 'TWITCH';
	`

	rows, err := Postgres.Pool.Query(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error getting tokens ", err)
		return
	}
	defer rows.Close()

	validated, deleted := 0, 0
	for rows.Next() {
		var con common.PlatformOauth

		err := rows.Scan(&con.AccessToken, &con.PlatformID, &con.RefreshToken)
		if err != nil {
			utils.Error.Println("Error scanning token: ", err)
			continue
		}

		valid, _, err := utils.ValidateHelixToken(con.AccessToken, false)
		if err != nil {
			utils.Error.Println("Error validating token ", err)
			continue
		}

		if !valid {
			ok, err := refreshOrDelete(con)
			if err != nil {
				utils.Error.Println("Error refreshing token ", err)
				deleted++
				continue
			}

			if ok {
				validated++
			} else {
				deleted++
			}

			continue
		} else {
			validated++
		}

		time.Sleep(200 * time.Millisecond)
	}

	utils.Info.Printf(
		"Validated %d helix tokens, and deleted %d expired tokens",
		validated,
		deleted,
	)
}

func refreshAllHelixTokens() {
	utils.Info.Println("Refreshing all Twitch tokens")

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

	rows, err := Postgres.Pool.Query(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error getting tokens ", err)
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
			utils.Error.Println("Error scanning token: ", err)
			continue
		}

		ok, err := refreshOrDelete(con)
		if err != nil {
			utils.Error.Println("Error refreshing token ", err)
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

	utils.Info.Printf(
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

func deleteOldDumps(files []string, max int) {
	utils.Debug.Printf("Checking for old dump files, current count: %d, max: %d", len(files), max)
	if len(files) <= max {
		return
	}

	sort.Slice(files, sortFiles(files))

	filesToDelete := files[:len(files)-max+1]
	for _, file := range filesToDelete {
		err := os.Remove(file)
		if err != nil {
			utils.Error.Println("Failed deleting dump file ", err)
		}
	}

	utils.Info.Printf("Deleted %d old dump files", len(filesToDelete))
}

func backupPostgres(natsClient *utils.NatsClient) func() {
	return func() {
		backupPostgresWithPublisher(natsClient)
	}
}

func backupPostgresWithPublisher(natsClient *utils.NatsClient) {
	utils.Debug.Println("Backing up Postgres")

	if err := os.MkdirAll(dumpPath, os.ModePerm); err != nil {
		utils.Error.Println("Failed to create backup folder:", err)
		return
	}

	files, err := filepath.Glob(filepath.Join(dumpPath, "*.sql.zst"))
	if err != nil {
		utils.Error.Println("Failed to list dump files:", err)
		return
	}

	deleteOldDumps(files, maxFiles)

	numThreads := runtime.NumCPU()

	filePath := filepath.Join(
		dumpPath,
		fmt.Sprintf("data_%d.sql.zst", time.Now().Unix()),
	)

	cmd := exec.Command("sh", "-c", fmt.Sprintf(
		"PGPASSWORD=%s pg_dump -d %s -U %s -h %s | zstd -3 --threads=%d > %s",
		pgPassword, dbName, dbUser, dbHost, numThreads, filePath,
	))

	defer func() {
		if err := cmd.Process.Release(); err != nil {
			utils.Error.Println("Failed to release pg_dump process:", err)

			if err := cmd.Process.Kill(); err != nil {
				utils.Error.Fatalln("Failed to kill pg_dump process:", err)
			}
		}
	}()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	if err := cmd.Run(); err != nil {
		utils.Error.Println("Failed to execute pg_dump:", err, stderr.String())
		return
	}
	duration := time.Since(start)

	stat, err := os.Stat(filePath)
	if err != nil {
		utils.Error.Println("Failed to get backup file size:", err)
		return
	}

	backupSize := float64(stat.Size()) / (1024 * 1024 * 1024)

	dbSize, err := getDatabaseSize(dbName)
	if err != nil {
		utils.Error.Println("Failed to get database size:", err)
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
		utils.Error.Println("Failed to JSON stringify message:", err)
		utils.Info.Println(message)
		return
	}

	err = natsClient.Publish("postgres-backup:%s", jsonMessage)
	if err != nil {
		utils.Error.Println("Failed to publish to queue:", err)
		return
	}

	utils.Info.Println(message)
}

func getDatabaseSize(dbName string) (string, error) {
	query := `SELECT pg_size_pretty(pg_database_size($1)) AS size`
	rows, err := Postgres.Pool.Query(context.Background(), query, dbName)
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

	return "", fmt.Errorf("no rows returned for database size query")
}

func optimizeClickhouse() {
	// offset any concurrent crons
	time.Sleep(5 * time.Minute)

	utils.Info.Println("Optimizing Clickhouse tables")

	config := utils.LoadConfig()
	if config.Clickhouse.Database == "" {
		utils.Error.Println("Clickhouse database is not configured")
		return
	}

	query := `SELECT table FROM system.tables WHERE database = ?`

	rows, err := Clickhouse.Query(context.Background(), query, config.Clickhouse.Database)
	if err != nil {
		utils.Error.Println("Failed to query Clickhouse tables:", err)
		return
	}

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			utils.Error.Println("Failed to scan Clickhouse table:", err)
			continue
		}

		query := fmt.Sprintf("OPTIMIZE TABLE %s.%s FINAL", config.Clickhouse.Database, table)
		if err := Clickhouse.Exec(context.Background(), query); err != nil {
			utils.Error.Println("Failed to optimize Clickhouse table:", err)
		}

		utils.Info.Printf(
			"Optimized Clickhouse table %s.%s",
			config.Clickhouse.Database,
			table,
		)

		time.Sleep(5 * time.Second)
	}
}
