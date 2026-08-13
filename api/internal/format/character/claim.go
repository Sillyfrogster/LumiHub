package character

import (
	"encoding/json"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

const (
	V2 = "chara_card_v2"
	V3 = "chara_card_v3"
)

func CCv2(file probe.Inspection) (format.Claim, bool) {
	if claim, ok := authoritativeClaim(file, V2); ok {
		return claim, true
	}
	if hasRecognizedSpec(file) {
		return format.Claim{}, false
	}
	for _, payload := range file.Payloads {
		if hasLegacyShape(payload.Root) {
			return format.CompatibilityClaim(payload), true
		}
	}
	return format.Claim{}, false
}

func CCv3(file probe.Inspection) (format.Claim, bool) {
	return authoritativeClaim(file, V3)
}

func authoritativeClaim(file probe.Inspection, formatID string) (format.Claim, bool) {
	for _, payload := range file.Payloads {
		if spec, _ := payload.String("spec"); spec == formatID {
			return format.AuthoritativeClaim(payload, "spec")
		}
	}
	return format.Claim{}, false
}

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
