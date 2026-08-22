package block

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

const (
	MaxPayloadBytes    = 8 << 20
	MaxCollectionItems = 5000
	MaxItemBytes       = 8 << 20
)

// ValidateContentLimits applies the catalog ceiling on every route that saves
// element content, including import and hand editing.
func ValidateContentLimits(elements []Element) error {
	for _, element := range elements {
		items := elementItems(element.Content)
		if len(items) > MaxCollectionItems {
			return fmt.Errorf(
				"%s has %d items; the limit is %d",
				element.Role.Label(), len(items), MaxCollectionItems,
			)
		}
		for index, item := range items {
			encoded, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("measure %s item %d: %w", element.Role.Label(), index+1, err)
			}
			if len(encoded) > MaxItemBytes {
				return fmt.Errorf(
					"%s item %d has %d bytes; the limit is %d",
					element.Role.Label(), index+1, len(encoded), MaxItemBytes,
				)
			}
		}
		payload, err := json.Marshal(element.Content)
		if err != nil {
			return fmt.Errorf("measure %s: %w", element.Role.Label(), err)
		}
		if len(payload) > MaxPayloadBytes {
			return fmt.Errorf(
				"%s payload has %d bytes; the limit is %d",
				element.Role.Label(), len(payload), MaxPayloadBytes,
			)
		}
	}
	return nil
}

// validateItemIDs refuses an element whose items have no id of their own.
// Preserved data keys against these ids, so two items sharing one id would be
// one owner and an item with none could never own anything.
func validateItemIDs(element Element, name string) error {
	seen := make(map[uuid.UUID]struct{})
	for index, id := range ItemIDs(element.Content) {
		if id == uuid.Nil {
			return fmt.Errorf("%s item %d must keep an id before saving", name, index+1)
		}
		if _, taken := seen[id]; taken {
			return fmt.Errorf(
				"two items in %s use the same id. Give each one its own before saving", name,
			)
		}
		seen[id] = struct{}{}
	}
	return nil
}

// elementItems returns everything inside one element that a limit is measured
// against. It is the one place the item types are listed, so a count and the
// items themselves can never disagree.
func elementItems(content Content) []any {
	var items []any
	switch value := content.(type) {
	case TextSet:
		for _, item := range value.Texts {
			items = append(items, item)
		}
	case DialogueSample:
		for _, item := range value.Turns {
			items = append(items, item)
		}
	case ImageSet:
		for _, item := range value.Images {
			items = append(items, item)
		}
	case FieldList:
		for _, item := range value.Fields {
			items = append(items, item)
		}
	case LinkList:
		for _, item := range value.Links {
			items = append(items, item)
		}
	case EntryTable:
		for _, item := range value.Entries {
			items = append(items, item)
		}
	case PromptList:
		for _, item := range value.Groups {
			items = append(items, item)
		}
		for _, item := range value.Fragments {
			items = append(items, item)
		}
	case VariableSchema:
		for _, item := range value.Variables {
			items = append(items, item)
		}
	case SettingGroup:
		for _, item := range value.Settings {
			items = append(items, item)
		}
	case ScriptList:
		for _, item := range value.Scripts {
			items = append(items, item)
		}
	case ColorSet:
		for _, mode := range value.Modes {
			for _, item := range mode.Colors {
				items = append(items, item)
			}
		}
	case StylesheetSet:
		for _, item := range value.Stylesheets {
			items = append(items, item)
		}
		for _, item := range value.Assets {
			items = append(items, item)
		}
	case RecordList:
		for _, item := range value.Records {
			items = append(items, item)
		}
	}
	return items
}

// ValidateStructure checks whether a block can be read without special cases.
func ValidateStructure(holder Block) error {
	if err := ValidateContentLimits(holder.Elements); err != nil {
		return err
	}
	available := holder.Layout.Slots()
	if len(available) == 0 {
		return fmt.Errorf("choose a known layout before saving")
	}
	if len(holder.Elements) > len(available) {
		stranded := make([]string, 0, len(holder.Elements)-len(available))
		for i, element := range holder.Elements[len(available):] {
			stranded = append(stranded, elementName(element, len(available)+i))
		}
		return fmt.Errorf(
			"%s has no room for %s in %s. Move or remove %s before changing the layout",
			holder.Definition, strings.Join(stranded, ", "), holder.Layout,
			strings.ToLower(strings.Join(stranded, ", ")),
		)
	}
	occupied := make(map[Slot]string, len(holder.Elements))
	identities := make(map[uuid.UUID]string, len(holder.Elements))
	for i, element := range holder.Elements {
		name := elementName(element, i)
		if !element.Type.Known() {
			return fmt.Errorf("%s uses unknown element type %q", name, element.Type)
		}
		if element.ID == uuid.Nil {
			return fmt.Errorf("%s must keep a valid element id before saving", name)
		}
		if prior, exists := identities[element.ID]; exists {
			return fmt.Errorf(
				"%s and %s use the same id. Give each element its own id before saving",
				prior, name,
			)
		}
		identities[element.ID] = name
		if err := validateItemIDs(element, name); err != nil {
			return err
		}
		supportsDisplay := element.Type == TypeProse || element.Type == TypeTextSet
		if supportsDisplay {
			if !element.Options.Display.Known() {
				return fmt.Errorf(
					"%s uses display %q. Choose rich or verbatim before saving",
					name, element.Options.Display,
				)
			}
		} else if element.Options.Display != "" {
			return fmt.Errorf(
				"%s does not support a display option. Remove it before saving",
				name,
			)
		}
		if element.Type == TypeImageSet {
			if !element.Options.ItemSize.Known() {
				return fmt.Errorf(
					"%s draws its images at %q. Choose %s before saving",
					name, element.Options.ItemSize, joinItemSizes(),
				)
			}
		} else if element.Options.ItemSize != "" {
			return fmt.Errorf(
				"%s holds no images, so it takes no image size. Remove it before saving",
				name,
			)
		}
		if !slices.Contains(available, element.Slot) {
			return fmt.Errorf(
				"%s is in slot %q, but %s has only %s. Move it to one of those slots before saving",
				name, element.Slot, holder.Layout, joinSlots(available),
			)
		}
		if prior, exists := occupied[element.Slot]; exists {
			return fmt.Errorf(
				"%s and %s both use slot %q. Move one before saving",
				prior, name, element.Slot,
			)
		}
		occupied[element.Slot] = name
	}
	return nil
}

// ValidateBuilderConstraints checks the kind catalog's rules across an asset.
func ValidateBuilderConstraints(kind string, before []Block, after []Block) error {
	beforeByID := make(map[uuid.UUID]Block, len(before))
	for _, holder := range before {
		beforeByID[holder.ID] = holder
	}
	seen := make(map[DefinitionID]struct{}, len(after))
	for _, holder := range after {
		definition, ok := holder.Definition.Definition(kind)
		if !ok {
			return fmt.Errorf("%s is not part of the %s catalog", holder.Definition, kind)
		}
		if _, repeated := seen[holder.Definition]; repeated && !definition.Repeatable {
			return fmt.Errorf(
				"%s can be on this page once. Remove the second one before saving",
				definition.Title,
			)
		}
		seen[holder.Definition] = struct{}{}
		if !slices.Contains(definition.Layouts, holder.Layout) {
			available := make([]string, len(definition.Layouts))
			for i, layout := range definition.Layouts {
				available[i] = string(layout)
			}
			return fmt.Errorf(
				"%s can use %s. Choose one of those layouts before saving",
				definition.Title, strings.Join(available, " or "),
			)
		}
		if holder.Width.Columns() == 0 {
			return fmt.Errorf("choose full, two thirds, half or a third before saving")
		}
		minimum := holder.Layout.MinimumWidth()
		if holder.Width.Columns() >= minimum.Columns() {
			continue
		}
		original := beforeByID[holder.ID]
		switch {
		case original.Layout != holder.Layout && original.Width == holder.Width:
			return fmt.Errorf(
				"%s needs %s, and this section is %s. Widen it first",
				holder.Layout, minimum.label(), holder.Width.label(),
			)
		case original.Layout == holder.Layout && original.Width != holder.Width:
			return fmt.Errorf(
				"%s needs %s, and this section is %s. Choose another layout before narrowing it",
				holder.Layout, minimum.label(), holder.Width.label(),
			)
		default:
			return fmt.Errorf(
				"%s needs %s, and this section is %s. Change one before saving",
				holder.Layout, minimum.label(), holder.Width.label(),
			)
		}
	}

	roleCount := make(map[Role]int)
	elementIDs := make(map[uuid.UUID]string)
	existingIDs := make(map[uuid.UUID]struct{})
	for _, holder := range before {
		for _, element := range holder.Elements {
			existingIDs[element.ID] = struct{}{}
		}
	}
	for _, holder := range after {
		for i, element := range holder.Elements {
			name := elementName(element, i)
			if prior, exists := elementIDs[element.ID]; exists {
				return fmt.Errorf(
					"%s and %s use the same id. Give each element its own id before saving",
					prior, name,
				)
			}
			if _, exists := existingIDs[element.ID]; !exists {
				return fmt.Errorf("%s must keep its existing id before saving", name)
			}
			elementIDs[element.ID] = name
			if element.Role == "" {
				continue
			}
			if element.Role.Allows(element.Type) {
				roleCount[element.Role]++
				if element.Role.Cardinality() == Singular && roleCount[element.Role] > 1 {
					return fmt.Errorf(
						"%s can appear only once. Remove the extra %s before saving",
						element.Role.Label(), strings.ToLower(element.Role.Label()),
					)
				}
				continue
			}
			allowed := element.Role.AllowedTypes()
			types := make([]string, len(allowed))
			for j, allowedType := range allowed {
				types[j] = string(allowedType)
			}
			if len(types) == 0 {
				return fmt.Errorf("%s uses an unknown role. Remove that role before saving", name)
			}
			return fmt.Errorf(
				"%s must use %s content. Change its element type before saving",
				name, strings.Join(types, " or "),
			)
		}
	}
	for _, originalBlock := range before {
		definition, ok := originalBlock.Definition.Definition(kind)
		if !ok {
			continue
		}
		for _, original := range originalBlock.Elements {
			if !originalBlock.Pinned(original.Role, kind) {
				continue
			}
			if pinnedElementPresent(after, originalBlock.ID, original) {
				continue
			}
			return fmt.Errorf(
				"%s is pinned to %s. Restore it in that block before saving",
				original.Role.Label(), definition.Title,
			)
		}
	}
	return nil
}

func pinnedElementPresent(blocks []Block, blockID uuid.UUID, original Element) bool {
	for _, holder := range blocks {
		if holder.ID != blockID {
			continue
		}
		for _, candidate := range holder.Elements {
			if candidate.ID == original.ID && candidate.Role == original.Role && candidate.Type == original.Type {
				return true
			}
		}
	}
	return false
}

// elementName is what a refusal calls one element. The position stands in only
// where the element type is not one Illarin knows.
func elementName(element Element, index int) string {
	if name := element.Label(); name != "" {
		return name
	}
	return fmt.Sprintf("Element %d", index+1)
}

func joinItemSizes() string {
	sizes := ItemSizes()
	names := make([]string, len(sizes))
	for i, size := range sizes {
		names[i] = string(size)
	}
	return joinWithOr(names)
}

// joinWithOr writes a closed vocabulary the way a refusal reads it, so a
// creator is told every value they may choose instead.
func joinWithOr(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
	}
}

func joinSlots(values []Slot) string {
	names := make([]string, len(values))
	for i, value := range values {
		names[i] = string(value)
	}
	return strings.Join(names, ", ")
}
