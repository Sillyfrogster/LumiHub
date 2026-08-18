package character

import (
	"bytes"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/google/uuid"
)

// Every key of extensions is a namespace of its own, so listing what an asset
// preserves never means opening a payload and reading around inside it.
func TestEachExtensionKeyIsPreservedAsItsOwnNamespace(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"extensions":{
				"chub":{"full_path":"ana/quiet"},
				"depth_prompt":{"depth":4,"prompt":"","role":"system"},
				"talkativeness":"0.5"
			}}
	}`)
	preserved := namespaces(resolveAndParse(t, file))

	for _, want := range []string{"chub", "depth_prompt", "talkativeness"} {
		if _, ok := preserved[want]; !ok {
			t.Errorf("%s was not preserved as its own namespace", want)
		}
	}
	if _, ok := preserved["extensions"]; ok {
		t.Error("the whole extensions object was preserved as one namespace")
	}
	if got := string(preserved["chub"]); got != `{"full_path":"ana/quiet"}` {
		t.Errorf("chub payload = %s, want the key's value verbatim", got)
	}
}

// Nothing accepted is discarded. A value that records nothing is stored like
// any other, and no filter runs over emptiness at import.
func TestValuesThatRecordNothingAreStoredAnyway(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"extensions":{"depth_prompt":{"depth":4,"prompt":"","role":"system"},
				"fav":false,"world":"","talkativeness":"0.5"}}
	}`)
	preserved := namespaces(resolveAndParse(t, file))

	for _, want := range []string{"depth_prompt", "fav", "world", "talkativeness"} {
		if _, ok := preserved[want]; !ok {
			t.Errorf("%s records nothing and was dropped rather than stored", want)
		}
	}

	// The same four are what the panel leaves out, which is display and not
	// storage.
	declaration := CCv3Module{}.Declaration()
	for namespace, payload := range preserved {
		if namespace == cardNamespace {
			continue
		}
		if !declaration.RecordsNothing(namespace, payload) {
			t.Errorf("%s is boilerplate and the panel would still show it", namespace)
		}
	}
}

// A namespace holding something the creator chose keeps its place in the panel.
func TestBoilerplateHidesOnlyWhatRecordsNothing(t *testing.T) {
	declaration := CCv3Module{}.Declaration()
	cases := []struct {
		namespace string
		payload   string
		hidden    bool
	}{
		{"world", `""`, true},
		{"world", `"Anna and Liz"`, false},
		{"fav", `false`, true},
		{"fav", `true`, false},
		{"talkativeness", `"0.5"`, true},
		{"talkativeness", `0.5`, true},
		{"talkativeness", `"0.9"`, false},
		{"depth_prompt", `{"depth":4,"prompt":"","role":"system"}`, true},
		{"depth_prompt", `{"depth":4,"prompt":"Stay in character","role":"system"}`, false},
		{"chub", `{}`, false},
	}
	for _, item := range cases {
		got := declaration.RecordsNothing(item.namespace, []byte(item.payload))
		if got != item.hidden {
			t.Errorf("%s carrying %s: hidden = %v, want %v",
				item.namespace, item.payload, got, item.hidden)
		}
	}
}

// A lorebook Illarin holds as content has no preserved second copy, wherever
// the card kept it. This is the 2.84 MB of duplicated books.
func TestALorebookHeldAsContentIsNotPreservedASecondTime(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"extensions":{"character_book":{"name":"Ana's world","scan_depth":4,
				"entries":[{"keys":["ledger"],"content":"A debt.","uid":91}]}}}
	}`)
	parsed := resolveAndParse(t, file)

	content, ok := elementContent(parsed.Elements, block.RoleLorebookEntries)
	if !ok {
		t.Fatal("the book inside extensions did not become content")
	}
	entries := content.(block.EntryTable).Entries
	if len(entries) != 1 || entries[0].Text != "A debt." {
		t.Fatalf("entries = %+v, want the book read as content", entries)
	}
	for _, remainder := range parsed.Remainder {
		if bytes.Contains(remainder.Payload, []byte(`"A debt."`)) {
			t.Errorf("%s preserved a second copy of the book: %s",
				remainder.Namespace, remainder.Payload)
		}
	}
}

// The half Illarin understands becomes content and only the rest is preserved,
// keyed against the entry that carried it.
func TestAHalfUnderstoodBookSplitsRatherThanDuplicates(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"character_book":{"name":"Ana's world","scan_depth":4,
				"entries":[{"keys":["ledger"],"content":"A debt.","uid":91,"probability":75}]}}
	}`)
	parsed := resolveAndParse(t, file)
	content, _ := elementContent(parsed.Elements, block.RoleLorebookEntries)
	entry := content.(block.EntryTable).Entries[0]

	var bookRow, entryRow format.Remainder
	for _, remainder := range parsed.Remainder {
		switch remainder.Owner {
		case format.OwnerElement:
			bookRow = remainder
		case format.OwnerItem:
			entryRow = remainder
		}
	}
	if bookRow.Namespace != "character_book" ||
		!bytes.Contains(bookRow.Payload, []byte(`"scan_depth":4`)) ||
		bytes.Contains(bookRow.Payload, []byte(`"entries"`)) {
		t.Errorf("book remainder = %+v, want the book's own keys without its entries", bookRow)
	}
	if bookRow.OwnerID == uuid.Nil {
		t.Error("the book's remainder names no element")
	}
	// A format's own identifier is preserved data like anything else.
	if entryRow.OwnerID != entry.ID ||
		!bytes.Contains(entryRow.Payload, []byte(`"uid":91`)) ||
		!bytes.Contains(entryRow.Payload, []byte(`"probability":75`)) {
		t.Errorf("entry remainder = %+v, want it keyed against entry %s", entryRow, entry.ID)
	}
	if bytes.Contains(entryRow.Payload, []byte(`"content"`)) {
		t.Error("the entry preserved the text it had already become")
	}
}

// Preserved data keys against an id, so the entry beside a deleted one never
// inherits its fields.
func TestEachEntryGetsItsOwnIDAndItsOwnPreservedFields(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"character_book":{"entries":[
				{"keys":["one"],"content":"First","uid":1},
				{"keys":["two"],"content":"Second","uid":2}]}}
	}`)
	parsed := resolveAndParse(t, file)
	content, _ := elementContent(parsed.Elements, block.RoleLorebookEntries)
	entries := content.(block.EntryTable).Entries

	if entries[0].ID == uuid.Nil || entries[0].ID == entries[1].ID {
		t.Fatalf("entry ids = %s and %s", entries[0].ID, entries[1].ID)
	}
	byEntry := make(map[uuid.UUID][]byte)
	for _, remainder := range parsed.Remainder {
		if remainder.Owner == format.OwnerItem {
			byEntry[remainder.OwnerID] = remainder.Payload
		}
	}
	if !bytes.Contains(byEntry[entries[0].ID], []byte(`"uid":1`)) ||
		!bytes.Contains(byEntry[entries[1].ID], []byte(`"uid":2`)) {
		t.Errorf("entry ids and preserved uids do not line up: %v", byEntry)
	}
}

// A card whose extensions carry a key named for the card body would ask for
// two namespaces of one name. It travels back out whole instead.
func TestAnExtensionNamedForTheCardBodyStaysInsideIt(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Hello",
			"extensions":{"card":{"odd":true},"chub":{"full_path":"ana/quiet"}}}
	}`)
	preserved := namespaces(resolveAndParse(t, file))

	if !bytes.Contains(preserved[cardNamespace], []byte(`"odd":true`)) {
		t.Errorf("card namespace = %s, want the clashing key kept inside it",
			preserved[cardNamespace])
	}
	if _, ok := preserved["chub"]; !ok {
		t.Error("the clash cost the namespace beside it")
	}
}

// namespaces reads the asset-owned rows into the panel's own view of them.
func namespaces(parsed format.Parsed) map[string][]byte {
	found := make(map[string][]byte)
	for _, remainder := range parsed.Remainder {
		if remainder.Owner == format.OwnerAsset {
			found[remainder.Namespace] = remainder.Payload
		}
	}
	return found
}

// Every item a reader produces carries an id from the moment it is read, not
// from the first time a creator saves over it. Preserved data keys against
// these ids, and two items sharing the nil id would be one owner.
func TestEveryImportedItemArrivesWithAnIDOfItsOwn(t *testing.T) {
	file := jsonCard(t, `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Ana","description":"Quiet","first_mes":"Welcome back.",
			"alternate_greetings":["You again.","Still here?"],
			"group_only_greetings":["All of you made it."],
			"mes_example":"<START>\nAna: Sit down.\nYou: I brought the ledger.",
			"character_book":{"entries":[{"keys":["ledger"],"content":"A debt."}]}}
	}`)

	seen := make(map[uuid.UUID]block.Role)
	for _, element := range resolveAndParse(t, file).Elements {
		ids := block.ItemIDs(element.Content)
		for index, id := range ids {
			if id == uuid.Nil {
				t.Errorf("%s item %d arrived with no id", element.Role, index+1)
				continue
			}
			if owner, taken := seen[id]; taken {
				t.Errorf("%s item %d shares an id with %s", element.Role, index+1, owner)
			}
			seen[id] = element.Role
		}
	}
	if len(seen) < 7 {
		t.Errorf("read %d items with ids, want every greeting, turn and entry", len(seen))
	}
}
