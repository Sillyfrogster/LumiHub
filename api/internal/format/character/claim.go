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

func hasLegacyShape(root map[string]json.RawMessage) bool {
	for _, field := range []string{"name", "description", "personality", "scenario", "first_mes"} {
		if _, ok := root[field]; !ok {
			return false
		}
	}
	return true
}
