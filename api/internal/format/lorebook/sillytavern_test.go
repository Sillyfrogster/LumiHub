package lorebook

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
)

// worldInfo is what SillyTavern's own export writes: entries keyed by their
// position in the list, each carrying the names SillyTavern uses.
const worldInfo = `{
	"entries": {
		"0": {
			"uid": 0,
			"key": ["Timeline", "2167"],
			"keysecondary": ["Eridu"],
			"comment": "Story Timeline",
			"content": "It is widely believed the story takes place in 2167.",
			"constant": true,
			"selective": true,
			"order": 100,
			"position": 0,
			"disable": false,
			"excludeRecursion": true,
			"preventRecursion": false,
			"delayUntilRecursion": true,
			"probability": 100,
			"useProbability": true,
			"depth": 4
		},
		"1": {
			"uid": 1,
			"key": ["New Eridu"],
			"keysecondary": [],
			"comment": "The last city",
			"content": "The last city.",
			"constant": false,
			"selective": false,
			"caseSensitive": true,
			"order": 20,
			"position": 4,
			"disable": true
		}
	}
}`

// SillyTavern's export is told apart from the book a character card carries by
// the one thing that differs at the top level: its entries are keyed rather
// than listed. Neither file matches the other's module.
func TestASillyTavernWorldInfoFileIsRecognisedAsItsOwnFormat(t *testing.T) {
	registry := testRegistry(t)

	resolution, claimed, err := registry.Resolve(document(t, worldInfo))
	if err != nil || !claimed {
		t.Fatalf("resolve the world info file: claimed %v, %v", claimed, err)
	}
	if resolution.Module.ID() != SillyTavernID {
		t.Errorf("world info read as %q, want %q", resolution.Module.ID(), SillyTavernID)
	}

	resolution, claimed, err = registry.Resolve(document(t, twoEntries))
	if err != nil || !claimed {
		t.Fatalf("resolve the listed book: claimed %v, %v", claimed, err)
	}
	if resolution.Module.ID() != ID {
		t.Errorf("the listed book read as %q, want %q", resolution.Module.ID(), ID)
	}
}

// The keys are the whole point of an entry. SillyTavern spells them `key` and
// names the entry `comment`, and a book whose keys did not arrive is a book
// that never fires.
func TestSillyTavernEntriesKeepTheirKeysAndTheirNames(t *testing.T) {
	table := onlyEntryTable(t, parse(t, worldInfo).Elements)
	if len(table.Entries) != 2 {
		t.Fatalf("read %d entries, want two", len(table.Entries))
	}

	first := table.Entries[0]
	if !slices.Equal(first.Keys, []string{"Timeline", "2167"}) {
		t.Errorf("keys = %q, want the two the file carried", first.Keys)
	}
	if !slices.Equal(first.SecondaryKeys, []string{"Eridu"}) {
		t.Errorf("secondary keys = %q, want the one the file carried", first.SecondaryKeys)
	}
	if first.Name != "Story Timeline" {
		t.Errorf("name = %q, want the comment the file carried", first.Name)
	}
	if first.Text == "" {
		t.Error("the entry arrived with no text")
	}
	if !first.Constant || !first.Selective || first.Order != 100 {
		t.Errorf("entry = %+v, want its constant, selective and order kept", first)
	}
	if !first.Recursion.Exclude || first.Recursion.Prevent || !first.Recursion.DelayUntil {
		t.Errorf("recursion = %+v, want what the file said", first.Recursion)
	}
	if !table.Entries[1].CaseSensitive {
		t.Error("case sensitivity did not arrive")
	}
}

// SillyTavern switches an entry off and the card formats switch one on, so the
// two spellings mean the opposite of each other. Reading one as the other
// would publish a book with every switched-off entry live.
func TestSillyTavernDisableIsReadAsTheOppositeOfEnabled(t *testing.T) {
	table := onlyEntryTable(t, parse(t, worldInfo).Elements)
	if !table.Entries[0].Enabled {
		t.Error("an entry the file did not disable came back switched off")
	}
	if table.Entries[1].Enabled {
		t.Error("an entry the file disabled came back switched on")
	}

	table = onlyEntryTable(t, parse(t, `{"entries":{"0":{"key":["a"],"content":"x"}}}`).Elements)
	if !table.Entries[0].Enabled {
		t.Error("an entry that says nothing about being disabled came back switched off")
	}
}

// Illarin models before and after the character and nothing else. A placement
// it has no wording for is left unset and travels back out untouched rather
// than being rounded to the nearest one it does have.
func TestASillyTavernPlacementIllarinHasNoWordingForIsLeftAlone(t *testing.T) {
	parsed := parse(t, worldInfo)
	table := onlyEntryTable(t, parsed.Elements)

	if table.Entries[0].Position != block.BeforeCharacter {
		t.Errorf("position = %q, want before the character", table.Entries[0].Position)
	}
	if table.Entries[1].Position != "" {
		t.Errorf("position = %q, want it left unset", table.Entries[1].Position)
	}

	kept := entryLeftovers(t, parsed, table.Entries[1].ID)
	if string(kept["position"]) != "4" {
		t.Errorf("preserved position = %s, want the 4 the file wrote", kept["position"])
	}
}

// What SillyTavern carries that the entry table has no place for is preserved
// against the entry it came from, so a download puts it back where it was.
func TestWhatSillyTavernCarriesBeyondTheEntryTableIsPreserved(t *testing.T) {
	parsed := parse(t, worldInfo)
	table := onlyEntryTable(t, parsed.Elements)

	kept := entryLeftovers(t, parsed, table.Entries[0].ID)
	for _, name := range []string{"uid", "probability", "useProbability", "depth"} {
		if _, held := kept[name]; !held {
			t.Errorf("%s was not preserved: %v", name, kept)
		}
	}
	for _, consumed := range []string{"key", "comment", "content", "disable"} {
		if _, held := kept[consumed]; held {
			t.Errorf("%s became content and was preserved as well", consumed)
		}
	}
}

// A book that goes out in the format it came in comes back the same. The
// entries are keyed the way SillyTavern keys them, the names are its own, and
// everything Illarin never modelled is back on the entry it came from.
func TestARoundTripThroughSillyTavernComesBackTheSame(t *testing.T) {
	parsed := parse(t, worldInfo)
	table := onlyEntryTable(t, parsed.Elements)

	artifact, err := SillyTavernModule{}.Write(context.Background(), format.ExportAsset{
		Kind: Kind,
		Elements: []block.Element{{
			Role: block.RoleLorebookEntries, Type: block.TypeEntryTable, Content: table,
		}},
		Preserved: parsed.Remainder,
	})
	if err != nil {
		t.Fatalf("write the world info file: %v", err)
	}

	var written struct {
		Entries map[string]map[string]json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(artifact.Body, &written); err != nil {
		t.Fatalf("read the written file: %v", err)
	}
	if len(written.Entries) != 2 {
		t.Fatalf("wrote %d entries, want two keyed entries", len(written.Entries))
	}

	first, held := written.Entries["0"]
	if !held {
		t.Fatalf("entries are keyed %v, want them keyed by position", written.Entries)
	}
	var keys []string
	if err := json.Unmarshal(first["key"], &keys); err != nil {
		t.Fatalf("read the written keys: %v", err)
	}
	if !slices.Equal(keys, []string{"Timeline", "2167"}) {
		t.Errorf("written keys = %q, want the ones that came in", keys)
	}
	if string(first["comment"]) != `"Story Timeline"` {
		t.Errorf("written comment = %s, want the name that came in", first["comment"])
	}
	if string(first["disable"]) != "false" {
		t.Errorf("written disable = %s, want the switch back the way round it arrived",
			first["disable"])
	}
	if string(first["uid"]) != "0" {
		t.Errorf("written uid = %s, want the one preservation kept", first["uid"])
	}
	if string(written.Entries["1"]["position"]) != "4" {
		t.Errorf("written position = %s, want the placement preservation kept",
			written.Entries["1"]["position"])
	}
	if string(written.Entries["1"]["disable"]) != "true" {
		t.Errorf("written disable = %s, want the disabled entry still disabled",
			written.Entries["1"]["disable"])
	}
}

func entryLeftovers(
	t *testing.T,
	parsed format.Parsed,
	entry uuid.UUID,
) map[string]json.RawMessage {
	t.Helper()
	for _, row := range parsed.Remainder {
		if row.Owner == format.OwnerItem && row.OwnerID == entry {
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(row.Payload, &payload); err != nil {
				t.Fatalf("read the preserved entry: %v", err)
			}
			return payload
		}
	}
	t.Fatalf("nothing was preserved for entry %s", entry)
	return nil
}
