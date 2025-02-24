package common

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type EmoteType string

const (
	EmoteTypeChannel     EmoteType = "CHANNEL"
	EmoteTypeGlobal      EmoteType = "GLOBAL"
	EmoteTypeStandard    EmoteType = "STANDARD"
	EmoteTypeZeroWidth   EmoteType = "ZERO_WIDTH"
	EmoteTypeModifier    EmoteType = "MODIFIER"
	EmoteTypeEmoji       EmoteType = "EMOJI"
)

type ConflictType string

const (
	ConflictTypeExact          ConflictType = "exact"
	ConflictTypeID             ConflictType = "id"
	ConflictTypeName           ConflictType = "name"
	ConflictTypeCloseMatchAdd  ConflictType = "close-match-add"
	ConflictTypeCloseMatchRemove ConflictType = "close-match-remove"
	ConflictTypeAlias          ConflictType = "alias"
)

type Conflict struct {
	Type  ConflictType `json:"type"`
	Emote *NormalEmote `json:"emote"`
}

type NormalEmote struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Alias      *string   `json:"alias,omitempty"`
	NewAlias   *string   `json:"new_alias,omitempty"`
	URL        *string   `json:"url,omitempty"`
	Error      *string   `json:"error,omitempty"`
	Conflict   *Conflict `json:"conflict,omitempty"`
	Count      *int      `json:"count,omitempty"`
	SetID      string    `json:"set_id"`
	Provider   Platforms `json:"provider"`
	Type       EmoteType `json:"type"`
	Animated   bool      `json:"animated"`
	Platform   Platforms `json:"platform"`
}

type StvEmote struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Data struct {
		Name    	string `json:"name"`
		Animated	bool   `json:"animated"`
		ID   			string `json:"id"`
	} `json:"data"`
}

type BttvEmote struct {
	ID          	string `json:"id"`
	Code        	string `json:"code"`
	CodeOriginal	string `json:"codeOriginal"`
	Animated    	bool   `json:"animated"`
}

type FfzEmote struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Animated bool   `json:"animated"`
}

type TwitchEmote struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Format []string `json:"format"`
	Images struct {
		URL4X string `json:"url_4x"`
	} `json:"images"`
}

func NewNormalEmote(
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
			return nil, errors.New("invalid data for normal emote")
		}

		if emote.SetID == "" {
			return nil, errors.New("set id is required")
		} else {
			setID = emote.SetID
		}
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
			return nil, errors.New("invalid data for STV emote")
		}
		emote.ID = data.ID
		emote.Name = data.Data.Name
		if data.Name != data.Data.Name {
			emote.Alias = &data.Name
		}
		emote.Animated = data.Data.Animated
		url, err := EmoteIdToURL(data.ID, provider, 4)
		if err != nil {
			return nil, err
		}
		emote.URL = &url

	case "BTTV":
		data, ok := emoteData.(BttvEmote)
		if !ok {
			return nil, errors.New("invalid data for BTTV emote")
		}
		emote.ID = data.ID
		emote.Name = data.CodeOriginal
		if data.CodeOriginal != data.Code {
			emote.Alias = &data.Code
		}
		emote.Animated = data.Animated
		url, err := EmoteIdToURL(data.ID, provider, 4)
		if err != nil {
			return nil, err
		}
		emote.URL = &url

	case "FFZ":
		data, ok := emoteData.(FfzEmote)
		if !ok {
			return nil, errors.New("invalid data for FFZ emote")
		}
		emote.ID = fmt.Sprintf("%d", data.ID)
		emote.Name = data.Name
		emote.Animated = data.Animated
		url, err := EmoteIdToURL(strconv.Itoa(data.ID), provider, 4)
		if err != nil {
			return nil, err
		}
		emote.URL = &url

	case "TWITCH":
		data, ok := emoteData.(TwitchEmote)
		if !ok {
			return nil, errors.New("invalid data for Twitch emote")
		}
		emote.ID = data.ID
		emote.Name = data.Name
		emote.Animated = slices.Contains(data.Format, "animated")
		emote.URL = &data.Images.URL4X

	default:
		return nil, errors.New("invalid normal emote provider provided")
	}

	if emote.ID == "" || emote.Name == "" {
		return nil, errors.New("emote is missing id or name")
	}

	return emote, nil
}


func IsObjectId(id string) bool {
	return regexp.MustCompile(`^[0-9a-fA-F]{24}$`).MatchString(id)
}

func IsULID(id string) bool {
	return regexp.MustCompile(`^[0-9A-HJ-NP-Z]{26}$`).MatchString(id)
}

func EmoteIdToURL(id string, provider Platforms, size uint8) (string, error) {
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
			return "", fmt.Errorf("emote provider not found: %s", provider)
		}
}
