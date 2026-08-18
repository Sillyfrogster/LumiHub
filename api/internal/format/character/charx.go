package character

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Sillyfrogster/LumiHub/api/internal/block"
	"github.com/Sillyfrogster/LumiHub/api/internal/format"
	"github.com/Sillyfrogster/LumiHub/api/internal/media"
	"github.com/Sillyfrogster/LumiHub/api/internal/probe"
)

// embeddedPrefix is how a CharX card points at a file beside it in the archive.
// The misspelling is the standard's.
const embeddedPrefix = "embeded://"

// CharXModule reads a CharX archive: a card and the pictures it names, zipped
// together. The card inside declares chara_card_v3, so the module owns that
// spec rather than one of its own.
type CharXModule struct{}

func (CharXModule) ID() string { return CharX }

func (CharXModule) Declaration() format.Declaration { return declaration(CharX) }

func (CharXModule) OwnedSpecs() []string { return []string{V3} }

func (m CharXModule) BrowseDefinition() format.BrowseDefinition {
	return browseDefinition(m.ExportTargets())
}

func (CharXModule) ExportTargets() []format.BrowseOption { return exportTargets() }

func (m CharXModule) Claim(file probe.Inspection) (format.Claim, bool) {
	return format.ClaimByDeclaration(file, m.Declaration())
}

func (m CharXModule) Parse(
	_ context.Context,
	file probe.Inspection,
	claim format.Claim,
) (format.Parsed, error) {
	read, err := readCard(file, claim, 3, m.ID())
	if err != nil {
		return format.Parsed{}, err
	}
	return read.parsed(m.ID(), archivedImages(read, file))
}

type cardAsset struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
	Name string `json:"name"`
	Ext  string `json:"ext"`
}

// archivedImages gives a role to each picture the card names in the archive.
// The card decides what is in it; a picture nothing points at is left alone.
func archivedImages(read card, file probe.Inspection) []format.Media {
	var assets []cardAsset
	if raw, ok := read.fields["assets"]; ok {
		_ = json.Unmarshal(raw, &assets)
	}
	var found []format.Media
	hasAvatar := false
	for _, asset := range assets {
		path, embedded := strings.CutPrefix(asset.URI, embeddedPrefix)
		if !embedded {
			continue
		}
		image, located := archivedImage(file, path)
		if !located {
			continue
		}
		role, wanted := assetRole(asset, hasAvatar)
		if !wanted {
			continue
		}
		if role == media.Avatar {
			hasAvatar = true
		}
		elementRole := block.Role("")
		if role == media.Expression {
			elementRole = block.RoleExpressions
		} else if role == media.Gallery {
			elementRole = block.RoleGallery
		}
		found = append(found, format.Media{
			Role: role, ImageID: image, ElementRole: elementRole, Name: asset.Name,
		})
	}
	return found
}

// assetRole maps a CharX asset type onto LumiHub's role vocabulary. A card
// carries one avatar, so a second icon becomes an alternate.
func assetRole(asset cardAsset, hasAvatar bool) (media.Role, bool) {
	switch asset.Type {
	case "icon":
		if !hasAvatar && asset.Name == "main" {
			return media.Avatar, true
		}
		return media.AvatarAlt, true
	case "emotion":
		return media.Expression, true
	case "user_icon":
		// The reader's own picture, not the character's.
		return "", false
	default:
		return media.Gallery, true
	}
}

func archivedImage(file probe.Inspection, path string) (uint32, bool) {
	wanted := strings.TrimPrefix(strings.ReplaceAll(path, "\\", "/"), "./")
	for _, image := range file.Images {
		if image.Locator.Container != probe.ZIP {
			continue
		}
		if strings.TrimPrefix(image.Locator.Name, "./") == wanted {
			return image.ID, true
		}
	}
	return 0, false
}
