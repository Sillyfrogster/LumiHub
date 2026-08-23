package character

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format/keys"
)

func TestEveryPreservedKeyComesBackByteIdentical(t *testing.T) {
	source := `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{
			"name":"Ana","description":"Quiet","first_mes":"Hello",
			"tags":["archivist"],"creation_date":1717200000,
			"character_book":{"name":"Ana's world","scan_depth":4,"token_budget":512,
				"entries":[
					{"keys":["ledger"],"content":"A debt.","uid":91,"probability":75},
					{"keys":["mira"],"content":"A name.","uid":92,"group":"people"}]},
			"extensions":{
				"chub":{"full_path":"ana/quiet","related_lorebooks":[]},
				"risuai":{"customScripts":[],"bias":null},
				"depth_prompt":{"depth":4,"prompt":"","role":"system"},
				"talkativeness":"0.5","fav":false,"world":""
			}
		}
	}`
	parsed := resolveAndParse(t, jsonCard(t, source))
	content, ok := elementContent(parsed.Elements, block.RoleLorebookEntries)
	if !ok {
		t.Fatal("the book did not become content")
	}
	entries := content.(block.EntryTable).Entries

	// The creator edits an unrelated block. Nothing here touches the book or
	// the namespaces the card arrived with.
	written := map[string]json.RawMessage{
		"name":        json.RawMessage(`"Ana"`),
		"description": json.RawMessage(`"Quieter than she was."`),
		"first_mes":   json.RawMessage(`"Hello"`),
		"character_book": bookAsWritten(t, entries,
			map[string]json.RawMessage{"name": json.RawMessage(`"Ana's world"`)}),
	}
	if err := RestorePreserved(written, entries, parsed.Remainder); err != nil {
		t.Fatalf("restore preserved data: %v", err)
	}

	original := cardBody(t, source)
	for _, key := range []string{"tags", "creation_date"} {
		if !reflect.DeepEqual(compact(t, written[key]), compact(t, original[key])) {
			t.Errorf("%s = %s, want %s", key, written[key], original[key])
		}
	}
	for namespace, value := range keys.Object(original["extensions"]) {
		got := keys.Object(written["extensions"])[namespace]
		if !reflect.DeepEqual(compact(t, got), compact(t, value)) {
			t.Errorf("%s = %s, want %s byte for byte", namespace, got, value)
		}
	}
	book := keys.Object(written["character_book"])
	for _, key := range []string{"scan_depth", "token_budget"} {
		if !reflect.DeepEqual(compact(t, book[key]),
			compact(t, keys.Object(original["character_book"])[key])) {
			t.Errorf("the book's %s did not come back: %s", key, book[key])
		}
	}
	var restored []map[string]json.RawMessage
	if err := json.Unmarshal(book["entries"], &restored); err != nil {
		t.Fatalf("read the restored entries: %v", err)
	}
	if string(restored[0]["uid"]) != "91" || string(restored[0]["probability"]) != "75" {
		t.Errorf("entry 1 came back as %v", restored[0])
	}
	if string(restored[1]["uid"]) != "92" || string(restored[1]["group"]) != `"people"` {
		t.Errorf("entry 2 came back as %v", restored[1])
	}
	// The creator's own edit stands. A preserved copy never overwrites it.
	if string(written["description"]) != `"Quieter than she was."` {
		t.Errorf("description = %s, want the creator's edit", written["description"])
	}
}

// Deleting an entry takes its preserved keys with it, and the entry beside it
// keeps its own.
func TestADeletedEntryTakesItsPreservedKeysWithIt(t *testing.T) {
	source := `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"character_book":{"entries":[
				{"keys":["one"],"content":"First","uid":1},
				{"keys":["two"],"content":"Second","uid":2}]}}
	}`
	parsed := resolveAndParse(t, jsonCard(t, source))
	content, _ := elementContent(parsed.Elements, block.RoleLorebookEntries)
	entries := content.(block.EntryTable).Entries

	kept := entries[1:]
	written := map[string]json.RawMessage{
		"character_book": bookAsWritten(t, kept, map[string]json.RawMessage{}),
	}
	if err := RestorePreserved(written, kept, parsed.Remainder); err != nil {
		t.Fatalf("restore preserved data: %v", err)
	}
	var restored []map[string]json.RawMessage
	if err := json.Unmarshal(keys.Object(written["character_book"])["entries"], &restored); err != nil {
		t.Fatalf("read the restored entries: %v", err)
	}
	if len(restored) != 1 {
		t.Fatalf("restored %d entries, want the one that survived", len(restored))
	}
	if string(restored[0]["uid"]) != "2" {
		t.Errorf("the surviving entry came back with uid %s, want its own", restored[0]["uid"])
	}
}

// bookAsWritten is the book a writer builds from the entries a creator can
// edit, before any preserved key goes back into it.
func bookAsWritten(
	t *testing.T,
	entries []block.Entry,
	book map[string]json.RawMessage,
) json.RawMessage {
	t.Helper()
	written := make([]map[string]json.RawMessage, len(entries))
	for index, entry := range entries {
		keys, err := json.Marshal(entry.Keys)
		if err != nil {
			t.Fatalf("write entry keys: %v", err)
		}
		text, err := json.Marshal(entry.Text)
		if err != nil {
			t.Fatalf("write entry text: %v", err)
		}
		written[index] = map[string]json.RawMessage{"keys": keys, "content": text}
	}
	entriesJSON, err := json.Marshal(written)
	if err != nil {
		t.Fatalf("write entries: %v", err)
	}
	book["entries"] = entriesJSON
	body, err := json.Marshal(book)
	if err != nil {
		t.Fatalf("write book: %v", err)
	}
	return body
}

func cardBody(t *testing.T, source string) map[string]json.RawMessage {
	t.Helper()
	var root struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(source), &root); err != nil {
		t.Fatalf("read the source card: %v", err)
	}
	return root.Data
}

// compact reads a value into its own shape, so two spellings of one JSON value
// compare as the same value.
func compact(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("read %s: %v", raw, err)
	}
	return value
}

// v1 wrote lumihub_art_display into 36 cards and the name is theirs now, so the rename leaves it alone and this holds the line.
func TestTheGrandfatheredArtDisplayKeyComesBackByteIdentical(t *testing.T) {
	source := `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{
			"name":"Ana","description":"Quiet","first_mes":"Hello",
			"extensions":{"lumihub_art_display":"avatar"}
		}
	}`
	parsed := resolveAndParse(t, jsonCard(t, source))
	written := map[string]json.RawMessage{
		"name":        json.RawMessage(`"Ana"`),
		"description": json.RawMessage(`"Quieter than she was."`),
		"first_mes":   json.RawMessage(`"Hello"`),
	}
	if err := RestorePreserved(written, nil, parsed.Remainder); err != nil {
		t.Fatalf("restore preserved data: %v", err)
	}
	got := keys.Object(written["extensions"])["lumihub_art_display"]
	if !reflect.DeepEqual(compact(t, got), compact(t, json.RawMessage(`"avatar"`))) {
		t.Errorf("lumihub_art_display = %s, want \"avatar\" byte for byte", got)
	}
}
