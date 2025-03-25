// Package common provides common types and structures used across the application.
package common

import (
	"encoding/json"
	"time"
)

// Platforms represents the different platforms supported by the bot.
type Platforms string

//nolint:revive
const (
	TWITCH  Platforms = "TWITCH"
	DISCORD Platforms = "DISCORD"
	KICK    Platforms = "KICK"
	STV     Platforms = "STV"
)

// PermissionLevel represents the permission level of a user interally with the api and bot.
type PermissionLevel uint8

//nolint:revive
const (
	DEVELOPER   PermissionLevel = 4
	ADMIN       PermissionLevel = 3
	MOD         PermissionLevel = 2
	USER        PermissionLevel = 1
	BLACKLISTED PermissionLevel = 0
)

// User represents a user on a platform, including their first seen date, username, display name,.
type User struct {
	FirstSeen   time.Time        `json:"first_seen"`
	Username    string           `json:"username"`
	Display     string           `json:"display"`
	Settings    UserSettings     `json:"settings"`
	Connections []UserConnection `json:"connections,omitempty"`
	ID          int              `json:"user_id"`
	Level       int              `json:"level"`
}

// UserSettings represents the settings for a user on a platform, including language preferences,.
type UserSettings struct {
	Language       string `json:"language"`
	IgnoreDropped  bool   `json:"ignore_dropped"`
	ColorResponses bool   `json:"color_responses"`
	NoReply        bool   `json:"no_reply"`
	IsBot          bool   `json:"is_bot"`
	IsSelfBot      bool   `json:"is_selfbot"`
}

// UserMeta represents the metadata associated with a user on a platform.
type UserMeta = json.RawMessage

// UserConnection represents a connection between a user and a platform.
type UserConnection struct {
	Platform Platforms `json:"platform"`
	Username string    `json:"platform_username"`
	Display  string    `json:"platform_display"`
	UserID   string    `json:"platform_id"`
	PFP      string    `json:"platform_pfp"`
	Meta     UserMeta  `json:"platform_metadata"`
	ID       int       `json:"user_id"`
}

// KickChannelMeta represents the metadata for a channel on Kick, including the channel ID, chatroom ID, and user ID.
type KickChannelMeta struct {
	ChannelID  string `json:"channel_id"`
	ChatroomID string `json:"chatroom_id"`
	UserID     string `json:"user_id"`
}

// TwitchChannelMeta represents the metadata for a channel on Twitch, including whether the bot and channel are banned.
type TwitchChannelMeta struct {
	BotBanned    bool `json:"bot_banned"`
	TwitchBanned bool `json:"twitch_banned"`
}

// TwitchUserMeta represents the metadata for a user on Twitch, including their color and roles.
type TwitchUserMeta struct {
	Color string      `json:"color,omitempty"`
	Roles TwitchRoles `json:"roles,omitempty"`
}

// StvUserMeta represents the metadata for a user on 7TV, including their paint ID and roles.
type StvUserMeta struct {
	PaintID string   `json:"paint_id,omitempty"`
	Roles   []string `json:"roles,omitempty"`
}

// TwitchRoles represents the roles a user can have on Twitch, such as staff, partner, or affiliate.
type TwitchRoles struct {
	IsStaff     bool `json:"isStaff,omitempty"`
	IsPartner   bool `json:"isPartner,omitempty"`
	IsAffiliate bool `json:"isAffiliate,omitempty"`
}

// Channel represents a channel on a platform, including its blocks, settings, commands, and other metadata.
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

// FilteredBlocks represents the blocks of users and commands in a channel.
type FilteredBlocks struct {
	Users    *[]Block `json:"users"`
	Commands *[]Block `json:"commands"`
}

// Block represents a block connection between a user and a command or another user.
type Block struct {
	ChannelID     string    `json:"channel_id"`
	BlockType     BlockType `json:"block_type"`
	CommandName   string    `json:"command_name,omitempty"`
	ID            int       `json:"user_id"`
	BlockedUserID int       `json:"blocked_user_id"`
}

// BlockType represents the type of block connection that can be made between a user or a command.
type BlockType string

//nolint:revive
const (
	UserBlock    BlockType = "USER"
	CommandBlock BlockType = "COMMAND"
	GlobalBlock  BlockType = "GLOBAL"
)

// AddedByData represents the data of a user who added a channel.
type AddedByData struct {
	AddedAt  time.Time `json:"addedAt"`
	Username string    `json:"username"`
	ID       string    `json:"id"`
}

// ChannelSettings represents the settings for a channel, including bot settings, cooldowns, language,
// permission levels, and other configurations.
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

// CommandSettings represents the settings for a command in a channel, including its permissions, cooldowns,
// and usage limits.
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

// PlatformOauth represents the OAuth token and metadata for a user on a specific platform, including.
type PlatformOauth struct {
	AddedAt      time.Time `json:"added_at"`
	PlatformID   string    `json:"platform_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Platform     Platforms `json:"platform"`
	Scope        []string  `json:"scope"`
	ExpiresIn    int       `json:"expires_in"`
}

// ChannelCommand represents a single custom command, including its properties and settings.
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

// Potatoes represents the structure of potato-related data for a user.
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

// PotatoAnalytics contains various statistics related to potato interactions.
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

// PotatoSettings represents the settings related to potato interactions, such as verbosity.
type PotatoSettings struct {
	NotVerbose bool `json:"not_verbose"`
}

// PotatoData combines the Potatoes, PotatoAnalytics, and PotatoSettings structures into a single structure.
type PotatoData struct {
	Potatoes
	PotatoAnalytics
	PotatoSettings
}

// Redirect represents a URL redirect structure, typically used for OAuth flows.
type Redirect struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// ErrorMessage represents a structure for error messages returned in API responses.
type ErrorMessage struct {
	Message string `json:"message"`
}

// GenericResponse represents a generic API response structure, which can include data and errors.
type GenericResponse[T any] struct {
	Data   *[]T            `json:"data"`
	Errors *[]ErrorMessage `json:"errors,omitempty"`
}

// TwitchValidation represents the structure of a Twitch OAuth validation response.
type TwitchValidation struct {
	ClientID   string   `json:"client_id"`
	Login      string   `json:"login"`
	UserID     string   `json:"user_id"`
	Scopes     []string `json:"scopes"`
	ExpiresIn  int      `json:"expires_in"`
	StatusCode int      `json:"status_code"`
}

// GenericOAUTHResponse represents a generic OAuth response structure.
type GenericOAUTHResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	TokenType    string   `json:"token_type"`
	Scope        []string `json:"scope"`
	ExpiresIn    int      `json:"expires_in"`
}

// Command represents chat command, with its permissions, description, and other details.
type Command struct {
	Conditions          CommandConditions      `json:"conditions"`
	BotRequires         BotCommandRequirements `json:"botRequires"`
	Description         string                 `json:"description"`
	DetailedDescription string                 `json:"detailedDescription,omitempty"`
	Title               string                 `json:"title"`
	Usage               string                 `json:"usage"`
	Category            CommandCategories      `json:"category"`
	Name                string                 `json:"name"`
	UserRequires        UserRequires           `json:"userRequires"`
	Aliases             []string               `json:"aliases"`
	Flags               []FlagDetails          `json:"flags"`
	Cooldown            int                    `json:"cooldown"`
	Level               PermissionLevel        `json:"level"`
}

// CommandCategories represents the category of a command.
type CommandCategories string

//nolint:revive
const (
	Development CommandCategories = "development"
	Deprecated  CommandCategories = "deprecated"
	Moderation  CommandCategories = "moderation"
	Utilities   CommandCategories = "utilities"
	Unlisted    CommandCategories = "unlisted"
	Settings    CommandCategories = "settings"
	Stream      CommandCategories = "stream"
	Potato      CommandCategories = "potato"
	Emotes      CommandCategories = "emotes"
	Anime       CommandCategories = "anime"
	Music       CommandCategories = "music"
	Spam        CommandCategories = "spam"
	Misc        CommandCategories = "misc"
	Fun         CommandCategories = "fun"
)

// FlagDetails represents the details of a command flag, including its requirements, usage, and validation function.
type FlagDetails struct {
	UserRequires *UserRequires                                                 `json:"user_requires,omitempty"`
	Usage        *string                                                       `json:"usage,omitempty"`
	Multi        *bool                                                         `json:"multi,omitempty"`
	Check        func(params Flags, flag FlagDetails) (FlagCheckResult, error) `json:"-"`
	Name         string                                                        `json:"name"`
	Type         string                                                        `json:"type"`
	Description  string                                                        `json:"description"`
	Aliases      []string                                                      `json:"aliases,omitempty"`
	Level        PermissionLevel                                               `json:"level"`
	Required     bool                                                          `json:"required"`
}

// FlagCheckResult represents the result of a flag check, indicating whether the flag is valid,
// and any associated error or requirement.
type FlagCheckResult struct {
	MustBe *string `json:"must_be,omitempty"`
	Error  *string `json:"error,omitempty"`
	Valid  bool    `json:"valid"`
}

// UserRequires represents the required user permission level for a command or flag.
type UserRequires string

//nolint:revive
const (
	None        UserRequires = "NONE"
	Subscriber  UserRequires = "SUBSCRIBER"
	VIP         UserRequires = "VIP"
	Mod         UserRequires = "MOD"
	Ambassador  UserRequires = "AMBASSADOR"
	Broadcaster UserRequires = "BROADCASTER"
)

// Flags represents a map of flags, where each flag is identified by a string key and can hold any type of value.
type Flags map[string]interface{}

// CommandConditions represents the conditions under which a command can be executed.
type CommandConditions struct {
	Ryan         *bool `json:"ryan,omitempty"`
	OfflineOnly  *bool `json:"offlineOnly,omitempty"`
	Whisperable  *bool `json:"whisperable,omitempty"`
	IgnoreBots   *bool `json:"ignoreBots,omitempty"`
	IsBlockable  *bool `json:"isBlockable,omitempty"`
	IsNotPipable *bool `json:"isNotPipable,omitempty"`
}

// BotCommandRequirements represents the requirements for a bot to execute a command,.
type BotCommandRequirements string

//nolint:revive
const (
	BotNone BotCommandRequirements = "NONE"
	BotVIP  BotCommandRequirements = "VIP"
	BotMod  BotCommandRequirements = "MOD"
)
