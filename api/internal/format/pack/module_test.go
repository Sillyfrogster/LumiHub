package pack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/google/uuid"
)

const samplePack = `{
	"packName":"Archive companions",
	"packAuthor":"A creator",
	"coverUrl":"https://images.example/pack.png",
	"version":2,
	"packExtras":[{"type":"note","name":"Read first","description":"Kept whole."}],
	"lumiaItems":[{
		"lumiaName":"Archivist",
		"lumiaDefinition":"A guide to a quiet archive.",
		"lumiaPersonality":"Patient and exact.",
		"lumiaBehavior":"Answers with citations.",
		"avatarUrl":"https://images.example/avatar.png",
		"genderIdentity":2,
		"authorName":"A creator",
		"version":3,
		"futureField":{"kept":true}
	}],
	"loomItems":[],
	"futureTop":"kept"
}`

func TestPackModuleDeclaresTheLumiverseContract(t *testing.T) {
	module := Module{}
	declaration := module.Declaration()
	if declaration.ID != ID || declaration.Kind != Kind ||
		!declaration.Direction.Read || !declaration.Direction.Write {
		t.Fatalf("declaration = %+v, want the read-and-write Pack module", declaration)
	}
	if err := format.ValidateDeclaration(declaration); err != nil {
		t.Fatalf("declaration is incomplete: %v", err)
	}
	if support := declaration.Roles[block.RolePackItems]; support.Read.Grade != format.SupportFull || support.Write.Grade != format.SupportFull {
		t.Errorf("pack item support = %+v, want full in both directions", support)
	}
	wantSlots := []format.SlotDeclaration{
		{Name: "lumiaName", Type: format.ValueString},
		{Name: "lumiaDefinition", Type: format.ValueString},
		{Name: "lumiaPersonality", Type: format.ValueString},
		{Name: "lumiaBehavior", Type: format.ValueString},
		{Name: "avatarUrl", Type: format.ValueString},
		{Name: "genderIdentity", Type: format.ValueNumber},
		{Name: "authorName", Type: format.ValueString},
		{Name: "version", Type: format.ValueNumber},
	}
	if !reflect.DeepEqual(declaration.Slots, wantSlots) {
		t.Errorf("declared Lumia fields = %+v, want %+v", declaration.Slots, wantSlots)
	}
	if _, ok := any(module).(format.Writer); !ok {
		t.Fatal("the Pack module declares writing but has no writer")
	}
}

func TestPackReadsItemsWithoutFetchingImagesAndWritesPreservedFieldsBack(t *testing.T) {
	parsed := parse(t, []byte(samplePack))
	if parsed.Header.Name != "Archive companions" ||
		parsed.Header.CreditedAuthor != "A creator" || parsed.Header.AssetVersion != "2" {
		t.Errorf("header = %+v", parsed.Header)
	}
	if len(parsed.Media) != 0 {
		t.Fatalf("Pack import tried to bring in %d external images", len(parsed.Media))
	}
	records := packRecords(t, parsed)
	if len(records.Records) != 1 || records.Records[0].LumiaName != "Archivist" ||
		records.Records[0].AvatarURL != nil {
		t.Fatalf("Pack items = %+v", records)
	}

	written := write(t, format.ExportAsset{
		Kind: Kind, Header: parsed.Header, Elements: parsed.Elements, Preserved: parsed.Remainder,
	})
	var document map[string]json.RawMessage
	if err := json.Unmarshal(written.Body, &document); err != nil {
		t.Fatalf("decode written Pack: %v", err)
	}
	if string(document["coverUrl"]) != `"https://images.example/pack.png"` ||
		!bytes.Contains(document["packExtras"], []byte(`"Read first"`)) ||
		string(document["futureTop"]) != `"kept"` {
		t.Errorf("written Pack lost top-level preserved data: %s", written.Body)
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(document["lumiaItems"], &items); err != nil || len(items) != 1 {
		t.Fatalf("decode written Lumia items: %v / %s", err, document["lumiaItems"])
	}
	if string(items[0]["avatarUrl"]) != `"https://images.example/avatar.png"` ||
		!bytes.Contains(items[0]["futureField"], []byte(`"kept":true`)) {
		t.Errorf("written Lumia lost its preserved fields: %s", document["lumiaItems"])
	}
}

func TestPackWritesIllarinCoverAndItemImages(t *testing.T) {
	parsed := parse(t, []byte(samplePack))
	records := packRecords(t, parsed)
	mediaID := uuid.New()
	records.Records[0].AvatarURL = &mediaID
	parsed.Elements[0].Content = records

	written := write(t, format.ExportAsset{
		Kind: Kind, Header: parsed.Header, Elements: parsed.Elements, Preserved: parsed.Remainder,
		Cover: &format.ExportMedia{URL: "https://illarin.test/media/cover"},
		Images: map[uuid.UUID]format.ExportMedia{
			mediaID: {URL: "https://illarin.test/media/item"},
		},
	})
	var document struct {
		CoverURL   string `json:"coverUrl"`
		LumiaItems []struct {
			AvatarURL string `json:"avatarUrl"`
		} `json:"lumiaItems"`
	}
	if err := json.Unmarshal(written.Body, &document); err != nil {
		t.Fatalf("decode written Pack: %v", err)
	}
	if document.CoverURL != "https://illarin.test/media/cover" ||
		len(document.LumiaItems) != 1 ||
		document.LumiaItems[0].AvatarURL != "https://illarin.test/media/item" {
		t.Errorf("written image URLs = %+v", document)
	}
}

func TestMalformedOptionalFieldsRoundTripUntilTheirModeledValueChanges(t *testing.T) {
	parsed := parse(t, []byte(`{
		"packName":"Archive companions",
		"packAuthor":false,
		"version":{"future":true},
		"lumiaItems":[{
			"lumiaName":"Archivist",
			"lumiaDefinition":["kept"],
			"lumiaPersonality":false,
			"lumiaBehavior":5,
			"avatarUrl":{"future":true},
			"genderIdentity":9,
			"authorName":["kept"],
			"version":0
		}],
		"loomItems":[]
	}`))

	assertMalformed := func(t *testing.T, written format.Artifact) {
		t.Helper()
		var document map[string]json.RawMessage
		if err := json.Unmarshal(written.Body, &document); err != nil {
			t.Fatalf("decode written Pack: %v", err)
		}
		if string(document["packAuthor"]) != "false" ||
			string(document["version"]) != `{"future":true}` {
			t.Errorf("malformed Pack header fields were replaced: %s", written.Body)
		}
		var items []map[string]json.RawMessage
		if err := json.Unmarshal(document["lumiaItems"], &items); err != nil || len(items) != 1 {
			t.Fatalf("decode written Lumia items: %v / %s", err, document["lumiaItems"])
		}
		want := map[string]string{
			"lumiaDefinition":  `["kept"]`,
			"lumiaPersonality": "false",
			"lumiaBehavior":    "5",
			"avatarUrl":        `{"future":true}`,
			"genderIdentity":   "9",
			"authorName":       `["kept"]`,
			"version":          "0",
		}
		for key, expected := range want {
			if got := string(items[0][key]); got != expected {
				t.Errorf("%s = %s, want preserved %s", key, got, expected)
			}
		}
	}

	assertMalformed(t, write(t, format.ExportAsset{
		Kind: Kind, Header: parsed.Header, Elements: parsed.Elements, Preserved: parsed.Remainder,
	}))

	records := packRecords(t, parsed)
	records.Records[0].LumiaDefinition = "Now modeled."
	records.Records[0].GenderIdentity = 1
	records.Records[0].Version = 4
	parsed.Elements[0].Content = records
	written := write(t, format.ExportAsset{
		Kind: Kind, Header: parsed.Header, Elements: parsed.Elements, Preserved: parsed.Remainder,
	})
	var document struct {
		Items []struct {
			Definition string `json:"lumiaDefinition"`
			Gender     int    `json:"genderIdentity"`
			Version    int    `json:"version"`
		} `json:"lumiaItems"`
	}
	if err := json.Unmarshal(written.Body, &document); err != nil || len(document.Items) != 1 {
		t.Fatalf("decode edited Lumia item: %v / %s", err, written.Body)
	}
	if document.Items[0].Definition != "Now modeled." ||
		document.Items[0].Gender != 1 || document.Items[0].Version != 4 {
		t.Errorf("edited modeled fields did not replace preservation: %+v", document.Items[0])
	}
}

func packRecords(t *testing.T, parsed format.Parsed) block.RecordList {
	t.Helper()
	for _, element := range parsed.Elements {
		if element.Role == block.RolePackItems {
			records, ok := element.Content.(block.RecordList)
			if ok {
				return records
			}
		}
	}
	t.Fatal("Pack parse returned no record list")
	return block.RecordList{}
}

func parse(t *testing.T, data []byte) format.Parsed {
	t.Helper()
	file, err := probe.Inspect(
		context.Background(), memoryStore{data: data}, uuid.New(), int64(len(data)), "pack.json",
	)
	if err != nil {
		t.Fatalf("inspect Pack: %v", err)
	}
	module := Module{}
	claim, ok := module.Claim(file)
	if !ok {
		t.Fatal("the Pack signature did not claim the file")
	}
	parsed, err := module.Parse(context.Background(), file, claim)
	if err != nil {
		t.Fatalf("parse Pack: %v", err)
	}
	return parsed
}

func write(t *testing.T, asset format.ExportAsset) format.Artifact {
	t.Helper()
	written, err := (Module{}).Write(context.Background(), asset)
	if err != nil {
		t.Fatalf("write Pack: %v", err)
	}
	return written
}

type memoryStore struct{ data []byte }

func (store memoryStore) ReadRange(
	_ context.Context,
	_ uuid.UUID,
	offset int64,
	length int64,
) (io.ReadCloser, error) {
	if offset < 0 || offset+length > int64(len(store.data)) {
		return nil, errors.New("range outside the Pack")
	}
	return io.NopCloser(bytes.NewReader(store.data[offset : offset+length])), nil
}
