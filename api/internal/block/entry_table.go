package block

import (
	"encoding/json"
	"fmt"
)

// EntryTable is a lorebook. A model reads an entry once one of its key words
// turns up in the conversation.
//
// It stays its own element type rather than a general table because key
// matching and recursion are what a creator edits here.
type EntryTable struct {
	Entries []Entry `json:"entries"`
}

// Entry is one lorebook entry.
type Entry struct {
	// Name is the creator's own label. It reaches no model.
	Name string `json:"name,omitempty"`
	// Keys are the words that switch the entry on.
	Keys []string `json:"keys"`
	// SecondaryKeys narrow the match. With Selective set, one of these has to
	// appear as well as one of the keys.
	SecondaryKeys []string `json:"secondaryKeys,omitempty"`
	Selective     bool     `json:"selective,omitempty"`
	CaseSensitive bool     `json:"caseSensitive,omitempty"`
	// Constant entries are on whatever the conversation says.
	Constant bool `json:"constant,omitempty"`
	// Enabled is the creator's switch. A switched-off entry stays in the book.
	Enabled bool `json:"enabled"`
	// Order places the entry among the entries that fired with it.
	Order int `json:"order"`
	// Position is where the entry's text goes relative to the character, and
	// is unset where the book leaves it to whatever reads it.
	Position EntryPosition `json:"position,omitempty"`
	// Recursion is how the entry behaves on the passes after the first.
	Recursion EntryRecursion `json:"recursion"`
	Text      string         `json:"text"`
}

// EntryPosition is where an entry's text is inserted.
type EntryPosition string

const (
	BeforeCharacter EntryPosition = "before_character"
	AfterCharacter  EntryPosition = "after_character"
)

// Known reports whether the position belongs to the closed vocabulary. An
// unset position is the book leaving the choice open, so it is known too.
func (p EntryPosition) Known() bool {
	return p == "" || p == BeforeCharacter || p == AfterCharacter
}

// EntryPositions returns the positions an entry may take.
func EntryPositions() []EntryPosition {
	return []EntryPosition{BeforeCharacter, AfterCharacter}
}

// EntryRecursion is how an entry takes part in the passes after the first.
type EntryRecursion struct {
	// Exclude keeps this entry's text from switching other entries on.
	Exclude bool `json:"exclude,omitempty"`
	// Prevent keeps other entries' text from switching this one on.
	Prevent bool `json:"prevent,omitempty"`
	// DelayUntil holds this entry back until a later pass.
	DelayUntil bool `json:"delayUntil,omitempty"`
}

// Empty reports whether the book would show a reader nothing. An entry with
// keys and no text is a match that produces nothing, so it counts for nothing.
func (t EntryTable) Empty() bool {
	for _, entry := range t.Entries {
		if entry.Text != "" {
			return false
		}
	}
	return true
}

// decodeEntryTable reads a save request into a book. Every entry is read on
// its own terms, so a refusal names the entry a creator has to go back to.
func decodeEntryTable(raw json.RawMessage) (Content, error) {
	var incoming struct {
		Entries *[]struct {
			Name          string         `json:"name,omitempty"`
			Keys          *[]string      `json:"keys"`
			SecondaryKeys []string       `json:"secondaryKeys,omitempty"`
			Selective     bool           `json:"selective,omitempty"`
			CaseSensitive bool           `json:"caseSensitive,omitempty"`
			Constant      bool           `json:"constant,omitempty"`
			Enabled       *bool          `json:"enabled"`
			Order         int            `json:"order"`
			Position      EntryPosition  `json:"position,omitempty"`
			Recursion     EntryRecursion `json:"recursion"`
			Text          *string        `json:"text"`
		} `json:"entries"`
	}
	if err := decodeContentJSON(raw, &incoming); err != nil {
		return nil, err
	}
	if incoming.Entries == nil {
		return nil, fmt.Errorf("entries must be present as a list")
	}
	entries := make([]Entry, len(*incoming.Entries))
	for i, item := range *incoming.Entries {
		if item.Keys == nil || item.Text == nil || item.Enabled == nil {
			return nil, fmt.Errorf(
				"entry %d must include keys as a list, text as a string and enabled as a yes or no",
				i+1,
			)
		}
		if !item.Position.Known() {
			return nil, fmt.Errorf(
				"entry %d sits at %q. Choose before or after the character before saving",
				i+1, item.Position,
			)
		}
		entries[i] = Entry{
			Name:          item.Name,
			Keys:          *item.Keys,
			SecondaryKeys: item.SecondaryKeys,
			Selective:     item.Selective,
			CaseSensitive: item.CaseSensitive,
			Constant:      item.Constant,
			Enabled:       *item.Enabled,
			Order:         item.Order,
			Position:      item.Position,
			Recursion:     item.Recursion,
			Text:          *item.Text,
		}
	}
	return EntryTable{Entries: entries}, nil
}
