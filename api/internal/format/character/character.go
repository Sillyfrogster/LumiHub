// Package character reads the three character card standards. CCv2, CCv3 and
// CharX have different field sets and different round-trip rules, so each gets
// its own module; what they share lives here.
package character

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
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

// labels name a format in a download menu, where a reader picks without
// having to learn what the formats are.
var labels = map[string]string{
	V2:    "Character Card V2",
	V3:    "Character Card V3",
	CharX: "CharX",
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
		block.RoleGroupGreetings, block.RoleSystemPrompt,
		block.RolePostHistoryInstructions, block.RoleCreatorNotes,
		block.RoleLorebookEntries, block.RoleGallery, block.RoleExpressions,
	} {
		roles[role] = format.DirectionalRoleSupport{
			Read:  format.RoleSupport{Grade: format.SupportFull},
			Write: format.RoleSupport{Grade: format.SupportFull},
		}
	}
	// A card holds its greetings as plain strings and its example exchange as
	// one run of lines, so both carry whole until an asset uses a part of
	// Illarin's own model that the card has no room for. Neither condition
	// fires on an imported card, which is the point of stating them as
	// conditions rather than as a sentence every character would show.
	roles[block.RoleGreetings] = format.DirectionalRoleSupport{
		Read: format.RoleSupport{Grade: format.SupportFull},
		Write: format.RoleSupport{
			Grade: format.SupportPartial,
			Condition: &format.ContentCondition{
				Description: "a name written on a greeting, because a card holds greetings as plain text",
				Matches:     hasNamedText,
			},
		},
	}
	roles[block.RoleExampleDialogue] = format.DirectionalRoleSupport{
		Read: format.RoleSupport{Grade: format.SupportFull},
		Write: format.RoleSupport{
			Grade: format.SupportPartial,
			Condition: &format.ContentCondition{
				Description: "the line breaks inside a message, because a card holds the example as one line per turn",
				Matches:     hasMultilineTurn,
			},
		},
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
	if id != V2 {
		// The standard names an asset type for a face and none for a gallery,
		// so the pictures travel under an extension type. They are in the file
		// and only a client that knows the type will show them.
		gallery := roles[block.RoleGallery]
		gallery.Write.Destination = "an " + galleryAssetType +
			" asset, which only a client that knows the type will show"
		roles[block.RoleGallery] = gallery
	}
	// The keys this module turns into content. Everything else the card
	// carries is preserved, which is what makes the remainder a per-key
	// answer rather than a per-namespace one.
	//
	// `assets` is not one of them. A CharX card's asset list names files, and
	// Illarin holds those pictures as media of its own, but the list also
	// names a reader's own icon and files that live elsewhere. Reading part of
	// it is not consuming it, so the whole list is preserved.
	consumedKeys := []string{
		"name", "nickname", "character_version", "creator", "description",
		"personality", "scenario", "first_mes", "alternate_greetings",
		"mes_example", "system_prompt", "post_history_instructions",
		"creator_notes", "character_book",
	}
	if id != V2 {
		consumedKeys = append(consumedKeys, "group_only_greetings")
	}
	return format.Declaration{
		ID: id, Label: labels[id], Kind: Kind,
		Direction:   format.Direction{Read: true, Write: true},
		Recognition: recognition, Roles: roles,
		Limits: format.ContentLimits{
			PayloadBytes: block.MaxPayloadBytes, CollectionItems: block.MaxCollectionItems,
			ItemBytes: block.MaxItemBytes,
		},
		ConsumedKeys: consumedKeys,
		// SillyTavern stamps these four onto every card it writes, whether or
		// not the creator touched them, so half a real corpus carries four
		// namespaces that record nothing. They are stored like everything
		// else; this list only keeps them out of the creator's panel.
		Boilerplate: []format.Boilerplate{
			{Namespace: "depth_prompt", Path: []string{"prompt"}},
			{Namespace: "world"},
			{Namespace: "fav"},
			{Namespace: "talkativeness", Unchosen: []string{"0.5"}},
		},
		Preservation: format.PreservationDeclaration{
			Body: cardNamespace, Container: []string{extensionsKey},
		},
		// The three card standards share one field vocabulary, and the writers
		// build a file out of roles rather than out of another format's bytes,
		// so every character origin is a tested origin for every character
		// writer. That is a deliberate addition to the default of one's own
		// format and Illarin-authored assets (ADR-0020), and the round trips
		// that back it are in this package's tests.
		TestedOrigins: []string{V2, V3, CharX, format.OriginIllarin},
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
	// The book is read once. Its entries carry ids Illarin mints here, and the
	// preserved data keys against those ids, so reading it twice would key
	// against ids nothing else has.
	book := c.lorebook()
	elements := c.elements(formatID, book)
	return format.Parsed{
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
		Elements:  elements,
		Remainder: c.remainder(formatID, book, elements),
	}, nil
}

// remainder is everything the card carried that did not become content, one
// namespace at a time.
//
// It is computed per key. The declaration names the keys this module consumes;
// a declared key whose value did not fit is not consumed, so a bad value
// degrades into preservation instead of being lost. Every key of `extensions`
// becomes a namespace of its own, which is how a namespace Illarin half
// understands is split rather than kept twice.
func (c card) remainder(
	formatID string,
	book lorebook,
	elements []block.Element,
) []format.Remainder {
	consumed := c.consumed(formatID, book)
	extensions := c.extensions()
	remainder := make([]format.Remainder, 0, len(extensions)+2)

	body := make(map[string]json.RawMessage, len(c.fields))
	for key, raw := range c.fields {
		if key == extensionsKey || consumed[key] {
			continue
		}
		body[key] = raw
	}
	// A card whose extensions carry a key named for the card body itself
	// would ask for two namespaces of one name. Nothing in a real corpus does,
	// and the key travels back out whole either way, so it stays where it is
	// rather than being split out beside its namesake.
	if collision, clash := extensions[cardNamespace]; clash {
		body[extensionsKey], _ = json.Marshal(map[string]json.RawMessage{cardNamespace: collision})
		delete(extensions, cardNamespace)
	}
	if len(body) > 0 {
		payload, _ := json.Marshal(body)
		remainder = append(remainder, format.Remainder{
			Owner: format.OwnerAsset, Namespace: cardNamespace, Payload: payload,
		})
	}

	if consumed[bookKey] {
		delete(extensions, bookKey)
		remainder = append(remainder, book.preserved(elements)...)
	}
	for _, namespace := range slices.Sorted(maps.Keys(extensions)) {
		remainder = append(remainder, format.Remainder{
			Owner: format.OwnerAsset, Namespace: namespace, Payload: extensions[namespace],
		})
	}
	return remainder
}

// consumed reports which of the module's declared keys this card actually
// turned into content. A declared key the card wrote as the wrong shape is not
// among them.
func (c card) consumed(formatID string, book lorebook) map[string]bool {
	consumed := make(map[string]bool)
	for _, key := range declaration(formatID).ConsumedKeys {
		raw, present := c.fields[key]
		if !present {
			continue
		}
		switch key {
		case bookKey:
			consumed[key] = book.found
		case "alternate_greetings", "group_only_greetings":
			var texts []string
			consumed[key] = json.Unmarshal(raw, &texts) == nil
		default:
			var text string
			consumed[key] = json.Unmarshal(raw, &text) == nil
		}
	}
	// A book the card kept inside its extensions is content all the same, so
	// the extensions key holding it is consumed rather than the card's own.
	if book.found {
		consumed[bookKey] = true
	}
	return consumed
}

func (c card) elements(formatID string, book lorebook) []block.Element {
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
	if book.found {
		elements = append(elements, block.Element{
			Type: block.TypeEntryTable, Role: block.RoleLorebookEntries, Content: book.table,
		})
	}
	// Every element carries an id from the moment it is read, because the
	// preserved data beside it points at that id.
	for i := range elements {
		elements[i].ID = uuid.New()
	}
	return elements
}

func (c card) greetings(primary, alternates string) []block.TextItem {
	items := make([]block.TextItem, 0)
	if primary != "" {
		items = append(items, block.TextItem{ID: block.NewItemID(), Text: c.text(primary)})
	}
	var values []string
	if raw, ok := c.fields[alternates]; ok {
		_ = json.Unmarshal(raw, &values)
	}
	for _, value := range values {
		items = append(items, block.TextItem{ID: block.NewItemID(), Text: value})
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
			ID:      block.NewItemID(),
			Speaker: strings.TrimSpace(speaker), Text: strings.TrimSpace(text),
		})
	}
	return turns
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
	if isPopulatedObject(c.fields[bookKey]) {
		return true
	}
	return isPopulatedObject(c.extensions()[bookKey])
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

// hasNamedText reports whether a creator gave one of a set's texts a name.
func hasNamedText(content block.Content) bool {
	set, ok := content.(block.TextSet)
	if !ok {
		return false
	}
	for _, item := range set.Texts {
		if item.Name != "" {
			return true
		}
	}
	return false
}

// hasMultilineTurn reports whether one turn of an example exchange spans more
// than one line, which is what runs together when a card is read back.
func hasMultilineTurn(content block.Content) bool {
	sample, ok := content.(block.DialogueSample)
	if !ok {
		return false
	}
	for _, turn := range sample.Turns {
		if strings.ContainsAny(turn.Text, "\r\n") {
			return true
		}
	}
	return false
}
