package block

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type RecordSchema string

const LumiaRecordSchema RecordSchema = "lumia"

func (schema RecordSchema) Known() bool { return schema == LumiaRecordSchema }

type RecordList struct {
	Schema  RecordSchema  `json:"schema"`
	Records []LumiaRecord `json:"records"`
}

func (list RecordList) Empty() bool { return len(list.Records) == 0 }

type LumiaRecord struct {
	ID               uuid.UUID  `json:"id"`
	LumiaName        string     `json:"lumiaName"`
	LumiaDefinition  string     `json:"lumiaDefinition"`
	LumiaPersonality string     `json:"lumiaPersonality"`
	LumiaBehavior    string     `json:"lumiaBehavior"`
	AvatarURL        *uuid.UUID `json:"avatarUrl,omitempty"`
	GenderIdentity   int        `json:"genderIdentity"`
	AuthorName       string     `json:"authorName"`
	Version          int        `json:"version"`
}

func decodeRecordList(raw json.RawMessage) (Content, error) {
	var incoming struct {
		Schema  *RecordSchema `json:"schema"`
		Records *[]struct {
			ID               uuid.UUID  `json:"id,omitempty"`
			LumiaName        *string    `json:"lumiaName"`
			LumiaDefinition  *string    `json:"lumiaDefinition"`
			LumiaPersonality *string    `json:"lumiaPersonality"`
			LumiaBehavior    *string    `json:"lumiaBehavior"`
			AvatarURL        *uuid.UUID `json:"avatarUrl,omitempty"`
			GenderIdentity   *int       `json:"genderIdentity"`
			AuthorName       *string    `json:"authorName"`
			Version          *int       `json:"version"`
		} `json:"records"`
	}
	if err := decodeContentJSON(raw, &incoming); err != nil {
		return nil, err
	}
	if incoming.Schema == nil || !incoming.Schema.Known() {
		return nil, fmt.Errorf("schema must name a record schema Illarin supports")
	}
	if incoming.Records == nil {
		return nil, fmt.Errorf("records must be present as a list")
	}
	records := make([]LumiaRecord, len(*incoming.Records))
	for i, item := range *incoming.Records {
		if item.LumiaName == nil || item.LumiaDefinition == nil ||
			item.LumiaPersonality == nil || item.LumiaBehavior == nil ||
			item.GenderIdentity == nil || item.AuthorName == nil || item.Version == nil {
			return nil, fmt.Errorf("record %d must include every Lumia field", i+1)
		}
		if *item.GenderIdentity < 0 || *item.GenderIdentity > 2 {
			return nil, fmt.Errorf("record %d gender identity must be 0, 1 or 2", i+1)
		}
		if *item.Version < 1 {
			return nil, fmt.Errorf("record %d version must be at least 1", i+1)
		}
		records[i] = LumiaRecord{
			ID: itemID(item.ID), LumiaName: *item.LumiaName,
			LumiaDefinition: *item.LumiaDefinition, LumiaPersonality: *item.LumiaPersonality,
			LumiaBehavior: *item.LumiaBehavior, AvatarURL: item.AvatarURL,
			GenderIdentity: *item.GenderIdentity, AuthorName: *item.AuthorName,
			Version: *item.Version,
		}
	}
	return RecordList{Schema: *incoming.Schema, Records: records}, nil
}

func decodeStoredRecordList(raw json.RawMessage) (Content, error) {
	var list RecordList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, err
	}
	if !list.Schema.Known() {
		return nil, fmt.Errorf("record schema %q is not supported", list.Schema)
	}
	return list, nil
}
