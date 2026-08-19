package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func startPreset(t *testing.T, r http.Handler, session *http.Cookie, app string) startedAsset {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/assets",
		strings.NewReader(`{"kind":"preset","app":"`+app+`"}`))
	request.Header.Set("Content-Type", "application/json")
	response := send(t, r, authorized(request, session))
	if response.Code != http.StatusCreated {
		t.Fatalf("start a preset for %s: status = %d, want 201: %s",
			app, response.Code, response.Body.String())
	}
	var started startedAsset
	if err := json.Unmarshal(response.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode the started asset: %v", err)
	}
	return started
}

// Creating a preset asks which app it is for, once. Without an answer there
// are no names to give the settings, so the question is not skippable.
func TestAPresetCannotBeStartedWithoutSayingWhichAppItIsFor(t *testing.T) {
	r, session := newVerifiedTestRouter(t)

	for _, body := range []string{`{"kind":"preset"}`, `{"kind":"preset","app":""}`} {
		request := httptest.NewRequest(http.MethodPost, "/v1/assets", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := send(t, r, authorized(request, session))
		if response.Code != http.StatusBadRequest {
			t.Errorf("POST %s status = %d, want 400: %s",
				body, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), "sillytavern") {
			t.Errorf("refusal of %s does not name the apps: %s", body, response.Body.String())
		}
	}
}

// A kind that depends on no app is asked nothing, and an answer sent for one
// is refused rather than quietly ignored.
func TestAKindThatDependsOnNoAppRefusesAnAnswer(t *testing.T) {
	r, session := newVerifiedTestRouter(t)

	request := httptest.NewRequest(http.MethodPost, "/v1/assets",
		strings.NewReader(`{"kind":"character","app":"sillytavern"}`))
	request.Header.Set("Content-Type", "application/json")
	response := send(t, r, authorized(request, session))
	if response.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400: %s", response.Code, response.Body.String())
	}
}

// The answer seeds the slot names and nothing else. The settings arrive named
// and empty, and the two apps share none of their names.
func TestTheAppAnsweredSeedsItsOwnSlotNamesAndNoValues(t *testing.T) {
	r, session := newVerifiedTestRouter(t)

	named := map[string][]string{}
	for _, app := range []string{"sillytavern", "lumiverse"} {
		started := startPreset(t, r, session, app)

		core := blockNamed(t, started.Blocks, "preset_core")
		if !core.Required || core.Hideable || !core.IsEmpty {
			t.Errorf("%s preset core = %+v, want required, not hideable and empty", app, core)
		}

		settings := blockNamed(t, started.Blocks, "settings")
		if len(settings.Elements) != 3 {
			t.Fatalf("%s settings holds %d elements, want three groups",
				app, len(settings.Elements))
		}
		for _, element := range settings.Elements {
			if element.Type != "setting_group" {
				t.Errorf("%s settings element = %s, want a setting group", app, element.Type)
			}
			var group struct {
				Settings []struct {
					Name  string          `json:"name"`
					Type  string          `json:"type"`
					Value json.RawMessage `json:"value"`
				} `json:"settings"`
			}
			if err := json.Unmarshal(element.Content, &group); err != nil {
				t.Fatalf("decode %s %s: %v", app, element.Role, err)
			}
			if len(group.Settings) == 0 {
				t.Errorf("%s %s arrived with no slot names", app, element.Role)
			}
			for _, setting := range group.Settings {
				if setting.Value != nil {
					t.Errorf("%s %s seeded %s with a value", app, element.Role, setting.Name)
				}
				if setting.Type == "" {
					t.Errorf("%s %s seeded %s with no type", app, element.Role, setting.Name)
				}
				named[app] = append(named[app], setting.Name)
			}
			// A group of named slots with nothing in any of them shows a
			// reader nothing, and is a form its owner fills in.
			if !element.IsEmpty {
				t.Errorf("%s %s reads as content before anyone filled it in", app, element.Role)
			}
		}

		nudges := blockNamed(t, started.Blocks, "nudges")
		if len(nudges.Elements) != 1 || nudges.Elements[0].Type != "text_set" {
			t.Fatalf("%s nudges = %+v, want one text set", app, nudges.Elements)
		}

		// Absent until something fills them: neither is part of the seed.
		for _, definition := range []string{"variables", "scripts"} {
			for _, held := range started.Blocks {
				if held.Definition == definition {
					t.Errorf("%s seeded a %s section nothing fills", app, definition)
				}
			}
		}
	}

	for _, one := range named["sillytavern"] {
		for _, other := range named["lumiverse"] {
			if one == other && one != "temperature" && one != "seed" {
				t.Errorf("both apps seeded %q, and they share no settings names", one)
			}
		}
	}
}

// The app is not identity. It seeds names and is stored nowhere, so a preset
// built by hand has no origin format and nothing offers to switch it.
func TestTheAppAnsweredIsStoredNowhere(t *testing.T) {
	r, session := newVerifiedTestRouter(t)

	started := startPreset(t, r, session, "sillytavern")

	page := readAsset(t, r, session, started.ID)
	if strings.Contains(page, "sillytavern") {
		t.Errorf("the preset page carries the app that was answered: %s", page)
	}
}

// A preset publishes on a name, an answered adult content question and one
// prompt fragment, and the checklist names the section holding them.
func TestAPresetIsReadyToPublishOnItsNameRatingAndOneFragment(t *testing.T) {
	r, session := newVerifiedTestRouter(t)

	started := startPreset(t, r, session, "lumiverse")

	var fragments *string
	for _, item := range started.Readiness {
		if item.ID != "prompt_fragments" {
			continue
		}
		fragments = item.BlockID
		if item.Met {
			t.Error("an empty preset already has a prompt fragment")
		}
	}
	if fragments == nil {
		t.Fatalf("readiness = %+v, want a prompt fragment naming its section", started.Readiness)
	}
	core := blockNamed(t, started.Blocks, "preset_core")
	if *fragments != core.ID {
		t.Errorf("the fragment check points at %s, want the preset core", *fragments)
	}
}

func readAsset(t *testing.T, r http.Handler, session *http.Cookie, id string) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/v1/assets/"+id, nil)
	response := send(t, r, authorized(request, session))
	if response.Code != http.StatusOK {
		t.Fatalf("read the asset: status = %d: %s", response.Code, response.Body.String())
	}
	return response.Body.String()
}
