package format

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
)

// App describes formats an application can open for download recommendations.
type App struct {
	ID    string
	Label string
	// Reads names the formats the app opens.
	Reads []string
}

// Apps returns the applications Illarin names, in the order it names them.
func Apps() []App {
	return []App{
		{ID: "sillytavern", Label: "SillyTavern", Reads: []string{"chara_card_v2", "chara_card_v3"}},
		{ID: "risu", Label: "RisuAI", Reads: []string{"chara_card_v2", "chara_card_v3", "charx"}},
		{ID: "lumiverse", Label: "Lumiverse", Reads: []string{"chara_card_v2", "chara_card_v3"}},
	}
}

// reach is how many of the named apps open a format. It is the whole of what
// "the formats most apps can open" means.
func reach(formatID string) int {
	count := 0
	for _, app := range Apps() {
		if slices.Contains(app.Reads, formatID) {
			count++
		}
	}
	return count
}

// Verdict is how much of one role's content a target keeps. Where that content
// lands is a separate fact, carried by a destination note on any of the three.
type Verdict string

const (
	Carried Verdict = "carried"
	Reduced Verdict = "reduced"
	Dropped Verdict = "dropped"
)

// RoleLoss is one role measured against one target, on one asset.
type RoleLoss struct {
	Role    block.Role `json:"role"`
	Label   string     `json:"label"`
	Verdict Verdict    `json:"verdict"`
	// Reason names what went, and stands only on a reduced verdict.
	Reason string `json:"reason,omitempty"`
	// Destination names where the content lands when that is not the format's
	// standard home for it, such as an extensions namespace only some clients
	// read.
	Destination string `json:"destination,omitempty"`
	// Sample is a glance at what is at stake, so a reader recognises in two
	// seconds whether they care.
	Sample block.Sample `json:"sample"`
}

// Lossy reports whether this verdict costs the asset something.
func (l RoleLoss) Lossy() bool { return l.Verdict != Carried }

// Target is one format an asset may be downloaded as, and what the trip costs
// it. A target that is not offered is absent rather than listed as
// unavailable, so the menu is a list of choices.
type Target struct {
	Format string `json:"format"`
	Label  string `json:"label"`
	// Recommended is computed by the widest-compatibility rule and is never a
	// creator's choice, because a loss report is a fact where a preference is
	// not.
	Recommended bool       `json:"recommended"`
	Roles       []RoleLoss `json:"roles"`
}

// Losses returns the verdicts that cost the asset something.
func (t Target) Losses() []RoleLoss {
	losses := make([]RoleLoss, 0, len(t.Roles))
	for _, role := range t.Roles {
		if role.Lossy() {
			losses = append(losses, role)
		}
	}
	return losses
}

// CapabilitySubject is the asset as the export gates read it.
type CapabilitySubject struct {
	Kind string
	// Origin is the format the asset arrived in. Empty means it was authored
	// in Illarin, which is an origin in its own right and never an unknown.
	Origin string
	// Elements are the asset's content, hidden blocks included.
	Elements []block.Element
	// AllowedCrossPlatform names the cross-platform targets this creator has
	// allowed. Nothing grants an allowance yet, so a cross-platform target is
	// refused. The three character card formats are ordinary targets and never
	// meet this gate.
	AllowedCrossPlatform []string
}

func (s CapabilitySubject) origin() string {
	if s.Origin == "" {
		return OriginIllarin
	}
	return s.Origin
}

// OfferedTargets returns tested, permitted formats and their content costs.
func (r *Registry) OfferedTargets(subject CapabilitySubject) []Target {
	ids := make([]string, 0, len(r.modules))
	for id := range r.modules {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	offered := make([]Target, 0, len(ids))
	for _, id := range ids {
		declaration := r.modules[id].Declaration()
		if !declaration.Direction.Write || declaration.Kind != subject.Kind {
			continue
		}
		if !slices.Contains(declaration.TestedOrigins, subject.origin()) {
			continue
		}
		if declaration.CrossPlatform &&
			!slices.Contains(subject.AllowedCrossPlatform, declaration.ID) {
			continue
		}
		roles, survives := lossReport(declaration, subject)
		if !survives {
			continue
		}
		offered = append(offered, Target{
			Format: declaration.ID, Label: declaration.Label, Roles: roles,
		})
	}
	recommend(offered, r)
	return offered
}

// lossReport measures one writer's declaration against what this asset really
// holds, and reports whether the asset survives the trip.
//
// A role the asset has no content for gets no verdict at all. The test is loss
// rather than emptiness, so a creator who left a field alone is never punished
// for it. A target is blocked only where the asset has content for a role its
// kind requires and none of it survives. Nothing is withheld for losing
// optional content, however much of it.
func lossReport(declaration Declaration, subject CapabilitySubject) ([]RoleLoss, bool) {
	required := block.RequiredRoles(subject.Kind)
	report := make([]RoleLoss, 0, len(block.Roles()))
	for _, role := range block.Roles() {
		written := writtenContent(subject.Elements, role)
		if len(written) == 0 {
			continue
		}
		support := declaration.Roles[role].Write
		loss := RoleLoss{
			Role: role, Label: role.Label(), Verdict: Carried,
			Destination: support.Destination, Sample: block.TakeSample(written),
		}
		switch {
		case support.Grade == SupportNone:
			loss.Verdict = Dropped
			loss.Destination = ""
		case support.Grade == SupportPartial && matchesAny(support.Condition, written):
			loss.Verdict = Reduced
			loss.Reason = support.Condition.Description
		}
		if loss.Verdict == Dropped && slices.Contains(required, role) {
			return nil, false
		}
		report = append(report, loss)
	}
	return report, true
}

// writtenContent returns the content an asset actually holds under a role. A
// repeatable role may carry several elements and an empty one carries nothing.
func writtenContent(elements []block.Element, role block.Role) []block.Content {
	written := make([]block.Content, 0, 1)
	for _, element := range elements {
		if element.Role != role || element.Content == nil || element.Content.Empty() {
			continue
		}
		written = append(written, element.Content)
	}
	return written
}

func matchesAny(condition *ContentCondition, written []block.Content) bool {
	if condition == nil || condition.Matches == nil {
		return false
	}
	for _, content := range written {
		if condition.Matches(content) {
			return true
		}
	}
	return false
}

// recommend marks the format that loses the least among the formats most apps
// can open. Recommending a format somebody's app refuses is a worse failure
// than dropping a gallery, so reach comes first and loss breaks the tie. Where
// two formats are read as widely and lose as little, the one that can carry
// more wins.
func recommend(targets []Target, r *Registry) {
	best := -1
	for i := range targets {
		if best < 0 || outranks(targets[i], targets[best], r) {
			best = i
		}
	}
	if best >= 0 {
		targets[best].Recommended = true
	}
}

func outranks(candidate, holder Target, r *Registry) bool {
	for _, comparison := range []int{
		reach(candidate.Format) - reach(holder.Format),
		len(holder.Losses()) - len(candidate.Losses()),
		writableRoles(r, candidate.Format) - writableRoles(r, holder.Format),
		strings.Compare(holder.Format, candidate.Format),
	} {
		if comparison != 0 {
			return comparison > 0
		}
	}
	return false
}

// writableRoles counts the roles a format can carry at all, which is what
// separates two formats that lose nothing on this particular asset.
func writableRoles(r *Registry, formatID string) int {
	declaration, ok := r.Declaration(formatID)
	if !ok {
		return 0
	}
	count := 0
	for _, support := range declaration.Roles {
		if support.Write.Grade != SupportNone {
			count++
		}
	}
	return count
}

// CapabilityStamp digests every writer's declared capability and the app table
// the recommendation reads. A stored loss report records the stamp it was
// computed under, so a deploy that changes a declaration recomputes what it
// invalidated rather than waiting for somebody to remember a version number.
func (r *Registry) CapabilityStamp() string {
	ids := make([]string, 0, len(r.modules))
	for id := range r.modules {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	digest := sha256.New()
	for _, app := range Apps() {
		fmt.Fprintf(digest, "app\x00%s\x00%s\n", app.ID, strings.Join(app.Reads, ","))
	}
	for _, id := range ids {
		declaration := r.modules[id].Declaration()
		if !declaration.Direction.Write {
			continue
		}
		fmt.Fprintf(digest, "module\x00%s\x00%s\x00%s\x00%v\x00%s\n",
			declaration.ID, declaration.Kind, declaration.Label,
			declaration.CrossPlatform, strings.Join(declaration.TestedOrigins, ","))
		for _, role := range block.Roles() {
			support := declaration.Roles[role].Write
			condition := ""
			if support.Condition != nil {
				condition = support.Condition.Description
			}
			fmt.Fprintf(digest, "role\x00%s\x00%s\x00%s\x00%s\n",
				role, support.Grade, condition, support.Destination)
		}
	}
	return hex.EncodeToString(digest.Sum(nil))[:16]
}
