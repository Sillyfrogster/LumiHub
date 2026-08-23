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
		{ID: "sillytavern", Label: "SillyTavern", Reads: []string{
			"chara_card_v2", "chara_card_v3",
			"lorebook_sillytavern", "preset_sillytavern", "theme_sillytavern",
		}},
		{ID: "risu", Label: "RisuAI", Reads: []string{
			"chara_card_v2", "chara_card_v3", "charx", "lorebook_sillytavern",
		}},
		{ID: "lumiverse", Label: "Lumiverse", Reads: []string{
			"chara_card_v2", "chara_card_v3",
			"lorebook", "preset_lumiverse", "theme_lumiverse", "pack_lumiverse",
		}},
	}
}

func reach(formatID string) int {
	count := 0
	for _, app := range Apps() {
		if slices.Contains(app.Reads, formatID) {
			count++
		}
	}
	return count
}

// Verdict describes how much content a target keeps.
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
	// Destination names a nonstandard output location.
	Destination string       `json:"destination,omitempty"`
	Sample      block.Sample `json:"sample"`
}

// Lossy reports whether this verdict costs the asset something.
func (l RoleLoss) Lossy() bool { return l.Verdict != Carried }

// Target is one offered download format and its content costs.
type Target struct {
	Format string `json:"format"`
	Label  string `json:"label"`
	// Recommended is computed from compatibility and loss.
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
	// Origin is empty for content authored in Illarin.
	Origin   string
	Elements []block.Element
	// AllowedCrossPlatform names explicitly allowed targets.
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
		case support.Grade == SupportNone || matchesAny(support.DropWhen, written):
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

// recommend prefers reach, fewer losses, and wider role support.
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

// CapabilityStamp identifies the current writer and app declarations.
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
