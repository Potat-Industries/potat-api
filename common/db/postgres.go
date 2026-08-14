// Package db provides database clients and functions to retrieve or update data.

package db

import (
	"context"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/Potat-Industries/potat-api/common"
	"github.com/Potat-Industries/potat-api/common/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresClient is a wrapper around the pgxpool.Pool to manage database connections and queries.

type PostgresClient struct {
	*pgxpool.Pool
}

// LoaderKey is used to identify a user or channel in the database.

type LoaderKey struct {
	ID *int

	UserID *string

	Username *string

	Platform *string
}

// ErrPostgresNoRows is an alias for pgx.ErrNoRows to handle cases where no rows are returned from a query.

var (
	ErrPostgresNoRows = pgx.ErrNoRows

	errInvalidType = fmt.Errorf("invalid channel type")
)

// InitPostgres initializes a new Postgres client with the provided configuration.

func InitPostgres(ctx context.Context, config common.Config) (*PostgresClient, error) {
	dbConfig, err := loadConfig(config)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		return nil, err
	}

	return &PostgresClient{pool}, nil
}

func loadConfig(config common.Config) (*pgxpool.Config, error) { //nolint:unparam
	user := config.Postgres.User

	if user == "" {
		user = "postgres"
	}

	host := config.Postgres.Host

	if host == "" {
		host = "localhost" //nolint:goconst
	}

	port := config.Postgres.Port

	if port == "" {
		port = "5432"
	}

	database := config.Postgres.Database

	if database == "" {
		database = "postgres"
	}

	constring := fmt.Sprintf(

		"postgres://%s:%s@%s:%s/%s",

		user,

		config.Postgres.Password,

		host,

		port,

		database,
	)

	dbConfig, err := pgxpool.ParseConfig(constring)
	if err != nil {
		logger.Error.Panicln("Error parsing database config", err)

		return nil, err
	}

	dbConfig.MaxConns = 32

	dbConfig.MinConns = 4

	dbConfig.MaxConnIdleTime = 1 * time.Minute

	dbConfig.MaxConnLifetime = 30 * time.Minute

	dbConfig.HealthCheckPeriod = 5 * time.Minute

	dbConfig.ConnConfig.ConnectTimeout = 10 * time.Second

	return dbConfig, nil
}

// CheckTableExists checks if a table exists in the database and creates it if it doesn't.

func (db *PostgresClient) CheckTableExists(ctx context.Context, createTable string) {
	_, err := db.Pool.Exec(ctx, createTable)
	if err != nil {
		logger.Error.Fatalf("Failed to create table: %v", err)
	}
}

// Ping checks the connection to the database.

func (db *PostgresClient) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// GetUserByName retrieves a user by their username from the database.

func (db *PostgresClient) GetUserByName(ctx context.Context, username string) (*common.User, error) {
	query := `

		SELECT

				users.user_id,

				users.username,

				users.display,

				users.first_seen,

				users.level,

				users.settings,

				json_agg(uc) AS connections

		FROM users

		LEFT JOIN user_connections uc ON users.user_id = uc.user_id

		WHERE users.username = $1

		GROUP BY users.user_id;

	`

	var user common.User

	err := db.Pool.QueryRow(ctx, query, username).Scan(

		&user.ID,

		&user.Username,

		&user.Display,

		&user.FirstSeen,

		&user.Level,

		&user.Settings,

		&user.Connections,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUserByInternalID retrieves a user by their internal ID from the database.

func (db *PostgresClient) GetUserByInternalID(ctx context.Context, id int) (*common.User, error) {
	query := `

		SELECT

			u.user_id,

			username,

			display,

			first_seen,

			level,

			settings,

			json_agg(uc) as connections

		FROM users u

		JOIN user_connections uc ON u.user_id = uc.user_id

		WHERE u.user_id = $1

		GROUP BY u.user_id;

	`

	var user common.User

	err := db.Pool.QueryRow(ctx, query, id).Scan(

		&user.ID,

		&user.Username,

		&user.Display,

		&user.FirstSeen,

		&user.Level,

		&user.Settings,

		&user.Connections,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (db *PostgresClient) GetUserByPlatformID(
	ctx context.Context, platformID string, platform common.Platforms,
) (*common.User, error) {
	query := `

		SELECT

			u.user_id,

			u.username,

			u.display,

			u.first_seen,

			u.level,

			u.settings,

			json_agg(uc) AS connections

		FROM users u

		JOIN user_connections uc ON u.user_id = uc.user_id

		WHERE u.user_id = (

			SELECT user_id FROM user_connections WHERE platform_id = $1 AND platform = $2 LIMIT 1

		)

		GROUP BY u.user_id;

	`

	var user common.User

	err := db.Pool.QueryRow(ctx, query, platformID, string(platform)).Scan(

		&user.ID,

		&user.Username,

		&user.Display,

		&user.FirstSeen,

		&user.Level,

		&user.Settings,

		&user.Connections,
	)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetChannelBlocks retrieves all blocks for a given channel from the database.

func (db *PostgresClient) GetChannelBlocks(ctx context.Context, channelID string) *[]common.Block {
	query := `
		SELECT
			user_id,
			block_id,
			channel_id,
			block_type,
			block_data
		FROM blocks
		WHERE channel_id = $1
	`

	rows, err := db.Pool.Query(ctx, query, channelID)
	if err != nil {
		return nil
	}

	defer rows.Close()

	var blocks []common.Block

	for rows.Next() {
		var block common.Block

		err := rows.Scan(

			&block.ID,

			&block.BlockedUserID,

			&block.ChannelID,

			&block.BlockType,

			&block.CommandName,
		)
		if err != nil {
			return nil
		}

		blocks = append(blocks, block)
	}

	return &blocks
}

// GetChannelCommands retrieves all custom channel commands for a given channel from the database.

func (db *PostgresClient) GetChannelCommands(ctx context.Context, channelID string) *[]common.ChannelCommand {
	query := `

		SELECT

			command_id,

			user_id,

			channel_id,

			name,

			user_trigger_ids,

			user_ignore_ids,

			trigger,

			response,

			run_command,

			active,

			active_online,

			active_offline,

			reply,

			whisper,

			announce,

			cooldown,

			delay,

			use_count,

			created,

			modified,

			platform,

			help

		FROM custom_channel_commands

		WHERE channel_id = $1

	`

	rows, err := db.Pool.Query(ctx, query, channelID)
	if err != nil {
		return nil
	}

	defer rows.Close()

	var commands []common.ChannelCommand

	for rows.Next() {
		var command common.ChannelCommand

		err := rows.Scan(

			&command.CommandID,

			&command.UserID,

			&command.ChannelID,

			&command.Name,

			&command.UserTriggerIDs,

			&command.UserIgnoreIDs,

			&command.Trigger,

			&command.Response,

			&command.RunCommand,

			&command.Active,

			&command.ActiveOnline,

			&command.ActiveOffline,

			&command.Reply,

			&command.Whisper,

			&command.Announce,

			&command.Cooldown,

			&command.Delay,

			&command.UseCount,

			&command.Created,

			&command.Modified,

			&command.Platform,

			&command.Help,
		)
		if err != nil {
			return nil
		}

		commands = append(commands, command)
	}

	return &commands
}

// GetChannelByName retrieves a channel by its username and platform from the database.

func (db *PostgresClient) GetChannelByName(
	ctx context.Context,

	username string,

	platform common.Platforms,
) (*common.Channel, error) {
	return db.getChannelByType(ctx, username, platform, "NAME")
}

// GetChannelByID retrieves a channel by its ID and platform from the database.

func (db *PostgresClient) GetChannelByID(
	ctx context.Context,

	channelID string,

	platform common.Platforms,
) (*common.Channel, error) {
	return db.getChannelByType(ctx, channelID, platform, "ID")
}

func (db *PostgresClient) getChannelByType( //nolint:cyclop

	ctx context.Context,

	value string,

	platform common.Platforms,

	chanType string,
) (*common.Channel, error) {
	query := `

	  SELECT

		  c.channel_id,

			c.username,

			c.joined_at,

			c.added_by,

			c.platform,

			c.settings,

			c.editors,

			c.ambassadors,

			c.meta,

			c.state

		FROM channels c

	`

	switch chanType {
	case "ID":

		query += `WHERE c.channel_id = $1 `

	case "NAME":

		query += `WHERE c.username = $1 `

	default:

		return nil, errInvalidType
	}

	query += `AND platform = $2;`

	var channel common.Channel

	err := db.Pool.QueryRow(ctx, query, value, platform).Scan(

		&channel.ChannelID,

		&channel.Username,

		&channel.JoinedAt,

		&channel.AddedBy,

		&channel.Platform,

		&channel.Settings,

		&channel.Editors,

		&channel.Ambassadors,

		&channel.Meta,

		&channel.State,
	)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup

	wg.Add(2)

	var commands *[]common.ChannelCommand

	go func() {
		defer wg.Done()

		cmds := db.GetChannelCommands(ctx, channel.ChannelID)

		if cmds != nil {
			commands = cmds
		}
	}()

	var blocks []common.Block

	go func() {
		defer wg.Done()

		bs := db.GetChannelBlocks(ctx, channel.ChannelID)

		if bs != nil {
			blocks = *bs
		}
	}()

	wg.Wait()

	if commands != nil {
		channel.Commands = commands
	} else {
		channel.Commands = &[]common.ChannelCommand{}
	}

	if len(blocks) > 0 {
		channel.Blocks = common.FilteredBlocks{
			Users: &[]common.Block{},

			Commands: &[]common.Block{},
		}

		for _, block := range blocks {
			switch block.BlockType {
			case common.UserBlock:

				*channel.Blocks.Users = append(*channel.Blocks.Users, block)

			case common.CommandBlock:

				*channel.Blocks.Commands = append(*channel.Blocks.Commands, block)

			case common.GlobalBlock:

				continue
			}
		}
	} else {
		channel.Blocks = common.FilteredBlocks{}
	}

	return &channel, nil
}

// GetPotatoData retrieves potato data for a user from the database.

func (db *PostgresClient) GetPotatoData(ctx context.Context, username string) (*common.PotatoData, error) {
	query := `

		SELECT

			p.user_id,

			p.potato_count,

			p.potato_prestige,

			p.potato_rank,

			p.tax_multiplier,

			p.first_seen,

			p.stole_from,

			p.stole_amount,

			p.trampled_by,

			a.average_response_time,

			a.eat_count,

			a.harvest_count,

			a.stolen_count,

			a.theft_count,

			a.trampled_count,

			a.trample_count,

			a.cdr_count,

			a.quiz_count,

			a.quiz_complete_count,

			a.guard_buy_count,

			a.fertilizer_buy_count,

			a.cdr_buy_count,

			a.new_quiz_buy_count,

			a.gamble_win_count,

			a.gamble_loss_count,

			a.gamble_wins_total,

			a.gamble_losses_total,

			a.duel_win_count,

			a.duel_loss_count,

			a.duel_wins_amount,

			a.duel_losses_amount,

			a.duel_caught_losses,

			a.average_response_count,

			s.not_verbose

		FROM ( SELECT user_id FROM users WHERE username = $1 ) u

		INNER JOIN potatoes p ON p.user_id = u.user_id

		INNER JOIN potato_analytics a ON u.user_id = a.user_id

		INNER JOIN potato_settings s ON u.user_id = s.user_id;

	`

	var data common.PotatoData

	err := db.Pool.QueryRow(ctx, query, username).Scan(

		&data.ID,

		&data.PotatoCount,

		&data.PotatoPrestige,

		&data.PotatoRank,

		&data.TaxMultiplier,

		&data.FirstSeen,

		&data.StoleFrom,

		&data.StoleAmount,

		&data.TrampledBy,

		&data.AverageResponseTime,

		&data.EatCount,

		&data.HarvestCount,

		&data.StolenCount,

		&data.TheftCount,

		&data.TrampledCount,

		&data.TrampleCount,

		&data.CDRCount,

		&data.QuizCount,

		&data.QuizCompleteCount,

		&data.GuardBuyCount,

		&data.FertilizerBuyCount,

		&data.CDRBuyCount,

		&data.NewQuizBuyCount,

		&data.GambleWinCount,

		&data.GambleLossCount,

		&data.GambleWinsTotal,

		&data.GambleLossesTotal,

		&data.DuelWinCount,

		&data.DuelLossCount,

		&data.DuelWinsAmount,

		&data.DuelLossesAmount,

		&data.DuelCaughtLosses,

		&data.AverageResponseCount,

		&data.NotVerbose,
	)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// BatchUserConections retrieves user connections for a batch of user IDs from the database.

func (db *PostgresClient) BatchUserConections(
	ctx context.Context,

	ids []int,
) *map[int][]common.UserConnection {
	query := `

		SELECT

			user_id,

			platform_id,

			platform_username,

			platform_display,

			platform_pfp,

			platform,

			platform_metadata

		FROM user_connections

		WHERE user_id = ANY($1::INT[])

	`

	rows, err := db.Pool.Query(ctx, query, ids)
	if err != nil {
		return nil
	}

	defer rows.Close()

	users := make(map[int][]common.UserConnection)

	for rows.Next() {
		var connection common.UserConnection

		err := rows.Scan(

			&connection.ID,

			&connection.UserID,

			&connection.Username,

			&connection.Display,

			&connection.PFP,

			&connection.Platform,

			&connection.Meta,
		)
		if err != nil {
			return nil
		}

		users[connection.ID] = append(users[connection.ID], connection)
	}

	return &users
}

// GetRedirectByKey retrieves a URL redirect from the database by its key.

func (db *PostgresClient) GetRedirectByKey(ctx context.Context, key string) (string, error) {
	query := `SELECT url FROM url_redirects WHERE key = $1`

	var url string

	err := db.Pool.QueryRow(ctx, query, key).Scan(&url)
	if err != nil {
		return "", err
	}

	return url, nil
}

// GetKeyByRedirect retrieves the key associated with a given URL redirect from the database.

func (db *PostgresClient) GetKeyByRedirect(ctx context.Context, url string) (string, error) {
	query := `SELECT key FROM url_redirects WHERE url = $1`

	var key string

	err := db.Pool.QueryRow(ctx, query, url).Scan(&key)
	if err != nil {
		return "", err
	}

	return key, nil
}

// RedirectExists checks if a URL redirect exists in the database by its key.

func (db *PostgresClient) RedirectExists(ctx context.Context, key string) bool {
	query := `SELECT EXISTS(SELECT 1 FROM url_redirects WHERE key = $1)`

	var exists bool

	err := db.Pool.QueryRow(ctx, query, key).Scan(&exists)
	if err != nil {
		return false
	}

	return exists
}

// NewRedirect inserts a new URL redirect into the database.

func (db *PostgresClient) NewRedirect(ctx context.Context, key, url string) error {
	query := `INSERT INTO url_redirects (key, url) VALUES ($1, $2)`

	_, err := db.Pool.Exec(ctx, query, key, url)

	return err
}

// GetHaste retrieves a hastebin text document from the database by its key.

func (db *PostgresClient) GetHaste(ctx context.Context, key string) (string, error) {
	query := `

		UPDATE haste

		SET access_count = access_count + 1

		WHERE key = $1

		RETURNING convert_from(zstd_decompress(content::bytea), 'utf-8') AS text;

	`

	var text string

	err := db.Pool.QueryRow(ctx, query, encode(key)).Scan(&text)
	if err != nil {
		return "", err
	}

	return text, nil
}

// NewHaste inserts a new compressed hastebin text document into the database.

func (db *PostgresClient) NewHaste(
	ctx context.Context,

	key string,

	text []byte,

	source string,
) error {
	query := `

		INSERT INTO haste (key, content, source)

		VALUES ($1, zstd_compress($2, null, 8), $3)

		ON CONFLICT (key) DO NOTHING;

	`

	_, err := db.Pool.Exec(ctx, query, encode(key), text, source)

	return err
}

func encode(data string) string {
	hash := md5.New() //nolint:gosec

	hash.Write([]byte(data))

	return hex.EncodeToString(hash.Sum(nil))
}

// NewUpload inserts a new file into the database and returns the creation timestamp.

func (db *PostgresClient) NewUpload(
	ctx context.Context,

	key string,

	file []byte,

	name string,

	mimeType string,
) (bool, *time.Time) {
	query := `

		INSERT INTO file_store (file, file_name, mime_type, key)

		VALUES ($1, $2, $3, $4)

		RETURNING created_at;

	`

	var createdAt time.Time

	err := db.Pool.QueryRow(ctx, query, file, name, mimeType, key).Scan(&createdAt)
	if err != nil {
		logger.Error.Println("Error scanning upload", err)

		return false, nil
	}

	return true, &createdAt
}

// GetFileByKey retrieves a file from the database by its key.

func (db *PostgresClient) GetFileByKey(
	ctx context.Context,

	key string,
) ([]byte, string, *string, *time.Time, error) {
	query := `

		SELECT file, mime_type, file_name, created_at

		FROM file_store

		WHERE key = $1

	`

	var content []byte

	var mimeType string

	var fileName *string

	var createdAt time.Time

	err := db.Pool.QueryRow(ctx, query, key).Scan(

		&content,

		&mimeType,

		&fileName,

		&createdAt,
	)
	if err != nil {
		return nil, "", nil, nil, err
	}

	return content, mimeType, fileName, &createdAt, nil
}

// DeleteFileByKey deletes a file from the database by its key.

func (db *PostgresClient) DeleteFileByKey(
	ctx context.Context,

	key string,
) bool {
	query := `

		DELETE FROM file_store

		WHERE key = $1

	`

	_, err := db.Pool.Exec(ctx, query, key)

	return err == nil
}

// GetUploadCreatedAt retrieves the creation timestamp of an upload by its key.

func (db *PostgresClient) GetUploadCreatedAt(
	ctx context.Context,

	key string,
) (*time.Time, error) {
	query := `

		SELECT created_at

		FROM file_store

		WHERE key = $1

	`

	var createdAt time.Time

	err := db.Pool.QueryRow(ctx, query, key).Scan(&createdAt)
	if err != nil {
		return nil, err
	}

	return &createdAt, nil
}

// GetAllChannels retrieves all channels with state = 'JOINED', ordered by username.

func (db *PostgresClient) GetAllChannels(ctx context.Context) ([]common.ChannelListItem, error) {
	query := `

		SELECT channel_id, username, platform, state

		FROM channels

		WHERE state = 'JOINED'

		ORDER BY username

	`

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var channels []common.ChannelListItem

	for rows.Next() {
		var ch common.ChannelListItem

		if err := rows.Scan(&ch.ChannelID, &ch.Username, &ch.Platform, &ch.State); err != nil {
			return nil, err
		}

		channels = append(channels, ch)
	}

	return channels, rows.Err()
}

// UpdateUserSettings replaces the settings JSONB column for a user.

func (db *PostgresClient) UpdateUserSettings(ctx context.Context, userID int, settings common.UserSettings) error {
	query := `UPDATE users SET settings = $1 WHERE user_id = $2`

	_, err := db.Pool.Exec(ctx, query, settings, userID)

	return err
}

// UpdateChannelSettings replaces the settings JSONB column for a channel.

func (db *PostgresClient) UpdateChannelSettings(
	ctx context.Context,

	channelID string,

	platform string,

	settings common.ChannelSettings,
) error {
	query := `UPDATE channels SET settings = $1 WHERE channel_id = $2 AND platform = $3`

	_, err := db.Pool.Exec(ctx, query, settings, channelID, platform)

	return err
}

// GetChannelSettingsByID retrieves only the settings column for a channel by its ID.

func (db *PostgresClient) GetChannelSettingsByID(
	ctx context.Context,

	channelID string,

	platform string,
) (common.ChannelSettings, error) {
	var settings common.ChannelSettings

	err := db.Pool.QueryRow(

		ctx,

		`SELECT settings FROM channels WHERE channel_id = $1 AND platform = $2`,

		channelID,

		platform,
	).Scan(&settings)

	return settings, err
}

// GetCommandSettings retrieves all command settings rows for a given channel.

func (db *PostgresClient) GetCommandSettings(
	ctx context.Context,

	channelID string,
) ([]common.CommandSettings, error) {
	query := `

		SELECT

			channel_id,

			command,

			permission,

			users_blacklisted,

			users_whitelisted,

			custom_cooldown,

			channel_usage,

			is_enabled,

			offline_only,

			silent_errors,

			allow_bots,

			platform,

			ambassador_granted

		FROM command_settings

		WHERE channel_id = $1

		ORDER BY command

	`

	rows, err := db.Pool.Query(ctx, query, channelID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var results []common.CommandSettings

	for rows.Next() {
		var cs common.CommandSettings

		if err := rows.Scan(

			&cs.ChannelID,

			&cs.Command,

			&cs.Permission,

			&cs.UsersBlacklisted,

			&cs.UsersWhitelisted,

			&cs.CustomCooldown,

			&cs.ChannelUsage,

			&cs.IsEnabled,

			&cs.OfflineOnly,

			&cs.SilentErrors,

			&cs.AllowBots,

			&cs.Platform,

			&cs.AmbassadorGranted,
		); err != nil {
			return nil, err
		}

		results = append(results, cs)
	}

	return results, rows.Err()
}

// UpsertCommandSettings inserts or updates a single command's settings row.

func (db *PostgresClient) UpsertCommandSettings(ctx context.Context, cs common.CommandSettings) error {
	query := `

		INSERT INTO command_settings (

			channel_id, command, permission, users_blacklisted, users_whitelisted,

			custom_cooldown, is_enabled, offline_only, silent_errors, allow_bots,

			platform, ambassador_granted

		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)

		ON CONFLICT (channel_id, command, platform) DO UPDATE SET

			permission         = EXCLUDED.permission,

			users_blacklisted  = EXCLUDED.users_blacklisted,

			users_whitelisted  = EXCLUDED.users_whitelisted,

			custom_cooldown    = EXCLUDED.custom_cooldown,

			is_enabled         = EXCLUDED.is_enabled,

			offline_only       = EXCLUDED.offline_only,

			silent_errors      = EXCLUDED.silent_errors,

			allow_bots         = EXCLUDED.allow_bots,

			platform           = EXCLUDED.platform,

			ambassador_granted = EXCLUDED.ambassador_granted

	`

	platform := cs.Platform

	if platform == "" {
		platform = "TWITCH"
	}

	_, err := db.Pool.Exec(

		ctx, query,

		cs.ChannelID, cs.Command, cs.Permission,

		cs.UsersBlacklisted, cs.UsersWhitelisted,

		cs.CustomCooldown, cs.IsEnabled, cs.OfflineOnly,

		cs.SilentErrors, cs.AllowBots, platform, cs.AmbassadorGranted,
	)

	return err
}

// ResetCommandSettings resets a single command's overrides back to their defaults.

func (db *PostgresClient) ResetCommandSettings(ctx context.Context, channelID, command string) error {
	query := `

		UPDATE command_settings SET

			is_enabled         = TRUE,

			offline_only       = NULL,

			custom_cooldown    = NULL,

			silent_errors      = FALSE,

			users_whitelisted  = NULL,

			users_blacklisted  = NULL,

			allow_bots         = NULL,

			permission         = NULL,

			ambassador_granted = FALSE

		WHERE channel_id = $1 AND command = $2

	`

	_, err := db.Pool.Exec(ctx, query, channelID, command)

	return err
}

// GetChannelAmbassadors returns the ambassadors slice for a channel, used for auth checks.

func (db *PostgresClient) GetChannelAmbassadors(
	ctx context.Context,

	channelID string,

	platform common.Platforms,
) ([]string, error) {
	query := `SELECT ambassadors FROM channels WHERE channel_id = $1 AND platform = $2`

	var ambassadors []string

	err := db.Pool.QueryRow(ctx, query, channelID, platform).Scan(&ambassadors)
	if err != nil {
		return nil, err
	}

	return ambassadors, nil
}

// GetUserReminders retrieves all pending reminders for a user on a given platform.

func (db *PostgresClient) GetUserReminders(
	ctx context.Context,

	userID string,

	platform common.Platforms,
) ([]common.Reminder, error) {
	query := `

		SELECT

			reminder_id, user_id, recipient_id, channel_id,

			message, ready_at, set_at, afk_withheld, status, platform, sent_at, type

		FROM reminders

		WHERE recipient_id = $1 AND platform = $2

		ORDER BY set_at DESC

	`

	rows, err := db.Pool.Query(ctx, query, userID, platform)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var reminders []common.Reminder

	for rows.Next() {
		var r common.Reminder

		if err := rows.Scan(

			&r.ReminderID, &r.UserID, &r.RecipientID, &r.ChannelID,

			&r.Message, &r.ReadyAt, &r.SetAt, &r.AfkWithheld, &r.Status, &r.Platform, &r.SentAt, &r.Type,
		); err != nil {
			return nil, err
		}

		reminders = append(reminders, r)
	}

	return reminders, rows.Err()
}

// DeleteReminder hard-deletes a reminder by ID, verifying the owner platform ID.

func (db *PostgresClient) DeleteReminder(ctx context.Context, reminderID int, recipientID string) error {
	query := `DELETE FROM reminders WHERE reminder_id = $1 AND recipient_id = $2`

	_, err := db.Pool.Exec(ctx, query, reminderID, recipientID)

	return err
}

// UpsertOAuthToken stores or refreshes a platform OAuth token for a given user.

func (db *PostgresClient) UpsertOAuthToken(
	ctx context.Context,

	platformID string,

	platform common.Platforms,

	accessToken string,

	refreshToken string,

	scope []string,

	expiresIn int,
) error {
	query := `

		INSERT INTO connection_oauth (

			platform_id, access_token, refresh_token, scope, expires_in, added_at, platform

		) VALUES ($1, $2, $3, $4, $5, $6, $7)

		ON CONFLICT (platform_id, platform) DO UPDATE SET

			access_token  = EXCLUDED.access_token,

			refresh_token = EXCLUDED.refresh_token,

			scope         = EXCLUDED.scope,

			expires_in    = EXCLUDED.expires_in,

			added_at      = EXCLUDED.added_at

	`

	_, err := db.Pool.Exec(

		ctx, query,

		platformID, accessToken, refreshToken, scope, expiresIn, time.Now(), platform,
	)

	return err
}
