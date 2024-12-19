package common

import "time"

type Platforms string

const (
	TWITCH  Platforms = "TWITCH"
	DISCORD Platforms = "DISCORD"
	KICK    Platforms = "KICK"
	STV     Platforms = "STV"
)

type User struct {
	ID      		int              `json:"user_id"`
	Username    string           `json:"username"`
	Display     string           `json:"display"`
	FirstSeen   time.Time        `json:"first_seen"`
	Level       int              `json:"level"`
	Settings    UserSettings     `json:"settings"`
	Connections []UserConnection `json:"connections,omitempty"`
}

type UserSettings struct {
	IgnoreDropped  bool  	`json:"ignore_dropped"`
	ColorResponses bool		`json:"color_responses"`
	NoReply        bool  	`json:"no_reply"`
	Language       string	`json:"language"`
	IsBot          bool  	`json:"is_bot"`
	IsSelfBot      bool  	`json:"is_selfbot"`
}

type UserConnection struct {
	ID   		 int                    `json:"user_id"`
	Platform Platforms              `json:"platform"`
	Username string                 `json:"platform_username"`
	Display  string                 `json:"platform_display"`
	UserID   string                 `json:"platform_id"`
	PFP      string                 `json:"platform_pfp"`
	Meta     map[string]interface{} `json:"platform_metadata"`
}

type KickChannelMeta struct {
	ChannelID  string `json:"channel_id"`
	ChatroomID string `json:"chatroom_id"`
	UserID     string `json:"user_id"`
}

type TwitchChannelMeta struct {
	BotBanned    bool	`json:"bot_banned"`
	TwitchBanned bool	`json:"twitch_banned"`
}

type TwitchUserMeta struct {
	Color string      `json:"color"`
	Roles TwitchRoles `json:"roles"`
}

type StvUserMeta struct {
	Roles   []string `json:"roles"`
	PaintID string   `json:"paint_id"`
}

type TwitchRoles struct {
	IsStaff     bool `json:"isStaff"`
	IsPartner   bool `json:"isPartner"`
	IsAffiliate bool `json:"isAffiliate"`
}

type Channel struct {
	ChannelID    string          				`json:"channel_id"`
	Username     string          				`json:"username"`
	JoinedAt     *time.Time      				`json:"joined_at,omitempty"`
	AddedBy      []AddedByData   				`json:"added_by,omitempty"`
	Platform     Platforms       				`json:"platform"`
	Settings     ChannelSettings 				`json:"settings"`
	Editors      []string        				`json:"editors"`
	Ambassadors  []string        				`json:"ambassadors"`
	Meta         map[string]interface{} `json:"meta"`
	State        string          				`json:"state"`
	Commands     *[]ChannelCommand			`json:"commands,omitempty"`
	Blocks       FilteredBlocks  				`json:"blocks,omitempty"`
}

type FilteredBlocks struct {
	Users    *[]Block `json:"users"`
	Commands *[]Block `json:"commands"`
}

type Block struct {
	ID 			      int       `json:"user_id"`
	ChannelID   	string    `json:"channel_id"`
	BlockedUserID int     	`json:"blocked_user_id"`
	BlockType   	BlockType `json:"block_type"`
	CommandName   string    `json:"command_name,omitempty"`
}

type BlockType string

const (
	UserBlock    BlockType = "USER"
	CommandBlock BlockType = "COMMAND"
	GlobalBlock  BlockType = "GLOBAL"
)

type AddedByData struct {
	Username string    `json:"username"`
	ID       string    `json:"id"`
	AddedAt  time.Time `json:"addedAt"`
}

type ChannelSettings struct {
	Prefix             string   `json:"prefix"`
	ColorResponses     bool     `json:"color_responses"`
	Permission         string   `json:"permission"`
	NoReply            bool     `json:"no_reply"`
	SilentErrors       bool     `json:"silent_errors"`
	OfflineOnly        bool     `json:"offline_only"`
	WhisperOnly        bool     `json:"whisper_only"`
	FirstMsgResponses  bool     `json:"first_msg_responses"`
	Language           string   `json:"language"`
	ChannelCooldown    *int     `json:"channel_cooldown,omitempty"`
	UserCooldown       *int     `json:"user_cooldown,omitempty"`
	ForceLanguage      bool     `json:"force_language"`
	UsersBlacklisted   []string `json:"users_blacklisted"`
	PajBot             *string  `json:"paj_bot,omitempty"`
}

type CommandSettings struct {
	ChannelID        string   `json:"channel_id"`
	Command          string   `json:"command"`
	IsEnabled        bool     `json:"is_enabled"`
	OfflineOnly      bool     `json:"offline_only"`
	Permission       string   `json:"permission"`
	CustomCooldown   int      `json:"custom_cooldown"`
	ChannelUsage     int      `json:"channel_usage"`
	SilentErrors     bool     `json:"silent_errors"`
	UsersBlacklisted []string `json:"users_blacklisted"`
	UsersWhitelisted []string `json:"users_whitelisted"`
	AllowBots        bool     `json:"allow_bots"`
}

type PlatformOauth struct {
	PlatformID   string    `json:"platform_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Scope        []string  `json:"scope"`
	ExpiresIn    int       `json:"expires_in"`
	AddedAt      time.Time `json:"added_at"`
	Platform     Platforms `json:"platform"`
}

type ChannelCommand struct {
	UserID         int       `json:"user_id"`
	CommandID      int       `json:"command_id"`
	ChannelID      string    `json:"channel_id"`
	Name           *string   `json:"name"`
	UserTriggerIDs []string  `json:"user_trigger_ids"`
	UserIgnoreIDs  []string  `json:"user_ignore_ids"`
	Trigger        string    `json:"trigger"`
	Response       string    `json:"response"`
	RunCommand     *string   `json:"run_command"`
	Active         bool      `json:"active"`
	ActiveOnline   bool      `json:"active_online"`
	ActiveOffline  bool      `json:"active_offline"`
	Reply          bool      `json:"reply"`
	Whisper        bool      `json:"whisper"`
	Announce       bool      `json:"announce"`
	Cooldown       int       `json:"cooldown"`
	Delay          int       `json:"delay"`
	UseCount       int       `json:"use_count"`
	Created        time.Time `json:"created"`
	Modified       time.Time `json:"modified"`
	Platform       string    `json:"platform"`
	Help           *string   `json:"help"`
}

type Potatoes struct {
	ID     		     int     `json:"user_id"`
	PotatoCount    int     `json:"potato_count"`
	PotatoPrestige int     `json:"potato_prestige"`
	PotatoRank     int     `json:"potato_rank"`
	TaxMultiplier  int		 `json:"tax_multiplier"`
	FirstSeen      string  `json:"first_seen"`
	StoleFrom      *string `json:"stole_from"`
	StoleAmount    *int    `json:"stole_amount"`
	TrampledBy     *string `json:"trampled_by"`
}

type PotatoAnalytics struct {
	AverageResponseTime   string `json:"average_response_time"`
	EatCount              int    `json:"eat_count"`
	HarvestCount          int    `json:"harvest_count"`
	StolenCount           int    `json:"stolen_count"`
	TheftCount            int    `json:"theft_count"`
	TrampledCount         int    `json:"trampled_count"`
	TrampleCount          int    `json:"trample_count"`
	CDRCount              int    `json:"cdr_count"`
	QuizCount             int    `json:"quiz_count"`
	QuizCompleteCount     int    `json:"quiz_complete_count"`
	GuardBuyCount         int    `json:"guard_buy_count"`
	FertilizerBuyCount    int    `json:"fertilizer_buy_count"`
	CDRBuyCount           int    `json:"cdr_buy_count"`
	NewQuizBuyCount       int    `json:"new_quiz_buy_count"`
	GambleWinCount        int    `json:"gamble_win_count"`
	GambleLossCount       int    `json:"gamble_loss_count"`
	GambleWinsTotal       int    `json:"gamble_wins_total"`
	GambleLossesTotal     int    `json:"gamble_losses_total"`
	DuelWinCount          int    `json:"duel_win_count"`
	DuelLossCount         int    `json:"duel_loss_count"`
	DuelWinsAmount        int    `json:"duel_wins_amount"`
	DuelLossesAmount      int    `json:"duel_losses_amount"`
	DuelCaughtLosses      int    `json:"duel_caught_losses"`
	AverageResponseCount  int    `json:"average_response_count"`
}

type PotatoSettings struct {
	NotVerbose bool `json:"not_verbose"`
}

type PotatoData struct {
	Potatoes
	PotatoAnalytics
	PotatoSettings
}

type Redirect struct {
	Key string `json:"key"`
	URL string `json:"url"`
}