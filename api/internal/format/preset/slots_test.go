package preset

import (
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
)

// Every app Illarin offers has slot names for all three settings groups and
// for its nudges, so choosing one never lands a creator on an empty form.
func TestEveryAppOfferedHasSlotNamesForEveryGroup(t *testing.T) {
	for _, app := range Apps() {
		if !app.Known() || app.Label() == "" {
			t.Errorf("%s is offered and has no name of its own", app)
		}
		elements, err := Seed(app)
		if err != nil {
			t.Fatalf("seed %s: %v", app, err)
		}
		if len(elements) != 4 {
			t.Fatalf("%s seeded %d elements, want three settings groups and the nudges",
				app, len(elements))
		}
		for _, role := range []block.Role{
			block.RoleSamplerSettings, block.RoleCompletionSettings,
			block.RoleAdvancedSettings, block.RolePromptNudges,
		} {
			found := false
			for _, element := range elements {
				if element.Role != role {
					continue
				}
				found = true
				// A group of named slots reads as empty on purpose, because
				// names are a form and not content, so this counts the items.
				if len(block.ItemIDs(element.Content)) == 0 {
					t.Errorf("%s seeded %s with no slot names", app, role)
				}
			}
			if !found {
				t.Errorf("%s seeded nothing for %s", app, role)
			}
		}
	}
}

// The seed supplies names and never values. A slot nobody filled in is what a
// writer leaves out of the file rather than writing a zero into.
func TestTheSeedSuppliesNamesAndNoValues(t *testing.T) {
	elements, err := Seed(SillyTavern)
	if err != nil {
		t.Fatalf("seed SillyTavern: %v", err)
	}
	for _, element := range elements {
		group, ok := element.Content.(block.SettingGroup)
		if !ok {
			continue
		}
		if group.Supplied() != 0 {
			t.Errorf("%s arrived with %d settings filled in", element.Role, group.Supplied())
		}
		for _, setting := range group.Settings {
			if setting.Name == "" {
				t.Errorf("%s carries a slot with no name", element.Role)
			}
			if !setting.Type.Known() {
				t.Errorf("%s carries %s at unknown type %q",
					element.Role, setting.Name, setting.Type)
			}
		}
	}
}

// An app Illarin has no slot names for is refused rather than seeded empty.
func TestAnAppWithNoSlotNamesIsRefused(t *testing.T) {
	if _, err := Seed(App("koboldcpp")); err == nil {
		t.Error("an app Illarin knows nothing about was seeded anyway")
	}
}

// The two apps share none of their settings names. Illarin places the names it
// is given and models nothing about what any of them controls, so a name in
// one app's file means nothing in the other's.
func TestTheTwoAppsShareAlmostNoSettingsNames(t *testing.T) {
	names := map[App]map[string]struct{}{}
	for _, app := range Apps() {
		elements, err := Seed(app)
		if err != nil {
			t.Fatalf("seed %s: %v", app, err)
		}
		names[app] = map[string]struct{}{}
		for _, element := range elements {
			group, ok := element.Content.(block.SettingGroup)
			if !ok {
				continue
			}
			for _, setting := range group.Settings {
				names[app][setting.Name] = struct{}{}
			}
		}
	}
	shared := []string{}
	for name := range names[SillyTavern] {
		if _, both := names[Lumiverse][name]; both {
			shared = append(shared, name)
		}
	}
	// temperature and seed are spelled the same way in both, and nothing else
	// is. A longer list means one app's names have leaked into the other's.
	if len(shared) > 2 {
		t.Errorf("the two apps share %v, want no more than temperature and seed", shared)
	}
}
