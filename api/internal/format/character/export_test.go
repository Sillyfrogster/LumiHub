package character

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
)

func TestEveryCharacterModuleAcceptsOnlyItsClosedPatchFields(t *testing.T) {
	wanted := format.Patch{
		format.FieldDescription:             "New description",
		format.FieldPersonality:             "New personality",
		format.FieldScenario:                "New scenario",
		format.FieldFirstMessage:            "New greeting",
		format.FieldSystemPrompt:            "New system prompt",
		format.FieldPostHistoryInstructions: "New post-history instructions",
		format.FieldCreatorNotes:            "New creator notes",
		format.FieldCharacterVersion:        "2.1",
	}

	for _, module := range Modules() {
		t.Run(module.ID(), func(t *testing.T) {
			patcher, ok := module.(format.Patcher)
			if !ok {
				t.Fatalf("module %q has no patch interface", module.ID())
			}
			if err := patcher.ValidatePatch(wanted); err != nil {
				t.Fatalf("the required character fields were refused: %v", err)
			}
			if err := patcher.ValidatePatch(format.Patch{"extensions.depth_prompt": "lost"}); err == nil {
				t.Fatal("an arbitrary JSON path was accepted")
			}
		})
	}
}

func TestPNGExportPreservesEveryChunkExceptTheCardChunks(t *testing.T) {
	picture := testPNG(t)
	end := len(picture) - 12
	source := slices.Concat(
		picture[:end],
		pngChunk("tEXt", []byte("note\x00keep these bytes")),
		pngChunk("tEXt", slices.Concat([]byte("chara\x00"), []byte(`legacy shadow`))),
		pngChunk("tEXt", slices.Concat([]byte("ccv3\x00"), []byte(`eyJzcGVjIjoiY2hhcmFfY2FyZF92MyIsInNwZWNfdmVyc2lvbiI6IjMuMCIsImRhdGEiOnsiZGVzY3JpcHRpb24iOiJCZWZvcmUifX0=`))),
		pngChunk("vpAg", []byte{0, 9, 8, 7, 6}),
		picture[end:],
	)

	exported, err := CCv3Module{}.Export(context.Background(), format.ExportRequest{
		Source: bytes.NewReader(source),
		Target: "risu",
		Patch:  format.Patch{format.FieldDescription: "After"},
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	got, err := io.ReadAll(exported.Artifact)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if before, after := nonCardChunks(t, source), nonCardChunks(t, got); !slices.Equal(before, after) {
		t.Fatal("a non-card PNG chunk changed")
	}

	file := inspect(t, got, "patched.png")
	claim := claimFor(t, CCv3Module{}, file)
	fields, ok := Fields(file, claim)
	if !ok {
		t.Fatal("the exported PNG has no CCv3 body")
	}
	if description := stringField(t, fields, "description"); description != "After" {
		t.Fatalf("description = %q, want After", description)
	}
}

func TestPNGExportUsesTheCardDiscriminatorInsteadOfItsChunkName(t *testing.T) {
	picture := testPNG(t)
	end := len(picture) - 12
	card := base64.StdEncoding.EncodeToString([]byte(`{
		"spec":"chara_card_v2","spec_version":"2.0","data":{"description":"Before"}
	}`))
	source := slices.Concat(
		picture[:end],
		pngChunk("tEXt", slices.Concat([]byte("ccv3\x00"), []byte(card))),
		picture[end:],
	)

	exported, err := CCv2Module{}.Export(context.Background(), format.ExportRequest{
		Source: bytes.NewReader(source), Target: "sillytavern",
		Patch: format.Patch{format.FieldDescription: "After"},
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	written, _ := io.ReadAll(exported.Artifact)
	file := inspect(t, written, "patched.png")
	claim := claimFor(t, CCv2Module{}, file)
	fields, ok := Fields(file, claim)
	if !ok || stringField(t, fields, "description") != "After" {
		t.Fatal("the V2 card in the V3-named chunk was not patched")
	}
}

func nonCardChunks(t *testing.T, source []byte) []string {
	t.Helper()
	var chunks []string
	for offset := 8; offset < len(source); {
		if offset+12 > len(source) {
			t.Fatal("truncated PNG chunk")
		}
		length := int(binary.BigEndian.Uint32(source[offset : offset+4]))
		end := offset + 12 + length
		if end > len(source) {
			t.Fatal("PNG chunk exceeds the file")
		}
		kind := string(source[offset+4 : offset+8])
		data := source[offset+8 : offset+8+length]
		keyword, _, _ := bytes.Cut(data, []byte{0})
		if kind != "tEXt" || string(keyword) != "chara" && string(keyword) != "ccv3" {
			chunks = append(chunks, string(source[offset:end]))
		}
		offset = end
	}
	return chunks
}

func TestCCv3JSONExportLeavesUntouchedExtensionsByteIdentical(t *testing.T) {
	source := []byte(`{
		"spec":"chara_card_v3",
		"spec_version":"3.0",
		"data":{
			"name":"Ana",
			"description":"Before",
			"extensions": { "risu": {"order": [3, 1, 2]}, "chub": { "id": 17 } }
		}
	}`)
	wantExtensions := rawField(t, rawField(t, source, "data"), "extensions")

	exported, err := CCv3Module{}.Export(context.Background(), format.ExportRequest{
		Source: bytes.NewReader(source),
		Target: "sillytavern",
		Patch:  format.Patch{format.FieldDescription: "After"},
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	got, err := io.ReadAll(exported.Artifact)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	data := rawField(t, got, "data")
	if description := stringField(t, decodeObject(t, data), "description"); description != "After" {
		t.Fatalf("description = %q, want After", description)
	}
	if extensions := rawField(t, data, "extensions"); !bytes.Equal(extensions, wantExtensions) {
		t.Fatalf("extensions changed\n got: %s\nwant: %s", extensions, wantExtensions)
	}
}

func TestCharXExportKeepsEmbededPathsAndMergesCreatorMedia(t *testing.T) {
	card := `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{
			"description":"Before",
			"assets":[{"type":"icon","uri":"embeded://assets/icon/images/main.png","name":"main","ext":"png"}]
		}
	}`
	source := charXBytes(t, card, map[string][]byte{
		"assets/icon/images/main.png": testPNG(t),
		"platform.json":               []byte(`{"keep": true}`),
	})
	managed := testPNG(t)
	exported, err := CharXModule{}.Export(context.Background(), format.ExportRequest{
		Source: bytes.NewReader(source),
		Target: "lumiverse",
		Patch:  format.Patch{format.FieldDescription: "After"},
		Media: []format.ExportMedia{{
			Role: "expression", MediaType: "image/png", Data: managed,
		}},
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	written, err := io.ReadAll(exported.Artifact)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	entries := readCharX(t, written)
	data := rawField(t, entries["card.json"], "data")
	if description := stringField(t, decodeObject(t, data), "description"); description != "After" {
		t.Fatalf("description = %q, want After", description)
	}
	var assets []cardAsset
	if err := json.Unmarshal(rawField(t, data, "assets"), &assets); err != nil {
		t.Fatalf("read assets: %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("assets = %+v, want source and managed pictures", assets)
	}
	for _, asset := range assets {
		if strings.HasPrefix(asset.URI, "embedded://") {
			t.Fatalf("CHARX path was corrected to the wrong spelling: %q", asset.URI)
		}
		if !strings.HasPrefix(asset.URI, embeddedPrefix) {
			t.Fatalf("CHARX path = %q, want %s", asset.URI, embeddedPrefix)
		}
	}
	managedPath := strings.TrimPrefix(assets[1].URI, embeddedPrefix)
	if !bytes.Equal(entries[managedPath], managed) {
		t.Fatal("managed media bytes were not embedded at the path in card.json")
	}
	if !bytes.Equal(entries["platform.json"], []byte(`{"keep": true}`)) {
		t.Fatal("a non-card CHARX entry changed")
	}
	if len(exported.UnembeddedMedia) != 0 {
		t.Fatalf("CHARX left %d media files outside the archive", len(exported.UnembeddedMedia))
	}
	if assets[1].Name != "media-1" || !strings.HasSuffix(assets[1].URI, "/media-1.png") {
		t.Fatalf("managed media did not get an artifact-local identity: %+v", assets[1])
	}
}

func TestCharXConvertsToCCv3JSONAndCCv2PNG(t *testing.T) {
	picture := testPNG(t)
	card := `{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"description":"Before","assets":[
			{"type":"icon","uri":"embeded://assets/icon/images/main.png","name":"main","ext":"png"}
		]}
	}`
	source := charXBytes(t, card, map[string][]byte{"assets/icon/images/main.png": picture})

	jsonExport, err := CharXModule{}.Export(context.Background(), format.ExportRequest{
		Source: bytes.NewReader(source), Target: targetCCv3JSON,
		Patch: format.Patch{format.FieldDescription: "JSON target"},
	})
	if err != nil {
		t.Fatalf("CCv3 JSON export: %v", err)
	}
	jsonBytes, _ := io.ReadAll(jsonExport.Artifact)
	data := rawField(t, jsonBytes, "data")
	if got := stringField(t, decodeObject(t, data), "description"); got != "JSON target" {
		t.Fatalf("JSON description = %q", got)
	}
	if !bytes.Contains(rawField(t, data, "assets"), []byte(`"data:image/png;base64,`)) {
		t.Fatal("CCv3 JSON did not inline archive media")
	}

	pngExport, err := CharXModule{}.Export(context.Background(), format.ExportRequest{
		Source: bytes.NewReader(source), Target: targetCCv2PNG,
		Patch: format.Patch{format.FieldDescription: "PNG target"},
	})
	if err != nil {
		t.Fatalf("CCv2 PNG export: %v", err)
	}
	pngBytes, _ := io.ReadAll(pngExport.Artifact)
	file := inspect(t, pngBytes, "converted.png")
	claim := claimFor(t, CCv2Module{}, file)
	fields, ok := Fields(file, claim)
	if !ok || stringField(t, fields, "description") != "PNG target" {
		t.Fatal("CCv2 PNG did not contain the converted patched card")
	}
	if !bytes.Contains(fields["assets"], []byte(`"data:image/png;base64,`)) {
		t.Fatal("CCv2 PNG card did not carry the archive media")
	}
}

func TestEveryCharacterModuleProducesEveryTargetItDeclares(t *testing.T) {
	tests := []struct {
		module format.Module
		source []byte
	}{
		{module: CCv2Module{}, source: []byte(`{"spec":"chara_card_v2","spec_version":"2.0","data":{"description":"Before"}}`)},
		{module: CCv3Module{}, source: []byte(`{"spec":"chara_card_v3","spec_version":"3.0","data":{"description":"Before"}}`)},
		{module: CharXModule{}, source: charXBytes(t,
			`{"spec":"chara_card_v3","spec_version":"3.0","data":{"description":"Before"}}`, nil)},
	}
	for _, test := range tests {
		t.Run(test.module.ID(), func(t *testing.T) {
			exporter, ok := test.module.(format.Exporter)
			if !ok {
				t.Fatalf("module %q has no export interface", test.module.ID())
			}
			declarer := test.module.(format.ExportTargetDeclarer)
			for _, target := range declarer.ExportTargets() {
				exported, err := exporter.Export(context.Background(), format.ExportRequest{
					Source: bytes.NewReader(test.source), Target: target.Value,
					Patch: format.Patch{format.FieldDescription: target.Value},
				})
				if err != nil {
					t.Errorf("target %q: %v", target.Value, err)
					continue
				}
				written, err := io.ReadAll(exported.Artifact)
				if err != nil || len(written) == 0 {
					t.Errorf("target %q produced no artifact: %v", target.Value, err)
				}
			}
		})
	}
}

func TestLegacyCardIgnoresAPatchFieldItsFormatDoesNotHave(t *testing.T) {
	source := []byte(`{
		"name":"Legacy","description":"Before","personality":"Quiet",
		"scenario":"A room","first_mes":"Hello"
	}`)
	exported, err := CCv2Module{}.Export(context.Background(), format.ExportRequest{
		Source: bytes.NewReader(source), Target: "sillytavern",
		Patch: format.Patch{format.FieldSystemPrompt: "This field has no V1 target"},
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	written, _ := io.ReadAll(exported.Artifact)
	if _, added := decodeObject(t, written)["system_prompt"]; added {
		t.Fatal("a patch whose target does not exist was added to the card")
	}
}

func rawField(t *testing.T, object []byte, field string) json.RawMessage {
	t.Helper()
	return decodeObject(t, object)[field]
}

func decodeObject(t *testing.T, source []byte) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal(source, &object); err != nil {
		t.Fatalf("decode JSON object: %v", err)
	}
	return object
}

func charXBytes(t *testing.T, card string, entries map[string][]byte) []byte {
	t.Helper()
	var output bytes.Buffer
	archive := zip.NewWriter(&output)
	writeZipEntry(t, archive, "card.json", []byte(card))
	for name, data := range entries {
		writeZipEntry(t, archive, name, data)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close CHARX: %v", err)
	}
	return output.Bytes()
}

func writeZipEntry(t *testing.T, archive *zip.Writer, name string, data []byte) {
	t.Helper()
	entry, err := archive.Create(name)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readCharX(t *testing.T, source []byte) map[string][]byte {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	if err != nil {
		t.Fatalf("open CHARX: %v", err)
	}
	entries := make(map[string][]byte, len(archive.File))
	for _, entry := range archive.File {
		opened, err := entry.Open()
		if err != nil {
			t.Fatalf("open %s: %v", entry.Name, err)
		}
		data, err := io.ReadAll(opened)
		closeErr := opened.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("read %s: %v, close: %v", entry.Name, err, closeErr)
		}
		entries[entry.Name] = data
	}
	return entries
}
