package asset

import (
	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/preset"
)

// kindsAskedForAnApp lists kinds whose initial slot names depend on an app. The
// choice seeds content but is not stored.
var kindsAskedForAnApp = map[string]struct{}{"preset": {}}

// KindAsksForAnApp reports whether creating this kind from nothing asks which
// app it is for.
func KindAsksForAnApp(kind string) bool {
	_, asked := kindsAskedForAnApp[kind]
	return asked
}

// Apps returns the apps a kind can be built for, in the order they are
// offered. A kind that is asked nothing returns none.
func Apps(kind string) []string {
	if !KindAsksForAnApp(kind) {
		return nil
	}
	apps := make([]string, 0, len(preset.Apps()))
	for _, app := range preset.Apps() {
		apps = append(apps, string(app))
	}
	return apps
}

// seedElements returns the elements a from-nothing asset starts with. Only a
// kind that is asked which app it is for has any, and what it returns is named
// slots with nothing in them. The seed supplies names and never values.
func seedElements(kind string, app string) ([]block.Element, error) {
	if !KindAsksForAnApp(kind) {
		if app != "" {
			return nil, ErrAppNotAnswered
		}
		return nil, nil
	}
	chosen := preset.App(app)
	if !chosen.Known() {
		return nil, ErrAppNotAnswered
	}
	return preset.Seed(chosen)
}
