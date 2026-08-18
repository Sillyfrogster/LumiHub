// Package character reads the three character card standards. CCv2, CCv3 and
// CharX have different field sets and different round-trip rules, so each gets
// its own module; what they share lives here.
package character

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

// Kind is what a character card is to a person. It comes from the module, so a
// creator never picks it.
const Kind = "character"

// Seeded catalog text is a prefill the creator confirms, so it is bounded
// rather than however long the note in the file happens to be.
const (
	maxBlurbRunes = 400
	maxTags       = 32
	maxTagRunes   = 64
)

// A card carries whatever namespaces its tools wrote. These bounds keep one
// odd file from filling the facet table.
const (
	maxExtensionNamespaces = 64
	maxNamespaceRunes      = 64
)

// exportTargets are the platforms a character card reaches. Every character
// format reaches all of them, which is the point: naming a platform narrows
// what a reader sees and never unlocks anything.
func exportTargets() []format.BrowseOption {
	return []format.BrowseOption{
		{Value: "sillytavern", Label: "SillyTavern"},
		{Value: "risu", Label: "Risu"},
		{Value: "lumiverse", Label: "Lumiverse"},
	}
}

// browsableExtensions are the namespaces browse offers as filters. A card gets
// a facet for every namespace it carries; these are the ones worth putting in
// front of a reader, because each names something the card can do rather than a
// colour, a bookmark or a tool's watermark.
var browsableExtensions = []format.BrowseOption{
	{Value: "depth_prompt", Label: "Depth prompt"},
	{Value: "regex_scripts", Label: "Regex scripts"},
	{Value: "alternate_character_name", Label: "Alternate name"},
	{Value: "tavern_helper", Label: "TavernHelper scripts"},
	{Value: "lumiverse_modules", Label: "Lumiverse modules"},
	{Value: "landing_perspective_layers", Label: "Perspective layers"},
	{Value: "chub", Label: "Chub metadata"},
}

var patchableFields = []format.Field{
	format.FieldDescription,
	format.FieldPersonality,
	format.FieldScenario,
	format.FieldFirstMessage,
	format.FieldSystemPrompt,
	format.FieldPostHistoryInstructions,
	format.FieldCreatorNotes,
	format.FieldCharacterVersion,
}

func validatePatch(patch format.Patch) error {
	return format.ValidatePatchFields(patch, patchableFields...)
}

func browseFacets() []format.BrowseFacet {
	return []format.BrowseFacet{
		{Key: "has_lorebook", Label: "Lorebook", Options: []format.BrowseOption{
			{Value: "true", Label: "Included"},
			{Value: "false", Label: "None"},
		}},
		{Key: "extension", Label: "Extensions", Options: browsableExtensions},
	}
}

func browseDefinition(targets []format.BrowseOption) format.BrowseDefinition {
	return format.BrowseDefinition{
		Kind:          Kind,
		ExportTargets: targets,
		Facets:        browseFacets(),
	}
}

func declaration(id string) format.Declaration {
	recognition := []format.Recognition{{
		Kind:       format.RecognitionDiscriminator,
		Containers: []probe.Container{probe.JSON, probe.PNG, probe.JPEG, probe.WebP, probe.GIF},
		Path:       []string{"spec"}, Values: []string{id},
	}}
	if id == V2 {
		// A v3 card keeps a v2 copy of itself, so a file with both is a v3 card.
		recognition[0].SupersededBy = []string{V3}
		recognition = append(recognition, format.Recognition{
			Kind: format.RecognitionSignature, LegacyOnly: true,
			Containers: []probe.Container{probe.JSON, probe.PNG, probe.JPEG, probe.WebP, probe.GIF},
			Required: map[string]format.ValueType{
				"name": format.ValueString, "description": format.ValueString,
				"personality": format.ValueString, "scenario": format.ValueString,
				"first_mes": format.ValueString,
			},
		})
	}
	if id == CharX {
		recognition = []format.Recognition{{
			Kind: format.RecognitionDiscriminator, Containers: []probe.Container{probe.ZIP},
			Path: []string{"spec"}, Values: []string{V3},
		}}
	}
	roles := make(map[block.Role]format.DirectionalRoleSupport)
	for _, role := range []block.Role{
		block.RoleDescription, block.RolePersonality, block.RoleScenario,
		block.RoleGreetings, block.RoleGroupGreetings, block.RoleExampleDialogue,
		block.RoleSystemPrompt, block.RolePostHistoryInstructions,
		block.RoleCreatorNotes, block.RoleLorebookEntries,
		block.RoleGallery, block.RoleExpressions,
	} {
		roles[role] = format.DirectionalRoleSupport{
			Read:  format.RoleSupport{Grade: format.SupportFull},
			Write: format.RoleSupport{Grade: format.SupportFull},
		}
	}
	if id == V2 {
		for _, role := range []block.Role{
			block.RoleGroupGreetings, block.RoleGallery, block.RoleExpressions,
		} {
			roles[role] = format.DirectionalRoleSupport{
				Read:  format.RoleSupport{Grade: format.SupportNone},
				Write: format.RoleSupport{Grade: format.SupportNone},
			}
		}
	}
	if id == V3 {
		for _, role := range []block.Role{block.RoleGallery, block.RoleExpressions} {
			roles[role] = format.DirectionalRoleSupport{
				Read:  format.RoleSupport{Grade: format.SupportNone},
				Write: format.RoleSupport{Grade: format.SupportFull},
			}
		}
	}
	consumedKeys := []string{
		"name", "nickname", "character_version", "creator", "description",
		"personality", "scenario", "first_mes", "alternate_greetings",
		"mes_example", "system_prompt", "post_history_instructions",
		"creator_notes", "character_book",
	}
	if id != V2 {
		consumedKeys = append(consumedKeys, "group_only_greetings")
	}
	if id == CharX {
		consumedKeys = append(consumedKeys, "assets")
	}
	return format.Declaration{
		ID: id, Kind: Kind, Direction: format.Direction{Read: true, Write: true},
		Recognition: recognition, Roles: roles,
		Limits: format.ContentLimits{
			PayloadBytes: block.MaxPayloadBytes, CollectionItems: block.MaxCollectionItems,
			ItemBytes: block.MaxItemBytes,
		},
		ConsumedKeys: consumedKeys,
		Boilerplate: []format.Boilerplate{
			{Namespace: "extensions", Path: []string{"depth_prompt", "prompt"}},
		},
		Preservation:  format.PreservationDeclaration{Namespaces: []string{"card", "extensions"}},
		TestedOrigins: []string{id, "illarin"},
	}
}

// card is one card's spec-defined body, whatever container carried it.
type card struct {
	fields map[string]json.RawMessage
}

// readCard checks the payload's version against the one this module implements
// and reads the card body out of it.
func readCard(file probe.Inspection, claim format.Claim, implemented int, moduleID string) (card, error) {
	payload, ok := claim.Payload(file)
	if !ok {
		return card{}, fmt.Errorf("%s payload: the claimed payload is missing", moduleID)
	}
	if err := readableVersion(payload, implemented); err != nil {
		return card{}, fmt.Errorf("%s spec_version: %w", moduleID, err)
	}
	fields, ok := Fields(file, claim)
	if !ok {
		return card{}, fmt.Errorf("%s data: missing or not an object", moduleID)
	}
	return card{fields: fields}, nil
}

// readableVersion checks what the payload says about itself against the version
// this module implements. A later major version may have changed what a field
// means, so it is refused rather than guessed at. A later minor version only
// adds, so it is read and the additions are left alone.
func readableVersion(payload probe.Payload, implemented int) error {
	declared, ok := payload.String("spec_version")
	if !ok || declared == "" {
		return nil
	}
	text, _, _ := strings.Cut(declared, ".")
	major, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("spec_version %q is not a version", declared)
	}
	if major > implemented {
		return format.UnsupportedVersion(
			fmt.Errorf("spec_version %q is past version %d", declared, implemented),
		)
	}
	return nil
}

// parsed is what ingest stores. Only the format id and which pictures count
// differ between the three standards.
func (c card) parsed(formatID string, pictures []format.Media) (format.Parsed, error) {
	for _, required := range []string{"description", "first_mes"} {
		if raw, present := c.fields[required]; present {
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return format.Parsed{}, fmt.Errorf(
					"%s could not read %s: expected a string", formatID, required,
				)
			}
		}
	}
	parsed := format.Parsed{
		Kind:      Kind,
		Format:    formatID,
		Blurb:     c.blurb(),
		Tags:      c.tags(),
		Facets:    c.facets(),
		Media:     pictures,
		CreatedAt: c.createdAt(),
		Header: format.Header{
			Name: c.name(), AssetVersion: c.text("character_version"),
			CreditedAuthor: c.text("creator"), Nickname: c.text("nickname"),
		},
		Elements: c.elements(formatID),
	}
	parsed.Remainder = c.remainder(formatID)
	return parsed, nil
}

func (c card) remainder(formatID string) []format.Remainder {
	consumed := make(map[string]bool)
	_, bookLocation, bookRemainder, bookOK := c.lorebook()
	for _, field := range []string{
		"name", "nickname", "character_version", "creator", "description",
		"personality", "scenario", "first_mes", "mes_example", "system_prompt",
		"post_history_instructions", "creator_notes",
	} {
		if raw, ok := c.fields[field]; ok {
			var value string
			consumed[field] = json.Unmarshal(raw, &value) == nil
		}
	}
	listFields := []string{"alternate_greetings"}
	if formatID != V2 {
		listFields = append(listFields, "group_only_greetings")
	}
	for _, field := range listFields {
		if raw, ok := c.fields[field]; ok {
			var value []string
			consumed[field] = json.Unmarshal(raw, &value) == nil
		}
	}
	if bookOK && bookLocation == "card" {
		consumed["character_book"] = true
	}

	cardRemainder := make(map[string]json.RawMessage)
	for key, raw := range c.fields {
		if key == "extensions" || consumed[key] {
			continue
		}
		cardRemainder[key] = raw
	}
	if bookLocation == "card" && len(bookRemainder) > 0 {
		cardRemainder["character_book"] = bookRemainder
	}
	remainder := make([]format.Remainder, 0, 2)
	if len(cardRemainder) > 0 {
		payload, _ := json.Marshal(cardRemainder)
		remainder = append(remainder, format.Remainder{Namespace: "card", Payload: payload})
	}
	extensions := c.extensions()
	if bookOK && bookLocation == "extensions" {
		delete(extensions, "character_book")
		if len(bookRemainder) > 0 {
			extensions["character_book"] = bookRemainder
		}
	}
	if len(extensions) > 0 {
		payload, _ := json.Marshal(extensions)
		remainder = append(remainder, format.Remainder{Namespace: "extensions", Payload: payload})
	}
	return remainder
}

func (c card) elements(formatID string) []block.Element {
	elements := []block.Element{
		{Type: block.TypeProse, Role: block.RoleDescription, Content: block.Prose{Text: c.text("description")}},
		{Type: block.TypeProse, Role: block.RolePersonality, Content: block.Prose{Text: c.text("personality")}},
		{Type: block.TypeProse, Role: block.RoleScenario, Content: block.Prose{Text: c.text("scenario")}},
		{Type: block.TypeTextSet, Role: block.RoleGreetings, Content: block.TextSet{Texts: c.greetings("first_mes", "alternate_greetings")}},
		{Type: block.TypeDialogueSample, Role: block.RoleExampleDialogue, Content: block.DialogueSample{Turns: dialogueTurns(c.text("mes_example"))}},
	}
	optionalProse := []struct {
		field string
		role  block.Role
	}{
		{"system_prompt", block.RoleSystemPrompt},
		{"post_history_instructions", block.RolePostHistoryInstructions},
		{"creator_notes", block.RoleCreatorNotes},
	}
	for _, item := range optionalProse {
		if text := c.text(item.field); text != "" {
			elements = append(elements, block.Element{
				Type: block.TypeProse, Role: item.role, Content: block.Prose{Text: text},
			})
		}
	}
	if greetings := c.greetings("", "group_only_greetings"); formatID != V2 && len(greetings) > 0 {
		elements = append(elements, block.Element{
			Type: block.TypeTextSet, Role: block.RoleGroupGreetings,
			Content: block.TextSet{Texts: greetings},
		})
	}
	if book, _, _, ok := c.lorebook(); ok {
		elements = append(elements, block.Element{
			Type: block.TypeEntryTable, Role: block.RoleLorebookEntries, Content: book,
		})
	}
	return elements
}

func (c card) greetings(primary, alternates string) []block.TextItem {
	items := make([]block.TextItem, 0)
	if primary != "" {
		items = append(items, block.TextItem{Text: c.text(primary)})
	}
	var values []string
	if raw, ok := c.fields[alternates]; ok {
		_ = json.Unmarshal(raw, &values)
	}
	for _, value := range values {
		items = append(items, block.TextItem{Text: value})
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
			speaker = ""
			text = line
		}
		turns = append(turns, block.DialogueTurn{
			Speaker: strings.TrimSpace(speaker), Text: strings.TrimSpace(text),
		})
	}
	return turns
}

func (c card) lorebook() (block.EntryTable, string, json.RawMessage, bool) {
	raw := c.fields["character_book"]
	location := "card"
	if len(raw) == 0 {
		raw = c.extensions()["character_book"]
		location = "extensions"
	}
	var source map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &source) != nil {
		return block.EntryTable{}, "", nil, false
	}
	var entryPayloads []json.RawMessage
	if entriesRaw, present := source["entries"]; !present || json.Unmarshal(entriesRaw, &entryPayloads) != nil {
		return block.EntryTable{}, "", nil, false
	}
	delete(source, "entries")
	entries := make([]block.Entry, 0, len(entryPayloads))
	entryRemainders := make([]json.RawMessage, len(entryPayloads))
	hasEntryRemainder := false
	for index, payload := range entryPayloads {
		var fields map[string]json.RawMessage
		if json.Unmarshal(payload, &fields) != nil || fields == nil {
			entryRemainders[index] = payload
			hasEntryRemainder = true
			continue
		}
		item := block.Entry{Enabled: true}
		consumeLorebookField(fields, "name", &item.Name)
		consumeLorebookField(fields, "keys", &item.Keys)
		consumeLorebookField(fields, "secondary_keys", &item.SecondaryKeys)
		consumeLorebookField(fields, "selective", &item.Selective)
		consumeLorebookField(fields, "case_sensitive", &item.CaseSensitive)
		consumeLorebookField(fields, "constant", &item.Constant)
		consumeLorebookField(fields, "enabled", &item.Enabled)
		consumeLorebookField(fields, "insertion_order", &item.Order)
		consumeLorebookField(fields, "content", &item.Text)
		var position string
		if consumeLorebookField(fields, "position", &position) {
			switch position {
			case "", "before_char", "before_character":
				if position != "" {
					item.Position = block.BeforeCharacter
				}
			case "after_char", "after_character":
				item.Position = block.AfterCharacter
			default:
				fields["position"], _ = json.Marshal(position)
			}
		}
		entries = append(entries, item)
		if len(fields) > 0 {
			entryRemainders[index], _ = json.Marshal(fields)
			hasEntryRemainder = true
		}
	}
	if hasEntryRemainder {
		for index := range entryRemainders {
			if len(entryRemainders[index]) == 0 {
				entryRemainders[index] = json.RawMessage("null")
			}
		}
		source["entries"], _ = json.Marshal(entryRemainders)
	}
	var remainder json.RawMessage
	if len(source) > 0 {
		remainder, _ = json.Marshal(source)
	}
	return block.EntryTable{Entries: entries}, location, remainder, true
}

func consumeLorebookField[T any](fields map[string]json.RawMessage, name string, target *T) bool {
	raw, present := fields[name]
	if !present || json.Unmarshal(raw, target) != nil {
		return false
	}
	delete(fields, name)
	return true
}

func (c card) text(name string) string {
	raw, ok := c.fields[name]
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func (c card) name() string { return strings.TrimSpace(c.text("name")) }

// blurb is what a person reads while browsing, so it comes from the creator's
// notes. A card's description is a prompt written to be fed to a model and
// never becomes the blurb.
func (c card) blurb() string {
	return truncate(strings.TrimSpace(c.text("creator_notes")), maxBlurbRunes)
}

func (c card) tags() []string {
	var values []string
	if raw, ok := c.fields["tags"]; ok {
		_ = json.Unmarshal(raw, &values)
	}
	tags := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || len([]rune(trimmed)) > maxTagRunes {
			continue
		}
		if !slices.Contains(tags, trimmed) {
			tags = append(tags, trimmed)
		}
		if len(tags) == maxTags {
			break
		}
	}
	return tags
}

// createdAt is the date the file carries. CCv3 records it in seconds since the
// epoch; CCv2 has nowhere to put one.
func (c card) createdAt() *time.Time {
	raw, ok := c.fields["creation_date"]
	if !ok {
		return nil
	}
	var seconds int64
	if err := json.Unmarshal(raw, &seconds); err != nil || seconds <= 0 {
		return nil
	}
	made := time.Unix(seconds, 0).UTC()
	return &made
}

// facets are what browse can filter on. An embedded lorebook is one of them:
// it stays part of the card it came from rather than becoming a second asset
// nobody uploaded and nobody can update on its own.
func (c card) facets() []format.Facet {
	facets := []format.Facet{
		{Key: "has_lorebook", Value: strconv.FormatBool(c.hasLorebook())},
	}
	for _, namespace := range c.extensionNamespaces() {
		facets = append(facets, format.Facet{Key: "extension", Value: namespace})
	}
	return facets
}

func (c card) hasLorebook() bool {
	if isPopulatedObject(c.fields["character_book"]) {
		return true
	}
	return isPopulatedObject(c.extensions()["character_book"])
}

// extensionNamespaces names every top-level key under extensions, so what a
// platform wrote into a card becomes a filter rather than opaque bytes.
func (c card) extensionNamespaces() []string {
	namespaces := make([]string, 0)
	for namespace := range c.extensions() {
		if namespace == "" || len([]rune(namespace)) > maxNamespaceRunes {
			continue
		}
		namespaces = append(namespaces, namespace)
	}
	slices.Sort(namespaces)
	if len(namespaces) > maxExtensionNamespaces {
		namespaces = namespaces[:maxExtensionNamespaces]
	}
	return namespaces
}

func (c card) extensions() map[string]json.RawMessage {
	var extensions map[string]json.RawMessage
	if raw, ok := c.fields["extensions"]; ok {
		_ = json.Unmarshal(raw, &extensions)
	}
	return extensions
}

func isPopulatedObject(raw json.RawMessage) bool {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return false
	}
	return len(object) > 0
}

// documentImage gives the card's own picture the avatar role when the file
// carrying the card is itself an image. The file is read, never rewritten.
func documentImage(file probe.Inspection) []format.Media {
	for _, image := range file.Images {
		if image.Locator.Container != probe.ZIP {
			return []format.Media{{Role: media.Avatar, ImageID: image.ID}}
		}
	}
	return nil
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cut := string(runes[:limit])
	if space := strings.LastIndexFunc(cut, unicode.IsSpace); space > 0 {
		cut = cut[:space]
	}
	return strings.TrimSpace(cut)
}

// Modules returns every character card module, so the server registers the set
// rather than remembering to add each one.
func Modules() []format.Reader {
	return []format.Reader{CCv2Module{}, CCv3Module{}, CharXModule{}}
}
