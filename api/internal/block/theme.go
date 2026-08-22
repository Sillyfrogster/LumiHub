package block

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// ColorSet is a theme's named colours, kept in the modes its source names.
// Names stay at face value because one theme format knows nothing about
// another format's vocabulary.
type ColorSet struct {
	Modes []ColorMode `json:"modes"`
}

type ColorMode struct {
	Name   string  `json:"name,omitempty"`
	Colors []Color `json:"colors"`
}

type Color struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Value string    `json:"value"`
}

func (s ColorSet) Empty() bool {
	for _, mode := range s.Modes {
		for _, color := range mode.Colors {
			if color.Value != "" {
				return false
			}
		}
	}
	return true
}

// StylesheetSet keeps every stylesheet and the files those stylesheets resolve
// against. A file belongs to this element rather than to a generic file block.
type StylesheetSet struct {
	Global      string            `json:"global"`
	Stylesheets []Stylesheet      `json:"stylesheets"`
	Assets      []StylesheetAsset `json:"assets"`
}

type Stylesheet struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	CSS     string    `json:"css"`
	Enabled bool      `json:"enabled"`
}

type StylesheetAsset struct {
	ID        uuid.UUID `json:"id"`
	Path      string    `json:"path"`
	MediaType string    `json:"mediaType,omitempty"`
	Data      []byte    `json:"data"`
}

func (s StylesheetSet) Empty() bool {
	if s.Global != "" || len(s.Assets) > 0 {
		return false
	}
	for _, sheet := range s.Stylesheets {
		if sheet.CSS != "" {
			return false
		}
	}
	return true
}

func decodeColorSet(raw json.RawMessage) (Content, error) {
	var incoming struct {
		Modes *[]struct {
			Name   string `json:"name,omitempty"`
			Colors *[]struct {
				ID    uuid.UUID `json:"id,omitempty"`
				Name  *string   `json:"name"`
				Value *string   `json:"value"`
			} `json:"colors"`
		} `json:"modes"`
	}
	if err := decodeContentJSON(raw, &incoming); err != nil {
		return nil, err
	}
	if incoming.Modes == nil {
		return nil, fmt.Errorf("modes must be present as a list")
	}
	modes := make([]ColorMode, len(*incoming.Modes))
	for i, mode := range *incoming.Modes {
		if mode.Colors == nil {
			return nil, fmt.Errorf("mode %d must include colours as a list", i+1)
		}
		colors := make([]Color, len(*mode.Colors))
		for j, color := range *mode.Colors {
			if color.Name == nil || color.Value == nil {
				return nil, fmt.Errorf("mode %d colour %d must include name and value as strings", i+1, j+1)
			}
			colors[j] = Color{
				ID: itemID(color.ID), Name: *color.Name, Value: *color.Value,
			}
		}
		modes[i] = ColorMode{Name: mode.Name, Colors: colors}
	}
	return ColorSet{Modes: modes}, nil
}

func decodeStylesheetSet(raw json.RawMessage) (Content, error) {
	var incoming struct {
		Global      *string            `json:"global"`
		Stylesheets *[]Stylesheet      `json:"stylesheets"`
		Assets      *[]StylesheetAsset `json:"assets"`
	}
	if err := decodeContentJSON(raw, &incoming); err != nil {
		return nil, err
	}
	if incoming.Global == nil || incoming.Stylesheets == nil || incoming.Assets == nil {
		return nil, fmt.Errorf("global, stylesheets and assets must be present")
	}
	stylesheets := *incoming.Stylesheets
	for i := range stylesheets {
		stylesheets[i].ID = itemID(stylesheets[i].ID)
	}
	assets := *incoming.Assets
	for i := range assets {
		if assets[i].Path == "" {
			return nil, fmt.Errorf("asset %d must include a path", i+1)
		}
		assets[i].ID = itemID(assets[i].ID)
	}
	return StylesheetSet{
		Global: *incoming.Global, Stylesheets: stylesheets, Assets: assets,
	}, nil
}
