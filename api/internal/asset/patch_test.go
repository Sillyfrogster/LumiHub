package asset

import (
	"bytes"
	"context"
	"encoding/json"
	"image/color"
	"io"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/character"
)

func TestCreatorFilePatchPersistsAcrossARevisionAndOverridesTheNewSource(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %q: %v", module.ID(), err)
		}
	}
	svc, _ := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "patch.owner")
	created := ingestOne(t, svc, ownerID, "card.json", []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"Original name","creator_notes":"Original notes","description":"First source"}
	}`))

	if err := svc.SetFilePatch(context.Background(), FilePatchInput{
		OwnerID: ownerID,
		AssetID: created.ID,
		Patch:   format.Patch{format.FieldDescription: "Creator override"},
	}); err != nil {
		t.Fatalf("SetFilePatch: %v", err)
	}
	assertExportedDescription(t, svc, created.ID, "sillytavern", "Creator override")

	operation := addRevision(t, svc, ownerID, created.ID, "card.json", []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0",
		"data":{"name":"New source name","creator_notes":"New source notes","description":"Second source"}
	}`))
	if operation.Asset == nil {
		t.Fatalf("revision did not finish: %+v", operation)
	}
	if operation.Asset.Name != created.Name || operation.Asset.Blurb != created.Blurb {
		t.Fatalf("catalog metadata was re-seeded: %+v", operation.Asset)
	}
	assertExportedDescription(t, svc, created.ID, "risu", "Creator override")
}

func TestReconciliationPatchStopsAtItsRevision(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %q: %v", module.ID(), err)
		}
	}
	svc, _ := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "reconciliation.owner")
	created := ingestOne(t, svc, ownerID, "card.json", []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0","data":{"description":"Historical source"}
	}`))
	if err := svc.setReconciliationPatch(context.Background(), created.ID, created.CurrentRevisionID,
		format.Patch{format.FieldDescription: "Reconciled value"}); err != nil {
		t.Fatalf("set reconciliation patch: %v", err)
	}
	assertExportedDescription(t, svc, created.ID, "lumiverse", "Reconciled value")

	operation := addRevision(t, svc, ownerID, created.ID, "card.json", []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0","data":{"description":"Genuine revision"}
	}`))
	if operation.Asset == nil {
		t.Fatalf("revision did not finish: %+v", operation)
	}
	assertExportedDescription(t, svc, created.ID, "lumiverse", "Genuine revision")
}

func TestUnsupportedExportTargetFallsBackToRaw(t *testing.T) {
	svc, _ := newTestService(t)
	ownerID := revisionOwner(t, svc, "raw.owner")
	source := []byte{0, 255, 12, 42}
	created := ingestOne(t, svc, ownerID, "opaque.lumitheme", source)

	exported, err := svc.OpenExport(context.Background(), created.ID, nil, "not-a-target")
	if err != nil {
		t.Fatalf("OpenExport: %v", err)
	}
	defer exported.Artifact.Close()
	written, err := io.ReadAll(exported.Artifact)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if exported.Target != format.RawTarget || !bytes.Equal(written, source) {
		t.Fatalf("fallback = target %q bytes %v, want raw %v", exported.Target, written, source)
	}
}

func TestOpenExportCarriesMediaTheTargetCannotEmbed(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %q: %v", module.ID(), err)
		}
	}
	svc, _ := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "export.media.owner")
	created := ingestOne(t, svc, ownerID, "card.json", []byte(`{
		"spec":"chara_card_v3","spec_version":"3.0","data":{"description":"Source"}
	}`))
	picture := testPNG(t, 2, 2, color.White)
	if _, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: ownerID, AssetID: created.ID, Role: MediaGallery, File: bytes.NewReader(picture),
	}); err != nil {
		t.Fatalf("AddMedia: %v", err)
	}

	exported, err := svc.OpenExport(context.Background(), created.ID, nil, "risu")
	if err != nil {
		t.Fatalf("OpenExport: %v", err)
	}
	defer exported.Artifact.Close()
	if len(exported.UnembeddedMedia) != 1 ||
		exported.UnembeddedMedia[0].Role != MediaGallery ||
		!bytes.Equal(exported.UnembeddedMedia[0].Data, picture) {
		t.Fatalf("unembedded media = %+v, want the creator gallery image", exported.UnembeddedMedia)
	}
}

func assertExportedDescription(t *testing.T, svc *Service, assetID [16]byte, target, want string) {
	t.Helper()
	exported, err := svc.OpenExport(context.Background(), assetID, nil, target)
	if err != nil {
		t.Fatalf("OpenExport: %v", err)
	}
	defer exported.Artifact.Close()
	written, err := io.ReadAll(exported.Artifact)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	var card struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(written, &card); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	var description string
	if err := json.Unmarshal(card.Data["description"], &description); err != nil {
		t.Fatalf("decode description: %v", err)
	}
	if description != want {
		t.Fatalf("description = %q, want %q", description, want)
	}
}
