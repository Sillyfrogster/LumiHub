package format

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/google/uuid"
)

// writerModule is a declared writer with no reader, so the gates can be stated
// against declarations rather than against one shipped format's behaviour.
type writerModule struct {
	declaration Declaration
}

func (m writerModule) ID() string               { return m.declaration.ID }
func (m writerModule) Declaration() Declaration { return m.declaration }
func (writerModule) Write(context.Context, ExportAsset) (Artifact, error) {
	return Artifact{}, nil
}

func writerDeclaration(id string, grades map[block.Role]SupportGrade) Declaration {
	roles := make(map[block.Role]DirectionalRoleSupport, len(grades))
	for role, grade := range grades {
		roles[role] = DirectionalRoleSupport{
			Read:  RoleSupport{Grade: SupportNone},
			Write: RoleSupport{Grade: grade},
		}
	}
	return Declaration{
		ID: id, Label: id, Kind: "character", Direction: Direction{Write: true},
		Roles:         roles,
		Limits:        ContentLimits{PayloadBytes: 1024, CollectionItems: 100, ItemBytes: 100},
		ConsumedKeys:  []string{"payload"},
		Preservation:  PreservationDeclaration{Body: "card"},
		TestedOrigins: []string{id, OriginIllarin},
	}
}

func registryOf(t *testing.T, declarations ...Declaration) *Registry {
	t.Helper()
	registry := NewRegistry()
	for _, declaration := range declarations {
		if err := registry.Register(writerModule{declaration: declaration}); err != nil {
			t.Fatalf("register %s: %v", declaration.ID, err)
		}
	}
	return registry
}

func fullCharacterGrades() map[block.Role]SupportGrade {
	grades := make(map[block.Role]SupportGrade)
	for _, role := range block.Roles() {
		grades[role] = SupportFull
	}
	return grades
}

func described(role block.Role, body string) block.Element {
	return block.Element{
		ID: uuid.New(), Type: block.TypeProse, Role: role, Content: block.Prose{Text: body},
	}
}

func writtenGreetings(texts ...string) block.Element {
	items := make([]block.TextItem, 0, len(texts))
	for _, body := range texts {
		items = append(items, block.TextItem{ID: uuid.New(), Text: body})
	}
	return block.Element{
		ID: uuid.New(), Type: block.TypeTextSet, Role: block.RoleGreetings,
		Content: block.TextSet{Texts: items},
	}
}

func filledCharacter(extra ...block.Element) []block.Element {
	return append([]block.Element{
		described(block.RoleDescription, "Keeps the archive."),
		writtenGreetings("Hello"),
	}, extra...)
}

func targetNamed(targets []Target, id string) (Target, bool) {
	for _, target := range targets {
		if target.Format == id {
			return target, true
		}
	}
	return Target{}, false
}

// Structural compatibility makes a writer eligible for nothing. A writer that
// has not been tested against this asset's origin is not offered, however well
// its declaration lines up (ADR-0020).
func TestAnUntestedOriginOffersNoTarget(t *testing.T) {
	registry := registryOf(t, writerDeclaration("preset_lumiverse", fullCharacterGrades()))
	targets := registry.OfferedTargets(CapabilitySubject{
		Kind: "character", Origin: "chara_card_v2", Elements: filledCharacter(),
	})
	if len(targets) != 0 {
		t.Fatalf("targets = %+v, want none for an untested origin", targets)
	}
}

// The builder is an origin in its own right, so a character built from nothing
// is offered every writer tested against Illarin-authored assets.
func TestAnAssetBuiltFromNothingIsOfferedEveryWriterTestedAgainstIllarin(t *testing.T) {
	registry := registryOf(t,
		writerDeclaration("chara_card_v2", fullCharacterGrades()),
		writerDeclaration("chara_card_v3", fullCharacterGrades()),
	)
	targets := registry.OfferedTargets(CapabilitySubject{
		Kind: "character", Elements: filledCharacter(),
	})
	if len(targets) != 2 {
		t.Fatalf("targets = %+v, want both writers", targets)
	}
}

// A target is blocked only where the asset has content for a role the kind
// requires and none of it survives.
func TestATargetThatDropsARequiredRoleIsNotOffered(t *testing.T) {
	grades := fullCharacterGrades()
	grades[block.RoleGreetings] = SupportNone
	registry := registryOf(t,
		writerDeclaration("chara_card_v2", grades),
		writerDeclaration("chara_card_v3", fullCharacterGrades()),
	)
	targets := registry.OfferedTargets(CapabilitySubject{
		Kind: "character", Elements: filledCharacter(),
	})
	if _, offered := targetNamed(targets, "chara_card_v2"); offered {
		t.Error("a target that drops every greeting was offered")
	}
	if _, offered := targetNamed(targets, "chara_card_v3"); !offered {
		t.Error("the target that carries the greetings was not offered")
	}
}

// The test is loss, not emptiness. A creator who left a field alone is never
// punished for it, so a writer that carries no greetings at all is still
// offered for an asset that has none.
func TestEmptyInAndEmptyOutIsNoLossAndBlocksNothing(t *testing.T) {
	grades := fullCharacterGrades()
	grades[block.RoleGreetings] = SupportNone
	registry := registryOf(t, writerDeclaration("chara_card_v2", grades))
	targets := registry.OfferedTargets(CapabilitySubject{
		Kind:     "character",
		Elements: []block.Element{described(block.RoleDescription, "Keeps the archive.")},
	})
	if len(targets) != 1 {
		t.Fatalf("targets = %+v, want the target offered", targets)
	}
	if losses := targets[0].Losses(); len(losses) != 0 {
		t.Errorf("losses = %+v, want none reported for a field nobody filled in", losses)
	}
}

// Nothing is withheld for losing optional content, however much of it.
// Deciding a download is not worth having is the reader's call.
func TestATargetDroppingEveryOptionalRoleIsStillOffered(t *testing.T) {
	grades := map[block.Role]SupportGrade{
		block.RoleDescription: SupportFull,
		block.RoleGreetings:   SupportFull,
	}
	optional := []block.Element{}
	for _, role := range block.Roles() {
		if role == block.RoleDescription || role == block.RoleGreetings {
			continue
		}
		grades[role] = SupportNone
		if role.Allows(block.TypeProse) {
			optional = append(optional, described(role, "Something."))
		}
	}
	registry := registryOf(t, writerDeclaration("chara_card_v2", grades))
	targets := registry.OfferedTargets(CapabilitySubject{
		Kind: "character", Elements: filledCharacter(optional...),
	})
	if len(targets) != 1 {
		t.Fatalf("targets = %+v, want the target offered with its losses stated", targets)
	}
	if len(targets[0].Losses()) != len(optional) {
		t.Errorf("losses = %+v, want one per dropped optional role", targets[0].Losses())
	}
}

// A partial grade fires only where its condition holds against this asset, so
// the same format reports nothing for one asset and a reason for another.
func TestAPartialGradeFiresOnlyWhereItsConditionHolds(t *testing.T) {
	grades := fullCharacterGrades()
	declaration := writerDeclaration("chara_card_v2", grades)
	declaration.Roles[block.RoleGreetings] = DirectionalRoleSupport{
		Read: RoleSupport{Grade: SupportNone},
		Write: RoleSupport{
			Grade: SupportPartial,
			Condition: &ContentCondition{
				Description: "a name written on a greeting",
				Matches: func(content block.Content) bool {
					set, ok := content.(block.TextSet)
					if !ok {
						return false
					}
					return slices.ContainsFunc(set.Texts, func(item block.TextItem) bool {
						return item.Name != ""
					})
				},
			},
		},
	}
	registry := registryOf(t, declaration)

	plain := registry.OfferedTargets(CapabilitySubject{
		Kind: "character", Elements: filledCharacter(),
	})
	if losses := plain[0].Losses(); len(losses) != 0 {
		t.Errorf("losses = %+v, want none where the condition does not hold", losses)
	}

	named := writtenGreetings("Hello")
	set := named.Content.(block.TextSet)
	set.Texts[0].Name = "First meeting"
	named.Content = set
	withName := registry.OfferedTargets(CapabilitySubject{
		Kind: "character",
		Elements: []block.Element{
			described(block.RoleDescription, "Keeps the archive."), named,
		},
	})
	losses := withName[0].Losses()
	if len(losses) != 1 || losses[0].Verdict != Reduced ||
		!strings.Contains(losses[0].Reason, "a name written on a greeting") {
		t.Fatalf("losses = %+v, want one reduced verdict naming what went", losses)
	}
	if losses[0].Sample.Count != 1 || len(losses[0].Sample.Texts) != 1 {
		t.Errorf("sample = %+v, want a glance at what is at stake", losses[0].Sample)
	}
}

// A destination note is an independent fact from how much survives, so it
// rides on a carried verdict.
func TestADestinationNoteRidesOnACarriedVerdict(t *testing.T) {
	declaration := writerDeclaration("chara_card_v3", fullCharacterGrades())
	declaration.Roles[block.RoleCreatorNotes] = DirectionalRoleSupport{
		Read: RoleSupport{Grade: SupportNone},
		Write: RoleSupport{
			Grade: SupportFull, Destination: "an extensions namespace only some clients read",
		},
	}
	registry := registryOf(t, declaration)
	targets := registry.OfferedTargets(CapabilitySubject{
		Kind:     "character",
		Elements: filledCharacter(described(block.RoleCreatorNotes, "Built over a weekend.")),
	})
	var found RoleLoss
	for _, role := range targets[0].Roles {
		if role.Role == block.RoleCreatorNotes {
			found = role
		}
	}
	if found.Verdict != Carried || found.Destination == "" {
		t.Fatalf("creator notes verdict = %+v, want carried with a destination", found)
	}
	if len(targets[0].Losses()) != 0 {
		t.Errorf("losses = %+v, want a destination note counted as no loss", targets[0].Losses())
	}
}

// The recommendation is the widest-compatibility rule and not the least-loss
// rule. Recommending a format somebody's app refuses is a worse failure than
// dropping a gallery.
func TestTheRecommendationPrefersReachOverCarryingTheMost(t *testing.T) {
	wide := writerDeclaration("chara_card_v3", fullCharacterGrades())
	wide.Roles[block.RoleGallery] = DirectionalRoleSupport{
		Read: RoleSupport{Grade: SupportNone}, Write: RoleSupport{Grade: SupportNone},
	}
	narrow := writerDeclaration("charx", fullCharacterGrades())
	registry := registryOf(t, wide, narrow)

	gallery := block.Element{
		ID: uuid.New(), Type: block.TypeImageSet, Role: block.RoleGallery,
		Content: block.ImageSet{Images: []block.ImageItem{{ID: uuid.New(), MediaID: uuid.New()}}},
	}
	targets := registry.OfferedTargets(CapabilitySubject{
		Kind: "character", Elements: filledCharacter(gallery),
	})
	recommended, lossiest := "", ""
	for _, target := range targets {
		if target.Recommended {
			recommended = target.Format
		}
		if len(target.Losses()) == 0 {
			lossiest = target.Format
		}
	}
	if lossiest != "charx" {
		t.Fatalf("the narrow target lost something; the rules do not disagree here")
	}
	if recommended != "chara_card_v3" {
		t.Fatalf("recommended = %q, want the format more apps can open", recommended)
	}
}

// A cross-platform target is offered only where the creator has allowed it,
// and nothing grants an allowance yet.
func TestACrossPlatformTargetIsRefusedWithoutAnAllowance(t *testing.T) {
	declaration := writerDeclaration("preset_sillytavern", fullCharacterGrades())
	declaration.CrossPlatform = true
	declaration.TestedOrigins = append(declaration.TestedOrigins, "chara_card_v2")
	registry := registryOf(t, declaration)
	subject := CapabilitySubject{
		Kind: "character", Origin: "chara_card_v2", Elements: filledCharacter(),
	}
	if targets := registry.OfferedTargets(subject); len(targets) != 0 {
		t.Fatalf("targets = %+v, want a cross-platform target withheld", targets)
	}
	subject.AllowedCrossPlatform = []string{"preset_sillytavern"}
	if targets := registry.OfferedTargets(subject); len(targets) != 1 {
		t.Fatal("an allowed cross-platform target was still withheld")
	}
}

// Preserved data goes back to the family it came from and nowhere else.
func TestPreservedDataTravelsByOriginMatchAlone(t *testing.T) {
	card := writerDeclaration("chara_card_v3", fullCharacterGrades())
	card.Preservation = PreservationDeclaration{Body: "card", Container: []string{"extensions"}}
	sibling := writerDeclaration("charx", fullCharacterGrades())
	sibling.Preservation = card.Preservation
	stranger := writerDeclaration("theme_lumiverse", fullCharacterGrades())
	stranger.Preservation = PreservationDeclaration{Body: "bundle"}

	if !TravelsWithOrigin(card, sibling) {
		t.Error("preserved data did not travel to its own family")
	}
	if TravelsWithOrigin(card, stranger) {
		t.Error("preserved data reached another family")
	}
}

// A declaration change is a deploy, and the stamp is what tells a stored report
// it was computed under a contract that no longer holds.
func TestTheCapabilityStampMovesWithADeclaration(t *testing.T) {
	before := registryOf(t, writerDeclaration("chara_card_v2", fullCharacterGrades()))
	grades := fullCharacterGrades()
	grades[block.RoleGallery] = SupportNone
	after := registryOf(t, writerDeclaration("chara_card_v2", grades))

	if before.CapabilityStamp() == after.CapabilityStamp() {
		t.Fatal("the stamp did not move when a writer's role support changed")
	}
	if before.CapabilityStamp() != registryOf(t,
		writerDeclaration("chara_card_v2", fullCharacterGrades()),
	).CapabilityStamp() {
		t.Fatal("the stamp is not stable for one contract")
	}
}
