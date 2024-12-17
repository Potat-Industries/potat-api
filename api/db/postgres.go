package db

import (
	"context"
	"fmt"
	"sync"
	"time"

	potat "potat-api/api/types"
	"potat-api/api/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

var Postgres *DB

func InitPostgres(config utils.Config) error {
	dbConfig, err := loadConfig(config)
	if err != nil {
		return err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		return err
	}

	Postgres = &DB{Pool: pool}
	return nil
}

func loadConfig(config utils.Config) (*pgxpool.Config, error) {
	constring := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		config.Postgres.User,
		config.Postgres.Password,
		config.Postgres.Host,
		config.Postgres.Port,
		config.Postgres.Database,
	)

	dbConfig, err := pgxpool.ParseConfig(constring)
	if err != nil {
		utils.Error.Panicln("Error parsing database config", err)
	}

	dbConfig.MaxConns = 32
	dbConfig.MinConns = 4
	dbConfig.MaxConnIdleTime = 1 * time.Minute
	dbConfig.MaxConnLifetime = 30 * time.Minute
	dbConfig.HealthCheckPeriod = 5 * time.Minute
	dbConfig.ConnConfig.ConnectTimeout = 10 * time.Second

	return dbConfig, nil
}

func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

func (db *DB) GetUserByName(ctx context.Context, username string) (*potat.User, error) {
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
		WHERE uc.platform_username = $1
		  AND uc.platform = 'TWITCH'
		GROUP BY users.user_id;
	`

	var user potat.User
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

func (db *DB) GetUserByInternalID(ctx context.Context, id int) (*potat.User, error) {
	query := `
		SELECT
			users.user_id,
			username,
			display,
			first_seen,
			level,
			settings,
			json_agg(uc) as connections
		FROM users
		JOIN user_connections uc ON users.user_id = uc.user_id
		WHERE user_id = $1
	`

	var user potat.User
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

func (db *DB) GetChannelBlocks(ctx context.Context, channelID string) (*[]potat.Block) {
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

	var blocks []potat.Block
	for rows.Next() {
		var block potat.Block
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

func (db *DB) GetChannelCommands(ctx context.Context, channelID string) *[]potat.ChannelCommand {
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

	var commands []potat.ChannelCommand
	for rows.Next() {
		var command potat.ChannelCommand
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

func (db *DB) GetChannelByName(ctx context.Context, username string) (*potat.Channel, error) {
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
		WHERE username = $1;
	`

	var channel potat.Channel
	err := db.Pool.QueryRow(ctx, query, username).Scan(
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
	
	var commands *[]potat.ChannelCommand
	go func() {
		defer wg.Done()
		cmds := db.GetChannelCommands(ctx, channel.ChannelID)
		if cmds != nil {
			commands = cmds
		}
	}()
	
	
	var blocks []potat.Block
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
		channel.Commands = &[]potat.ChannelCommand{}
	}
	
	if len(blocks) > 0 {
		channel.Blocks = potat.FilteredBlocks{
			Users:    &[]potat.Block{},
			Commands: &[]potat.Block{},
		}
	
		for _, block := range blocks {
			if block.BlockType == potat.UserBlock {
				*channel.Blocks.Users = append(*channel.Blocks.Users, block)
			} else if block.BlockType == potat.CommandBlock {
				*channel.Blocks.Commands = append(*channel.Blocks.Commands, block)
			}
		}
	} else {
		channel.Blocks = potat.FilteredBlocks{}
	}
	
	return &channel, nil
}

func (db *DB) GetPotatoData(ctx context.Context, username string) (*potat.PotatoData, error) {
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

	var data potat.PotatoData

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