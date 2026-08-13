package character

import (
	"encoding/json"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

func TestCCv2UsesTheSpecRepresentationWithoutShadowFields(t *testing.T) {
	file := probe.Inspection{Payloads: []probe.Payload{{
		ID: 0,
		Root: object(`{
			"spec":"chara_card_v2",
			"data":{"description":"canonical"},
			"description":"shadow",
			"personality":"shadow only"
		}`),
	}}}

	claim, ok := CCv2(file)
	if !ok {
		t.Fatal("CCv2 did not claim its own spec")
	}
	fields, ok := Fields(file, claim)
	if !ok {
		t.Fatal("selected CCv2 representation is missing")
	}
	if got := stringField(t, fields, "description"); got != "canonical" {
		t.Fatalf("description = %q, want canonical", got)
	}
	if _, merged := fields["personality"]; merged {
		t.Fatal("a top-level shadow field was merged into the spec representation")
	}
}

func TestCCv2UsesLegacyShapeOnlyWithoutARecognizedSpec(t *testing.T) {
	legacy := object(`{
		"name":"Legacy",
		"description":"Description",
		"personality":"Personality",
		"scenario":"Scenario",
		"first_mes":"Hello"
	}`)

	if _, ok := CCv2(probe.Inspection{Payloads: []probe.Payload{{ID: 0, Root: legacy}}}); !ok {
		t.Fatal("CCv2 did not make a compatibility claim for a legacy shape")
	}

	legacy["spec"] = json.RawMessage(`"chara_card_v3"`)
	file := probe.Inspection{Payloads: []probe.Payload{{ID: 0, Root: legacy}}}
	if _, ok := CCv2(file); ok {
		t.Fatal("CCv2 claimed legacy shadow fields beside a recognized CCv3 spec")
	}
	if _, ok := CCv3(file); !ok {
		t.Fatal("CCv3 did not claim its own spec")
	}
}

func object(source string) map[string]json.RawMessage {
	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(source), &root); err != nil {
		panic(err)
	}
	return root
}

func stringField(t *testing.T, root map[string]json.RawMessage, name string) string {
	t.Helper()
	var value string
	if err := json.Unmarshal(root[name], &value); err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return value
}
