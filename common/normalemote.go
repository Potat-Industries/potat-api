package common

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	errInvalidEmoteData     = errors.New("invalid emote data")
	errInvalidEmoteProvider = errors.New("invalid emote provider")
	errNoSetID              = errors.New("set id is required")
	errInvalidStvEmote      = errors.New("invalid 7TV emote data")
	errInvalidBttvEmote     = errors.New("invalid BTTV emote data")
	errInvalidFfzEmote      = errors.New("invalid FFZ emote data")
	errInvalidTwitchEmote   = errors.New("invalid Twitch emote data")
	errMissingIDOrName      = errors.New("emote is missing id or name")
)

// EmoteType represents the type of emote, such as standard, global, or channel-specific.
type EmoteType string

//nolint:revive
const (
	EmoteTypeChannel   EmoteType = "CHANNEL"
	EmoteTypeGlobal    EmoteType = "GLOBAL"
	EmoteTypeStandard  EmoteType = "STANDARD"
	EmoteTypeZeroWidth EmoteType = "ZERO_WIDTH"
	EmoteTypeModifier  EmoteType = "MODIFIER"
	EmoteTypeEmoji     EmoteType = "EMOJI"
)

// ConflictType represents the type of possible conflict on emote actions.
type ConflictType string

//nolint:revive
const (
	ConflictTypeExact            ConflictType = "exact"
	ConflictTypeID               ConflictType = "id"
	ConflictTypeName             ConflictType = "name"
	ConflictTypeCloseMatchAdd    ConflictType = "close-match-add"
	ConflictTypeCloseMatchRemove ConflictType = "close-match-remove"
	ConflictTypeAlias            ConflictType = "alias"
)

// Conflict represents the conflicted emote and the type of conflict.
type Conflict struct {
	Emote *NormalEmote `json:"emote"`
	Type  ConflictType `json:"type"`
}

// NormalEmote represents a normal emote with its properties and potential conflicts.
type NormalEmote struct {
	Conflict *Conflict `json:"conflict,omitempty"`
	Alias    *string   `json:"alias,omitempty"`
	NewAlias *string   `json:"new_alias,omitempty"`
	URL      *string   `json:"url,omitempty"`
	Error    *string   `json:"error,omitempty"`
	Count    *int      `json:"count,omitempty"`
	Name     string    `json:"name"`
	ID       string    `json:"id"`
	SetID    string    `json:"set_id"`
	Provider Platforms `json:"provider"`
	Type     EmoteType `json:"type"`
	Platform Platforms `json:"platform"`
	Animated bool      `json:"animated"`
}

// StvEmote represents a 7TV emote with its properties.
type StvEmote struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Data struct {
		Name     string `json:"name"`
		ID       string `json:"id"`
		Animated bool   `json:"animated"`
	} `json:"data"`
}

// BttvEmote represents a BTTV emote with its properties.
type BttvEmote struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	CodeOriginal string `json:"codeOriginal"`
	Animated     bool   `json:"animated"`
}

// FfzEmote represents a FFZ emote with its properties.
type FfzEmote struct {
	Name     string `json:"name"`
	ID       int    `json:"id"`
	Animated bool   `json:"animated"`
}

// TwitchEmote represents a Twitch emote with its properties.
type TwitchEmote struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Images struct {
		URL4X string `json:"url_4x"`
	} `json:"images"`
	Format []string `json:"format"`
}

// NewNormalEmote returns a normalized emote object based on the provided emote data and platform.
func NewNormalEmote( //nolint:cyclop
	emoteData interface{},
	setID string,
	provider Platforms,
	emoteType EmoteType,
	animated bool,
	platform Platforms,
) (*NormalEmote, error) {
	if setID == "" {
		emote, ok := emoteData.(NormalEmote)
		if !ok {
			return nil, errInvalidEmoteData
		}

		if emote.SetID == "" {
			return nil, errNoSetID
		}

		setID = emote.SetID
	}

	emote := &NormalEmote{
		SetID:    setID,
		Provider: provider,
		Type:     emoteType,
		Animated: animated,
		Platform: platform,
	}

	switch provider {
	case "STV":
		data, ok := emoteData.(StvEmote)
		if !ok {
			return nil, errInvalidStvEmote
		}
		emote.ID = data.ID
		emote.Name = data.Data.Name
		if data.Name != data.Data.Name {
			emote.Alias = &data.Name
		}
		emote.Animated = data.Data.Animated
		url, err := EmoteIDToURL(data.ID, provider, 4)
		if err != nil {
			return nil, err
		}
		emote.URL = &url

	case "BTTV":
		data, ok := emoteData.(BttvEmote)
		if !ok {
			return nil, errInvalidBttvEmote
		}
		emote.ID = data.ID
		emote.Name = data.CodeOriginal
		if data.CodeOriginal != data.Code {
			emote.Alias = &data.Code
		}
		emote.Animated = data.Animated
		url, err := EmoteIDToURL(data.ID, provider, 4)
		if err != nil {
			return nil, err
		}
		emote.URL = &url

	case "FFZ":
		data, ok := emoteData.(FfzEmote)
		if !ok {
			return nil, errInvalidFfzEmote
		}
		emote.ID = fmt.Sprintf("%d", data.ID)
		emote.Name = data.Name
		emote.Animated = data.Animated
		url, err := EmoteIDToURL(strconv.Itoa(data.ID), provider, 4)
		if err != nil {
			return nil, err
		}
		emote.URL = &url

	case "TWITCH":
		data, ok := emoteData.(TwitchEmote)
		if !ok {
			return nil, errInvalidTwitchEmote
		}
		emote.ID = data.ID
		emote.Name = data.Name
		emote.Animated = slices.Contains(data.Format, "animated")
		emote.URL = &data.Images.URL4X

	default:
		return nil, errInvalidEmoteProvider
	}

	if emote.ID == "" || emote.Name == "" {
		return nil, errMissingIDOrName
	}

	return emote, nil
}

// IsObjectID checks if the provided string is a valid MongoDB ObjectId.
func IsObjectID(id string) bool {
	return regexp.MustCompile(`^[0-9a-fA-F]{24}$`).MatchString(id)
}

// IsULID checks if the provided string is a valid ULID (Universally Unique Lexicographically Sortable Identifier).
func IsULID(id string) bool {
	return regexp.MustCompile(`^[0-9A-HJ-NP-Z]{26}$`).MatchString(id)
}

// EmoteIDToURL generates the URL for an emote based on its ID, provider, and size.
func EmoteIDToURL(id string, provider Platforms, size uint8) (string, error) {
	if size < 1 {
		size = 1
	} else if size > 4 {
		size = 4
	}

	providerStr := strings.ToLower(string(provider))

	switch providerStr {
	case "ffz":
		return fmt.Sprintf("https://cdn.frankerfacez.com/emote/%s/%d", id, size), nil
	case "7tv", "stv":
		return fmt.Sprintf("https://cdn.7tv.app/emote/%s/%dx.avif", id, size), nil
	case "bttv":
		return fmt.Sprintf("https://cdn.betterttv.net/emote/%s/%dx", id, size), nil
	case "twitch":
		return fmt.Sprintf("https://static-cdn.jtvnw.net/emoticons/v2/%s/default/dark/%d.0", id, size), nil
	case "discord":
		return fmt.Sprintf("https://cdn.discordapp.com/emojis/%s.webp?size=96&animated=true", id), nil
	case "kick":
		return fmt.Sprintf("https://files.kick.com/emotes/%s/fullsize", id), nil
	default:
		return "", errInvalidEmoteProvider
	}
}
