package theme

import (
	"encoding/json"
	"maps"

	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/format/keys"
	"github.com/google/uuid"
)

type itemRemainder struct {
	namespace string
	id        uuid.UUID
	fields    map[string]json.RawMessage
}

func themeRemainder(namespace string, source map[string]json.RawMessage, items ...itemRemainder) []format.Remainder {
	rows := make([]format.Remainder, 0, len(items)+1)
	if len(source) > 0 {
		rows = append(rows, format.Remainder{
			Owner: format.OwnerAsset, Namespace: namespace, Payload: raw(source),
		})
	}
	for _, item := range items {
		if len(item.fields) == 0 {
			continue
		}
		rows = append(rows, format.Remainder{
			Owner: format.OwnerItem, OwnerID: item.id,
			Namespace: item.namespace, Payload: raw(item.fields),
		})
	}
	return rows
}

type keptTheme struct {
	asset map[string]json.RawMessage
	items map[string]map[uuid.UUID]map[string]json.RawMessage
}

func keepTheme(rows []format.Remainder) keptTheme {
	kept := keptTheme{
		asset: make(map[string]json.RawMessage),
		items: make(map[string]map[uuid.UUID]map[string]json.RawMessage),
	}
	for _, row := range rows {
		switch row.Owner {
		case format.OwnerAsset:
			kept.asset[row.Namespace] = row.Payload
		case format.OwnerItem:
			if kept.items[row.Namespace] == nil {
				kept.items[row.Namespace] = make(map[uuid.UUID]map[string]json.RawMessage)
			}
			kept.items[row.Namespace][row.OwnerID] = keys.Object(row.Payload)
		}
	}
	return kept
}

func (k keptTheme) body(namespace string) map[string]json.RawMessage {
	return maps.Clone(keys.Object(k.asset[namespace]))
}

func (k keptTheme) item(namespace string, id uuid.UUID) map[string]json.RawMessage {
	fields := maps.Clone(k.items[namespace][id])
	if fields == nil {
		fields = make(map[string]json.RawMessage)
	}
	return fields
}
