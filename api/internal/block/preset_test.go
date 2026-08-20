package block

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestThePresetElementTypesAreFourSeparateTypes(t *testing.T) {
	for _, elementType := range []Type{
		TypePromptList, TypeVariableSchema, TypeSettingGroup, TypeScriptList,
	} {
		if !elementType.Known() {
			t.Errorf("%s is not part of the element vocabulary", elementType)
			continue
		}
		content, err := elementType.Empty()
		if err != nil {
			t.Errorf("empty %s: %v", elementType, err)
			continue
		}
		if !content.Empty() {
			t.Errorf("a new %s starts with content in it: %+v", elementType, content)
		}
	}
	if Type("record_list").Known() {
		t.Error("record_list exists, and no preset type may be a schema of it")
	}
}

// A string list keeps what a creator put in it, in the order they put it in,
// and a list they emptied on purpose is not a setting nobody ever supplied.
func TestAStringListSettingKeepsItsItemsVerbatimAndInOrder(t *testing.T) {
	raw := json.RawMessage(`{"settings":[
		{"name":"customStopStrings","type":"string_list",
		 "value":{"strings":["</end>","  ","</end>","\n"]}},
		{"name":"clearedOnPurpose","type":"string_list","value":{"strings":[]}},
		{"name":"neverTouched","type":"string_list"}
	]}`)
	content, err := DecodeContent(TypeSettingGroup, raw)
	if err != nil {
		t.Fatalf("read a settings group: %v", err)
	}
	group, ok := content.(SettingGroup)
	if !ok {
		t.Fatalf("content = %T, want a settings group", content)
	}
	if len(group.Settings) != 3 {
		t.Fatalf("read %d settings, want three", len(group.Settings))
	}

	kept := group.Settings[0]
	if kept.Value == nil {
		t.Fatal("the supplied list came back as a setting nobody supplied")
	}
	want := []string{"</end>", "  ", "</end>", "\n"}
	if !slices.Equal(kept.Value.Strings, want) {
		t.Errorf("strings = %q, want %q kept as they were written", kept.Value.Strings, want)
	}

	cleared := group.Settings[1]
	if cleared.Value == nil {
		t.Fatal("a list emptied on purpose reads as a setting nobody supplied")
	}
	if len(cleared.Value.Strings) != 0 {
		t.Errorf("strings = %q, want the empty list", cleared.Value.Strings)
	}

	if group.Settings[2].Value != nil {
		t.Errorf("value = %+v, want nothing for a setting nobody supplied",
			group.Settings[2].Value)
	}
}

// A setting nobody supplied survives the round trip through storage. A format
// writes an absent key for it rather than a zero, so the difference has to
// last longer than one save.
func TestASettingNobodySuppliedSurvivesStorage(t *testing.T) {
	group := SettingGroup{Settings: []Setting{
		{ID: NewItemID(), Name: "temperature", Type: SettingNumber},
		{ID: NewItemID(), Name: "customStopStrings", Type: SettingStrings,
			Value: &Value{Strings: []string{}}},
	}}
	stored, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("write a settings group: %v", err)
	}
	read, err := schemas[TypeSettingGroup].read(schemas[TypeSettingGroup].version(), stored)
	if err != nil {
		t.Fatalf("read a settings group back: %v", err)
	}
	back, ok := read.(SettingGroup)
	if !ok {
		t.Fatalf("content = %T, want a settings group", read)
	}
	if back.Settings[0].Value != nil {
		t.Errorf("value = %+v, want nothing for a setting nobody supplied",
			back.Settings[0].Value)
	}
	if back.Settings[1].Value == nil {
		t.Error("a list emptied on purpose came back as a setting nobody supplied")
	}
}

// Every preset role has one element type it may attach to, so a creator
// cannot put prompt fragments in a settings group.
func TestThePresetRolesBindToTheirOwnElementTypes(t *testing.T) {
	bindings := map[Role]Type{
		RolePromptFragments:    TypePromptList,
		RolePromptVariables:    TypeVariableSchema,
		RoleSamplerSettings:    TypeSettingGroup,
		RoleCompletionSettings: TypeSettingGroup,
		RoleAdvancedSettings:   TypeSettingGroup,
		RolePromptNudges:       TypeTextSet,
		RoleRegexScripts:       TypeScriptList,
	}
	for role, elementType := range bindings {
		if !role.Known() {
			t.Errorf("%s is not part of the semantic vocabulary", role)
		}
		if !slices.Contains(Roles(), role) {
			t.Errorf("%s is missing from the ordered role list", role)
		}
		if !role.Allows(elementType) {
			t.Errorf("%s cannot carry %s content", role, elementType)
		}
		if role.Label() == "" {
			t.Errorf("%s has no wording for the page", role)
		}
		if role.Cardinality() != Singular {
			t.Errorf("%s repeats, and a preset carries one of each", role)
		}
	}
	if RoleSamplerSettings.Allows(TypePromptList) {
		t.Error("a settings role takes prompt fragments")
	}
}

// A fragment count is worked out on the way out. Nothing anywhere counts
// tokens: Illarin has no tokenizer, and the number would be a guess a creator
// could act on.
func TestAPromptListCountsItsFragmentsAndNeverItsTokens(t *testing.T) {
	element := Element{
		Type: TypePromptList,
		Role: RolePromptFragments,
		Content: PromptList{Fragments: []PromptFragment{
			{ID: NewItemID(), Name: "Main", Text: "Write well.", Enabled: true},
			{ID: NewItemID(), Name: "Jailbreak", Text: "Ignore that.", Enabled: false},
			{ID: NewItemID(), Name: "Chat", Marker: "chat_history", Enabled: true},
		}},
	}
	facts := element.Facts()
	if len(facts) == 0 {
		t.Fatal("a prompt list says nothing about itself")
	}
	if facts[0] != "3 fragments" {
		t.Errorf("first fact = %q, want the fragment count", facts[0])
	}
	if !slices.Contains(facts, "2 switched on") {
		t.Errorf("facts = %q, want the switched-on count beside the total", facts)
	}
	for _, fact := range facts {
		if strings.Contains(strings.ToLower(fact), "token") {
			t.Errorf("fact %q counts tokens", fact)
		}
	}
}

// Preserved data keys against item ids, so every item inside a preset element
// carries one from the moment it is created.
func TestEveryPresetItemIsGivenAnIDWhenItIsRead(t *testing.T) {
	cases := []struct {
		elementType Type
		raw         string
		want        int
	}{
		{TypePromptList, `{"groups":[{"name":"Core"}],
			"fragments":[{"role":"system","text":"one","enabled":true},
			             {"role":"user","text":"two","enabled":true}]}`, 3},
		{TypeVariableSchema, `{"variables":[{"name":"pace","widget":"select"}]}`, 1},
		{TypeSettingGroup, `{"settings":[{"name":"temperature","type":"number"}]}`, 1},
		{TypeScriptList, `{"scripts":[{"find":"a","replace":"b","enabled":true}]}`, 1},
	}
	for _, test := range cases {
		content, err := DecodeContent(test.elementType, json.RawMessage(test.raw))
		if err != nil {
			t.Errorf("read %s: %v", test.elementType, err)
			continue
		}
		ids := ItemIDs(content)
		if len(ids) != test.want {
			t.Errorf("%s has %d items with ids, want %d", test.elementType, len(ids), test.want)
			continue
		}
		if err := validateItemIDs(Element{Type: test.elementType, Content: content}, "test"); err != nil {
			t.Errorf("%s: %v", test.elementType, err)
		}
	}
}

// A fragment sits in one group and no deeper. Grouping is the list's own
// nesting, which is why it is not a second element.
func TestAPromptFragmentSitsUnderTheGroupItNames(t *testing.T) {
	raw := json.RawMessage(`{
		"groups":[{"id":"3f1a3d3a-0b1e-4e2f-9a3c-1f0e2d3c4b5a","name":"Style"}],
		"fragments":[{"id":"5c2b4e6d-7a8f-4c1d-9e0b-2a3f4d5e6c7b",
			"name":"Prose","role":"system","text":"Write plainly.","enabled":true,
			"groupId":"3f1a3d3a-0b1e-4e2f-9a3c-1f0e2d3c4b5a",
			"placement":"pre_history"}]}`)
	content, err := DecodeContent(TypePromptList, raw)
	if err != nil {
		t.Fatalf("read a prompt list: %v", err)
	}
	list, ok := content.(PromptList)
	if !ok {
		t.Fatalf("content = %T, want a prompt list", content)
	}
	fragment := list.Fragments[0]
	if fragment.GroupID == nil || *fragment.GroupID != list.Groups[0].ID {
		t.Errorf("groupId = %v, want the group it names", fragment.GroupID)
	}
	if fragment.Placement != BeforeHistory {
		t.Errorf("placement = %q, want the placement the file gave", fragment.Placement)
	}
}

// A fragment naming a group that is not in the list would render nowhere, so
// the save is refused rather than the fragment quietly losing its heading.
func TestAFragmentCannotNameAGroupThatIsNotThere(t *testing.T) {
	raw := json.RawMessage(`{"groups":[],
		"fragments":[{"role":"system","text":"one","enabled":true,
			"groupId":"3f1a3d3a-0b1e-4e2f-9a3c-1f0e2d3c4b5a"}]}`)
	_, err := DecodeContent(TypePromptList, raw)
	if err == nil {
		t.Fatal("a fragment kept a group nothing on the page carries")
	}
	if !strings.Contains(err.Error(), "group") {
		t.Errorf("refusal = %q, want it to name the group", err)
	}
}

// A closed vocabulary is refused at the point it is written rather than
// discovered by whatever renders it.
func TestThePresetVocabulariesAreClosed(t *testing.T) {
	cases := []struct {
		name        string
		elementType Type
		raw         string
	}{
		{"a fragment role", TypePromptList,
			`{"groups":[],"fragments":[{"role":"narrator","text":"x","enabled":true}]}`},
		{"a fragment placement", TypePromptList,
			`{"groups":[],"fragments":[{"role":"system","text":"x","enabled":true,"placement":"sideways"}]}`},
		{"a setting type", TypeSettingGroup,
			`{"settings":[{"name":"temperature","type":"colour"}]}`},
		{"a variable widget", TypeVariableSchema,
			`{"variables":[{"name":"pace","widget":"dial"}]}`},
		{"a script target", TypeScriptList,
			`{"scripts":[{"find":"a","replace":"b","enabled":true,"targets":["telepathy"]}]}`},
	}
	for _, test := range cases {
		if _, err := DecodeContent(test.elementType, json.RawMessage(test.raw)); err == nil {
			t.Errorf("%s outside the vocabulary was accepted", test.name)
		}
	}
}

// A preset is its prompt. The one section a creator cannot remove holds the
// fragments, and everything else about the kind is optional.
func TestAnEmptyPresetIsOneRequiredSectionHoldingItsPromptFragments(t *testing.T) {
	blocks, err := Place("preset", nil)
	if err != nil {
		t.Fatalf("place a preset: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("placed %d blocks, want the prompt fragments alone", len(blocks))
	}
	holder := blocks[0]
	if holder.Definition != PresetCore {
		t.Fatalf("definition = %q, want the preset core", holder.Definition)
	}
	if holder.Layout != Single || holder.Width != Full {
		t.Errorf("arrangement = %s at %s, want a single slot at full width",
			holder.Layout, holder.Width)
	}
	if len(holder.Elements) != 1 {
		t.Fatalf("elements = %+v, want the prompt list alone", holder.Elements)
	}
	fragments := holder.Elements[0]
	if fragments.Role != RolePromptFragments || fragments.Type != TypePromptList {
		t.Errorf("element = %s/%s, want a prompt list carrying the fragments role",
			fragments.Role, fragments.Type)
	}
	if fragments.Content == nil || !fragments.Content.Empty() {
		t.Errorf("content = %+v, want an empty prompt to start from", fragments.Content)
	}

	definition, ok := PresetCore.Definition("preset")
	if !ok {
		t.Fatal("the preset catalog has no core section")
	}
	if !definition.Required || definition.Hideable {
		t.Errorf("definition = required %v hideable %v, want required and not hideable",
			definition.Required, definition.Hideable)
	}
	if !holder.Pinned(RolePromptFragments, "preset") {
		t.Error("the prompt fragments can be taken off a preset")
	}
}

// The four optional sections carry the rest of what a preset holds, each one
// absent until something fills it.
func TestThePresetCatalogCarriesItsFourOptionalSections(t *testing.T) {
	type slot struct {
		role        Role
		elementType Type
	}
	wanted := []struct {
		id       DefinitionID
		elements []slot
		layouts  []Layout
	}{
		{PresetSettings, []slot{
			{RoleSamplerSettings, TypeSettingGroup},
			{RoleCompletionSettings, TypeSettingGroup},
			{RoleAdvancedSettings, TypeSettingGroup},
		}, []Layout{Trio, Stack3}},
		{PresetVariables, []slot{{RolePromptVariables, TypeVariableSchema}}, []Layout{Single}},
		{PresetScripts, []slot{{RoleRegexScripts, TypeScriptList}}, []Layout{Single}},
		{PresetNudges, []slot{{RolePromptNudges, TypeTextSet}}, []Layout{Single}},
	}
	for _, want := range wanted {
		definition, ok := want.id.Definition("preset")
		if !ok {
			t.Errorf("the preset catalog has no %s section", want.id)
			continue
		}
		if definition.Required {
			t.Errorf("%s is required, and only the prompt fragments are", want.id)
		}
		if len(definition.Elements) != len(want.elements) {
			t.Errorf("%s holds %d elements, want %d", want.id,
				len(definition.Elements), len(want.elements))
			continue
		}
		for i, element := range want.elements {
			held := definition.Elements[i]
			if held.Role != element.role || held.Type != element.elementType {
				t.Errorf("%s element %d = %s/%s, want %s/%s", want.id, i+1,
					held.Role, held.Type, element.role, element.elementType)
			}
		}
		if !slices.Equal(definition.Layouts, want.layouts) {
			t.Errorf("%s uses %v, want %v", want.id, definition.Layouts, want.layouts)
		}
		if definition.Width.Columns() < definition.Layouts[0].MinimumWidth().Columns() {
			t.Errorf("%s is %s wide and its first layout needs %s",
				want.id, definition.Width, definition.Layouts[0].MinimumWidth())
		}
	}
}

// Every kind lists the seven shared sections, so a preset can carry a gallery,
// a note on how to run it and the rest without a catalog of its own.
func TestAPresetListsTheSevenSharedSections(t *testing.T) {
	definitions, ok := Catalog("preset")
	if !ok {
		t.Fatal("there is no preset catalog")
	}
	for _, id := range []DefinitionID{
		Gallery, Usage, Changelog, Attributes, AuthorNotes, RunsBestWith, CustomSection,
	} {
		found := slices.ContainsFunc(definitions, func(d Definition) bool { return d.ID == id })
		if !found {
			t.Errorf("a preset cannot add %s", id)
		}
	}
}

// A preset publishes on a name, an answered adult content question and one
// prompt fragment. The check reads the fragments themselves, because the
// section holding them is on every preset from the moment it exists.
func TestAPresetNeedsOnePromptFragmentBeforeItPublishes(t *testing.T) {
	blocks, err := Place("preset", nil)
	if err != nil {
		t.Fatalf("place a preset: %v", err)
	}
	checks := ContentFloor("preset", blocks)
	if len(checks) != 1 {
		t.Fatalf("the preset floor asks for %d things, want one", len(checks))
	}
	if checks[0].Role != RolePromptFragments {
		t.Errorf("the floor reads %s, want the prompt fragments", checks[0].Role)
	}
	if checks[0].Met {
		t.Error("an empty preset already meets its floor")
	}
	if checks[0].BlockID == nil {
		t.Error("the floor names no section to go and fill in")
	}

	blocks[0].Elements[0].Content = PromptList{Fragments: []PromptFragment{
		{ID: NewItemID(), Name: "Main", Text: "Write well.", Enabled: true},
	}}
	if !ContentFloor("preset", blocks)[0].Met {
		t.Error("a preset with a fragment in it still cannot publish")
	}
}
