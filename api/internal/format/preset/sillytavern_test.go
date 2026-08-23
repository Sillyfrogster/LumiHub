package preset

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
)

// The two flat SillyTavern files are told apart by required keys they do not
// share. The theme's are the colour and blur it always carries.
func TestTheSillyTavernSignatureIsDisjointFromTheThemes(t *testing.T) {
	themeKeys := []string{"main_text_color", "blur_strength"}
	recognition := (SillyTavernModule{}).Declaration().Recognition
	if len(recognition) != 1 || recognition[0].Kind != format.RecognitionSignature {
		t.Fatalf("recognition = %+v, want one structural signature", recognition)
	}
	required := recognition[0].Required
	if len(required) != 2 {
		t.Fatalf("required keys = %v, want the prompt list and its order", required)
	}
	for _, key := range themeKeys {
		if _, shared := required[key]; shared {
			t.Errorf("the preset signature requires %q, which is a theme's key", key)
		}
	}
	// A theme is not claimed by the preset module.
	theme := document(t, `{"name":"Glimmer","main_text_color":"rgba(1,1,1,1)","blur_strength":8}`)
	if _, claimed := (SillyTavernModule{}).Claim(theme); claimed {
		t.Error("the preset module claimed a SillyTavern theme")
	}
}

// Ordering is the difference the two modules each answer in their own code.
// SillyTavern holds it in a structure keyed by character, so the list's own
// order is not the order the file sends.
func TestTheSillyTavernOrderDecidesTheFragmentsAndTheirSwitches(t *testing.T) {
	parsed := parse(t, sillyTavernPreset)
	if parsed.Format != SillyTavernID {
		t.Fatalf("parsed format = %q", parsed.Format)
	}
	list := promptList(t, parsed.Elements)
	if len(list.Fragments) != 4 {
		t.Fatalf("read %d fragments, want 4", len(list.Fragments))
	}
	names := make([]string, 0, len(list.Fragments))
	for _, fragment := range list.Fragments {
		names = append(names, fragment.Name)
	}
	want := []string{"Author note", "| Prompt", "Chat History", "Not in the order"}
	if !slices.Equal(names, want) {
		t.Errorf("fragments came out as %v, want %v", names, want)
	}
	if list.Fragments[0].Enabled {
		t.Error("a fragment the order switches off came out switched on")
	}
	if !list.Fragments[1].Enabled || !list.Fragments[2].Enabled {
		t.Error("a fragment the order switches on came out switched off")
	}
	if list.Fragments[3].Enabled {
		t.Error("a fragment the order never names came out switched on")
	}
	if note := list.Fragments[0]; note.Placement != block.InHistory ||
		note.Depth == nil || *note.Depth != 2 {
		t.Errorf("the note = %+v, want it placed in the history at a depth", note)
	}
	if marker := list.Fragments[2]; marker.Marker != "chatHistory" {
		t.Errorf("marker = %q, want what the file holds a place for", marker.Marker)
	}

	// The other character's order is somebody else's copy and stays whole.
	body := preservedPayload(t, parsed.Remainder, format.OwnerAsset, sillyTavernNamespace)
	var others []map[string]any
	if err := json.Unmarshal(body["prompt_order"], &others); err != nil {
		t.Fatalf("read the preserved order: %v", err)
	}
	if len(others) != 1 || others[0]["character_id"] != float64(100000) {
		t.Errorf("preserved orders = %+v, want the one this app does not read", others)
	}
	if _, held := body["topP"]; !held {
		t.Error("a name belonging to the other preset format was read rather than preserved")
	}
}
