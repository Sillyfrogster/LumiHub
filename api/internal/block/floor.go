package block

import "github.com/google/uuid"

// Requirement names the element role that must contain content before publish.
type Requirement struct {
	// ID stays the same when the wording changes, so a page can anchor on it.
	ID     string
	Label  string
	Detail string
	Role   Role
}

// Check is one requirement measured against an asset, with the block a creator
// fills it in.
type Check struct {
	Requirement
	Met bool
	// BlockID is nil where the asset carries no block for the role.
	BlockID *uuid.UUID
}

// contentFloors is what each kind asks for beyond a name and an adult content
// answer. It is content in the one element that makes the kind mean anything,
// and a kind with no entry asks for nothing yet.
var contentFloors = map[string][]Requirement{
	"character": {
		{
			ID:     "description",
			Label:  "Description",
			Detail: "Write the description this character is built on.",
			Role:   RoleDescription,
		},
		{
			ID:     "greetings",
			Label:  "Greeting",
			Detail: "Write at least one opening message.",
			Role:   RoleGreetings,
		},
	},
	"lorebook": {
		{
			ID:     "entries",
			Label:  "Entry",
			Detail: "Write at least one entry.",
			Role:   RoleLorebookEntries,
		},
	},
	"preset": {
		{
			ID:     "prompt_fragments",
			Label:  "Prompt fragment",
			Detail: "Write at least one prompt fragment.",
			Role:   RolePromptFragments,
		},
	},
	"theme": {
		{
			ID:     "palette",
			Label:  "Palette",
			Detail: "Add at least one colour to the palette.",
			Role:   RoleThemeTokens,
		},
	},
}

// ContentFloor measures a kind's content requirements against an asset's
// blocks, in the order a creator reads them.
func ContentFloor(kind string, blocks []Block) []Check {
	requirements := contentFloors[kind]
	checks := make([]Check, 0, len(requirements))
	for _, requirement := range requirements {
		check := Check{Requirement: requirement}
		for _, holder := range blocks {
			for _, element := range holder.Elements {
				if element.Role != requirement.Role {
					continue
				}
				written := element.Content != nil && !element.Content.Empty()
				if check.BlockID == nil || (written && !check.Met) {
					id := holder.ID
					check.BlockID = &id
				}
				check.Met = check.Met || written
			}
		}
		checks = append(checks, check)
	}
	return checks
}

// RequiredRoles names the roles a kind asks for before an asset may be
// published. Export reads the same list, because a target that would drop
// content a kind requires is not the asset and is never offered.
func RequiredRoles(kind string) []Role {
	requirements := contentFloors[kind]
	roles := make([]Role, 0, len(requirements))
	for _, requirement := range requirements {
		roles = append(roles, requirement.Role)
	}
	return roles
}
