package db

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"potat-api/common"
	"potat-api/common/logger"
)

type PostgresClient struct {
	*pgxpool.Pool
}

type LoaderKey struct {
	ID       *int
	UserID   *string
	Username *string
	Platform *string
}

var PostgresNoRows = pgx.ErrNoRows

func InitPostgres(config common.Config) (*PostgresClient, error) {
	dbConfig, err := loadConfig(config)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		return nil, err
	}

	return &PostgresClient{pool}, nil
}

func loadConfig(config common.Config) (*pgxpool.Config, error) {
	user := config.Postgres.User
	if user == "" {
		user = "postgres"
	}

	host := config.Postgres.Host
	if host == "" {
		host = "localhost"
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
	}

	dbConfig.MaxConns = 32
	dbConfig.MinConns = 4
	dbConfig.MaxConnIdleTime = 1 * time.Minute
	dbConfig.MaxConnLifetime = 30 * time.Minute
	dbConfig.HealthCheckPeriod = 5 * time.Minute
	dbConfig.ConnConfig.ConnectTimeout = 10 * time.Second

	return dbConfig, nil
}

func (db *PostgresClient) CheckTableExists(createTable string) {
	_, err := db.Pool.Exec(context.Background(), createTable)
	if err != nil {
		logger.Error.Fatalf("Failed to create table: %v", err)
	}
}

func (db *PostgresClient) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

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

func (db *PostgresClient) GetChannelBlocks(ctx context.Context, channelID string) *[]common.Block {
	query := `
		SELECT
		  user_id
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

func (db *PostgresClient) GetChannelByID(
	ctx context.Context,
	channelID string,
	platform common.Platforms,
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
		WHERE channel_id = $1
		AND platform = $2;
	`

	var channel common.Channel
	err := db.Pool.QueryRow(ctx, query, channelID, platform).Scan(
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
			Users:    &[]common.Block{},
			Commands: &[]common.Block{},
		}

		for _, block := range blocks {
			if block.BlockType == common.UserBlock {
				*channel.Blocks.Users = append(*channel.Blocks.Users, block)
			} else if block.BlockType == common.CommandBlock {
				*channel.Blocks.Commands = append(*channel.Blocks.Commands, block)
			}
		}
	} else {
		channel.Blocks = common.FilteredBlocks{}
	}

	return &channel, nil
}

func (db *PostgresClient) GetChannelByName(
	ctx context.Context,
	username string,
	platform common.Platforms,
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
		WHERE username = $1
		AND platform = $2;
	`

	var channel common.Channel
	err := db.Pool.QueryRow(ctx, query, username, platform).Scan(
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
			Users:    &[]common.Block{},
			Commands: &[]common.Block{},
		}

		for _, block := range blocks {
			if block.BlockType == common.UserBlock {
				*channel.Blocks.Users = append(*channel.Blocks.Users, block)
			} else if block.BlockType == common.CommandBlock {
				*channel.Blocks.Commands = append(*channel.Blocks.Commands, block)
			}
		}
	} else {
		channel.Blocks = common.FilteredBlocks{}
	}

	return &channel, nil
}

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

	err := db.Pool.QueryRow(context.Background(), query, username).Scan(
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

func (db *PostgresClient) BatchUserConections(
	ctx context.Context,
	IDs []int,
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

	rows, err := db.Pool.Query(ctx, query, IDs)
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

func (db *PostgresClient) GetRedirectByKey(ctx context.Context, key string) (string, error) {
	query := `SELECT url FROM url_redirects WHERE key = $1`

	var url string
	err := db.Pool.QueryRow(ctx, query, key).Scan(&url)
	if err != nil {
		return "", err
	}

	return url, nil
}

func (db *PostgresClient) GetKeyByRedirect(ctx context.Context, url string) (string, error) {
	query := `SELECT key FROM url_redirects WHERE url = $1`

	var key string
	err := db.Pool.QueryRow(ctx, query, url).Scan(&key)
	if err != nil {
		return "", err
	}

	return key, nil
}

func (db *PostgresClient) RedirectExists(ctx context.Context, key string) bool {
	query := `SELECT EXISTS(SELECT 1 FROM url_redirects WHERE key = $1)`

	var exists bool

	err := db.Pool.QueryRow(ctx, query, key).Scan(&exists)
	if err != nil {
		return false
	}

	return exists
}

func (db *PostgresClient) NewRedirect(ctx context.Context, key, url string) error {
	query := `INSERT INTO url_redirects (key, url) VALUES ($1, $2)`

	_, err := db.Pool.Exec(ctx, query, key, url)

	return err
}

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
	hash := md5.New()
	hash.Write([]byte(data))

	return hex.EncodeToString(hash.Sum(nil))
}

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
