package theme

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/keys"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/google/uuid"
)

func readLumiverse(ctx context.Context, file probe.Inspection, payload probe.Payload) (format.Parsed, error) {
	source := maps.Clone(payload.Root)
	delete(source, "format")
	header := format.Header{}
	keys.Take(source, "name", &header.Name)
	keys.Take(source, "author", &header.CreditedAuthor)
	var description string
	if rawDescription, present := source["description"]; present && json.Unmarshal(rawDescription, &description) == nil {
		if len([]rune(description)) <= format.MaxBlurbRunes {
			header.Blurb = description
			delete(source, "description")
		}
	}
	var createdAt *time.Time
	var createdText string
	if keys.Take(source, "createdAt", &createdText) {
		if parsed, err := time.Parse(time.RFC3339, createdText); err == nil {
			createdAt = &parsed
		} else {
			source["createdAt"] = raw(createdText)
		}
	}

	var theme map[string]json.RawMessage
	if value, present := source["theme"]; !present || json.Unmarshal(value, &theme) != nil {
		return format.Parsed{}, format.MalformedInput(fmt.Errorf("%s theme: expected an object", LumiverseID))
	}
	delete(source, "theme")
	palette, err := readLumiversePalette(theme)
	if err != nil {
		return format.Parsed{}, err
	}
	elements := []block.Element{{
		ID: uuid.New(), Type: block.TypeColorSet, Role: block.RoleThemeTokens, Content: palette,
	}}
	if settings := readLumiverseControls(theme); len(settings) > 0 {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeSettingGroup, Role: block.RoleThemeControls,
			Content: block.SettingGroup{Settings: settings},
		})
	}
	if len(theme) > 0 {
		source["theme"] = raw(theme)
	}

	styles, itemRows := readLumiverseStyles(ctx, file, source)
	if !styles.Empty() {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeStylesheetSet, Role: block.RoleStylesheets,
			Content: styles,
		})
	}

	return format.Parsed{
		Kind: Kind, Format: LumiverseID, CreatedAt: createdAt,
		Header: header, Elements: elements,
		Remainder: themeRemainder(lumiverseNamespace, source, itemRows...),
	}, nil
}

func readLumiversePalette(theme map[string]json.RawMessage) (block.ColorSet, error) {
	rawModes, present := theme["baseColorsByMode"]
	if !present {
		return block.ColorSet{}, format.MalformedInput(fmt.Errorf(
			"%s palette: baseColorsByMode is required", LumiverseID,
		))
	}
	var sourceModes map[string]json.RawMessage
	if json.Unmarshal(rawModes, &sourceModes) != nil {
		return block.ColorSet{}, format.MalformedInput(fmt.Errorf(
			"%s palette: baseColorsByMode must be an object", LumiverseID,
		))
	}
	delete(theme, "baseColorsByMode")
	modes := make([]block.ColorMode, 0, len(sourceModes))
	leftoverModes := make(map[string]json.RawMessage)
	for _, modeName := range slices.Sorted(maps.Keys(sourceModes)) {
		var colors map[string]json.RawMessage
		if json.Unmarshal(sourceModes[modeName], &colors) != nil {
			leftoverModes[modeName] = sourceModes[modeName]
			continue
		}
		read := make([]block.Color, 0, len(lumiverseColors))
		for _, name := range lumiverseColors {
			var value string
			if keys.Take(colors, name, &value) {
				read = append(read, block.Color{ID: block.NewItemID(), Name: name, Value: value})
			}
		}
		if len(read) > 0 {
			modes = append(modes, block.ColorMode{Name: modeName, Colors: read})
		}
		if len(colors) > 0 {
			leftoverModes[modeName] = raw(colors)
		}
	}
	if len(modes) == 0 {
		return block.ColorSet{}, format.MalformedInput(fmt.Errorf(
			"%s palette: no supported colours were found", LumiverseID,
		))
	}
	if len(leftoverModes) > 0 {
		theme["baseColorsByMode"] = raw(leftoverModes)
	}
	return block.ColorSet{Modes: modes}, nil
}

func readLumiverseControls(theme map[string]json.RawMessage) []block.Setting {
	settings := make([]block.Setting, 0, len(lumiverseControls))
	for _, slot := range lumiverseControls {
		value, present := theme[slot.name]
		if !present {
			continue
		}
		if slot.name == "accent" {
			var object map[string]json.RawMessage
			if json.Unmarshal(value, &object) != nil {
				continue
			}
			text := string(value)
			delete(theme, slot.name)
			settings = append(settings, block.Setting{
				ID: block.NewItemID(), Name: slot.name, Type: block.SettingText,
				Value: &block.Value{Text: &text},
			})
			continue
		}
		read, ok := readSetting(value, slot.settingType)
		if !ok {
			continue
		}
		delete(theme, slot.name)
		settings = append(settings, block.Setting{
			ID: block.NewItemID(), Name: slot.name, Type: slot.settingType, Value: read,
		})
	}
	return settings
}

func readLumiverseStyles(
	ctx context.Context,
	file probe.Inspection,
	source map[string]json.RawMessage,
) (block.StylesheetSet, []itemRemainder) {
	styles := block.StylesheetSet{
		Stylesheets: []block.Stylesheet{}, Assets: []block.StylesheetAsset{},
	}
	keys.Take(source, "globalCSS", &styles.Global)
	rows := make([]itemRemainder, 0)

	if rawComponents, present := source["components"]; present {
		var components map[string]json.RawMessage
		if json.Unmarshal(rawComponents, &components) == nil {
			delete(source, "components")
			unread := make(map[string]json.RawMessage)
			for _, name := range slices.Sorted(maps.Keys(components)) {
				var fields map[string]json.RawMessage
				if json.Unmarshal(components[name], &fields) != nil {
					unread[name] = components[name]
					continue
				}
				var css string
				if !keys.Take(fields, "css", &css) {
					unread[name] = components[name]
					continue
				}
				enabled := true
				keys.Take(fields, "enabled", &enabled)
				id := block.NewItemID()
				styles.Stylesheets = append(styles.Stylesheets, block.Stylesheet{
					ID: id, Name: name, CSS: css, Enabled: enabled,
				})
				rows = append(rows, itemRemainder{
					namespace: lumiverseComponentNamespace, id: id, fields: fields,
				})
			}
			if len(unread) > 0 {
				source["components"] = raw(unread)
			}
		}
	}

	if rawAssets, present := source["assets"]; present {
		var descriptors []json.RawMessage
		if json.Unmarshal(rawAssets, &descriptors) == nil {
			delete(source, "assets")
			unread := make([]json.RawMessage, 0)
			for _, descriptor := range descriptors {
				var fields map[string]json.RawMessage
				if json.Unmarshal(descriptor, &fields) != nil {
					unread = append(unread, descriptor)
					continue
				}
				var archivePath, mediaType string
				if !keys.Take(fields, "archivePath", &archivePath) || !safeArchivePath(archivePath) {
					unread = append(unread, descriptor)
					continue
				}
				keys.Take(fields, "mimeType", &mediaType)
				opened, err := file.OpenZIPEntry(ctx, archivePath)
				if err != nil {
					unread = append(unread, descriptor)
					continue
				}
				data, err := io.ReadAll(opened)
				opened.Close()
				if err != nil {
					unread = append(unread, descriptor)
					continue
				}
				id := block.NewItemID()
				styles.Assets = append(styles.Assets, block.StylesheetAsset{
					ID: id, Path: archivePath, MediaType: mediaType, Data: data,
				})
				rows = append(rows, itemRemainder{
					namespace: lumiverseAssetNamespace, id: id, fields: fields,
				})
			}
			if len(unread) > 0 {
				source["assets"] = raw(unread)
			}
		}
	}
	return styles, rows
}

func (LumiverseModule) Write(_ context.Context, asset format.ExportAsset) (format.Artifact, error) {
	held := keepTheme(asset.Preserved)
	body := held.body(lumiverseNamespace)
	theme := keys.Object(body["theme"])
	delete(body, "theme")
	body["format"] = raw(3)
	body["name"] = raw(asset.Header.Name)
	body["author"] = raw(asset.Header.CreditedAuthor)
	body["description"] = raw(asset.Header.Blurb)

	if content, ok := asset.Content(block.RoleThemeTokens); ok {
		if palette, isPalette := content.(block.ColorSet); isPalette {
			modes := make(map[string]map[string]json.RawMessage)
			for name, preserved := range keys.Object(theme["baseColorsByMode"]) {
				modes[name] = keys.Object(preserved)
			}
			for _, mode := range palette.Modes {
				if modes[mode.Name] == nil {
					modes[mode.Name] = make(map[string]json.RawMessage)
				}
				for _, color := range mode.Colors {
					if color.Value != "" && slices.Contains(lumiverseColors, color.Name) {
						modes[mode.Name][color.Name] = raw(color.Value)
					}
				}
			}
			theme["baseColorsByMode"] = raw(modes)
		}
	}
	if content, ok := asset.Content(block.RoleThemeControls); ok {
		for _, setting := range themeSettings(content) {
			if setting.Value == nil || !slices.ContainsFunc(lumiverseControls, func(slot namedSlot) bool {
				return slot.name == setting.Name
			}) {
				continue
			}
			if setting.Name == "accent" && setting.Value.Text != nil {
				var accent map[string]json.RawMessage
				if json.Unmarshal([]byte(*setting.Value.Text), &accent) == nil {
					theme[setting.Name] = raw(accent)
				}
				continue
			}
			theme[setting.Name] = writeSetting(setting)
		}
	}
	body["theme"] = raw(theme)

	if content, ok := asset.Content(block.RoleStylesheets); ok {
		if styles, isStyles := content.(block.StylesheetSet); isStyles {
			body["globalCSS"] = raw(styles.Global)
			components := keys.Object(body["components"])
			for _, sheet := range styles.Stylesheets {
				fields := held.item(lumiverseComponentNamespace, sheet.ID)
				fields["css"] = raw(sheet.CSS)
				fields["enabled"] = raw(sheet.Enabled)
				components[sheet.Name] = raw(fields)
			}
			if len(components) > 0 {
				body["components"] = raw(components)
			}
			descriptors := preservedAssetDescriptors(body["assets"])
			for _, attached := range styles.Assets {
				if !safeArchivePath(attached.Path) {
					return format.Artifact{}, format.SafetyViolation(fmt.Errorf(
						"theme asset path %q is not safe", attached.Path,
					))
				}
				fields := held.item(lumiverseAssetNamespace, attached.ID)
				fields["archivePath"] = raw(attached.Path)
				fields["mimeType"] = raw(attached.MediaType)
				if _, present := fields["originalFilename"]; !present {
					fields["originalFilename"] = raw(path.Base(attached.Path))
				}
				descriptors = append(descriptors, raw(fields))
			}
			if len(descriptors) > 0 {
				body["assets"] = raw(descriptors)
			}
			return writeLumiverseBundle(body, styles.Assets)
		}
	}
	return writeLumiverseBundle(body, nil)
}

func preservedAssetDescriptors(value json.RawMessage) []json.RawMessage {
	var descriptors []json.RawMessage
	_ = json.Unmarshal(value, &descriptors)
	return descriptors
}

func writeLumiverseBundle(document map[string]json.RawMessage, assets []block.StylesheetAsset) (format.Artifact, error) {
	encoded, err := json.Marshal(document)
	if err != nil {
		return format.Artifact{}, fmt.Errorf("write the Lumiverse theme: %w", err)
	}
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	if err := writeZipEntry(archive, "theme.json", encoded); err != nil {
		return format.Artifact{}, err
	}
	for _, attached := range assets {
		if err := writeZipEntry(archive, attached.Path, attached.Data); err != nil {
			return format.Artifact{}, err
		}
	}
	if err := archive.Close(); err != nil {
		return format.Artifact{}, fmt.Errorf("finish the Lumiverse theme bundle: %w", err)
	}
	return format.Artifact{
		Body: output.Bytes(), MediaType: "application/zip", Extension: ".lumitheme",
	}, nil
}

func writeZipEntry(archive *zip.Writer, name string, data []byte) error {
	entry, err := archive.Create(name)
	if err != nil {
		return fmt.Errorf("create theme bundle entry %q: %w", name, err)
	}
	if _, err := entry.Write(data); err != nil {
		return fmt.Errorf("write theme bundle entry %q: %w", name, err)
	}
	return nil
}

func safeArchivePath(name string) bool {
	return name != "" && !strings.Contains(name, "\\") && !strings.HasPrefix(name, "/") &&
		path.Clean(name) == name && name != "." && name != ".." && !strings.HasPrefix(name, "../")
}
