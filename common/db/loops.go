package db

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"potat-api/common"
	"potat-api/common/utils"

	"github.com/robfig/cron/v3"
)

var (
	maxFiles = 10
	dumpPath = "./dump"
	dbName   = ""
	dbUser   = ""
	dbHost   = ""
	pgPassword = ""
)

func StartLoops(config common.Config) {
	dbName = config.Postgres.Database
	dbUser = config.Postgres.User
	dbHost = config.Postgres.Host
	pgPassword = config.Postgres.Password

	c := cron.New()
	c.AddFunc("@hourly", updateHourlyUsage)
	c.AddFunc("@daily", updateDailyUsage)
	c.AddFunc("@weekly", updateWeeklyUsage)
	c.AddFunc("0 * * * *", validateTokens)
	c.AddFunc("*/5 * * * *", updateColorView)
	c.AddFunc("*/5 * * * *", updateBadgeView)
	c.AddFunc("0 */12 * * *", backupPostgres)

	c.Start()

	defer c.Stop()


	// go backupClickhouse()
	go decrementDuels()
	go deleteOldUploads()
	go updateAggregateTable()
}

func decrementDuels() {
	for {
		time.Sleep(30 * time.Minute)

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

		utils.Debug.Println("Updated aggregate table")
	}
}

func updateHourlyUsage() {
	query := `UPDATE gpt_usage SET hourly_usage = 0;`

	_, err := Postgres.Pool.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating hourly usage", err)
	}

	utils.Debug.Println("Updated hourly usage")
}

func updateDailyUsage() {
	query := `UPDATE gpt_usage SET daily_usage = 0`

	_, err := Postgres.Pool.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating daily usage", err)
	}

	utils.Debug.Println("Updated daily usage")
}

func updateWeeklyUsage() {
	query := `UPDATE gpt_usage SET weekly_usage = 0`

	_, err := Postgres.Pool.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating weekly usage", err)
	}

	utils.Debug.Println("Updated weekly usage")
}

func updateColorView() {
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

	_, err := Postgres.Pool.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating color view", err)
	}

	utils.Debug.Println("Updated colors view")
}

func updateBadgeView() {
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

	_, err := Postgres.Pool.Exec(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error updating badge view", err)
	}

	utils.Debug.Println("Updated badges view")
}

func validateTokens() {
	query := `
		SELECT access_token, platform_id FROM connection_oauth WHERE platform = 'TWITCH';
	`

	rows, err := Postgres.Pool.Query(context.Background(), query)
	if err != nil {
		utils.Error.Println("Error getting tokens", err)
		return
	}

	defer rows.Close()

	var validated = 0
	var deleted = 0

	for rows.Next() {
		var con common.PlatformOauth

		err := rows.Scan(&con.AccessToken, &con.PlatformID)
		if err != nil {
				utils.Error.Println("Error scanning token:", err)
				continue
		}

		if con.AccessToken == "" {
			utils.Error.Println("Empty token for user_id", con.PlatformID)
			continue
		}

		valid, err := utils.ValidateHelixToken(con.AccessToken)
		if err != nil {
			utils.Error.Println("Error validating token", err)
			continue
		}

		if !valid {
			deleteQuery := `
				DELETE FROM connection_oauth
				WHERE platform_id = $1
				AND platform = $2
				AND access_token = $3;
			`

			_, err := Postgres.Pool.Exec(
				context.Background(),
				deleteQuery,
				con.PlatformID,
				"TWITCH",
				con.AccessToken,
			)
			if err != nil {
				utils.Error.Println("Error deleting token for user_id", con.PlatformID, ":", err)
			} else {
				deleted++
			}
		}

		time.Sleep(200 * time.Millisecond)
		validated++
	}

	utils.Info.Printf(
		"Validated %d helix tokens, and deleted %d expired tokens",
		validated,
		deleted,
	)
}

func sortFiles(files []string) func(i, j int) bool {
	return func (i, j int) bool {
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

	filesToDelete := files[:len(files) - max + 1]
	for _, file := range filesToDelete {
		err := os.Remove(file)
		if err != nil {
			utils.Error.Println("Failed deleting dump file ", err)
		}
	}

	utils.Info.Printf("Deleted %d old dump files", len(filesToDelete))
}

func backupPostgres() {
	utils.Debug.Println("Backing up Postgres")

	if err := os.MkdirAll(dumpPath, os.ModePerm); err != nil {
		utils.Error.Println("Failed to create backup folder:", err)
		return
	}

	files, err := filepath.Glob(filepath.Join(dumpPath, "*.sql.gz"))
	if err != nil {
		utils.Error.Println("Failed to list dump files:", err)
		return
	}

	deleteOldDumps(files, maxFiles)

	filePath := filepath.Join(dumpPath, fmt.Sprintf("data_%d.sql.gz", time.Now().Unix()))
	cmd := exec.Command("sh", "-c", fmt.Sprintf(
		"PGPASSWORD=%s pg_dump -d %s -U %s -h %s | gzip > %s",
		pgPassword, dbName, dbUser, dbHost, filePath,
	))

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

	queueMessage := fmt.Sprintf("postgres-backup:%s", string(jsonMessage))
	err = utils.PublishToQueue(context.Background(), queueMessage, 1*time.Minute)
	if err != nil {
		utils.Error.Println("Failed to publish to queue:", err)
		return
	}

	utils.Info.Println(message)
}

func getDatabaseSize(dbName string) (string, error) {
	query := `SELECT pg_size_pretty(pg_database_size($1)) as size`
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
