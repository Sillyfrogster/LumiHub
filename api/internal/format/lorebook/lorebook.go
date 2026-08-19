// Package lorebook reads and writes the two standalone lorebook files.
//
// One is the same object a character card carries under `character_book`,
// hoisted to the top level of a document of its own, so its entry vocabulary
// is the shared one and only the surroundings differ. The other is what
// SillyTavern's World Info panel exports, which spells every field its own way
// and keys its entries rather than listing them.
//
// Neither names itself, so each is recognised by the one structure it always
// has. That is also the one thing that differs between them, so a file can
// only ever match one of the two.
package lorebook

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/book"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
	"github.com/google/uuid"
)

// ID is the module's identity, and Kind is what a lorebook is to a person.
const (
	ID   = "lorebook"
	Kind = "lorebook"
)

// Where a lorebook's leftovers sit. `lorebook` is Illarin's name for the
// document's own top level, `lorebook_entry` for one entry's own keys, and
// every other namespace is a key of `extensions`.
const (
	bookNamespace  = "lorebook"
	entryNamespace = "lorebook_entry"
	extensionsKey  = "extensions"
	entriesKey     = "entries"
)

// Seeded catalog text is a prefill the creator confirms, so the blurb is
// bounded rather than however long the note in the file happens to be.
const maxBlurbRunes = 400

// Module reads and writes the standalone lorebook document.
type Module struct{}

func (Module) ID() string { return ID }

func (Module) Declaration() format.Declaration {
	return format.Declaration{
		ID: ID, Label: "Lorebook", Kind: Kind,
		Direction: format.Direction{Read: true, Write: true},
		// No lorebook file says what it is. A document holding a list of
		// entries is the whole of the evidence, and it is evidence no other
		// module asks for: a character card keeps its book nested under
		// `character_book` and never at its own top level.
		Recognition: []format.Recognition{{
			Kind:       format.RecognitionSignature,
			Containers: []probe.Container{probe.JSON},
			Required:   map[string]format.ValueType{entriesKey: format.ValueArray},
		}},
		// A lorebook page can hold a gallery and an author's note, because
		// every kind lists the shared sections. The file has nowhere to put
		// either, so a reader downloading one is told what stays behind.
		Roles: map[block.Role]format.DirectionalRoleSupport{
			block.RoleLorebookEntries: {
				Read:  format.RoleSupport{Grade: format.SupportFull},
				Write: format.RoleSupport{Grade: format.SupportFull},
			},
			block.RoleGallery: {
				Read:  format.RoleSupport{Grade: format.SupportNone},
				Write: format.RoleSupport{Grade: format.SupportNone},
			},
			block.RoleCreatorNotes: {
				Read:  format.RoleSupport{Grade: format.SupportNone},
				Write: format.RoleSupport{Grade: format.SupportNone},
			},
		},
		// A book names itself and nothing else. Its version, its credited
		// author and a character's alternate name have nowhere to go, so they
		// stay above the file rather than being invented inside it.
		Header: []format.HeaderField{format.HeaderName},
		// No named slots. A lorebook holds no settings group and no colour
		// set, so there is no typed knob for a module to declare.
		Slots: nil,
		Limits: format.ContentLimits{
			PayloadBytes: block.MaxPayloadBytes, CollectionItems: block.MaxCollectionItems,
			ItemBytes: block.MaxItemBytes,
		},
		// The document's own keys this module turns into content. Everything
		// else it carries is preserved, which is what makes the remainder a
		// per-key answer rather than a per-namespace one. A book's description
		// is not among them.
		ConsumedKeys: []string{entriesKey, "name"},
		// No boilerplate. A book is written by hand or by whatever exported
		// it, and no tool in the corpus stamps a namespace onto every one, so
		// there is nothing the creator's panel should hide.
		Boilerplate: nil,
		Preservation: format.PreservationDeclaration{
			Body: bookNamespace, Container: []string{extensionsKey},
		},
		// Its own format and Illarin-authored books. A card writer is not
		// tested to build a book out of a card's roles and does not claim to
		// be (ADR-0020).
		TestedOrigins: []string{ID, format.OriginIllarin},
	}
}

func (m Module) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.ClaimByDeclaration(file, m.Declaration())
}

// Parse reads the document into the one role a lorebook has, plus the name
// above it and everything else the file carried.
//
// The entry list is the required part. If it will not parse the import is
// refused and nothing is stored; past that point a failure costs only what
// failed, so three bad values in 285 entries leave the other 282 whole.
func (m Module) Parse(
	_ context.Context,
	file probe.Inspection,
	claim format.Claim,
) (format.Parsed, error) {
	payload, ok := claim.Payload(file)
	if !ok {
		return format.Parsed{}, fmt.Errorf("%s payload: the claimed payload is missing", ID)
	}
	// The probe's payload is read by every module that looked at this file, so
	// the leftovers are computed on a copy of it.
	source := maps.Clone(payload.Root)

	var entryPayloads []json.RawMessage
	if raw, present := source[entriesKey]; !present || json.Unmarshal(raw, &entryPayloads) != nil {
		return format.Parsed{}, format.MalformedInput(
			fmt.Errorf("%s entries: a lorebook's entries have to be a list", ID),
		)
	}
	delete(source, entriesKey)

	entries, entryFields := book.Read(entryPayloads)
	element := block.Element{
		ID: uuid.New(), Type: block.TypeEntryTable, Role: block.RoleLorebookEntries,
		Content: block.EntryTable{Entries: entries},
	}

	name, named := text(source, "name")
	if named {
		delete(source, "name")
	}
	// The book's own description stays in the file rather than becoming a
	// field. It seeds the line a reader sees while browsing, and the creator
	// confirms that line, so a description longer than a browse card can hold
	// is prefilled short and travels back out in full.
	//
	// It is taken before the remainder, which edits the same map.
	seeded := blurb(source)
	return format.Parsed{
		Kind: Kind, Format: ID,
		Blurb:     seeded,
		Header:    format.Header{Name: strings.TrimSpace(name)},
		Elements:  []block.Element{element},
		Remainder: remainder(source, entries, entryFields),
	}, nil
}

// remainder is everything the document carried that did not become content:
// the book's own leftover keys, one namespace per key of `extensions`, and one
// entry's leftover keys on the entry they came from.
func remainder(
	source map[string]json.RawMessage,
	entries []block.Entry,
	entryFields map[uuid.UUID]json.RawMessage,
) []format.Remainder {
	extensions := make(map[string]json.RawMessage)
	if raw, held := source[extensionsKey]; held {
		if json.Unmarshal(raw, &extensions) == nil {
			delete(source, extensionsKey)
		} else {
			extensions = make(map[string]json.RawMessage)
		}
	}
	// A document whose extensions carry a key named for the book itself would
	// ask for two namespaces of one name. It travels back out whole either
	// way, so it stays where it is rather than being split out beside its
	// namesake.
	if collision, clash := extensions[bookNamespace]; clash {
		source[extensionsKey], _ = json.Marshal(
			map[string]json.RawMessage{bookNamespace: collision},
		)
		delete(extensions, bookNamespace)
	}

	rows := make([]format.Remainder, 0, len(extensions)+len(entryFields)+1)
	if len(source) > 0 {
		payload, _ := json.Marshal(source)
		rows = append(rows, format.Remainder{
			Owner: format.OwnerAsset, Namespace: bookNamespace, Payload: payload,
		})
	}
	for _, namespace := range slices.Sorted(maps.Keys(extensions)) {
		rows = append(rows, format.Remainder{
			Owner: format.OwnerAsset, Namespace: namespace, Payload: extensions[namespace],
		})
	}
	for _, entry := range entries {
		fields, held := entryFields[entry.ID]
		if !held {
			continue
		}
		rows = append(rows, format.Remainder{
			Owner: format.OwnerItem, OwnerID: entry.ID,
			Namespace: entryNamespace, Payload: fields,
		})
	}
	return rows
}

// blurb is the line a person reads while browsing, taken from the book's own
// description where it has one.
func blurb(source map[string]json.RawMessage) string {
	description, _ := text(source, "description")
	return truncate(strings.TrimSpace(description), maxBlurbRunes)
}

// text reads a string key, answering false where the key is absent or is not a
// string. A key that will not read is a key this module did not consume.
func text(source map[string]json.RawMessage, name string) (string, bool) {
	raw, present := source[name]
	if !present {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}

func truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cut := string(runes[:limit])
	if space := strings.LastIndexFunc(cut, unicode.IsSpace); space > 0 {
		cut = cut[:space]
	}
	return strings.TrimSpace(cut)
}

// Modules returns every lorebook module, so the server registers the set
// rather than remembering to add each one.
func Modules() []format.Reader { return []format.Reader{Module{}, SillyTavernModule{}} }
