package preset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
)

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
