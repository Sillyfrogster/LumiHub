package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	characterformat "github.com/Sillyfrogster/Illarin/api/internal/format/character"
	v1 "github.com/Sillyfrogster/Illarin/api/internal/format/v1"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/google/uuid"
)

// recoveryChunk identifies the card shape eligible for greeting recovery.
const recoveryChunk = "ccv3"

// readVerifiedRecoveries returns the greetings a surviving card may put back into a row that has none.
func readVerifiedRecoveries(characters []v1.Row, backup *FileBackup) (v1.RecoveryAllowlist, error) {
	wanted := newBackupIndex()
	for at, source := range characters {
		row := source.(v1.CharacterRow)
		if row.Common.ImagePath != "" {
			wanted.want(row.Common.ImagePath, at)
		}
	}
	allowlist := make(v1.RecoveryAllowlist)
	err := backup.each(func(entry backupEntry) error {
		if !strings.EqualFold(path.Ext(entry.Name), ".png") {
			return nil
		}
		at, found := wanted.find(entry.Name)
		if !found {
			return nil
		}
		body, err := readBounded(entry.Body, block.MaxPayloadBytes)
		if err != nil {
			return fmt.Errorf("read a surviving v1 character card: %w", err)
		}
		row := characters[at].(v1.CharacterRow)
		recovery, recovered, err := recoveredGreeting(body, row)
		if err != nil || !recovered {
			return err
		}
		allowlist[row.Common.ID] = recovery
		return nil
	})
	if err != nil {
		return nil, err
	}
	return allowlist, nil
}

// recoveredGreeting reports the greeting the allowlist rule permits.
func recoveredGreeting(
	body []byte,
	row v1.CharacterRow,
) (v1.CharacterRecovery, bool, error) {
	inspection, err := probe.Inspect(
		context.Background(), cardBytes(body), uuid.New(), int64(len(body)), "card.png",
	)
	if err != nil {
		return v1.CharacterRecovery{}, false, fmt.Errorf("inspect a surviving v1 card: %w", err)
	}
	if !carriesRecoveryChunk(inspection) {
		return v1.CharacterRecovery{}, false, nil
	}
	held, err := rowAlternateGreetings(row)
	if err != nil {
		return v1.CharacterRecovery{}, false, err
	}
	if held != 0 {
		return v1.CharacterRecovery{}, false, nil
	}
	alternates, err := cardAlternateGreetings(inspection)
	if err != nil {
		return v1.CharacterRecovery{}, false, err
	}
	if len(alternates) != 1 {
		return v1.CharacterRecovery{}, false, nil
	}
	return v1.CharacterRecovery{AlternateGreeting: alternates[0]}, true, nil
}

func carriesRecoveryChunk(inspection probe.Inspection) bool {
	for _, payload := range inspection.Payloads {
		spec, _ := payload.String("spec")
		if payload.Locator.Name == recoveryChunk && spec == characterformat.V2 {
			return true
		}
	}
	return false
}

func rowAlternateGreetings(row v1.CharacterRow) (int, error) {
	if len(row.AlternateGreetings) == 0 || string(row.AlternateGreetings) == "null" {
		return 0, nil
	}
	var held []string
	if err := json.Unmarshal(row.AlternateGreetings, &held); err != nil {
		return 0, fmt.Errorf("read a v1 row's alternate greetings: %w", err)
	}
	return len(held), nil
}

func cardAlternateGreetings(inspection probe.Inspection) ([]string, error) {
	readers := []format.Reader{characterformat.CCv3Module{}, characterformat.CCv2Module{}}
	for _, reader := range readers {
		claim, claimed := reader.Claim(inspection)
		if !claimed {
			continue
		}
		parsed, err := reader.Parse(context.Background(), inspection, claim)
		if err != nil {
			return nil, fmt.Errorf("read a surviving v1 card: %w", err)
		}
		for _, element := range parsed.Elements {
			if element.Role != block.RoleGreetings {
				continue
			}
			texts := element.Content.(block.TextSet).Texts
			if len(texts) < 2 {
				return nil, nil
			}
			alternates := make([]string, 0, len(texts)-1)
			for _, item := range texts[1:] {
				alternates = append(alternates, item.Text)
			}
			return alternates, nil
		}
		return nil, nil
	}
	return nil, nil
}

type cardBytes []byte

func (card cardBytes) ReadRange(
	_ context.Context,
	_ uuid.UUID,
	offset int64,
	length int64,
) (io.ReadCloser, error) {
	if offset < 0 || length < 0 || offset+length > int64(len(card)) {
		return nil, errors.New("range outside the card")
	}
	return io.NopCloser(bytes.NewReader(card[offset : offset+length])), nil
}
