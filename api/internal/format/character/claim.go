package character

import (
	"encoding/json"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

const (
	V2    = "chara_card_v2"
	V3    = "chara_card_v3"
	CharX = "charx"
)

// A card standard says which containers may carry it. CCv2 and CCv3 define a
// JSON document and a raster image with the card in a text chunk; CharX defines
// the archive. So neither of the first two reads a payload out of an archive,
// and CharX reads nothing else. This is the standards talking, not a filename.
func isDocument(payload probe.Payload) bool {
	return payload.Locator.Container != probe.ZIP
}

func isArchived(payload probe.Payload) bool {
	return payload.Locator.Container == probe.ZIP
}

// CCv2 claims its own spec, and otherwise a card written before any spec
// existed. The legacy shape is only consulted where no recognised spec is
// present, so a card carrying both representations never merges them.
func CCv2(file probe.Inspection) (format.Claim, bool) {
	if claim, ok := authoritativeClaim(file, V2); ok {
		return claim, true
	}
	if hasRecognizedSpec(file) {
		return format.Claim{}, false
	}
	for _, payload := range file.Payloads {
		if isDocument(payload) && hasLegacyShape(payload.Root) {
			return format.CompatibilityClaim(payload), true
		}
	}
	return format.Claim{}, false
}

func CCv3(file probe.Inspection) (format.Claim, bool) {
	return authoritativeClaim(file, V3)
}

// CharXClaim reads the archive the CharX standard defines. The card inside
// declares chara_card_v3, because CharX is a container for a CCv3 card, which
// is why the module owns that spec rather than one of its own.
func CharXClaim(file probe.Inspection) (format.Claim, bool) {
	for _, payload := range file.Payloads {
		if !isArchived(payload) {
			continue
		}
		if spec, _ := payload.String("spec"); spec == V3 {
			return format.AuthoritativeClaim(payload, "spec")
		}
	}
	return format.Claim{}, false
}

func authoritativeClaim(file probe.Inspection, formatID string) (format.Claim, bool) {
	for _, payload := range file.Payloads {
		if !isDocument(payload) {
			continue
		}
		if spec, _ := payload.String("spec"); spec == formatID {
			return format.AuthoritativeClaim(payload, "spec")
		}
	}
	return format.Claim{}, false
}

// Fields returns the claimed payload's spec-defined body. A spec-bearing card
// keeps its fields under `data`, so top-level copies of the same names are
// shadow fields and are never merged in.
func Fields(file probe.Inspection, claim format.Claim) (map[string]json.RawMessage, bool) {
	payload, ok := claim.Payload(file)
	if !ok {
		return nil, false
	}
	spec, _ := payload.String("spec")
	if spec != V2 && spec != V3 {
		return payload.Root, true
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload.Root["data"], &fields); err != nil || fields == nil {
		return nil, false
	}
	return fields, true
}

func hasRecognizedSpec(file probe.Inspection) bool {
	for _, payload := range file.Payloads {
		if !isDocument(payload) {
			continue
		}
		spec, _ := payload.String("spec")
		if spec == V2 || spec == V3 {
			return true
		}
	}
	return false
}

func hasLegacyShape(root map[string]json.RawMessage) bool {
	for _, field := range []string{"name", "description", "personality", "scenario", "first_mes"} {
		if _, ok := root[field]; !ok {
			return false
		}
	}
	return true
}
