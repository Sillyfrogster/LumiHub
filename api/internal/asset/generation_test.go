package asset

import (
	"bytes"
	"context"
	"image/color"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// startedDraft is a character built from nothing, which is the shortest way to
// an asset with blocks whose content a test can move.
func startedDraft(t *testing.T, svc *Service) (uuid.UUID, uuid.UUID) {
	t.Helper()
	owner := uuid.New()
	draft, err := svc.StartFromNothing(context.Background(), owner, "character", "")
	if err != nil {
		t.Fatalf("start a draft: %v", err)
	}
	return owner, draft
}

func contentGeneration(t *testing.T, pool *pgxpool.Pool, assetID uuid.UUID) int {
	t.Helper()
	var generation int
	err := pool.QueryRow(context.Background(),
		`select content_generation from assets where id = $1`, assetID,
	).Scan(&generation)
	if err != nil {
		t.Fatalf("read the content generation: %v", err)
	}
	return generation
}

func draftBlocks(t *testing.T, pool *pgxpool.Pool, assetID uuid.UUID) []block.Block {
	t.Helper()
	blocks, err := readBlocks(context.Background(), pool, assetID)
	if err != nil {
		t.Fatalf("read the blocks: %v", err)
	}
	return blocks
}

func blockFor(t *testing.T, blocks []block.Block, definition block.DefinitionID) block.Block {
	t.Helper()
	for _, holder := range blocks {
		if holder.Definition == definition {
			return holder
		}
	}
	t.Fatalf("no %s block on the page", definition)
	return block.Block{}
}

func updateOf(holder block.Block) BlockUpdate {
	return BlockUpdate{
		Title: holder.Title, Layout: holder.Layout, Width: holder.Width,
		Elements: append([]block.Element(nil), holder.Elements...),
	}
}

func saveDescription(t *testing.T, svc *Service, owner, draft uuid.UUID, pool *pgxpool.Pool, text string) {
	t.Helper()
	core := blockFor(t, draftBlocks(t, pool, draft), "character_core")
	update := updateOf(core)
	update.Elements[0].Content = block.Prose{Text: text}
	if _, err := svc.SaveBlock(context.Background(), owner, draft, core.ID, update); err != nil {
		t.Fatalf("save the description: %v", err)
	}
}

func saveGreeting(t *testing.T, svc *Service, owner, draft uuid.UUID, pool *pgxpool.Pool, text string) {
	t.Helper()
	messages := blockFor(t, draftBlocks(t, pool, draft), "messages")
	update := updateOf(messages)
	update.Elements[0].Content = block.TextSet{
		Texts: []block.TextItem{{ID: block.NewItemID(), Text: text}},
	}
	if _, err := svc.SaveBlock(context.Background(), owner, draft, messages.ID, update); err != nil {
		t.Fatalf("save the greeting: %v", err)
	}
}

func TestAnAssetBuiltFromNothingStartsAtTheFirstContentGeneration(t *testing.T) {
	svc, pool := newTestService(t)
	_, draft := startedDraft(t, svc)

	if got := contentGeneration(t, pool, draft); got != 1 {
		t.Fatalf("content generation = %d, want 1", got)
	}
}

func TestEditingAnElementMovesTheContentGeneration(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)

	saveDescription(t, svc, owner, draft, pool, "She keeps the memories that books forget.")

	if got := contentGeneration(t, pool, draft); got != 2 {
		t.Fatalf("content generation = %d, want 2 after an edit", got)
	}
}

func TestSavingTheSameElementContentAgainLeavesTheContentGenerationAlone(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	saveDescription(t, svc, owner, draft, pool, "She keeps the memories that books forget.")
	before := contentGeneration(t, pool, draft)

	saveDescription(t, svc, owner, draft, pool, "She keeps the memories that books forget.")

	if got := contentGeneration(t, pool, draft); got != before {
		t.Fatalf("content generation = %d, want %d after saving the same words", got, before)
	}
}

func TestArrangingThePageLeavesTheContentGenerationAlone(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	saveDescription(t, svc, owner, draft, pool, "She keeps the memories that books forget.")
	before := contentGeneration(t, pool, draft)

	blocks := draftBlocks(t, pool, draft)
	arrangement := []BlockArrangement{
		{ID: blocks[1].ID, Hidden: false, Width: block.Half},
		{ID: blocks[0].ID, Hidden: false, Width: blocks[0].Width},
	}
	if _, err := svc.ArrangeBlocks(context.Background(), owner, draft, arrangement); err != nil {
		t.Fatalf("arrange the page: %v", err)
	}

	if got := contentGeneration(t, pool, draft); got != before {
		t.Fatalf("content generation = %d, want %d after a reorder and a width change", got, before)
	}
}

func TestChangingOnlyABlocksTitleAndLayoutLeavesTheContentGenerationAlone(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	saveDescription(t, svc, owner, draft, pool, "She keeps the memories that books forget.")
	before := contentGeneration(t, pool, draft)

	core := blockFor(t, draftBlocks(t, pool, draft), "character_core")
	update := updateOf(core)
	chosen := "Who she is"
	update.Title = &chosen
	update.Width = block.Half
	if _, err := svc.SaveBlock(context.Background(), owner, draft, core.ID, update); err != nil {
		t.Fatalf("save the block: %v", err)
	}

	if got := contentGeneration(t, pool, draft); got != before {
		t.Fatalf("content generation = %d, want %d after a title and width change", got, before)
	}
}

func TestRenamingAnAssetMovesTheContentGenerationAndTheAdultAnswerDoesNot(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)

	if err := svc.SetIdentity(context.Background(), Identity{
		OwnerID: owner, AssetID: draft, Name: "Ana",
	}); err != nil {
		t.Fatalf("name the draft: %v", err)
	}
	named := contentGeneration(t, pool, draft)
	if named != 2 {
		t.Fatalf("content generation = %d, want 2 after a name", named)
	}

	adult := true
	if err := svc.SetIdentity(context.Background(), Identity{
		OwnerID: owner, AssetID: draft, Name: "Ana", IsNSFW: &adult,
	}); err != nil {
		t.Fatalf("answer the adult content question: %v", err)
	}

	if got := contentGeneration(t, pool, draft); got != named {
		t.Fatalf("content generation = %d, want %d after only the adult content answer", got, named)
	}
}

func TestPublishingAndUnlistingLeaveTheContentGenerationAlone(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	saveDescription(t, svc, owner, draft, pool, "She keeps the memories that books forget.")
	saveGreeting(t, svc, owner, draft, pool, "The west shelf moved again. Come in.")
	adult := false
	if err := svc.SetIdentity(context.Background(), Identity{
		OwnerID: owner, AssetID: draft, Name: "Ana", IsNSFW: &adult,
	}); err != nil {
		t.Fatalf("fill in the header: %v", err)
	}
	before := contentGeneration(t, pool, draft)

	if _, err := svc.Publish(context.Background(), owner, draft); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := svc.SetDiscovery(context.Background(), owner, draft, DiscoveryUnlisted); err != nil {
		t.Fatalf("unlist: %v", err)
	}

	if got := contentGeneration(t, pool, draft); got != before {
		t.Fatalf("content generation = %d, want %d after publishing and unlisting", got, before)
	}
}

func TestDeletingAPreservedNamespaceMovesTheContentGeneration(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	if _, err := pool.Exec(context.Background(), `
		insert into asset_preserved_data (id, asset_id, owner_kind, owner_id, namespace, payload)
		values ($1, $2, 'asset', $2, 'depth_prompt', '{"depth":4}')
	`, uuid.New(), draft); err != nil {
		t.Fatalf("preserve a namespace: %v", err)
	}
	before := contentGeneration(t, pool, draft)

	if err := svc.DeletePreservedNamespace(
		context.Background(), owner, draft, "depth_prompt",
	); err != nil {
		t.Fatalf("delete the namespace: %v", err)
	}

	if got := contentGeneration(t, pool, draft); got != before+1 {
		t.Fatalf("content generation = %d, want %d after deleting preserved data", got, before+1)
	}
}

func TestANewCoverMovesTheContentGenerationAndAnUnusedPictureDoesNot(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	before := contentGeneration(t, pool, draft)

	if _, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: owner, AssetID: draft, Role: MediaGallery,
		File: bytes.NewReader(testPNG(t, 20, 10, color.Black)),
	}); err != nil {
		t.Fatalf("add a gallery picture: %v", err)
	}
	if got := contentGeneration(t, pool, draft); got != before {
		t.Fatalf("content generation = %d, want %d after a picture nothing points at", got, before)
	}

	if _, err := svc.AddMedia(context.Background(), AddMediaInput{
		OwnerID: owner, AssetID: draft, Role: MediaAvatar,
		File: bytes.NewReader(testPNG(t, 30, 15, color.White)),
	}); err != nil {
		t.Fatalf("add a cover: %v", err)
	}
	if got := contentGeneration(t, pool, draft); got != before+1 {
		t.Fatalf("content generation = %d, want %d after a new cover", got, before+1)
	}
}

func TestAddingAnEmptySectionLeavesTheContentGenerationAlone(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	before := contentGeneration(t, pool, draft)

	if _, err := svc.AddBlock(
		context.Background(), owner, draft, block.AuthorNotes, block.TypeProse,
	); err != nil {
		t.Fatalf("add a section: %v", err)
	}

	if got := contentGeneration(t, pool, draft); got != before {
		t.Fatalf("content generation = %d, want %d after an empty section", got, before)
	}
}

func TestRemovingASectionThatHeldContentMovesTheContentGeneration(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	added, err := svc.AddBlock(context.Background(), owner, draft, block.AuthorNotes, block.TypeProse)
	if err != nil {
		t.Fatalf("add a section: %v", err)
	}
	update := updateOf(added.Block)
	update.Elements[0].Content = block.Prose{Text: "Run her at a low temperature."}
	if _, err := svc.SaveBlock(context.Background(), owner, draft, added.Block.ID, update); err != nil {
		t.Fatalf("fill the section in: %v", err)
	}
	before := contentGeneration(t, pool, draft)

	if err := svc.RemoveBlock(context.Background(), owner, draft, added.Block.ID); err != nil {
		t.Fatalf("remove the section: %v", err)
	}

	if got := contentGeneration(t, pool, draft); got != before+1 {
		t.Fatalf("content generation = %d, want %d after removing filled-in content", got, before+1)
	}
}

// The creator's own version text is an exported field like any other. Editing
// it changes the file, and nothing here reads the text itself (ADR-0023).
func TestTheAssetVersionIsPartOfTheFingerprintAndTheDiscoveryStateIsNot(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	saveDescription(t, svc, owner, draft, pool, "She keeps the memories that books forget.")

	fingerprint := func() string {
		t.Helper()
		taken, err := svc.contentFingerprint(context.Background(), pool, draft)
		if err != nil {
			t.Fatalf("fingerprint the asset: %v", err)
		}
		return taken
	}
	before := fingerprint()

	if _, err := pool.Exec(context.Background(),
		`update assets set asset_version = '1.1' where id = $1`, draft); err != nil {
		t.Fatalf("set the version text: %v", err)
	}
	versioned := fingerprint()
	if versioned == before {
		t.Fatal("the version text is not part of what a download carries")
	}

	if _, err := pool.Exec(context.Background(),
		`update assets set discovery = 'unlisted', is_nsfw = true where id = $1`, draft,
	); err != nil {
		t.Fatalf("unlist the asset: %v", err)
	}
	if fingerprint() != versioned {
		t.Fatal("discovery or the adult content answer changed what a download carries")
	}
}

func TestHidingASectionAndMovingItsContentLeaveTheContentGenerationAlone(t *testing.T) {
	svc, pool := newTestService(t)
	owner, draft := startedDraft(t, svc)
	added, err := svc.AddBlock(context.Background(), owner, draft, block.AuthorNotes, block.TypeProse)
	if err != nil {
		t.Fatalf("add a section: %v", err)
	}
	update := updateOf(added.Block)
	update.Elements[0].Content = block.Prose{Text: "Run her at a low temperature."}
	if _, err := svc.SaveBlock(context.Background(), owner, draft, added.Block.ID, update); err != nil {
		t.Fatalf("fill the section in: %v", err)
	}
	before := contentGeneration(t, pool, draft)

	blocks := draftBlocks(t, pool, draft)
	arrangement := make([]BlockArrangement, len(blocks))
	for i, holder := range blocks {
		arrangement[i] = BlockArrangement{
			ID: holder.ID, Hidden: holder.ID == added.Block.ID, Width: holder.Width,
		}
	}
	if _, err := svc.ArrangeBlocks(context.Background(), owner, draft, arrangement); err != nil {
		t.Fatalf("hide the section: %v", err)
	}
	if got := contentGeneration(t, pool, draft); got != before {
		t.Fatalf("content generation = %d, want %d after hiding a section", got, before)
	}

	// The same content under a different section is the same file, because an
	// element keeps its identity wherever the creator puts it. Messages is
	// widened to a third slot first, so there is somewhere for it to land.
	messages := blockFor(t, draftBlocks(t, pool, draft), "messages")
	widened := updateOf(messages)
	widened.Layout = block.Stack3
	if _, err := svc.SaveBlock(context.Background(), owner, draft, messages.ID, widened); err != nil {
		t.Fatalf("widen the messages section: %v", err)
	}
	if _, err := svc.MoveBlockContent(
		context.Background(), owner, draft, added.Block.ID, messages.ID,
	); err != nil {
		t.Fatalf("move the content: %v", err)
	}
	if got := contentGeneration(t, pool, draft); got != before {
		t.Fatalf("content generation = %d, want %d after moving content between sections", got, before)
	}
}

// replacingModule parses whatever the test last handed it, so an import and the
// file that replaces it can differ by one field.
type replacingModule struct {
	claimsFirstPayload
	parsed *format.Parsed
}

func (replacingModule) ID() string { return "replacing" }

func (replacingModule) Declaration() format.Declaration {
	declaration := testReaderDeclaration("replacing", "character")
	declaration.Label = "Replacing format"
	declaration.Direction.Write = true
	declaration.Header = []format.HeaderField{format.HeaderName, format.HeaderAssetVersion}
	declaration.TestedOrigins = append(declaration.TestedOrigins, format.OriginIllarin)
	declaration.Roles = map[block.Role]format.DirectionalRoleSupport{
		block.RoleDescription: {
			Read:  format.RoleSupport{Grade: format.SupportFull},
			Write: format.RoleSupport{Grade: format.SupportFull},
		},
	}
	return declaration
}

func (m replacingModule) Parse(context.Context, probe.Inspection, format.Claim) (format.Parsed, error) {
	return *m.parsed, nil
}

func (replacingModule) Write(context.Context, format.ExportAsset) (format.Artifact, error) {
	return format.Artifact{Body: nil, MediaType: "text/plain", Extension: ".txt"}, nil
}

func TestAReplacementFileMovesTheContentGeneration(t *testing.T) {
	parsed := format.Parsed{
		Kind: "character", Format: "replacing",
		Header: format.Header{Name: "Wren", AssetVersion: "1.0"},
		Elements: []block.Element{
			{Type: block.TypeProse, Role: block.RoleDescription, Content: block.Prose{Text: "Keeps the archive."}},
		},
	}
	svc, pool := newTestServiceWithRegistry(t, registryWithModule(t, replacingModule{parsed: &parsed}))
	owner := revisionOwner(t, svc, "replacing.owner")
	imported := ingestOne(t, svc, owner, "wren.json", []byte(`{"payload":true}`))
	if got := contentGeneration(t, pool, imported.ID); got != 1 {
		t.Fatalf("content generation = %d, want 1 after an import", got)
	}

	parsed.Header.AssetVersion = "1.1"
	if operation := addRevision(
		t, svc, owner, imported.ID, "wren.json", []byte(`{"payload":true,"more":1}`),
	); operation.Status != IngestSuccess {
		t.Fatalf("the replacement did not finish: %+v", operation)
	}

	var version string
	if err := pool.QueryRow(context.Background(),
		`select asset_version from assets where id = $1`, imported.ID,
	).Scan(&version); err != nil {
		t.Fatalf("read the version text: %v", err)
	}
	if version != "1.1" {
		t.Fatalf("version text = %q, want the replacement's", version)
	}
	if got := contentGeneration(t, pool, imported.ID); got != 2 {
		t.Fatalf("content generation = %d, want 2 after a replacement file", got)
	}
}
