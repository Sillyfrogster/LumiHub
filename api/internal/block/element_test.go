package block

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAWriterStampsTheCurrentSchemaVersion(t *testing.T) {
	written, err := json.Marshal(Element{
		ID: uuid.New(), Type: TypeProse, Role: RoleDescription, Slot: "top",
		Options: Options{Display: DisplayRich},
		Content: Prose{Text: "She keeps the ledger and the ledger keeps her."},
	})
	if err != nil {
		t.Fatalf("write element: %v", err)
	}

	var stored struct {
		Version int             `json:"version"`
		Options Options         `json:"options"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(written, &stored); err != nil {
		t.Fatalf("read stored element: %v", err)
	}
	if stored.Version != schemas[TypeProse].version() {
		t.Errorf("stored version = %d, want the current %d",
			stored.Version, schemas[TypeProse].version())
	}
	if stored.Options.Display != DisplayRich {
		t.Errorf("display option = %q, want rich", stored.Options.Display)
	}

	var read Element
	if err := json.Unmarshal(written, &read); err != nil {
		t.Fatalf("read element: %v", err)
	}
	prose, ok := read.Content.(Prose)
	if !ok {
		t.Fatalf("content read back as %T, want prose", read.Content)
	}
	if prose.Text != "She keeps the ledger and the ledger keeps her." {
		t.Errorf("text = %q, want what was written", prose.Text)
	}
}

func TestContentFromANewerBuildIsRefusedRatherThanRead(t *testing.T) {
	stored := `{"id":"` + uuid.New().String() + `","type":"prose","slot":"top",` +
		`"version":99,"options":{},"content":{"text":"From the future."}}`

	var element Element
	err := json.Unmarshal([]byte(stored), &element)
	if err == nil {
		t.Fatalf("content at an unknown schema version was read as if it were current")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("refusal = %q, want it to name the version it found", err)
	}
}

func TestAReaderUpgradesContentWrittenAtAnOlderVersion(t *testing.T) {
	// Two versions of one element type: the first held a bare string, the
	// second holds the object every reader wants.
	versioned := schema{
		upgrade: []func(json.RawMessage) (json.RawMessage, error){
			func(old json.RawMessage) (json.RawMessage, error) {
				var text string
				if err := json.Unmarshal(old, &text); err != nil {
					return nil, err
				}
				return json.Marshal(Prose{Text: text})
			},
		},
		empty:  func() Content { return Prose{} },
		decode: decodeAs[Prose],
	}

	if versioned.version() != 2 {
		t.Fatalf("current version = %d, want one past the last upgrade", versioned.version())
	}

	content, err := versioned.read(1, json.RawMessage(`"An older shape."`))
	if err != nil {
		t.Fatalf("read version 1: %v", err)
	}
	if content.(Prose).Text != "An older shape." {
		t.Errorf("upgraded content = %+v, want the text carried forward", content)
	}

	current, err := versioned.read(2, json.RawMessage(`{"text":"The current shape."}`))
	if err != nil {
		t.Fatalf("read version 2: %v", err)
	}
	if current.(Prose).Text != "The current shape." {
		t.Errorf("current content = %+v, want it read without an upgrade", current)
	}
}

func TestAnEmptyElementCarriesNothingAReaderWouldSee(t *testing.T) {
	for _, elementType := range []Type{TypeProse, TypeTextSet, TypeDialogueSample, TypeImageSet} {
		content, err := elementType.Empty()
		if err != nil {
			t.Fatalf("empty %s: %v", elementType, err)
		}
		if !content.Empty() {
			t.Errorf("a new %s element already holds something", elementType)
		}
	}

	filled := TextSet{Texts: []TextItem{{Name: "Alternate", Text: "You again."}}}
	if filled.Empty() {
		t.Errorf("a text set holding a greeting reads as empty")
	}
	blank := TextSet{Texts: []TextItem{{Name: "Alternate"}}}
	if !blank.Empty() {
		t.Errorf("a text set holding only a name reads as filled")
	}
}
