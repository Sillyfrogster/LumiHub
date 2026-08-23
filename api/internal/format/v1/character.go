package v1

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/book"
	"github.com/Sillyfrogster/Illarin/api/internal/media"
	"github.com/google/uuid"
)

const (
	characterNamespace     = "card"
	characterBookNamespace = "character_book"
)

// LiftedNamespaces are LumiHub's own display keys, which sit inside creators'
// extensions against ADR-0007. Migration lifts them out rather than preserving
// them, so a creator's download stops carrying somebody else's bookkeeping.
func LiftedNamespaces() []string { return append([]string(nil), liftedNamespaces...) }

var liftedNamespaces = []string{
	"lumihub_art_display",
	"landing_perspective_layers",
	"_lumiverse_install_slug",
	"_lumiverse_install_source",
	"_lumiverse_library_scope",
}

func readCharacter(row CharacterRow, recovery *CharacterRecovery) (readResult, error) {
	alternates, err := readStrings(row.AlternateGreetings, "alternate greetings")
	if err != nil {
		return readResult{}, err
	}
	events := make([]Event, 0, 1)
	if recovery != nil {
		if len(alternates) > 0 {
			return readResult{}, fmt.Errorf("v1 character recovery cannot replace row greetings")
		}
		if recovery.AlternateGreeting == "" {
			return readResult{}, fmt.Errorf("v1 character recovery needs a greeting")
		}
		alternates = append(alternates, recovery.AlternateGreeting)
		events = append(events, Event{Kind: RecoveredAlternateGreeting})
	}
	groupGreetings, err := readStrings(row.GroupOnlyGreetings, "group-only greetings")
	if err != nil {
		return readResult{}, err
	}

	elements := []block.Element{
		prose(block.RoleDescription, row.Common.Description),
		prose(block.RolePersonality, row.Personality),
		prose(block.RoleScenario, row.Scenario),
		{
			ID: uuid.New(), Type: block.TypeTextSet, Role: block.RoleGreetings,
			Content: block.TextSet{Texts: textItems(row.FirstMessage, alternates)},
		},
		{
			ID: uuid.New(), Type: block.TypeDialogueSample, Role: block.RoleExampleDialogue,
			Content: block.DialogueSample{Turns: dialogueTurns(row.ExampleDialogue)},
		},
	}
	for _, optional := range []struct {
		role block.Role
		text string
	}{
		{block.RoleSystemPrompt, row.SystemPrompt},
		{block.RolePostHistoryInstructions, row.PostHistoryInstructions},
		{block.RoleCreatorNotes, row.CreatorNotes},
	} {
		if optional.text != "" {
			elements = append(elements, prose(optional.role, optional.text))
		}
	}
	if len(groupGreetings) > 0 {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeTextSet, Role: block.RoleGroupGreetings,
			Content: block.TextSet{Texts: textItems("", groupGreetings)},
		})
	}

	bookElement, bookRemainder, bookRead := readCharacterBook(row.CharacterBook)
	if bookRead {
		elements = append(elements, bookElement)
	}
	cover, sources, imageElements, assetsConsumed, recoveredNames, err := readCharacterImages(row.Images, row.Assets)
	if err != nil {
		return readResult{}, err
	}
	if recoveredNames > 0 {
		events = append(events, Event{Kind: RecoveredGalleryNames, Count: recoveredNames})
	}
	if !assetsConsumed && carriesAssetDescriptors(row.Assets) {
		events = append(events, Event{Kind: GalleryAssetsMismatch, Count: 1})
	}
	elements = append(elements, imageElements...)
	remainder := append(bookRemainder, characterRemainder(row, bookRead, assetsConsumed, len(imageElements) > 0)...)

	answer := row.Common.IsNSFW
	created := row.Common.CreatedAt
	return readResult{parsed: format.Parsed{
		Kind: CharacterKind, Format: ID, Tags: append([]string(nil), row.Common.Tags...),
		IsNSFW: &answer, CreatedAt: &created,
		Header: format.Header{
			Name: row.Common.Name, Blurb: row.Tagline, AssetVersion: row.CharacterVersion,
			CreditedAuthor: row.Creator, Nickname: row.Nickname,
		},
		Elements: elements, Remainder: remainder,
	}, cover: cover, media: sources, events: events}, nil
}

func readCharacterBook(raw json.RawMessage) (block.Element, []format.Remainder, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return block.Element{}, nil, false
	}
	var source map[string]json.RawMessage
	if json.Unmarshal(raw, &source) != nil {
		return block.Element{}, []format.Remainder{{
			Owner: format.OwnerAsset, Namespace: characterBookNamespace, Payload: append([]byte(nil), raw...),
		}}, false
	}
	var payloads []json.RawMessage
	if entries, present := source["entries"]; !present || json.Unmarshal(entries, &payloads) != nil {
		return block.Element{}, []format.Remainder{{
			Owner: format.OwnerAsset, Namespace: characterBookNamespace, Payload: append([]byte(nil), raw...),
		}}, false
	}
	delete(source, "entries")
	entries, leftovers := book.Read(payloads)
	element := block.Element{
		ID: uuid.New(), Type: block.TypeEntryTable, Role: block.RoleLorebookEntries,
		Content: block.EntryTable{Entries: entries},
	}
	remainder := make([]format.Remainder, 0, len(leftovers)+1)
	if len(source) > 0 {
		remainder = append(remainder, format.Remainder{
			Owner: format.OwnerElement, OwnerID: element.ID,
			Namespace: characterBookNamespace, Payload: mustJSON(source),
		})
	}
	for _, entry := range entries {
		if fields, found := leftovers[entry.ID]; found {
			remainder = append(remainder, format.Remainder{
				Owner: format.OwnerItem, OwnerID: entry.ID,
				Namespace: characterBookNamespace, Payload: fields,
			})
		}
	}
	return element, remainder, true
}

func readCharacterImages(
	rows []CharacterImageRow,
	assets json.RawMessage,
) (*SourceMedia, []SourceMedia, []block.Element, bool, int, error) {
	rows = append([]CharacterImageRow(nil), rows...)
	slices.SortStableFunc(rows, func(a, b CharacterImageRow) int { return a.Position - b.Position })
	plainGallery := make([]int, 0)
	for i, row := range rows {
		if row.Type == "gallery" {
			plainGallery = append(plainGallery, i)
		}
	}
	assetsConsumed, recoveredNames := recoverGalleryNames(rows, plainGallery, assets)

	var cover *SourceMedia
	sources := make([]SourceMedia, 0, len(rows))
	expressions := make([]block.ImageItem, 0)
	gallery := make([]block.ImageItem, 0)
	for _, row := range rows {
		source := SourceMedia{
			SourceID: row.ID, MediaID: uuid.New(), Path: row.Path, MediaType: row.MediaType,
			ByteSize: row.ByteSize, Name: row.Label, Position: row.Position,
		}
		switch row.Type {
		case "avatar":
			source.Role = media.Avatar
			copy := source
			cover = &copy
			continue
		case "expression":
			source.Role = media.Expression
			expressions = append(expressions, block.ImageItem{
				ID: block.NewItemID(), MediaID: source.MediaID, Name: source.Name,
			})
		case "gallery", "avatar_alt", "perspective_layer":
			source.Role = media.Gallery
			gallery = append(gallery, block.ImageItem{
				ID: block.NewItemID(), MediaID: source.MediaID, Name: source.Name,
			})
		default:
			return nil, nil, nil, false, 0, fmt.Errorf("v1 character image type %q is not declared", row.Type)
		}
		sources = append(sources, source)
	}
	elements := make([]block.Element, 0, 2)
	if len(expressions) > 0 {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeImageSet, Role: block.RoleExpressions,
			Content: block.ImageSet{Images: expressions},
		})
	}
	if len(gallery) > 0 {
		elements = append(elements, block.Element{
			ID: uuid.New(), Type: block.TypeImageSet, Role: block.RoleGallery,
			Content: block.ImageSet{Images: gallery},
		})
	}
	return cover, sources, elements, assetsConsumed, recoveredNames, nil
}

func recoverGalleryNames(rows []CharacterImageRow, gallery []int, raw json.RawMessage) (bool, int) {
	if len(raw) == 0 || string(raw) == "null" {
		return true, 0
	}
	var assets []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &assets) != nil || len(assets) == 0 {
		return false, 0
	}
	if len(assets) != len(gallery) {
		return false, 0
	}
	for i, descriptor := range assets {
		if descriptor.Type == "emotion" || descriptor.Name == "" || rows[gallery[i]].Label != "" {
			return false, 0
		}
	}
	for i, descriptor := range assets {
		rows[gallery[i]].Label = descriptor.Name
	}
	return true, len(assets)
}

// carriesAssetDescriptors reports a CCv3 asset array with entries in it, which
// is the only case where failing to line up loses a creator something.
func carriesAssetDescriptors(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var descriptors []json.RawMessage
	return json.Unmarshal(raw, &descriptors) == nil && len(descriptors) > 0
}

func characterRemainder(
	row CharacterRow,
	bookRead bool,
	assetsConsumed bool,
	hasImages bool,
) []format.Remainder {
	remainder := make([]format.Remainder, 0)
	body := make(map[string]any)
	if !assetsConsumed && len(row.Assets) > 0 && string(row.Assets) != "null" {
		body["assets"] = json.RawMessage(row.Assets)
	}
	if row.CreationDate != nil {
		body["creation_date"] = *row.CreationDate
	}
	if row.ModificationDate != nil {
		body["modification_date"] = *row.ModificationDate
	}
	if len(body) > 0 {
		remainder = append(remainder, format.Remainder{
			Owner: format.OwnerAsset, Namespace: characterNamespace, Payload: mustJSON(body),
		})
	}

	var extensions map[string]json.RawMessage
	if len(row.Extensions) > 0 && string(row.Extensions) != "null" && json.Unmarshal(row.Extensions, &extensions) != nil {
		return append(remainder, format.Remainder{
			Owner: format.OwnerAsset, Namespace: "v1_extensions", Payload: append([]byte(nil), row.Extensions...),
		})
	}
	for _, namespace := range liftedNamespaces {
		delete(extensions, namespace)
	}
	if rawModules, found := extensions["lumiverse_modules"]; found {
		var modules map[string]json.RawMessage
		if json.Unmarshal(rawModules, &modules) == nil {
			delete(modules, "landing_perspective_layers")
			if bookRead {
				delete(modules, "world_books")
			}
			if hasImages {
				delete(modules, "expressions")
			}
			if len(modules) == 0 {
				delete(extensions, "lumiverse_modules")
			} else {
				extensions["lumiverse_modules"] = mustJSON(modules)
			}
		}
	}
	for _, namespace := range slices.Sorted(maps.Keys(extensions)) {
		remainder = append(remainder, format.Remainder{
			Owner: format.OwnerAsset, Namespace: namespace, Payload: extensions[namespace],
		})
	}
	return remainder
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func prose(role block.Role, text string) block.Element {
	return block.Element{
		ID: uuid.New(), Type: block.TypeProse, Role: role, Content: block.Prose{Text: text},
	}
}

func readStrings(raw json.RawMessage, label string) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("v1 character %s: expected a list of text", label)
	}
	return values, nil
}

func textItems(first string, rest []string) []block.TextItem {
	items := make([]block.TextItem, 0, len(rest)+1)
	if first != "" {
		items = append(items, block.TextItem{ID: block.NewItemID(), Text: first})
	}
	for _, text := range rest {
		items = append(items, block.TextItem{ID: block.NewItemID(), Text: text})
	}
	return items
}

func dialogueTurns(sample string) []block.DialogueTurn {
	lines := strings.Split(strings.ReplaceAll(sample, "\r\n", "\n"), "\n")
	turns := make([]block.DialogueTurn, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "<START>" {
			continue
		}
		speaker, text, found := strings.Cut(line, ":")
		if !found {
			speaker, text = "", line
		}
		turns = append(turns, block.DialogueTurn{
			ID: block.NewItemID(), Speaker: strings.TrimSpace(speaker), Text: strings.TrimSpace(text),
		})
	}
	return turns
}
