package db

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"time"

	"potat-api/common"
	"potat-api/common/utils"

	"github.com/robfig/cron/v3"
)

var (
	maxFiles = 10
	dumpPath = "/..."
)

func StartLoops(config common.Config) {
	go decrementDuels()
	go deleteOldUploads()
	go updateAggregateTable()
	// go backupPostgres() TODO
	// go backupClickhouse()

	c := cron.New()
	c.AddFunc("@hourly", updateHourlyUsage)
	c.AddFunc("@daily", updateDailyUsage)
	c.AddFunc("@weekly", updateWeeklyUsage)

	c.Start()

	defer c.Stop()
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

func deleteOldDumps(files []string, max int) {
	if len(files) <= max {
		return
	}

	sort.Slice(files, func(i, j int) bool {
		fileI, _ := os.Stat(filepath.Join(dumpPath, files[i]))
		fileJ, _ := os.Stat(filepath.Join(dumpPath, files[j]))
		return fileI.ModTime().Before(fileJ.ModTime())
	})

	filesToDelete := files[:len(files)-max + 1]
	for _, file := range filesToDelete {
		err := os.Remove(filepath.Join(dumpPath, file))
		if err != nil {
			utils.Error.Println("Failed deleting dump file", err)
		}
	}

	utils.Info.Printf("Deleted %d old dump files", len(filesToDelete))
}

func BackupPostgres() {
	files, err := filepath.Glob(filepath.Join(dumpPath, "*.sql.gz"))
	if err != nil {
		utils.Error.Println("Failed listing dump files", err)
	}

	deleteOldDumps(files, maxFiles)

	// backup compressed dump
}

