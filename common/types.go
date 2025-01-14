package common

import (
	"encoding/json"
	"time"
)

type Platforms string

const (
	TWITCH  Platforms = "TWITCH"
	DISCORD Platforms = "DISCORD"
	KICK    Platforms = "KICK"
	STV     Platforms = "STV"
)

type PermissionLevel uint8

const (
	DEVELOPER 	PermissionLevel = 4
	ADMIN     	PermissionLevel = 3
	MOD       	PermissionLevel = 2
	USER     		PermissionLevel = 1
	BLACKLISTED	PermissionLevel = 0
)

type User struct {
	FirstSeen   time.Time        	`json:"first_seen"`
	Username    string           	`json:"username"`
	Display     string           	`json:"display"`
	Settings    UserSettings     	`json:"settings"`
	Connections []UserConnection 	`json:"connections,omitempty"`
	ID          int              	`json:"user_id"`
	Level       int              	`json:"level"`
}

type UserSettings struct {
	Language       string `json:"language"`
	IgnoreDropped  bool   `json:"ignore_dropped"`
	ColorResponses bool   `json:"color_responses"`
	NoReply        bool   `json:"no_reply"`
	IsBot          bool   `json:"is_bot"`
	IsSelfBot      bool   `json:"is_selfbot"`
}

type UserMeta = json.RawMessage

type UserConnection struct {
	Meta     UserMeta		`json:"platform_metadata"`
	Platform Platforms 	`json:"platform"`
	Username string    	`json:"platform_username"`
	Display  string    	`json:"platform_display"`
	UserID   string    	`json:"platform_id"`
	PFP      string    	`json:"platform_pfp"`
	ID       int       	`json:"user_id"`
}

type KickChannelMeta struct {
	ChannelID  string `json:"channel_id"`
	ChatroomID string `json:"chatroom_id"`
	UserID     string `json:"user_id"`
}

type TwitchChannelMeta struct {
	BotBanned    bool `json:"bot_banned"`
	TwitchBanned bool `json:"twitch_banned"`
}

type TwitchUserMeta struct {
	Color string      `json:"color,omitempty"`
	Roles TwitchRoles `json:"roles,omitempty"`
}

type StvUserMeta struct {
	PaintID string   `json:"paint_id,omitempty"`
	Roles   []string `json:"roles,omitempty"`
}

type TwitchRoles struct {
	IsStaff     bool `json:"isStaff,omitempty"`
	IsPartner   bool `json:"isPartner,omitempty"`
	IsAffiliate bool `json:"isAffiliate,omitempty"`
}

type Channel struct {
	Blocks      FilteredBlocks         `json:"blocks,omitempty"`
	JoinedAt    *time.Time             `json:"joined_at,omitempty"`
	Meta        map[string]interface{} `json:"meta"`
	Commands    *[]ChannelCommand      `json:"commands,omitempty"`
	ChannelID   string                 `json:"channel_id"`
	Username    string                 `json:"username"`
	Platform    Platforms              `json:"platform"`
	State       string                 `json:"state"`
	AddedBy     []AddedByData          `json:"added_by,omitempty"`
	Editors     []string               `json:"editors"`
	Ambassadors []string               `json:"ambassadors"`
	Settings    ChannelSettings        `json:"settings"`
}

type FilteredBlocks struct {
	Users    *[]Block `json:"users"`
	Commands *[]Block `json:"commands"`
}

type Block struct {
	ChannelID     string    `json:"channel_id"`
	BlockType     BlockType `json:"block_type"`
	CommandName   string    `json:"command_name,omitempty"`
	ID            int       `json:"user_id"`
	BlockedUserID int       `json:"blocked_user_id"`
}

type BlockType string

const (
	UserBlock    BlockType = "USER"
	CommandBlock BlockType = "COMMAND"
	GlobalBlock  BlockType = "GLOBAL"
)

type AddedByData struct {
	AddedAt  time.Time `json:"addedAt"`
	Username string    `json:"username"`
	ID       string    `json:"id"`
}

type ChannelSettings struct {
	PajBot            *string  `json:"paj_bot,omitempty"`
	UserCooldown      *int     `json:"user_cooldown,omitempty"`
	ChannelCooldown   *int     `json:"channel_cooldown,omitempty"`
	Language          string   `json:"language"`
	Permission        string   `json:"permission"`
	Prefix            string   `json:"prefix"`
	UsersBlacklisted  []string `json:"users_blacklisted"`
	NoReply           bool     `json:"no_reply"`
	FirstMsgResponses bool     `json:"first_msg_responses"`
	WhisperOnly       bool     `json:"whisper_only"`
	OfflineOnly       bool     `json:"offline_only"`
	ForceLanguage     bool     `json:"force_language"`
	SilentErrors      bool     `json:"silent_errors"`
	ColorResponses    bool     `json:"color_responses"`
}

type CommandSettings struct {
	ChannelID        string   `json:"channel_id"`
	Command          string   `json:"command"`
	Permission       string   `json:"permission"`
	UsersBlacklisted []string `json:"users_blacklisted"`
	UsersWhitelisted []string `json:"users_whitelisted"`
	CustomCooldown   int      `json:"custom_cooldown"`
	ChannelUsage     int      `json:"channel_usage"`
	IsEnabled        bool     `json:"is_enabled"`
	OfflineOnly      bool     `json:"offline_only"`
	SilentErrors     bool     `json:"silent_errors"`
	AllowBots        bool     `json:"allow_bots"`
}

type PlatformOauth struct {
	AddedAt      time.Time `json:"added_at"`
	PlatformID   string    `json:"platform_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Platform     Platforms `json:"platform"`
	Scope        []string  `json:"scope"`
	ExpiresIn    int       `json:"expires_in"`
}

type ChannelCommand struct {
	Created        time.Time `json:"created"`
	Modified       time.Time `json:"modified"`
	Help           *string   `json:"help"`
	Name           *string   `json:"name"`
	RunCommand     *string   `json:"run_command"`
	ChannelID      string    `json:"channel_id"`
	Trigger        string    `json:"trigger"`
	Response       string    `json:"response"`
	Platform       string    `json:"platform"`
	UserTriggerIDs []string  `json:"user_trigger_ids"`
	UserIgnoreIDs  []string  `json:"user_ignore_ids"`
	Cooldown       int       `json:"cooldown"`
	UserID         int       `json:"user_id"`
	Delay          int       `json:"delay"`
	UseCount       int       `json:"use_count"`
	CommandID      int       `json:"command_id"`
	Reply          bool      `json:"reply"`
	Whisper        bool      `json:"whisper"`
	Announce       bool      `json:"announce"`
	ActiveOffline  bool      `json:"active_offline"`
	ActiveOnline   bool      `json:"active_online"`
	Active         bool      `json:"active"`
}

type Potatoes struct {
	StoleFrom      *string `json:"stole_from"`
	StoleAmount    *int    `json:"stole_amount"`
	TrampledBy     *string `json:"trampled_by"`
	FirstSeen      string  `json:"first_seen"`
	ID             int     `json:"user_id"`
	PotatoCount    int     `json:"potato_count"`
	PotatoPrestige int     `json:"potato_prestige"`
	PotatoRank     int     `json:"potato_rank"`
	TaxMultiplier  int     `json:"tax_multiplier"`
}

type PotatoAnalytics struct {
	AverageResponseTime  string `json:"average_response_time"`
	EatCount             int    `json:"eat_count"`
	HarvestCount         int    `json:"harvest_count"`
	StolenCount          int    `json:"stolen_count"`
	TheftCount           int    `json:"theft_count"`
	TrampledCount        int    `json:"trampled_count"`
	TrampleCount         int    `json:"trample_count"`
	CDRCount             int    `json:"cdr_count"`
	QuizCount            int    `json:"quiz_count"`
	QuizCompleteCount    int    `json:"quiz_complete_count"`
	GuardBuyCount        int    `json:"guard_buy_count"`
	FertilizerBuyCount   int    `json:"fertilizer_buy_count"`
	CDRBuyCount          int    `json:"cdr_buy_count"`
	NewQuizBuyCount      int    `json:"new_quiz_buy_count"`
	GambleWinCount       int    `json:"gamble_win_count"`
	GambleLossCount      int    `json:"gamble_loss_count"`
	GambleWinsTotal      int    `json:"gamble_wins_total"`
	GambleLossesTotal    int    `json:"gamble_losses_total"`
	DuelWinCount         int    `json:"duel_win_count"`
	DuelLossCount        int    `json:"duel_loss_count"`
	DuelWinsAmount       int    `json:"duel_wins_amount"`
	DuelLossesAmount     int    `json:"duel_losses_amount"`
	DuelCaughtLosses     int    `json:"duel_caught_losses"`
	AverageResponseCount int    `json:"average_response_count"`
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

type ErrorMessage struct {
	Message string `json:"message"`
}

type GenericResponse[T any] struct {
	Data   *[]T   `json:"data"`
	Errors *[]ErrorMessage `json:"errors,omitempty"`
}

type TwitchValidation struct {
	ClientID   string   `json:"client_id"`
	Login      string   `json:"login"`
	Scopes     []string `json:"scopes"`
	UserID     string   `json:"user_id"`
	ExpiresIn  int      `json:"expires_in"`
	StatusCode int      `json:"status_code"`
}

type GenericOAUTHResponse struct {
	AccessToken  string 	`json:"access_token"`
	RefreshToken string 	`json:"refresh_token"`
	Scope        []string `json:"scope"`
	ExpiresIn    int    	`json:"expires_in"`
	TokenType    string 	`json:"token_type"`
}
