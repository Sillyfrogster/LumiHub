package v1

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/theme"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/google/uuid"
)

func readTheme(ctx context.Context, row ThemeRow) (readResult, error) {
	var config map[string]json.RawMessage
	if json.Unmarshal(row.Config, &config) != nil {
		return readResult{}, fmt.Errorf("v1 theme config: expected an object")
	}
	root := map[string]json.RawMessage{
		"format":      mustJSON(3),
		"name":        mustJSON(row.Common.Name),
		"description": mustJSON(row.Common.Description),
		"theme":       mustJSON(config),
		"globalCSS":   mustJSON(row.CustomCSS),
	}
	parsed, err := parseRoot(ctx, theme.LumiverseModule{}, probe.ZIP, root, int64(len(row.Bundle)))
	if err != nil {
		return readResult{}, fmt.Errorf("v1 theme: %w", err)
	}
	fonts, err := readThemeFonts(row.Bundle)
	if err != nil {
		return readResult{}, err
	}
	attachThemeFonts(&parsed, fonts)
	parsed.Format = ID
	parsed.Tags = append([]string(nil), row.Common.Tags...)
	answer := row.Common.IsNSFW
	created := row.Common.CreatedAt
	parsed.IsNSFW = &answer
	parsed.CreatedAt = &created
	parsed.Header.Name = row.Common.Name
	parsed.Header.CreditedAuthor = ""
	if len([]rune(row.Common.Description)) <= format.MaxBlurbRunes {
		parsed.Header.Blurb = row.Common.Description
	} else {
		parsed.Header.Blurb = ""
		parsed.Elements = append(parsed.Elements, block.Element{
			ID: uuid.New(), Type: block.TypeProse,
			Content: block.Prose{Text: row.Common.Description},
		})
	}
	parsed.Remainder = stripRemainderKeys(parsed.Remainder, theme.LumiverseID, "name", "description")
	events := make([]Event, 0, 1)
	if _, present := config["statusColors"]; !present {
		events = append(events, Event{Kind: MissingThemeStatusColors, Count: 1})
	}
	return readResult{parsed: parsed, events: events}, nil
}

func readThemeFonts(body []byte) ([]block.StylesheetAsset, error) {
	if len(body) == 0 {
		return nil, nil
	}
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("v1 theme bundle: %w", err)
	}
	fonts := make([]block.StylesheetAsset, 0)
	for _, entry := range archive.File {
		mediaType := fontMediaType(entry.Name)
		if entry.FileInfo().IsDir() || mediaType == "" {
			continue
		}
		clean := path.Clean(entry.Name)
		if clean != entry.Name || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
			return nil, fmt.Errorf("v1 theme font path %q is not safe", entry.Name)
		}
		opened, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("open v1 theme font %q: %w", entry.Name, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(opened, int64(block.MaxItemBytes)+1))
		closeErr := opened.Close()
		if readErr != nil || closeErr != nil {
			return nil, fmt.Errorf("read v1 theme font %q", entry.Name)
		}
		if len(data) > block.MaxItemBytes {
			return nil, format.LimitExceeded(fmt.Errorf("v1 theme font %q exceeds %d bytes", entry.Name, block.MaxItemBytes))
		}
		fonts = append(fonts, block.StylesheetAsset{
			ID: block.NewItemID(), Path: entry.Name, MediaType: mediaType, Data: data,
		})
	}
	return fonts, nil
}

func fontMediaType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	default:
		return ""
	}
}

func attachThemeFonts(parsed *format.Parsed, fonts []block.StylesheetAsset) {
	for i := range parsed.Elements {
		if parsed.Elements[i].Role != block.RoleStylesheets {
			continue
		}
		styles := parsed.Elements[i].Content.(block.StylesheetSet)
		styles.Assets = append(styles.Assets, fonts...)
		parsed.Elements[i].Content = styles
		return
	}
	if len(fonts) > 0 {
		parsed.Elements = append(parsed.Elements, block.Element{
			ID: uuid.New(), Type: block.TypeStylesheetSet, Role: block.RoleStylesheets,
			Content: block.StylesheetSet{Assets: fonts},
		})
	}
}
