package block

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

// ValidateStructure checks whether a block can be read without special cases.
func ValidateStructure(holder Block) error {
	available := holder.Layout.Slots()
	if len(available) == 0 {
		return fmt.Errorf("choose a known layout before saving")
	}
	occupied := make(map[Slot]Role, len(holder.Elements))
	identities := make(map[uuid.UUID]string, len(holder.Elements))
	for i, element := range holder.Elements {
		name := element.Role.Label()
		if name == "" {
			name = fmt.Sprintf("Element %d", i+1)
		}
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
		if !slices.Contains(available, element.Slot) {
			return fmt.Errorf(
				"%s is in slot %q, but %s has only %s. Move it to one of those slots before saving",
				name, element.Slot, holder.Layout, joinSlots(available),
			)
		}
		if prior, exists := occupied[element.Slot]; exists {
			priorName := prior.Label()
			if priorName == "" {
				priorName = "Another element"
			}
			return fmt.Errorf(
				"%s and %s both use slot %q. Move one before saving",
				priorName, name, element.Slot,
			)
		}
		occupied[element.Slot] = element.Role
	}
	return nil
}

// ValidateBuilderConstraints checks the kind catalog's rules across an asset.
func ValidateBuilderConstraints(kind string, before []Block, after []Block) error {
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
			name := element.Role.Label()
			if name == "" {
				name = fmt.Sprintf("Element %d", i+1)
			}
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

func joinSlots(values []Slot) string {
	names := make([]string, len(values))
	for i, value := range values {
		names[i] = string(value)
	}
	return strings.Join(names, ", ")
}
