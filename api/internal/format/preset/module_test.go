package preset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/character"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/lorebook"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

// tidyScript is the one script the Lumiverse fixture carries. The file keeps a
// copy of the list under `extensions` too, and both copies are the same list.
const tidyScript = `{
	"name": "Trim asterisks", "description": "Take the emphasis out",
	"find_regex": "/\\*+/g", "replace_string": "", "flags": "g",
	"disabled": false, "min_depth": 1, "max_depth": 4, "run_on_edit": true,
	"trim_strings": ["  "], "placement": ["ai_output"], "target": ["display"],
	"folder": "Tidy", "script_id": "tidy-1", "sort_order": 3
}`

// lumiversePreset is the shape a real Lumiverse preset has: an ordered block
// list with headings among it, a form held on the fragment it belongs to, four
// groups of named settings, and keys nothing here reads.
var lumiversePreset = `{
	"id": "a-preset", "schemaVersion": 2,
	"name": "Quiet Room",
	"description": "A calm narrator with a short leash.",
	"presetVersion": "1.2",
	"coverUrl": null, "isDefault": false, "modelProfiles": {},
	"customBody": {"enabled": false, "rawJson": "{}"},
	"blocks": [
		{
			"id": "cat-1", "name": "Core", "role": "system", "marker": "category",
			"content": "", "enabled": true, "group": null, "color": null,
			"depth": 0, "isLocked": false, "position": "pre_history",
			"categoryMode": null, "injectionTrigger": []
		},
		{
			"id": "frag-1", "name": "House rules", "role": "system", "marker": null,
			"content": "Write plainly.", "enabled": true, "group": "cat-1",
			"color": null, "depth": 0, "isLocked": false, "position": "pre_history",
			"categoryMode": null, "injectionTrigger": [],
			"variables": [{
				"id": "var-1", "name": "tone", "type": "select", "label": "Tone",
				"description": "How warm the narrator is",
				"defaultValue": "soft",
				"options": [
					{"id": "soft", "label": "Soft", "value": "gentle and unhurried"},
					{"id": "sharp", "label": "Sharp", "value": "clipped and cold"}
				]
			}]
		},
		{
			"id": "frag-2", "name": "Chat", "role": "system", "marker": "chat_history",
			"content": "", "enabled": false, "position": "in_history", "depth": 2
		}
	],
	"promptVariables": {"frag-1": {"tone": "sharp"}},
	"samplerOverrides": {"temperature": 0.8, "topP": null, "top_p": 0.9},
	"completionSettings": {"useSystemPrompt": true, "assistantPrefill": ""},
	"advancedSettings": {"seed": -1, "customStopStrings": []},
	"promptBehavior": {"newChatPrompt": "[Start again.]", "sendIfEmpty": ""},
	"regex_scripts": [` + tidyScript + `],
	"extensions": {"regex_scripts": [` + tidyScript + `], "risuai": {"folders": []}}
}`

// sillyTavernPreset is the shape a real SillyTavern preset has: flat settings,
// a prompt list, and the order it is sent in held separately by character.
const sillyTavernPreset = `{
	"temperature": 1, "top_p": 1, "openai_max_tokens": 8192,
	"names_behavior": -1, "use_sysprompt": true, "assistant_prefill": "",
	"seed": -1, "bias_preset_selected": "Default",
	"impersonation_prompt": "[Write as the user.]", "send_if_empty": "",
	"topP": 0.9,
	"prompts": [
		{
			"identifier": "main", "name": "| Prompt", "role": "system",
			"content": "Be brief.", "system_prompt": true, "injection_position": 0,
			"injection_depth": 4, "forbid_overrides": false, "injection_order": 100,
			"injection_trigger": []
		},
		{"identifier": "chatHistory", "name": "Chat History", "system_prompt": true, "marker": true},
		{
			"identifier": "note", "name": "Author note", "role": "system",
			"content": "Keep it short.", "injection_position": 1, "injection_depth": 2,
			"enabled": true
		},
		{"identifier": "unsent", "name": "Not in the order", "role": "system", "content": "Left out."}
	],
	"prompt_order": [
		{"character_id": 100000, "order": [{"identifier": "main", "enabled": true}]},
		{"character_id": 100001, "order": [
			{"identifier": "note", "enabled": false},
			{"identifier": "main", "enabled": true},
			{"identifier": "chatHistory", "enabled": true}
		]}
	],
	"extensions": {"regex_scripts": [{
		"id": "s1", "scriptName": "Trim", "findRegex": "/x/g", "replaceString": "",
		"trimStrings": [], "placement": [2], "disabled": false,
		"markdownOnly": true, "promptOnly": false, "runOnEdit": false,
		"substituteRegex": 0
	}]}
}`

func TestBothPresetModulesReadAndWriteWithAFullDeclaration(t *testing.T) {
	for _, module := range Modules() {
		declaration := module.Declaration()
		t.Run(declaration.ID, func(t *testing.T) {
			if declaration.Kind != Kind || declaration.ID != module.ID() {
				t.Errorf("declaration identity = %q/%q, want %q under %q",
					declaration.ID, declaration.Kind, module.ID(), Kind)
			}
			if !declaration.Direction.Read || !declaration.Direction.Write {
				t.Errorf("direction = %+v, want read and write", declaration.Direction)
			}
			if _, writes := module.(format.Writer); !writes {
				t.Fatal("the module declares a writer it does not have")
			}
			if err := format.ValidateDeclaration(declaration); err != nil {
				t.Fatalf("declaration: %v", err)
			}
			if len(declaration.Slots) == 0 {
				t.Error("a preset writer declares the named slots it fills")
			}
			for _, role := range []block.Role{
				block.RolePromptFragments, block.RoleSamplerSettings,
				block.RoleCompletionSettings, block.RoleAdvancedSettings,
				block.RolePromptNudges, block.RoleRegexScripts,
			} {
				if _, declared := declaration.Roles[role]; !declared {
					t.Errorf("the declaration says nothing about %s", role)
				}
			}
		})
	}
}

// The two preset formats are told apart by a marker on one and a structure on
// the other, and neither may shadow another module's evidence.
func TestRecognitionDoesNotOverlapAnotherModules(t *testing.T) {
	if err := testRegistry(t).ValidateDeclarations(); err != nil {
		t.Fatalf("declarations across every module: %v", err)
	}
}

func TestASchemaVersionIsAMarkerAndNeverAnUnsupportedVersion(t *testing.T) {
	for _, test := range []struct {
		name     string
		body     string
		claimant string
	}{
		{name: "schema version 1", body: `{"schemaVersion": 1, "blocks": []}`, claimant: LumiverseID},
		{name: "schema version 2", body: `{"schemaVersion": 2, "blocks": []}`, claimant: LumiverseID},
		{name: "a whole preset", body: lumiversePreset, claimant: LumiverseID},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := document(t, test.body)
			resolution, claimed, err := testRegistry(t).Resolve(file)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			claimant := ""
			if claimed {
				claimant = resolution.Module.ID()
			}
			if claimant != test.claimant {
				t.Fatalf("claimed by %q, want %q", claimant, test.claimant)
			}
			if !claimed {
				return
			}
			if _, err := resolution.Module.Parse(
				context.Background(), file, resolution.Claim,
			); err != nil {
				t.Fatalf("parse: %v", err)
			}
		})
	}

	// A marker outside the set is a file nothing recognises, named as far as
	// Illarin can name it. It is never a version of this format that could be
	// unsupported, because there is one Lumiverse preset behind every marker.
	t.Run("a marker outside the set", func(t *testing.T) {
		_, claimed, err := testRegistry(t).Resolve(document(t, `{"schemaVersion": 3, "blocks": []}`))
		if claimed {
			t.Fatal("a marker outside the set was claimed")
		}
		if !errors.Is(err, format.ErrUnsupportedFormat) {
			t.Fatalf("resolve error = %v, want an unsupported format", err)
		}
		if !strings.Contains(err.Error(), LumiverseID) ||
			!strings.Contains(err.Error(), "3") {
			t.Errorf("resolve error = %v, want the format and the marker named", err)
		}
		if reason, classified := format.FailureOf(err); classified &&
			reason == format.FailureUnsupportedVersion {
			t.Error("a marker was treated as a format version that could be unsupported")
		}
	})
}

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

func TestReadingALumiversePresetFillsTheRolesAndKeepsTheRest(t *testing.T) {
	parsed := parse(t, lumiversePreset)
	if parsed.Kind != Kind || parsed.Format != LumiverseID {
		t.Fatalf("parsed kind %q format %q", parsed.Kind, parsed.Format)
	}
	if parsed.Header.Name != "Quiet Room" || parsed.Header.AssetVersion != "1.2" {
		t.Errorf("header = %+v, want the preset's own name and version", parsed.Header)
	}
	if parsed.Header.Blurb != "A calm narrator with a short leash." {
		t.Errorf("blurb = %q, want the preset's description", parsed.Header.Blurb)
	}

	list := promptList(t, parsed.Elements)
	if len(list.Groups) != 1 || list.Groups[0].Name != "Core" {
		t.Fatalf("headings = %+v, want the one the file carries", list.Groups)
	}
	if len(list.Fragments) != 2 {
		t.Fatalf("read %d fragments, want 2", len(list.Fragments))
	}
	first, second := list.Fragments[0], list.Fragments[1]
	if first.Name != "House rules" || first.Text != "Write plainly." ||
		first.Role != block.PromptSystem || !first.Enabled ||
		first.Placement != block.BeforeHistory {
		t.Errorf("first fragment = %+v", first)
	}
	if first.GroupID == nil || *first.GroupID != list.Groups[0].ID {
		t.Error("the fragment naming a heading did not land under it")
	}
	// The second fragment carries no heading key at all, so it belongs to the
	// heading above it.
	if second.GroupID == nil || *second.GroupID != list.Groups[0].ID {
		t.Error("the fragment with no heading key did not take the heading above it")
	}
	if second.Marker != "chat_history" || second.Enabled ||
		second.Placement != block.InHistory || second.Depth == nil || *second.Depth != 2 {
		t.Errorf("second fragment = %+v", second)
	}

	schema := variableSchema(t, parsed.Elements)
	if len(schema.Variables) != 1 {
		t.Fatalf("read %d variables, want 1", len(schema.Variables))
	}
	variable := schema.Variables[0]
	if variable.Name != "tone" || variable.Widget != block.WidgetSelect ||
		variable.FragmentID == nil || *variable.FragmentID != first.ID {
		t.Errorf("variable = %+v, want it on the fragment it belongs to", variable)
	}
	if len(variable.Options) != 2 || variable.Options[0].Key != "soft" ||
		variable.Options[0].Value != "gentle and unhurried" {
		t.Errorf("choices = %+v, want the key and the text apart", variable.Options)
	}
	if variable.Default == nil || *variable.Default.Text != "soft" {
		t.Errorf("default = %+v, want the choice the file names", variable.Default)
	}
	if variable.Value == nil || *variable.Value.Text != "sharp" {
		t.Errorf("saved choice = %+v, want the one the creator saved", variable.Value)
	}

	samplers := settingGroup(t, parsed.Elements, block.RoleSamplerSettings)
	if len(samplers.Settings) != 2 {
		t.Fatalf("read %d samplers, want the two names this app has", len(samplers.Settings))
	}
	byName := make(map[string]block.Setting)
	for _, setting := range samplers.Settings {
		byName[setting.Name] = setting
	}
	if value := byName["temperature"].Value; value == nil || *value.Number != 0.8 {
		t.Errorf("temperature = %+v, want the file's own", value)
	}
	if byName["topP"].Value != nil {
		t.Error("a slot the file wrote as nothing was read as a value")
	}
	if _, read := byName["top_p"]; read {
		t.Error("the module read a name belonging to the other preset format")
	}

	if texts := textSet(t, parsed.Elements); len(texts.Texts) != 2 {
		t.Errorf("nudges = %+v, want the two the file carries", texts.Texts)
	}
	list2 := scriptList(t, parsed.Elements)
	if len(list2.Scripts) != 1 {
		t.Fatalf("read %d scripts, want 1", len(list2.Scripts))
	}
	script := list2.Scripts[0]
	if !script.Enabled {
		t.Error("a script the file did not disable came out switched off")
	}
	if script.Find != `/\*+/g` || script.Replace != "" || !script.RunOnEdit ||
		script.MinDepth == nil || *script.MinDepth != 1 {
		t.Errorf("script = %+v", script)
	}
	if !reflect.DeepEqual(script.Targets, []block.ScriptTarget{block.TargetModelOutput}) ||
		!reflect.DeepEqual(script.Affects, []block.ScriptEffect{block.EffectDisplay}) {
		t.Errorf("script runs over %v and changes %v", script.Targets, script.Affects)
	}

	body := preservedPayload(t, parsed.Remainder, format.OwnerAsset, lumiverseNamespace)
	for _, key := range []string{"id", "coverUrl", "isDefault", "modelProfiles", "customBody"} {
		if _, held := body[key]; !held {
			t.Errorf("the preset's %s was not preserved", key)
		}
	}
	var samplerLeftovers map[string]json.RawMessage
	if err := json.Unmarshal(body["samplerOverrides"], &samplerLeftovers); err != nil {
		t.Fatalf("read the preserved samplers: %v", err)
	}
	if _, held := samplerLeftovers["top_p"]; !held {
		t.Error("a settings name this app does not read was dropped rather than preserved")
	}
	if _, held := preserved(parsed.Remainder, format.OwnerAsset, "risuai"); !held {
		t.Error("the extensions namespace was not preserved as its own namespace")
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

// SillyTavern switches a script off where Illarin switches one on.
func TestTheScriptSwitchIsInvertedInBothFiles(t *testing.T) {
	for _, test := range []struct{ name, body string }{
		{"lumiverse", strings.Replace(lumiversePreset, `"disabled": false`, `"disabled": true`, 2)},
		{"sillytavern", strings.Replace(sillyTavernPreset, `"disabled": false`, `"disabled": true`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			list := scriptList(t, parse(t, test.body).Elements)
			if len(list.Scripts) != 1 || list.Scripts[0].Enabled {
				t.Errorf("scripts = %+v, want the disabled one switched off", list.Scripts)
			}
		})
	}
}

// Reading the file the writer produced gives back the same preset, and every
// key the file carried that Illarin has no place for comes back byte for byte.
func TestAPresetWrittenBackCarriesItsContentAndEveryPreservedKey(t *testing.T) {
	for _, test := range []struct {
		name   string
		body   string
		module format.Reader
		check  func(*testing.T, map[string]json.RawMessage)
	}{
		{
			name: "lumiverse", body: lumiversePreset, module: LumiverseModule{},
			check: func(t *testing.T, body map[string]json.RawMessage) {
				if string(body["description"]) != `"A calm narrator with a short leash."` {
					t.Errorf("description = %s, want the catalog blurb", body["description"])
				}
				if string(body["presetVersion"]) != `"1.2"` {
					t.Errorf("version = %s, want the asset's own", body["presetVersion"])
				}
				var samplers map[string]json.RawMessage
				if err := json.Unmarshal(body["samplerOverrides"], &samplers); err != nil {
					t.Fatalf("read the written samplers: %v", err)
				}
				if string(samplers["topP"]) != "null" {
					t.Errorf("topP = %s, want the empty slot the file spells", samplers["topP"])
				}
				if string(samplers["top_p"]) != "0.9" {
					t.Errorf("top_p = %s, want the preserved name back inside its object",
						samplers["top_p"])
				}
			},
		},
		{
			name: "sillytavern", body: sillyTavernPreset, module: SillyTavernModule{},
			check: func(t *testing.T, body map[string]json.RawMessage) {
				var orders []map[string]json.RawMessage
				if err := json.Unmarshal(body["prompt_order"], &orders); err != nil {
					t.Fatalf("read the written order: %v", err)
				}
				if len(orders) != 2 || string(orders[0]["character_id"]) != "100000" ||
					string(orders[1]["character_id"]) != "100001" {
					t.Errorf("orders = %+v, want the preserved one and this app's", orders)
				}
				if string(body["topP"]) != "0.9" {
					t.Errorf("topP = %s, want the preserved name back", body["topP"])
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed := parse(t, test.body)
			written := write(t, test.module, parsed)
			var body map[string]json.RawMessage
			if err := json.Unmarshal(written.Body, &body); err != nil {
				t.Fatalf("read the written preset: %v", err)
			}
			test.check(t, body)
			assertPreservedComesBack(t, parsed.Remainder, written.Body)

			again := parse(t, string(written.Body))
			if again.Format != parsed.Format {
				t.Fatalf("the written file reads as %q, want %q", again.Format, parsed.Format)
			}
			before, after := stripIDs(parsed.Elements), stripIDs(again.Elements)
			if !reflect.DeepEqual(before, after) {
				t.Errorf("the round trip changed the preset:\n%+v\n%+v", before, after)
			}
		})
	}
}

// Import fills the kind's blocks through its catalog rather than through the
// module, so a preset lands as the sections a preset has.
func TestAnImportedPresetIsPlacedIntoThePresetCatalog(t *testing.T) {
	for _, test := range []struct{ name, body string }{
		{"lumiverse", lumiversePreset},
		{"sillytavern", sillyTavernPreset},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed := parse(t, test.body)
			if err := block.ValidateContentLimits(parsed.Elements); err != nil {
				t.Fatalf("content limits: %v", err)
			}
			blocks, err := block.Place(parsed.Kind, parsed.Elements)
			if err != nil {
				t.Fatalf("place: %v", err)
			}
			definitions := make([]block.DefinitionID, 0, len(blocks))
			for _, placed := range blocks {
				definitions = append(definitions, placed.Definition)
			}
			if !slices.Contains(definitions, block.PresetCore) {
				t.Fatalf("placed %v, want the prompt fragments among them", definitions)
			}
			if !blocks[0].Pinned(block.RolePromptFragments, Kind) {
				t.Error("the prompt fragments can be taken off the page a preset is")
			}
		})
	}
}

// Neither preset format converts to the other, so neither writer is offered
// for the other's origin however alike the two shapes look.
func TestNeitherPresetWriterIsOfferedForTheOthersOrigin(t *testing.T) {
	for _, test := range []struct{ origin, offered string }{
		{LumiverseID, LumiverseID},
		{SillyTavernID, SillyTavernID},
	} {
		t.Run(test.origin, func(t *testing.T) {
			targets := testRegistry(t).OfferedTargets(format.CapabilitySubject{
				Kind: Kind, Origin: test.origin,
				Elements: []block.Element{{
					ID: uuid.New(), Type: block.TypePromptList, Role: block.RolePromptFragments,
					Content: block.PromptList{Fragments: []block.PromptFragment{{Text: "kept"}}},
				}},
			})
			if len(targets) != 1 || targets[0].Format != test.offered {
				t.Fatalf("offered %+v, want %q alone", targets, test.offered)
			}
		})
	}
}

// A preset built here is offered both writers, because the export gates read
// what the asset holds rather than a format it was born in.
func TestAPresetBuiltHereIsOfferedBothWriters(t *testing.T) {
	targets := testRegistry(t).OfferedTargets(format.CapabilitySubject{
		Kind: Kind,
		Elements: []block.Element{{
			ID: uuid.New(), Type: block.TypePromptList, Role: block.RolePromptFragments,
			Content: block.PromptList{Fragments: []block.PromptFragment{{Text: "kept"}}},
		}},
	})
	if len(targets) != 2 {
		t.Fatalf("offered %+v, want both preset writers", targets)
	}
}

// The declared ceiling clears the largest preset anyone actually has, so a
// payload over a megabyte imports rather than being refused.
func TestALargePayloadImportsWithinTheDeclaredLimits(t *testing.T) {
	const wanted = 1_060_000
	blocks := make([]string, 0, 512)
	filler := strings.Repeat("a scene that keeps going. ", 80)
	for size := 0; size < wanted+50_000; size += len(filler) + 120 {
		blocks = append(blocks, fmt.Sprintf(
			`{"id":"frag-%d","name":"Fragment %d","role":"system","content":%q,`+
				`"enabled":true,"position":"pre_history"}`,
			len(blocks), len(blocks), filler,
		))
	}
	body := `{"schemaVersion":1,"name":"Big","blocks":[` + strings.Join(blocks, ",") + `]}`
	if len(body) < wanted {
		t.Fatalf("the fixture is %d bytes, want at least %d", len(body), wanted)
	}

	declaration := (LumiverseModule{}).Declaration()
	if int64(len(body)) > int64(declaration.Limits.PayloadBytes) {
		t.Fatalf("%d bytes is over the declared payload limit of %d",
			len(body), declaration.Limits.PayloadBytes)
	}
	parsed := parse(t, body)
	if got := len(promptList(t, parsed.Elements).Fragments); got != len(blocks) {
		t.Fatalf("read %d fragments out of %d", got, len(blocks))
	}
	if err := block.ValidateContentLimits(parsed.Elements); err != nil {
		t.Fatalf("content limits: %v", err)
	}
}

// The block list is the required role. If it will not parse the import is
// refused and nothing is stored.
func TestABlockListThatIsNotAListRefusesTheImport(t *testing.T) {
	file := document(t, `{"schemaVersion": 1, "blocks": {"0": {}}}`)
	claim, claimed := (LumiverseModule{}).Claim(file)
	if !claimed {
		t.Fatal("the marker was not recognised")
	}
	_, err := (LumiverseModule{}).Parse(context.Background(), file, claim)
	if reason, classified := format.FailureOf(err); !classified ||
		reason != format.FailureMalformedInput {
		t.Fatalf("parse error = %v, want a malformed input refusal", err)
	}
}

// assertPreservedComesBack proves every preserved key is somewhere in the
// written file, exactly as it was stored.
func assertPreservedComesBack(t *testing.T, rows []format.Remainder, written []byte) {
	t.Helper()
	document := string(written)
	for _, row := range rows {
		var payload map[string]json.RawMessage
		if json.Unmarshal(row.Payload, &payload) != nil {
			continue
		}
		for key, value := range payload {
			if key == "prompt_order" || key == "samplerOverrides" {
				// Both are structures the writer rebuilt, and each is checked
				// where it is written rather than as one stored blob.
				continue
			}
			encoded, err := json.Marshal(map[string]json.RawMessage{key: value})
			if err != nil {
				t.Fatalf("encode the preserved %s: %v", key, err)
			}
			pair := strings.TrimSuffix(strings.TrimPrefix(string(encoded), "{"), "}")
			if !strings.Contains(document, pair) {
				t.Errorf("the preserved %s.%s did not come back as %s",
					row.Namespace, key, pair)
			}
		}
	}
}

func stripIDs(elements []block.Element) []block.Element {
	stripped := make([]block.Element, 0, len(elements))
	for _, element := range elements {
		element.ID = uuid.Nil
		switch content := element.Content.(type) {
		case block.PromptList:
			groups := make(map[uuid.UUID]int, len(content.Groups))
			for i := range content.Groups {
				groups[content.Groups[i].ID] = i
				content.Groups[i].ID = uuid.Nil
			}
			for i := range content.Fragments {
				content.Fragments[i].ID = uuid.Nil
				if at := content.Fragments[i].GroupID; at != nil {
					position := uuid.UUID{byte(groups[*at])}
					content.Fragments[i].GroupID = &position
				}
			}
			element.Content = content
		case block.VariableSchema:
			for i := range content.Variables {
				content.Variables[i].ID = uuid.Nil
				content.Variables[i].FragmentID = nil
			}
			element.Content = content
		case block.SettingGroup:
			for i := range content.Settings {
				content.Settings[i].ID = uuid.Nil
			}
			element.Content = content
		case block.ScriptList:
			for i := range content.Scripts {
				content.Scripts[i].ID = uuid.Nil
			}
			element.Content = content
		case block.TextSet:
			for i := range content.Texts {
				content.Texts[i].ID = uuid.Nil
			}
			element.Content = content
		}
		stripped = append(stripped, element)
	}
	return stripped
}

// testRegistry is the registry the server builds, so recognition and the
// export gates are exercised against the modules a real upload meets.
func testRegistry(t *testing.T) *format.Registry {
	t.Helper()
	registry := format.NewRegistry()
	for _, module := range slices.Concat(
		character.Modules(), lorebook.Modules(), Modules(),
	) {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %q: %v", module.ID(), err)
		}
	}
	return registry
}

func parse(t *testing.T, body string) format.Parsed {
	t.Helper()
	file := document(t, body)
	resolution, claimed, err := testRegistry(t).Resolve(file)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !claimed {
		t.Fatal("no module claimed the preset")
	}
	parsed, err := resolution.Module.Parse(context.Background(), file, resolution.Claim)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return parsed
}

func write(t *testing.T, module format.Reader, parsed format.Parsed) format.Artifact {
	t.Helper()
	writer, writes := module.(format.Writer)
	if !writes {
		t.Fatalf("%s does not write", module.ID())
	}
	written, err := writer.Write(context.Background(), format.ExportAsset{
		Kind: Kind, Header: parsed.Header, Elements: parsed.Elements,
		Preserved: parsed.Remainder,
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if written.MediaType != "application/json" || written.Extension != ".json" {
		t.Errorf("artifact = %s%s, want a JSON document", written.MediaType, written.Extension)
	}
	return written
}

func elementFor(t *testing.T, elements []block.Element, role block.Role) block.Content {
	t.Helper()
	for _, element := range elements {
		if element.Role == role {
			return element.Content
		}
	}
	t.Fatalf("no %s element in %+v", role, elements)
	return nil
}

func promptList(t *testing.T, elements []block.Element) block.PromptList {
	t.Helper()
	return elementFor(t, elements, block.RolePromptFragments).(block.PromptList)
}

func variableSchema(t *testing.T, elements []block.Element) block.VariableSchema {
	t.Helper()
	return elementFor(t, elements, block.RolePromptVariables).(block.VariableSchema)
}

func settingGroup(t *testing.T, elements []block.Element, role block.Role) block.SettingGroup {
	t.Helper()
	return elementFor(t, elements, role).(block.SettingGroup)
}

func textSet(t *testing.T, elements []block.Element) block.TextSet {
	t.Helper()
	return elementFor(t, elements, block.RolePromptNudges).(block.TextSet)
}

func scriptList(t *testing.T, elements []block.Element) block.ScriptList {
	t.Helper()
	return elementFor(t, elements, block.RoleRegexScripts).(block.ScriptList)
}

func preserved(
	rows []format.Remainder,
	owner format.Owner,
	namespace string,
) (format.Remainder, bool) {
	for _, row := range rows {
		if row.Owner == owner && row.Namespace == namespace {
			return row, true
		}
	}
	return format.Remainder{}, false
}

func preservedPayload(
	t *testing.T,
	rows []format.Remainder,
	owner format.Owner,
	namespace string,
) map[string]json.RawMessage {
	t.Helper()
	row, held := preserved(rows, owner, namespace)
	if !held {
		t.Fatalf("nothing preserved for %s %s", owner, namespace)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(row.Payload, &payload); err != nil {
		t.Fatalf("read the %s payload: %v", namespace, err)
	}
	return payload
}

// document inspects real bytes, so a test reads what the probe produces rather
// than a hand-made structure.
func document(t *testing.T, body string) probe.Inspection {
	t.Helper()
	data := []byte(body)
	file, err := probe.Inspect(
		context.Background(), memoryStore{data: data}, uuid.New(), int64(len(data)), "preset.json",
	)
	if err != nil {
		t.Fatalf("inspect the document: %v", err)
	}
	return file
}

type memoryStore struct{ data []byte }

func (s memoryStore) ReadRange(
	_ context.Context,
	_ uuid.UUID,
	offset, length int64,
) (io.ReadCloser, error) {
	if offset < 0 || offset+length > int64(len(s.data)) {
		return nil, errors.New("range outside the blob")
	}
	return io.NopCloser(bytes.NewReader(s.data[offset : offset+length])), nil
}

// Each module declares its own app's slot names and reads no others. A handful
// of names, such as `temperature`, happen to be spelled the same in both apps;
// what matters is that neither module reads the other's table.
func TestNeitherModuleKnowsTheOthersSlotNames(t *testing.T) {
	declared := func(module format.Module) map[string]bool {
		names := make(map[string]bool)
		for _, slot := range module.Declaration().Slots {
			names[slot.Name] = true
		}
		return names
	}
	lumiverse, sillyTavern := declared(LumiverseModule{}), declared(SillyTavernModule{})
	for name, app := range map[string]map[string]bool{
		"topP": sillyTavern, "sendInlineMedia": sillyTavern, "newChatPrompt": sillyTavern,
		"top_p": lumiverse, "media_inlining": lumiverse, "new_chat_prompt": lumiverse,
	} {
		if app[name] {
			t.Errorf("a module declares %q, which belongs to the other app", name)
		}
	}
	// Only the names the writer declares reach a file, so a setting from the
	// other app stays behind rather than being written into a file that would
	// ignore it.
	written := write(t, SillyTavernModule{}, format.Parsed{
		Elements: []block.Element{
			{
				ID: uuid.New(), Type: block.TypePromptList, Role: block.RolePromptFragments,
				Content: block.PromptList{Fragments: []block.PromptFragment{
					{ID: block.NewItemID(), Text: "kept", Enabled: true},
				}},
			},
			{
				ID: uuid.New(), Type: block.TypeSettingGroup, Role: block.RoleSamplerSettings,
				Content: block.SettingGroup{Settings: []block.Setting{
					{ID: block.NewItemID(), Name: "temperature", Type: block.SettingNumber,
						Value: &block.Value{Number: pointerTo(0.7)}},
					{ID: block.NewItemID(), Name: "topP", Type: block.SettingNumber,
						Value: &block.Value{Number: pointerTo(0.9)}},
				}},
			},
		},
	})
	var body map[string]json.RawMessage
	if err := json.Unmarshal(written.Body, &body); err != nil {
		t.Fatalf("read the written preset: %v", err)
	}
	if string(body["temperature"]) != "0.7" {
		t.Errorf("temperature = %s, want the name this app reads", body["temperature"])
	}
	if _, written := body["topP"]; written {
		t.Error("a name belonging to the other preset format was written into the file")
	}
	// The loss report says so rather than the file quietly losing it.
	targets := testRegistry(t).OfferedTargets(format.CapabilitySubject{
		Kind: Kind, Origin: SillyTavernID,
		Elements: []block.Element{{
			ID: uuid.New(), Type: block.TypeSettingGroup, Role: block.RoleSamplerSettings,
			Content: block.SettingGroup{Settings: []block.Setting{
				{ID: block.NewItemID(), Name: "topP", Type: block.SettingNumber,
					Value: &block.Value{Number: pointerTo(0.9)}},
			}},
		}},
	})
	if len(targets) != 1 {
		t.Fatalf("offered %+v, want the SillyTavern preset alone", targets)
	}
	if losses := targets[0].Losses(); len(losses) != 1 ||
		losses[0].Role != block.RoleSamplerSettings || losses[0].Verdict != format.Reduced {
		t.Errorf("losses = %+v, want the settings named as reduced", losses)
	}
}

// The blurb is the same text as the preset's own description, both ways.
func TestTheBlurbBindsBothWaysForALumiversePreset(t *testing.T) {
	parsed := parse(t, lumiversePreset)
	if parsed.Header.Blurb != "A calm narrator with a short leash." {
		t.Fatalf("blurb = %q, want the file's description", parsed.Header.Blurb)
	}
	edited := parsed.Header
	edited.Blurb = "Rewritten by the creator."
	written := write(t, LumiverseModule{}, format.Parsed{
		Header: edited, Elements: parsed.Elements, Remainder: parsed.Remainder,
	})
	var body map[string]json.RawMessage
	if err := json.Unmarshal(written.Body, &body); err != nil {
		t.Fatalf("read the written preset: %v", err)
	}
	if string(body["description"]) != `"Rewritten by the creator."` {
		t.Errorf("description = %s, want the blurb the creator wrote", body["description"])
	}
	// A writer that puts a header field in a file declares it, so the download
	// is recomputed when the creator changes it.
	if !slices.Contains((LumiverseModule{}).Declaration().Header, format.HeaderBlurb) {
		t.Error("the writer does not declare the blurb it writes")
	}
	if slices.Contains((SillyTavernModule{}).Declaration().Header, format.HeaderBlurb) {
		t.Error("the SillyTavern preset declares a blurb it has nowhere to put")
	}
}

func TestAVariableComesBackOnTheFragmentItBelongsTo(t *testing.T) {
	parsed := parse(t, lumiversePreset)
	written := write(t, LumiverseModule{}, parsed)
	again := parse(t, string(written.Body))

	list := promptList(t, again.Elements)
	schema := variableSchema(t, again.Elements)
	if len(schema.Variables) != 1 {
		t.Fatalf("read %d variables back, want 1", len(schema.Variables))
	}
	variable := schema.Variables[0]
	if variable.FragmentID == nil || *variable.FragmentID != list.Fragments[0].ID {
		t.Error("the variable came back on the wrong fragment")
	}
	if variable.Value == nil || *variable.Value.Text != "sharp" {
		t.Errorf("saved choice = %+v, want the one the creator saved", variable.Value)
	}
	if len(variable.Options) != 2 || variable.Options[1].Key != "sharp" ||
		variable.Options[1].Value != "clipped and cold" {
		t.Errorf("choices = %+v, want the key and the text apart", variable.Options)
	}
}

func pointerTo[T any](value T) *T { return &value }
