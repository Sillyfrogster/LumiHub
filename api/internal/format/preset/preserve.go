package preset

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/keys"
	"github.com/google/uuid"
)

// Both preset files keep what Illarin has no place for in the same two
// places: the file's own top level, and an `extensions` object whose every key
// belongs to whoever wrote it. The names differ and nothing else does, so the
// moves are here and each module supplies its own names.

// itemLeftover is what one item carried that Illarin has no place for, and the
// namespace it belongs under.
type itemLeftover struct {
	namespace string
	fields    map[string]json.RawMessage
}

// preservation is where one module keeps what a file carried.
type preservation struct {
	// body is Illarin's name for the file's own top level.
	body string
	// extensions is the key whose every entry is a namespace of its own.
	extensions string
	// reserved are this module's own namespace names.
	reserved []string
}

// remainder is everything the file carried that did not become content: the
// preset's own leftover keys, one namespace per key of `extensions`, and one
// item's leftover keys on the item they came from.
func (p preservation) remainder(
	source map[string]json.RawMessage,
	items ...map[uuid.UUID]itemLeftover,
) []format.Remainder {
	extensions := make(map[string]json.RawMessage)
	if raw, held := source[p.extensions]; held {
		if json.Unmarshal(raw, &extensions) == nil {
			delete(source, p.extensions)
		} else {
			extensions = make(map[string]json.RawMessage)
		}
	}
	// An extensions key named for one of this module's own namespaces would
	// ask for two namespaces of one name. It travels back out whole either
	// way, so it stays where it is rather than being split out beside its
	// namesake.
	clashes := make(map[string]json.RawMessage)
	for _, name := range p.reserved {
		if collision, clash := extensions[name]; clash {
			clashes[name] = collision
			delete(extensions, name)
		}
	}
	if len(clashes) > 0 {
		source[p.extensions], _ = json.Marshal(clashes)
	}

	rows := make([]format.Remainder, 0, len(extensions)+1)
	if len(source) > 0 {
		payload, _ := json.Marshal(source)
		rows = append(rows, format.Remainder{
			Owner: format.OwnerAsset, Namespace: p.body, Payload: payload,
		})
	}
	for _, namespace := range slices.Sorted(maps.Keys(extensions)) {
		rows = append(rows, format.Remainder{
			Owner: format.OwnerAsset, Namespace: namespace, Payload: extensions[namespace],
		})
	}
	for _, group := range items {
		for _, id := range slices.SortedFunc(maps.Keys(group), compareIDs) {
			payload, _ := json.Marshal(group[id].fields)
			rows = append(rows, format.Remainder{
				Owner: format.OwnerItem, OwnerID: id,
				Namespace: group[id].namespace, Payload: payload,
			})
		}
	}
	return rows
}

// restoreExtensions puts every preserved namespace but the module's own back
// under `extensions`, one key each. A namespace the writer already wrote wins.
func (p preservation) restoreExtensions(body map[string]json.RawMessage, held kept) {
	extensions := keys.Object(body[p.extensions])
	for namespace, payload := range held.asset {
		if namespace == p.body {
			continue
		}
		if _, written := extensions[namespace]; !written {
			extensions[namespace] = payload
		}
	}
	if len(extensions) > 0 {
		body[p.extensions] = keys.Must(extensions)
	}
}

func compareIDs(first, second uuid.UUID) int {
	return slices.Compare(first[:], second[:])
}

// scriptLeftovers puts each script's leftover keys under a namespace, dropping
// any whose script is no longer in the list.
func scriptLeftovers(
	scripts []block.Script,
	namespace string,
	fields map[uuid.UUID]map[string]json.RawMessage,
) map[uuid.UUID]itemLeftover {
	leftovers := make(map[uuid.UUID]itemLeftover, len(fields))
	for _, script := range scripts {
		if held, kept := fields[script.ID]; kept {
			leftovers[script.ID] = itemLeftover{namespace: namespace, fields: held}
		}
	}
	return leftovers
}

// kept is an asset's preserved data, sorted into what owns it. A writer asks it
// for one namespace at a time rather than walking the rows itself.
//
// An asset's namespaces stay as the bytes they were stored as, because a
// namespace holds whatever the file put there and not every one of them is an
// object. An item's are read as keys, which is the only shape a module ever
// preserves for one item.
type kept struct {
	asset map[string]json.RawMessage
	items map[string]map[uuid.UUID]map[string]json.RawMessage
}

func preservedBy(rows []format.Remainder) kept {
	held := kept{
		asset: make(map[string]json.RawMessage),
		items: make(map[string]map[uuid.UUID]map[string]json.RawMessage),
	}
	for _, row := range rows {
		switch row.Owner {
		case format.OwnerAsset:
			held.asset[row.Namespace] = row.Payload
		case format.OwnerItem:
			if held.items[row.Namespace] == nil {
				held.items[row.Namespace] = make(map[uuid.UUID]map[string]json.RawMessage)
			}
			held.items[row.Namespace][row.OwnerID] = keys.Object(row.Payload)
		}
	}
	return held
}

// item returns what one item of an element kept from the file it arrived in.
func (k kept) item(namespace string, id uuid.UUID) map[string]json.RawMessage {
	return k.items[namespace][id]
}

// object returns one preserved namespace as keys the writer can put back.
func (k kept) object(namespace string) map[string]json.RawMessage {
	return keys.Object(k.asset[namespace])
}

// itemName reads the identifier a file knew one item by, falling back to the
// id Illarin minted where the item never came from a file.
func itemName(held kept, namespace string, id uuid.UUID, key string) string {
	var name string
	if json.Unmarshal(held.item(namespace, id)[key], &name) == nil && name != "" {
		return name
	}
	return id.String()
}
