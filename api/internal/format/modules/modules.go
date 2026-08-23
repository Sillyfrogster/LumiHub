// Package modules names every format module Illarin has, in one place.
package modules

import (
	"fmt"
	"slices"

	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/character"
	"github.com/Sillyfrogster/Illarin/api/internal/format/lorebook"
	"github.com/Sillyfrogster/Illarin/api/internal/format/pack"
	"github.com/Sillyfrogster/Illarin/api/internal/format/preset"
	"github.com/Sillyfrogster/Illarin/api/internal/format/theme"
	v1 "github.com/Sillyfrogster/Illarin/api/internal/format/v1"
)

// All returns every module, and adding one to Illarin means adding it here and nowhere else.
func All() []format.Module {
	readers := slices.Concat(
		character.Modules(), lorebook.Modules(), preset.Modules(), theme.Modules(), pack.Modules(),
	)
	all := make([]format.Module, 0, len(readers)+1)
	for _, module := range readers {
		all = append(all, module)
	}
	return append(all, v1.Module{})
}

// Registry returns every module registered and checked against each other.
func Registry() (*format.Registry, error) {
	registry := format.NewRegistry()
	for _, module := range All() {
		if err := registry.Register(module); err != nil {
			return nil, fmt.Errorf("register the %s module: %w", module.ID(), err)
		}
	}
	if err := registry.ValidateDeclarations(); err != nil {
		return nil, fmt.Errorf("format declarations: %w", err)
	}
	return registry, nil
}
