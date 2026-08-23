package v1

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/block"
	"github.com/Sillyfrogster/Illarin/api/internal/format"
	characterformat "github.com/Sillyfrogster/Illarin/api/internal/format/character"
	lorebookformat "github.com/Sillyfrogster/Illarin/api/internal/format/lorebook"
	packformat "github.com/Sillyfrogster/Illarin/api/internal/format/pack"
	presetformat "github.com/Sillyfrogster/Illarin/api/internal/format/preset"
	themeformat "github.com/Sillyfrogster/Illarin/api/internal/format/theme"
	"github.com/Sillyfrogster/Illarin/api/internal/media"
	"github.com/Sillyfrogster/Illarin/api/internal/probe"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTheRealCorpusCrossesTheV1Reader(t *testing.T) {
	dump := repositoryFile(t, ".ai", "dump", "db_backup.sql")
	if _, err := os.Stat(dump); errors.Is(err, os.ErrNotExist) {
		t.Skip("the local v1 dump is absent")
	} else if err != nil {
		t.Fatal("the local v1 dump cannot be inspected")
	}

	pool := restoreV1Dump(t, dump)
	assertDeclaredColumns(t, pool)
	rows, recoveries := loadCorpusRows(t, pool)
	want := map[string]int{
		CharacterKind: 121, ThemeKind: 11, PresetKind: 9, LorebookKind: 2, PackKind: 9,
	}
	got := make(map[string]int)
	stats := corpusStats{}
	reader := Module{Recoveries: recoveries}
	writers := corpusWriterRegistry(t)
	for _, row := range rows {
		result, err := reader.Read(context.Background(), row)
		if err != nil {
			t.Fatalf("a %T row did not read", row)
		}
		got[result.Parsed.Kind]++
		measureCorpusResult(t, row, result, &stats)
		writeCorpusResult(t, writers, result, &stats)
	}
	if len(rows) != 152 {
		t.Fatalf("read %d assets, want 152", len(rows))
	}
	for kind, count := range want {
		if got[kind] != count {
			t.Errorf("%s rows = %d, want %d", kind, got[kind], count)
		}
	}
	if stats.characterImages != 357 || stats.expressions != 195 ||
		stats.gallery != 125 || stats.covers != 37 {
		t.Errorf(
			"character images = %d total, %d expressions, %d gallery and %d covers",
			stats.characterImages, stats.expressions, stats.gallery, stats.covers,
		)
	}
	if stats.galleryNamesRecovered != 6 {
		t.Errorf("recovered %d gallery names, want 6", stats.galleryNamesRecovered)
	}
	if stats.greetingsRecovered != 1 {
		t.Errorf("recovered %d alternate greetings, want 1", stats.greetingsRecovered)
	}
	if stats.lorebookEntries != 342 || stats.promptFragments != 817 || stats.lumiaRecords != 76 {
		t.Errorf(
			"placed %d lorebook entries, %d prompt fragments and %d Lumia records",
			stats.lorebookEntries, stats.promptFragments, stats.lumiaRecords,
		)
	}
	if stats.presetVersions != 41 || stats.changelogEntries != 35 || stats.sealedBlocks != 951 {
		t.Errorf(
			"read %d preset versions, %d changelog entries and %d sealed blocks",
			stats.presetVersions, stats.changelogEntries, stats.sealedBlocks,
		)
	}
	if stats.oversizedPresetBlurbs != 2 || stats.usageBlocks != 2 {
		t.Errorf(
			"moved %d oversized preset blurbs into %d usage blocks, want 2 and 2",
			stats.oversizedPresetBlurbs, stats.usageBlocks,
		)
	}
	if stats.characterTaglines != 21 {
		t.Errorf("used %d character taglines as blurbs, want 21", stats.characterTaglines)
	}
	if stats.externalPackMedia != 59 {
		t.Errorf("surfaced %d external pack images, want 59", stats.externalPackMedia)
	}
	if stats.themeBundles != 11 || stats.themeFonts == 0 {
		t.Errorf("read %d theme bundles carrying %d fonts", stats.themeBundles, stats.themeFonts)
	}
	if stats.writerArtifacts != 394 || stats.writerMediaInputs != 1071 {
		t.Errorf(
			"wrote %d v1-origin artifacts with %d character media inputs, want 394 and 1071",
			stats.writerArtifacts, stats.writerMediaInputs,
		)
	}
	if stats.assetsBelowFloor != 5 || stats.nonCharactersBelowFloor != 0 {
		t.Errorf(
			"content floor missed by %d assets including %d non-characters, want 5 and 0",
			stats.assetsBelowFloor, stats.nonCharactersBelowFloor,
		)
	}
}

func assertDeclaredColumns(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	tables := []string{
		"characters", "character_images", "worldbooks", "presets",
		"preset_versions", "preset_sealed_blocks", "themes", "dlc_packs",
	}
	declared := make(map[string]bool)
	for _, column := range (Module{}).Declaration().Columns {
		declared[column.Table+"."+column.Column] = true
	}
	rows, err := pool.Query(context.Background(), `
		select table_name, column_name
		  from information_schema.columns
		 where table_schema = 'public' and table_name = any($1)
	 order by table_name, ordinal_position`, tables)
	if err != nil {
		t.Fatal("the v1 source columns cannot be inspected")
	}
	defer rows.Close()
	for rows.Next() {
		var table, column string
		if rows.Scan(&table, &column) != nil {
			t.Fatal("a v1 source column cannot be decoded")
		}
		key := table + "." + column
		if !declared[key] {
			t.Errorf("source column %s has no declared disposition", key)
		}
		delete(declared, key)
	}
	if rows.Err() != nil {
		t.Fatal("v1 source columns did not finish")
	}
	for key := range declared {
		t.Errorf("declared source column %s does not exist", key)
	}
}

type corpusStats struct {
	characterImages         int
	expressions             int
	gallery                 int
	covers                  int
	galleryNamesRecovered   int
	greetingsRecovered      int
	lorebookEntries         int
	promptFragments         int
	lumiaRecords            int
	presetVersions          int
	changelogEntries        int
	sealedBlocks            int
	oversizedPresetBlurbs   int
	usageBlocks             int
	characterTaglines       int
	externalPackMedia       int
	themeBundles            int
	themeFonts              int
	writerArtifacts         int
	writerMediaInputs       int
	assetsBelowFloor        int
	nonCharactersBelowFloor int
}

func corpusWriterRegistry(t *testing.T) *format.Registry {
	t.Helper()
	registry := format.NewRegistry()
	modules := []format.Module{Module{}}
	for _, family := range [][]format.Reader{
		characterformat.Modules(), lorebookformat.Modules(), presetformat.Modules(), themeformat.Modules(), packformat.Modules(),
	} {
		for _, module := range family {
			modules = append(modules, module)
		}
	}
	for _, module := range modules {
		if err := registry.Register(module); err != nil {
			t.Fatal("the corpus writer registry could not be built")
		}
	}
	if err := registry.ValidateDeclarations(); err != nil {
		t.Fatal("the corpus writer declarations are invalid")
	}
	return registry
}

func writeCorpusResult(t *testing.T, registry *format.Registry, result Result, stats *corpusStats) {
	t.Helper()
	targets := registry.OfferedTargets(format.CapabilitySubject{
		Kind: result.Parsed.Kind, Origin: ID, Elements: result.Parsed.Elements,
	})
	want := map[string]int{CharacterKind: 3, LorebookKind: 1, PresetKind: 1, ThemeKind: 1, PackKind: 1}
	if len(targets) != want[result.Parsed.Kind] {
		t.Fatalf("a %s row offered %d writers, want %d", result.Parsed.Kind, len(targets), want[result.Parsed.Kind])
	}
	images := make(map[uuid.UUID]format.ExportMedia, len(result.Media))
	for _, source := range result.Media {
		images[source.MediaID] = corpusExportMedia(source)
	}
	var cover *format.ExportMedia
	if result.Cover != nil {
		converted := corpusExportMedia(*result.Cover)
		cover = &converted
	}
	for _, target := range targets {
		module, found := registry.ByID(target.Format)
		if !found {
			t.Fatal("an offered v1 writer is absent")
		}
		preserved := []format.Remainder(nil)
		origin, _ := registry.Declaration(ID)
		written, _ := registry.Declaration(target.Format)
		preservesOrigin := format.TravelsWithOrigin(origin, written)
		if preservesOrigin {
			preserved = result.Parsed.Remainder
		}
		artifact, err := module.(format.Writer).Write(context.Background(), format.ExportAsset{
			Kind: result.Parsed.Kind, Header: result.Parsed.Header,
			Elements: result.Parsed.Elements, Preserved: preserved, Cover: cover, Images: images,
		})
		if err != nil || len(artifact.Body) == 0 {
			t.Fatalf("a %s writer did not serialize a v1-origin asset", result.Parsed.Kind)
		}
		rereadCorpusArtifact(t, registry, written, result, artifact, preservesOrigin)
		stats.writerArtifacts++
		if result.Parsed.Kind == CharacterKind {
			stats.writerMediaInputs += len(images)
			if cover != nil {
				stats.writerMediaInputs++
			}
		}
	}
}

func rereadCorpusArtifact(
	t *testing.T,
	registry *format.Registry,
	declaration format.Declaration,
	source Result,
	artifact format.Artifact,
	preservesOrigin bool,
) {
	t.Helper()
	inspection, err := probe.Inspect(
		context.Background(), memoryRangeStore{body: artifact.Body}, uuid.New(),
		int64(len(artifact.Body)), "export"+artifact.Extension,
	)
	if err != nil {
		t.Fatalf("a %s v1 export could not be inspected", source.Parsed.Kind)
	}
	resolution, claimed, err := registry.Resolve(inspection)
	if err != nil || !claimed || resolution.Module.ID() != declaration.ID {
		t.Fatalf("a %s v1 export could not be recognised again", source.Parsed.Kind)
	}
	parsed, err := resolution.Module.Parse(context.Background(), inspection, resolution.Claim)
	if err != nil {
		t.Fatalf("a %s v1 export could not be read again", source.Parsed.Kind)
	}
	for _, field := range declaration.Header {
		if headerValue(parsed.Header, field) != headerValue(source.Parsed.Header, field) {
			t.Errorf("a %s v1 export changed its %s header field", source.Parsed.Kind, field)
		}
	}
	for _, role := range block.Roles() {
		support := declaration.Roles[role]
		if support.Read.Grade != format.SupportFull || support.Write.Grade != format.SupportFull ||
			role == block.RoleExpressions || role == block.RoleGallery {
			continue
		}
		beforeSemantics := roleSemantics(t, role, source.Parsed.Elements)
		afterSemantics := roleSemantics(t, role, parsed.Elements)
		if beforeSemantics != afterSemantics {
			t.Errorf(
				"the %s writer changed a v1 %s asset's %s content at %v",
				declaration.ID, source.Parsed.Kind, role,
				semanticDifferencePaths(beforeSemantics, afterSemantics),
			)
		}
	}
	for _, role := range []block.Role{block.RoleExpressions, block.RoleGallery} {
		support := declaration.Roles[role]
		if support.Write.Grade != format.SupportFull || support.Read.Grade != format.SupportFull {
			continue
		}
		before := mediaNames(source.Parsed.Elements, role)
		if len(before) == 0 {
			continue
		}
		after := make([]string, 0, len(before))
		for _, item := range parsed.Media {
			if item.ElementRole == role {
				after = append(after, item.Name)
			}
		}
		if !sameMediaNames(before, after) {
			t.Errorf(
				"the %s writer changed a v1 %s asset's %s media",
				declaration.ID, source.Parsed.Kind, role,
			)
		}
	}
	if source.Parsed.Kind == CharacterKind && source.Cover != nil && !hasAvatar(parsed.Media) {
		t.Error("a character v1 export did not read its cover back as the avatar")
	}
	if preservesOrigin && !remaindersPreserved(t, source.Parsed, parsed) {
		t.Errorf("the %s writer changed a v1 %s asset's preserved data", declaration.ID, source.Parsed.Kind)
	}
}

func roleSemantics(t *testing.T, role block.Role, elements []block.Element) string {
	t.Helper()
	contents := make([]any, 0)
	for _, element := range elements {
		if element.Role != role || element.Content.Empty() {
			continue
		}
		raw, err := element.ContentJSON()
		if err != nil {
			t.Fatal("fully supported corpus content could not be encoded")
		}
		var value any
		if json.Unmarshal(raw, &value) != nil {
			t.Fatal("fully supported corpus content could not be decoded")
		}
		contents = append(contents, value)
	}
	normalizeEmptyCollections(contents)
	canonicalizeIDs(contents)
	encoded, err := json.Marshal(contents)
	if err != nil {
		t.Fatal("fully supported corpus content could not be compared")
	}
	return string(encoded)
}

func normalizeEmptyCollections(value any) {
	collections := map[string]bool{
		"keys": true, "secondaryKeys": true, "texts": true, "turns": true,
		"images": true, "fields": true, "links": true, "entries": true,
		"groups": true, "fragments": true, "strings": true, "choices": true,
		"variables": true, "options": true, "trim": true, "targets": true,
		"affects": true, "scripts": true, "modes": true, "colors": true,
		"stylesheets": true, "assets": true, "records": true,
	}
	var visit func(any)
	visit = func(current any) {
		switch held := current.(type) {
		case []any:
			for _, item := range held {
				visit(item)
			}
		case map[string]any:
			for key, item := range held {
				if item == nil && collections[key] {
					held[key] = []any{}
					continue
				}
				visit(item)
			}
		}
	}
	visit(value)
}

func semanticDifferencePaths(before, after string) []string {
	var left, right any
	if json.Unmarshal([]byte(before), &left) != nil || json.Unmarshal([]byte(after), &right) != nil {
		return []string{"$"}
	}
	paths := make([]string, 0)
	collectDifferencePaths("$", left, right, &paths)
	return paths
}

func collectDifferencePaths(path string, left, right any, paths *[]string) {
	if len(*paths) >= 8 {
		return
	}
	switch before := left.(type) {
	case []any:
		after, ok := right.([]any)
		if !ok || len(before) != len(after) {
			*paths = append(*paths, path+".length")
			return
		}
		for i := range before {
			collectDifferencePaths(path+"["+strconv.Itoa(i)+"]", before[i], after[i], paths)
		}
	case map[string]any:
		after, ok := right.(map[string]any)
		if !ok {
			*paths = append(*paths, path)
			return
		}
		keys := make(map[string]bool, len(before)+len(after))
		for key := range before {
			keys[key] = true
		}
		for key := range after {
			keys[key] = true
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			leftValue, leftFound := before[key]
			rightValue, rightFound := after[key]
			if !leftFound || !rightFound {
				*paths = append(*paths, path+"."+key)
				continue
			}
			collectDifferencePaths(path+"."+key, leftValue, rightValue, paths)
		}
	default:
		leftJSON, _ := json.Marshal(left)
		rightJSON, _ := json.Marshal(right)
		if !bytes.Equal(leftJSON, rightJSON) {
			*paths = append(*paths, path)
		}
	}
}

func canonicalizeIDs(value any) {
	ids := make(map[string]string)
	collectIDs(value, ids)
	replaceIDs(value, ids)
}

func collectIDs(value any, ids map[string]string) {
	switch held := value.(type) {
	case []any:
		for _, item := range held {
			collectIDs(item, ids)
		}
	case map[string]any:
		keys := make([]string, 0, len(held))
		for key := range held {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if key == "id" {
				if id, ok := held[key].(string); ok {
					if _, known := ids[id]; !known {
						ids[id] = "#" + strconv.Itoa(len(ids)+1)
					}
				}
			}
			collectIDs(held[key], ids)
		}
	}
}

func replaceIDs(value any, ids map[string]string) {
	switch held := value.(type) {
	case []any:
		for i, item := range held {
			if text, ok := item.(string); ok {
				if replacement, known := ids[text]; known {
					held[i] = replacement
					continue
				}
			}
			replaceIDs(item, ids)
		}
	case map[string]any:
		for key, item := range held {
			if text, ok := item.(string); ok {
				if replacement, known := ids[text]; known {
					held[key] = replacement
					continue
				}
			}
			replaceIDs(item, ids)
		}
	}
}

func mediaNames(elements []block.Element, role block.Role) []string {
	names := make([]string, 0)
	for _, element := range elements {
		if element.Role != role {
			continue
		}
		images, ok := element.Content.(block.ImageSet)
		if !ok {
			continue
		}
		for _, item := range images.Images {
			names = append(names, item.Name)
		}
	}
	return names
}

func sameMediaNames(before, after []string) bool {
	if len(before) != len(after) {
		return false
	}
	for i := range before {
		// An unnamed image acquires a carrier-format filename on export. It is
		// still unnamed creator content; a name the creator supplied must match.
		if before[i] != "" && before[i] != after[i] {
			return false
		}
	}
	return true
}

func hasAvatar(items []format.Media) bool {
	for _, item := range items {
		if item.Role == media.Avatar {
			return true
		}
	}
	return false
}

func remaindersPreserved(t *testing.T, before, after format.Parsed) bool {
	t.Helper()
	type heldRemainder struct {
		Owner     format.Owner
		OwnerPath string
		Namespace string
		Payload   any
	}
	decode := func(parsed format.Parsed) ([]heldRemainder, bool) {
		owners := remainderOwnerPaths(parsed.Elements)
		rows := parsed.Remainder
		values := make([]heldRemainder, 0, len(rows))
		for _, row := range rows {
			var payload any
			if json.Unmarshal(row.Payload, &payload) != nil {
				t.Fatal("preserved corpus data could not be decoded")
			}
			ownerPath := "asset"
			if row.Owner != format.OwnerAsset {
				var found bool
				ownerPath, found = owners[row.OwnerID]
				if !found {
					return nil, false
				}
			}
			values = append(values, heldRemainder{
				Owner: row.Owner, OwnerPath: ownerPath,
				Namespace: row.Namespace, Payload: payload,
			})
		}
		return values, true
	}
	want, beforeOwnersKnown := decode(before)
	got, afterOwnersKnown := decode(after)
	if !beforeOwnersKnown || !afterOwnersKnown {
		return false
	}
	used := make([]bool, len(got))
	for _, source := range want {
		found := false
		for i, candidate := range got {
			if used[i] || candidate.Owner != source.Owner || candidate.OwnerPath != source.OwnerPath ||
				candidate.Namespace != source.Namespace ||
				!jsonContains(candidate.Payload, source.Payload) {
				continue
			}
			used[i] = true
			found = true
			break
		}
		if !found {
			return false
		}
	}
	return true
}

func remainderOwnerPaths(elements []block.Element) map[uuid.UUID]string {
	paths := make(map[uuid.UUID]string)
	roleOccurrences := make(map[string]int)
	for _, element := range elements {
		name := string(element.Role)
		if name == "" {
			name = string(element.Type)
		}
		occurrence := roleOccurrences[name]
		roleOccurrences[name]++
		prefix := name + "[" + strconv.Itoa(occurrence) + "]"
		paths[element.ID] = prefix
		add := func(id uuid.UUID, collection string, index int) {
			paths[id] = prefix + "." + collection + "[" + strconv.Itoa(index) + "]"
		}
		switch content := element.Content.(type) {
		case block.TextSet:
			for i, item := range content.Texts {
				add(item.ID, "texts", i)
			}
		case block.DialogueSample:
			for i, item := range content.Turns {
				add(item.ID, "turns", i)
			}
		case block.ImageSet:
			for i, item := range content.Images {
				add(item.ID, "images", i)
			}
		case block.FieldList:
			for i, item := range content.Fields {
				add(item.ID, "fields", i)
			}
		case block.LinkList:
			for i, item := range content.Links {
				add(item.ID, "links", i)
			}
		case block.EntryTable:
			for i, item := range content.Entries {
				add(item.ID, "entries", i)
			}
		case block.PromptList:
			for i, item := range content.Groups {
				add(item.ID, "groups", i)
			}
			for i, item := range content.Fragments {
				add(item.ID, "fragments", i)
			}
		case block.VariableSchema:
			for i, item := range content.Variables {
				add(item.ID, "variables", i)
			}
		case block.SettingGroup:
			for i, item := range content.Settings {
				add(item.ID, "settings", i)
			}
		case block.ScriptList:
			for i, item := range content.Scripts {
				add(item.ID, "scripts", i)
			}
		case block.ColorSet:
			for mode, group := range content.Modes {
				for i, item := range group.Colors {
					add(item.ID, "modes["+strconv.Itoa(mode)+"].colors", i)
				}
			}
		case block.StylesheetSet:
			for i, item := range content.Stylesheets {
				add(item.ID, "stylesheets", i)
			}
			for i, item := range content.Assets {
				add(item.ID, "assets", i)
			}
		case block.RecordList:
			for i, item := range content.Records {
				add(item.ID, "records", i)
			}
		}
	}
	return paths
}

func jsonContains(container, wanted any) bool {
	switch wantedValue := wanted.(type) {
	case map[string]any:
		containerValue, ok := container.(map[string]any)
		if !ok {
			return false
		}
		for key, value := range wantedValue {
			candidate, found := containerValue[key]
			if !found || !jsonContains(candidate, value) {
				return false
			}
		}
		return true
	case []any:
		containerValue, ok := container.([]any)
		if !ok || len(containerValue) != len(wantedValue) {
			return false
		}
		for i := range wantedValue {
			if !jsonContains(containerValue[i], wantedValue[i]) {
				return false
			}
		}
		return true
	default:
		wantedJSON, _ := json.Marshal(wanted)
		containerJSON, _ := json.Marshal(container)
		return bytes.Equal(wantedJSON, containerJSON)
	}
}

func headerValue(header format.Header, field format.HeaderField) string {
	switch field {
	case format.HeaderName:
		return header.Name
	case format.HeaderBlurb:
		return header.Blurb
	case format.HeaderAssetVersion:
		return header.AssetVersion
	case format.HeaderCreditedAuthor:
		return header.CreditedAuthor
	case format.HeaderNickname:
		return header.Nickname
	default:
		return ""
	}
}

func roleContent(elements []block.Element, role block.Role) block.Content {
	for _, element := range elements {
		if element.Role == role {
			return element.Content
		}
	}
	return nil
}

func contentItemCount(content block.Content) int {
	switch held := content.(type) {
	case block.Prose:
		if held.Text != "" {
			return 1
		}
	case block.TextSet:
		return len(held.Texts)
	case block.ImageSet:
		return len(held.Images)
	case block.EntryTable:
		return len(held.Entries)
	case block.PromptList:
		return len(held.Groups) + len(held.Fragments)
	case block.ColorSet:
		count := 0
		for _, mode := range held.Modes {
			count += len(mode.Colors)
		}
		return count
	case block.RecordList:
		return len(held.Records)
	}
	return 0
}

var corpusImage = func() []byte {
	data, err := base64.StdEncoding.DecodeString(
		"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
	)
	if err != nil {
		panic(err)
	}
	return data
}()

func corpusExportMedia(_ SourceMedia) format.ExportMedia {
	return format.ExportMedia{
		MediaType: "image/png", Data: corpusImage, URL: "/media/v1-corpus",
	}
}

func measureCorpusResult(t *testing.T, row Row, result Result, stats *corpusStats) {
	t.Helper()
	common := row.common()
	if result.AssetID != common.ID || result.OwnerID != common.OwnerID ||
		result.OriginFormat != ID || !result.CreatedAt.Equal(common.CreatedAt) ||
		!result.ContentUpdatedAt.Equal(common.CreatedAt) || result.ContentGeneration != 1 {
		t.Error("a v1 result did not preserve its identity, origin or creation state")
	}
	if result.Parsed.Header.Name != common.Name {
		t.Error("a v1 row name did not win over its payload")
	}
	if !slices.Equal(result.Parsed.Tags, common.Tags) {
		t.Error("a v1 row's tags did not pass through verbatim")
	}
	for _, remainder := range result.Parsed.Remainder {
		if slices.Contains(liftedNamespaces, remainder.Namespace) ||
			payloadContainsAnyKey(remainder.Payload, liftedNamespaces) {
			t.Error("a LumiHub display namespace survived in preserved data")
		}
	}

	blocks, err := block.Place(result.Parsed.Kind, result.Parsed.Elements)
	if err != nil {
		t.Fatalf("a %s row did not fit its kind catalog", result.Parsed.Kind)
	}
	assertDefaultPlacement(t, result.Parsed.Kind, blocks)
	missedFloor := false
	for _, check := range block.ContentFloor(result.Parsed.Kind, blocks) {
		missedFloor = missedFloor || !check.Met
	}
	if missedFloor {
		stats.assetsBelowFloor++
		if result.Parsed.Kind != CharacterKind {
			stats.nonCharactersBelowFloor++
		}
	}

	for _, holder := range blocks {
		if holder.Definition == block.Usage {
			stats.usageBlocks++
		}
		for _, element := range holder.Elements {
			switch content := element.Content.(type) {
			case block.ImageSet:
				switch element.Role {
				case block.RoleExpressions:
					stats.expressions += len(content.Images)
				case block.RoleGallery:
					stats.gallery += len(content.Images)
				}
			case block.PromptList:
				stats.promptFragments += len(content.Groups) + len(content.Fragments)
			case block.RecordList:
				stats.lumiaRecords += len(content.Records)
			case block.TextSet:
				if holder.Definition == block.Changelog {
					stats.changelogEntries += len(content.Texts)
				}
			case block.StylesheetSet:
				for _, asset := range content.Assets {
					if !strings.HasPrefix(asset.MediaType, "font/") || len(asset.Data) == 0 {
						t.Error("a theme bundle supplied something other than a font binary")
					}
					stats.themeFonts++
				}
			}
		}
	}

	switch source := row.(type) {
	case CharacterRow:
		stats.characterImages += len(result.Media)
		if result.Cover != nil {
			stats.characterImages++
			stats.covers++
		}
		if result.Parsed.Header.Blurb != source.Tagline {
			t.Error("a character did not take its blurb from its tagline")
		}
		if source.Tagline != "" {
			stats.characterTaglines++
		}
		for _, event := range result.Events {
			switch event.Kind {
			case RecoveredGalleryNames:
				stats.galleryNamesRecovered += event.Count
			case RecoveredAlternateGreeting:
				stats.greetingsRecovered++
			}
		}
		for _, remainder := range result.Parsed.Remainder {
			if payloadContainsAnyKey(remainder.Payload, []string{"world_books"}) {
				t.Error("a character lorebook survived as a frozen duplicate")
			}
		}
	case LorebookRow:
		for _, element := range result.Parsed.Elements {
			if element.Role == block.RoleLorebookEntries {
				stats.lorebookEntries += len(element.Content.(block.EntryTable).Entries)
			}
		}
	case PresetRow:
		stats.presetVersions += len(result.PreservedRecords)
		stats.sealedBlocks += len(result.SealedBlocks)
		for _, record := range result.PreservedRecords {
			if record.AssetID != source.Common.ID || record.OwnerID != source.Common.OwnerID ||
				record.Table != "preset_versions" || !json.Valid(record.Payload) {
				t.Error("a preset version was not preserved whole and bound to its asset")
			}
		}
		for _, sealed := range result.SealedBlocks {
			if sealed.AssetID != source.Common.ID || sealed.OwnerID != source.Common.OwnerID {
				t.Error("a sealed block was not bound to its preset")
			}
		}
		if len([]rune(source.Common.Description)) > format.MaxBlurbRunes {
			stats.oversizedPresetBlurbs++
			if result.Parsed.Header.Blurb != "" || !usageCarries(result.Parsed, source.Common.Description) {
				t.Error("an oversized preset blurb was not moved whole into usage")
			}
		} else if result.Parsed.Header.Blurb != source.Common.Description {
			t.Error("a preset row description did not win as its blurb")
		}
	case ThemeRow:
		if len(source.Bundle) == 0 {
			t.Error("a live theme bundle was not attached")
		}
		stats.themeBundles++
		if len(result.Media) != 0 ||
			(result.Cover != nil && result.Cover.Path != source.Common.ImagePath) {
			t.Error("a generated theme bundle became source media")
		}
	case PackRow:
		stats.externalPackMedia += len(result.ExternalMedia)
	}
}

func assertDefaultPlacement(t *testing.T, kind string, blocks []block.Block) {
	t.Helper()
	definitions, ok := block.Catalog(kind)
	if !ok {
		t.Fatalf("the %s catalog is absent", kind)
	}
	order := make(map[block.DefinitionID]int, len(definitions))
	required := make(map[block.DefinitionID]bool)
	for i, definition := range definitions {
		order[definition.ID] = i
		required[definition.ID] = definition.Required
	}
	last := -1
	for i, holder := range blocks {
		position, found := order[holder.Definition]
		if !found || position <= last || holder.Position != i {
			t.Errorf("a %s row did not use the catalog's default block order", kind)
			return
		}
		last = position
		delete(required, holder.Definition)
	}
	for _, missing := range required {
		if missing {
			t.Errorf("a %s row omitted a required catalog block", kind)
			return
		}
	}
}

func payloadContainsAnyKey(raw json.RawMessage, names []string) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	wanted := make(map[string]struct{}, len(names))
	for _, name := range names {
		wanted[name] = struct{}{}
	}
	var contains func(any) bool
	contains = func(candidate any) bool {
		switch node := candidate.(type) {
		case map[string]any:
			for key, child := range node {
				if _, found := wanted[key]; found || contains(child) {
					return true
				}
			}
		case []any:
			for _, child := range node {
				if contains(child) {
					return true
				}
			}
		}
		return false
	}
	return contains(value)
}

func usageCarries(parsed format.Parsed, want string) bool {
	for _, element := range parsed.Elements {
		if element.Role != "" || element.Type != block.TypeProse {
			continue
		}
		content, ok := element.Content.(block.Prose)
		return ok && content.Text == want
	}
	return false
}

func repositoryFile(t *testing.T, parts ...string) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("the repository path is unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", ".."))
	return filepath.Join(append([]string{root}, parts...)...)
}

func restoreV1Dump(t *testing.T, dump string) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	config, err := pgx.ParseConfig(base)
	if err != nil {
		t.Fatal("TEST_DATABASE_URL cannot be parsed")
	}
	adminConfig := config.Copy()
	adminConfig.Database = "postgres"
	admin, err := pgx.ConnectConfig(context.Background(), adminConfig)
	if err != nil {
		t.Skip("the test database cannot create a scratch database")
	}
	database := "v1_reader_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	identifier := pgx.Identifier{database}.Sanitize()
	if _, err := admin.Exec(context.Background(), "create database "+identifier+" template template0"); err != nil {
		admin.Close(context.Background())
		t.Skip("the test database cannot create a scratch database")
	}
	admin.Close(context.Background())
	t.Cleanup(func() {
		cleanup, cleanupErr := pgx.ConnectConfig(context.Background(), adminConfig)
		if cleanupErr != nil {
			return
		}
		defer cleanup.Close(context.Background())
		_, _ = cleanup.Exec(context.Background(), "drop database if exists "+identifier+" with (force)")
	})

	scratch := config.Copy()
	scratch.Database = database
	command := exec.CommandContext(
		context.Background(), "psql", "-X", "--quiet", "--set", "ON_ERROR_STOP=1",
		"--host", scratch.Host, "--port", strconv.FormatUint(uint64(scratch.Port), 10),
		"--username", scratch.User, "--dbname", database,
	)
	command.Env = append(os.Environ(), "PGPASSWORD="+scratch.Password)
	command.Stdin = dumpWithoutOwners(t, dump)
	command.Stdout = nil
	var restoreError bytes.Buffer
	command.Stderr = &restoreError
	if err := command.Run(); err != nil {
		t.Fatalf("the v1 dump did not restore into the scratch database: %s", strings.TrimSpace(restoreError.String()))
	}
	poolConfig, err := pgxpool.ParseConfig(base)
	if err != nil {
		t.Fatal("the scratch database settings cannot be parsed")
	}
	poolConfig.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		t.Fatal("the restored v1 database cannot be opened")
	}
	t.Cleanup(pool.Close)
	return pool
}

func dumpWithoutOwners(t *testing.T, dump string) *bytes.Reader {
	t.Helper()
	body, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal("the v1 dump cannot be opened")
	}
	lines := bytes.SplitAfter(body, []byte{'\n'})
	filtered := bytes.NewBuffer(make([]byte, 0, len(body)))
	filtered.WriteString("\\set VERBOSITY sqlstate\n")
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("ALTER ")) &&
			bytes.HasSuffix(trimmed, []byte(" OWNER TO postgres;")) {
			continue
		}
		filtered.Write(line)
	}
	return bytes.NewReader(filtered.Bytes())
}

func loadCorpusRows(t *testing.T, pool *pgxpool.Pool) ([]Row, RecoveryAllowlist) {
	t.Helper()
	images := loadCharacterImages(t, pool)
	versions := loadPresetVersions(t, pool)
	sealed := loadSealedBlocks(t, pool)
	themes := loadThemes(t, pool)
	attachThemeBundles(t, themes)
	characters := loadCharacters(t, pool, images)
	recoveries := verifiedCharacterRecovery(t, characters)
	result := make([]Row, 0, 152)
	result = append(result, characters...)
	result = append(result, themes...)
	result = append(result, loadPresets(t, pool, versions, sealed)...)
	result = append(result, loadLorebooks(t, pool)...)
	result = append(result, loadPacks(t, pool)...)
	return result, recoveries
}

func verifiedCharacterRecovery(t *testing.T, rows []Row) RecoveryAllowlist {
	t.Helper()
	archivePath := repositoryFile(t, ".ai", "dump", "backup_folder.tar.gz")
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		t.Fatal("the local file backup cannot be opened for card recovery")
	}
	defer archiveFile.Close()
	compressed, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatal("the local file backup cannot be opened as gzip for card recovery")
	}
	defer compressed.Close()

	wanted := make(map[string]int)
	for i, source := range rows {
		row := source.(CharacterRow)
		if row.Common.ImagePath == "" {
			continue
		}
		for _, key := range bundleKeys(row.Common.ImagePath) {
			wanted[key] = i
		}
	}
	cards := 0
	allowlistedCards := 0
	allowlist := make(RecoveryAllowlist, 1)
	reader := tar.NewReader(compressed)
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			t.Fatal("the local file backup cannot be read for card recovery")
		}
		if !header.FileInfo().Mode().IsRegular() ||
			!strings.EqualFold(path.Ext(header.Name), ".png") {
			continue
		}
		index, found := pathIndex(wanted, header.Name)
		if !found {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(reader, int64(block.MaxPayloadBytes)+1))
		if readErr != nil || len(body) > block.MaxPayloadBytes {
			t.Fatal("a surviving character card cannot be read within the format limit")
		}
		alternates, allowlisted, isCard := cardAlternateGreetings(t, body)
		if !isCard {
			continue
		}
		cards++
		if !allowlisted {
			continue
		}
		allowlistedCards++
		row := rows[index].(CharacterRow)
		rowAlternates, rowErr := readStrings(row.AlternateGreetings, "alternate greetings")
		if rowErr != nil {
			t.Fatal("a surviving character row's greetings cannot be checked")
		}
		if len(rowAlternates) != 0 || len(alternates) != 1 {
			t.Fatal("the verified card recovery no longer has exactly one missing greeting")
		}
		allowlist[row.Common.ID] = CharacterRecovery{AlternateGreeting: alternates[0]}
	}
	if cards != 3 || allowlistedCards != 1 || len(allowlist) != 1 {
		t.Fatalf(
			"checked %d surviving character cards, %d matched the recovery fingerprint and %d entered the allowlist; want 3, 1 and 1",
			cards, allowlistedCards, len(allowlist),
		)
	}
	return allowlist
}

type memoryRangeStore struct{ body []byte }

func (store memoryRangeStore) ReadRange(
	_ context.Context,
	_ uuid.UUID,
	offset int64,
	length int64,
) (io.ReadCloser, error) {
	if offset < 0 || length < 0 || offset+length > int64(len(store.body)) {
		return nil, errors.New("range outside the card")
	}
	return io.NopCloser(bytes.NewReader(store.body[offset : offset+length])), nil
}

func cardAlternateGreetings(t *testing.T, body []byte) ([]string, bool, bool) {
	t.Helper()
	inspection, err := probe.Inspect(
		context.Background(), memoryRangeStore{body: body}, uuid.New(), int64(len(body)), "card.png",
	)
	if err != nil {
		t.Fatal("a surviving character card cannot be inspected")
	}
	allowlisted := false
	for _, payload := range inspection.Payloads {
		spec, _ := payload.String("spec")
		if payload.Locator.Name == "ccv3" && spec == characterformat.V2 {
			allowlisted = true
			break
		}
	}
	readers := []format.Reader{characterformat.CCv3Module{}, characterformat.CCv2Module{}}
	for _, reader := range readers {
		claim, claimed := reader.Claim(inspection)
		if !claimed {
			continue
		}
		parsed, parseErr := reader.Parse(context.Background(), inspection, claim)
		if parseErr != nil {
			t.Fatal("a surviving character card cannot be parsed")
		}
		for _, element := range parsed.Elements {
			if element.Role != block.RoleGreetings {
				continue
			}
			texts := element.Content.(block.TextSet).Texts
			if len(texts) < 2 {
				return nil, allowlisted, true
			}
			alternates := make([]string, 0, len(texts)-1)
			for _, item := range texts[1:] {
				alternates = append(alternates, item.Text)
			}
			return alternates, allowlisted, true
		}
		return nil, allowlisted, true
	}
	return nil, false, false
}

func pathIndex(wanted map[string]int, name string) (int, bool) {
	for _, key := range bundleKeys(name) {
		if index, found := wanted[key]; found {
			return index, true
		}
	}
	return 0, false
}

func attachThemeBundles(t *testing.T, rows []Row) {
	t.Helper()
	archivePath := repositoryFile(t, ".ai", "dump", "backup_folder.tar.gz")
	archiveFile, err := os.Open(archivePath)
	if errors.Is(err, os.ErrNotExist) {
		t.Fatal("the local file backup needed for theme fonts is absent")
	} else if err != nil {
		t.Fatal("the local file backup cannot be opened")
	}
	defer archiveFile.Close()
	compressed, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatal("the local file backup is not a gzip archive")
	}
	defer compressed.Close()

	wanted := make(map[string]int, len(rows))
	wantedAt := make(map[int64][]int, len(rows))
	for i, source := range rows {
		row := source.(ThemeRow)
		if row.AssetBundleID == "" {
			t.Fatal("a theme has no generated bundle id")
		}
		for _, key := range bundleKeys(row.AssetBundleID) {
			wanted[key] = i
		}
		wantedAt[row.Common.CreatedAt.Unix()] = append(wantedAt[row.Common.CreatedAt.Unix()], i)
	}
	matched := make([]bool, len(rows))
	bundleTimes := make([]int64, 0, len(rows))
	bundleSizes := make([]int64, 0, len(rows))
	matchedSizes := make(map[int64]struct{}, len(rows))
	reader := tar.NewReader(compressed)
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			t.Fatal("the local file backup cannot be read")
		}
		if !header.FileInfo().Mode().IsRegular() {
			continue
		}
		if strings.EqualFold(path.Ext(header.Name), ".lumitheme") {
			bundleTimes = append(bundleTimes, header.ModTime.Unix())
			bundleSizes = append(bundleSizes, header.Size)
		}
		index, exact, found := bundleIndex(wanted, wantedAt, header)
		if !found {
			continue
		}
		if matched[index] {
			if exact {
				t.Fatal("a live theme bundle appears more than once in the file backup")
			}
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(reader, int64(block.MaxPayloadBytes)+1))
		if readErr != nil || len(body) > block.MaxPayloadBytes {
			t.Fatal("a live theme bundle cannot be read within the format limit")
		}
		row := rows[index].(ThemeRow)
		row.Bundle = body
		rows[index] = row
		matched[index] = true
		matchedSizes[header.Size] = struct{}{}
	}
	matchedCount := 0
	for _, found := range matched {
		if !found {
			continue
		}
		matchedCount++
	}
	if matchedCount == len(rows)-1 {
		unmatchedSizes := 0
		for _, size := range bundleSizes {
			if _, found := matchedSizes[size]; !found {
				unmatchedSizes++
			}
		}
		if unmatchedSizes == 1 {
			body := readBundleWithNewSize(t, archivePath, matchedSizes)
			for i, found := range matched {
				if found {
					continue
				}
				row := rows[i].(ThemeRow)
				row.Bundle = body
				rows[i] = row
				matched[i] = true
				matchedCount++
				break
			}
		}
	}
	if matchedCount != len(rows) {
		unmatchedSizes := 0
		for _, size := range bundleSizes {
			if _, found := matchedSizes[size]; !found {
				unmatchedSizes++
			}
		}
		for i, found := range matched {
			if found {
				continue
			}
			created := rows[i].(ThemeRow).Common.CreatedAt.Unix()
			nearest := int64(1<<63 - 1)
			nearestCount := 0
			for _, candidate := range bundleTimes {
				delta := candidate - created
				if delta < 0 {
					delta = -delta
				}
				if delta < nearest {
					nearest, nearestCount = delta, 1
				} else if delta == nearest {
					nearestCount++
				}
			}
			t.Fatalf(
				"matched %d live theme bundles, want %d, from %d bundles; nearest unmatched timestamp differs by %d seconds across %d candidates; %d bundle sizes do not match a live bundle",
				matchedCount, len(rows), len(bundleTimes), nearest, nearestCount, unmatchedSizes,
			)
		}
	}
}

func readBundleWithNewSize(t *testing.T, archivePath string, known map[int64]struct{}) []byte {
	t.Helper()
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		t.Fatal("the local file backup cannot be reopened")
	}
	defer archiveFile.Close()
	compressed, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatal("the local file backup cannot be reopened as gzip")
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var body []byte
	for {
		header, nextErr := reader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			t.Fatal("the local file backup cannot be reread")
		}
		if !header.FileInfo().Mode().IsRegular() ||
			!strings.EqualFold(path.Ext(header.Name), ".lumitheme") {
			continue
		}
		if _, found := known[header.Size]; found {
			continue
		}
		if body != nil {
			t.Fatal("more than one unmatched theme bundle remains")
		}
		body, err = io.ReadAll(io.LimitReader(reader, int64(block.MaxPayloadBytes)+1))
		if err != nil || len(body) > block.MaxPayloadBytes {
			t.Fatal("the unmatched theme bundle cannot be read within the format limit")
		}
	}
	if body == nil {
		t.Fatal("the unmatched theme bundle is absent")
	}
	return body
}

func bundleIndex(
	wanted map[string]int,
	wantedAt map[int64][]int,
	header *tar.Header,
) (int, bool, bool) {
	for _, key := range bundleKeys(header.Name) {
		if index, found := wanted[key]; found {
			return index, true, true
		}
	}
	candidates := wantedAt[header.ModTime.Unix()]
	if len(candidates) == 1 && strings.EqualFold(path.Ext(header.Name), ".lumitheme") {
		return candidates[0], false, true
	}
	return 0, false, false
}

func bundleKeys(value string) []string {
	clean := path.Clean(strings.ReplaceAll(value, "\\", "/"))
	base := path.Base(clean)
	stem := strings.TrimSuffix(base, path.Ext(base))
	keys := []string{clean, base, stem}
	for _, part := range strings.Split(clean, "/") {
		if part != "" && part != "." {
			keys = append(keys, part, strings.TrimSuffix(part, path.Ext(part)))
		}
	}
	return keys
}

func loadCharacterImages(t *testing.T, pool *pgxpool.Pool) map[uuid.UUID][]CharacterImageRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select id, character_id, image_type, coalesce(label, ''), file_path,
		       mime_type, file_size, sort_order
		  from character_images
	 order by character_id, sort_order, id`)
	if err != nil {
		t.Fatal("character image rows cannot be read")
	}
	defer rows.Close()
	result := make(map[uuid.UUID][]CharacterImageRow)
	for rows.Next() {
		var characterID uuid.UUID
		var row CharacterImageRow
		if rows.Scan(
			&row.ID, &characterID, &row.Type, &row.Label, &row.Path,
			&row.MediaType, &row.ByteSize, &row.Position,
		) != nil {
			t.Fatal("a character image row cannot be decoded")
		}
		result[characterID] = append(result[characterID], row)
	}
	if rows.Err() != nil {
		t.Fatal("character image rows did not finish")
	}
	return result
}

func loadCharacters(
	t *testing.T,
	pool *pgxpool.Pool,
	images map[uuid.UUID][]CharacterImageRow,
) []Row {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select id, owner_id, name, description, coalesce(image_path, ''),
		       downloads, views, favorites, tags, is_nsfw, created_at, updated_at,
		       coalesce(nickname, ''), personality, scenario, first_mes,
		       alternate_greetings, group_only_greetings, mes_example, creator,
		       creator_notes, character_version, system_prompt,
		       post_history_instructions, coalesce(tagline, ''), character_book,
		       extensions, assets, creation_date, modification_date
		  from characters
	 order by id`)
	if err != nil {
		t.Fatal("character rows cannot be read")
	}
	defer rows.Close()
	result := make([]Row, 0, 121)
	for rows.Next() {
		var common CommonRow
		var row CharacterRow
		var creationDate, modificationDate pgtype.Int8
		if rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.Favorites, &common.Tags, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Nickname, &row.Personality, &row.Scenario,
			&row.FirstMessage, &row.AlternateGreetings, &row.GroupOnlyGreetings,
			&row.ExampleDialogue, &row.Creator, &row.CreatorNotes, &row.CharacterVersion,
			&row.SystemPrompt, &row.PostHistoryInstructions, &row.Tagline, &row.CharacterBook,
			&row.Extensions, &row.Assets, &creationDate, &modificationDate,
		) != nil {
			t.Fatal("a character row cannot be decoded")
		}
		row.Common = common
		row.Images = images[common.ID]
		if creationDate.Valid {
			value := creationDate.Int64
			row.CreationDate = &value
		}
		if modificationDate.Valid {
			value := modificationDate.Int64
			row.ModificationDate = &value
		}
		result = append(result, row)
	}
	if rows.Err() != nil {
		t.Fatal("character rows did not finish")
	}
	return result
}

func loadPresetVersions(t *testing.T, pool *pgxpool.Pool) map[uuid.UUID][]PresetVersionRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select preset_id, id, version, changelog, snapshot, blocks_added, blocks_removed,
		       variables_added, variables_removed, block_count, variable_count,
		       created_by, created_at
		  from preset_versions
	 order by preset_id, created_at, id`)
	if err != nil {
		t.Fatal("preset version rows cannot be read")
	}
	defer rows.Close()
	result := make(map[uuid.UUID][]PresetVersionRow)
	for rows.Next() {
		var presetID uuid.UUID
		var row PresetVersionRow
		var createdBy pgtype.UUID
		if rows.Scan(
			&presetID, &row.ID, &row.Version, &row.Changelog, &row.Snapshot,
			&row.BlocksAdded, &row.BlocksRemoved, &row.VariablesAdded, &row.VariablesRemoved,
			&row.BlockCount, &row.VariableCount, &createdBy, &row.CreatedAt,
		) != nil {
			t.Fatal("a preset version row cannot be decoded")
		}
		if createdBy.Valid {
			value := uuid.UUID(createdBy.Bytes)
			row.CreatedBy = &value
		}
		result[presetID] = append(result[presetID], row)
	}
	if rows.Err() != nil {
		t.Fatal("preset version rows did not finish")
	}
	return result
}

func loadSealedBlocks(t *testing.T, pool *pgxpool.Pool) map[uuid.UUID][]SealedBlockRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select preset_id, id, version, block_key, content,
		       content_sha256, created_by, created_at, updated_at
		  from preset_sealed_blocks
	 order by preset_id, id`)
	if err != nil {
		t.Fatal("sealed block rows cannot be read")
	}
	defer rows.Close()
	result := make(map[uuid.UUID][]SealedBlockRow)
	for rows.Next() {
		var presetID uuid.UUID
		var row SealedBlockRow
		var createdBy pgtype.UUID
		var version pgtype.Text
		if rows.Scan(
			&presetID, &row.ID, &version, &row.Key, &row.Content,
			&row.SHA256, &createdBy, &row.CreatedAt, &row.UpdatedAt,
		) != nil {
			t.Fatal("a sealed block row cannot be decoded")
		}
		if version.Valid {
			value := version.String
			row.Version = &value
		}
		if createdBy.Valid {
			value := uuid.UUID(createdBy.Bytes)
			row.CreatedBy = &value
		}
		result[presetID] = append(result[presetID], row)
	}
	if rows.Err() != nil {
		t.Fatal("sealed block rows did not finish")
	}
	return result
}

func loadPresets(
	t *testing.T,
	pool *pgxpool.Pool,
	versions map[uuid.UUID][]PresetVersionRow,
	sealed map[uuid.UUID][]SealedBlockRow,
) []Row {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select id, owner_id, name, description, coalesce(image_path, ''), downloads,
		       views, favorites, tags, is_nsfw, created_at, updated_at,
		       preset, coalesce(latest_version, '')
		  from presets
	 order by id`)
	if err != nil {
		t.Fatal("preset rows cannot be read")
	}
	defer rows.Close()
	result := make([]Row, 0, 9)
	for rows.Next() {
		var common CommonRow
		var row PresetRow
		if rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.Favorites, &common.Tags, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Payload, &row.LatestVersion,
		) != nil {
			t.Fatal("a preset row cannot be decoded")
		}
		row.Common = common
		row.Versions = versions[common.ID]
		row.SealedBlocks = sealed[common.ID]
		result = append(result, row)
	}
	if rows.Err() != nil {
		t.Fatal("preset rows did not finish")
	}
	return result
}

func loadThemes(t *testing.T, pool *pgxpool.Pool) []Row {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select id, owner_id, name, description, coalesce(image_path, ''), downloads,
		       views, favorites, tags, is_nsfw, created_at, updated_at,
		       colors, config, coalesce(custom_css, ''), coalesce(asset_bundle_id, '')
		  from themes
	 order by id`)
	if err != nil {
		t.Fatal("theme rows cannot be read")
	}
	defer rows.Close()
	result := make([]Row, 0, 11)
	for rows.Next() {
		var common CommonRow
		var row ThemeRow
		if rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.Favorites, &common.Tags, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Colors, &row.Config,
			&row.CustomCSS, &row.AssetBundleID,
		) != nil {
			t.Fatal("a theme row cannot be decoded")
		}
		row.Common = common
		result = append(result, row)
	}
	if rows.Err() != nil {
		t.Fatal("theme rows did not finish")
	}
	return result
}

func loadLorebooks(t *testing.T, pool *pgxpool.Pool) []Row {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select id, owner_id, name, description, coalesce(image_path, ''), downloads,
		       views, favorites, tags, is_nsfw, created_at, updated_at, creator, entries
		  from worldbooks
	 order by id`)
	if err != nil {
		t.Fatal("lorebook rows cannot be read")
	}
	defer rows.Close()
	result := make([]Row, 0, 2)
	for rows.Next() {
		var common CommonRow
		var row LorebookRow
		if rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.Favorites, &common.Tags, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Creator, &row.Entries,
		) != nil {
			t.Fatal("a lorebook row cannot be decoded")
		}
		row.Common = common
		result = append(result, row)
	}
	if rows.Err() != nil {
		t.Fatal("lorebook rows did not finish")
	}
	return result
}

func loadPacks(t *testing.T, pool *pgxpool.Pool) []Row {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select id, owner_id, name, description, coalesce(image_path, ''), downloads,
		       views, favorites, is_nsfw, created_at, updated_at, pack_author,
		       coalesce(cover_url, ''), pack_version, pack_extras, lumia_items,
		       loom_items, loom_tools
		  from dlc_packs
	 order by id`)
	if err != nil {
		t.Fatal("pack rows cannot be read")
	}
	defer rows.Close()
	result := make([]Row, 0, 9)
	for rows.Next() {
		var common CommonRow
		var row PackRow
		if rows.Scan(
			&common.ID, &common.OwnerID, &common.Name, &common.Description, &common.ImagePath,
			&common.Downloads, &common.Views, &common.Favorites, &common.IsNSFW,
			&common.CreatedAt, &common.UpdatedAt, &row.Author, &row.CoverURL, &row.Version,
			&row.PackExtras, &row.LumiaItems, &row.LoomItems, &row.LoomTools,
		) != nil {
			t.Fatal("a pack row cannot be decoded")
		}
		row.Common = common
		result = append(result, row)
	}
	if rows.Err() != nil {
		t.Fatal("pack rows did not finish")
	}
	return result
}
