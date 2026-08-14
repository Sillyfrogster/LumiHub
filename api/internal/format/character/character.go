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

func browseDefinition() format.BrowseDefinition {
	return format.BrowseDefinition{
		Kind:          Kind,
		ExportTargets: exportTargets(),
		Facets:        browseFacets(),
	}
}

// card is one card's spec-defined body, whatever container carried it.
type card struct {
	fields map[string]json.RawMessage
}

// readCard checks the payload's version against the one this module implements
// and reads the card body out of it.
func readCard(file probe.Inspection, claim format.Claim, implemented int) (card, error) {
	payload, ok := claim.Payload(file)
	if !ok {
		return card{}, fmt.Errorf("the claimed payload is missing")
	}
	if err := readableVersion(payload, implemented); err != nil {
		return card{}, err
	}
	fields, ok := Fields(file, claim)
	if !ok {
		return card{}, fmt.Errorf("the card has no readable body")
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
func (c card) parsed(formatID string, pictures []format.Media) format.Parsed {
	return format.Parsed{
		Kind:      Kind,
		Format:    formatID,
		Name:      c.name(),
		Blurb:     c.blurb(),
		Tags:      c.tags(),
		Facets:    c.facets(),
		Media:     pictures,
		CreatedAt: c.createdAt(),
	}
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
func Modules() []format.Module {
	return []format.Module{CCv2Module{}, CCv3Module{}, CharXModule{}}
}
