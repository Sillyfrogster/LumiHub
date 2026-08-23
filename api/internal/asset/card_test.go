package asset

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"slices"
	"testing"

	"github.com/Sillyfrogster/Illarin/api/internal/format"
	"github.com/Sillyfrogster/Illarin/api/internal/format/character"
	"github.com/google/uuid"
)

// TestACardKeepsItsExactBytesWhileItsPictureIsExtracted is the round trip a
// creator cares about: LumiHub reads the card, takes a copy of the picture, and
// hands back the file it was given.
func TestACardKeepsItsExactBytesWhileItsPictureIsExtracted(t *testing.T) {
	registry := format.NewRegistry()
	for _, module := range character.Modules() {
		if err := registry.Register(module); err != nil {
			t.Fatalf("register %q: %v", module.ID(), err)
		}
	}
	svc, pool := newTestServiceWithRegistry(t, registry)
	ownerID := revisionOwner(t, svc, "card.owner")

	card := pngCardFile(t, `{
		"spec":"chara_card_v2","spec_version":"2.0",
		"data":{
			"name":"Ana",
			"description":"A quiet archivist.",
			"first_mes":"Hello.",
			"creator_notes":"A quiet archivist.",
			"extensions":{"depth_prompt":{"depth":4},"third_party":{"kept":true}}
		}
	}`)
	created := ingestOne(t, svc, ownerID, "ana.png", card)
	publishImported(t, svc, ownerID, created)
	if created.Kind != "character" || created.Format != character.V2 {
		t.Fatalf("asset = kind %q format %q", created.Kind, created.Format)
	}
	if created.Name != "Ana" || created.Blurb != "A quiet archivist." {
		t.Fatalf("catalog seed = %q, %q", created.Name, created.Blurb)
	}

	source, err := svc.OpenSource(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("OpenSource: %v", err)
	}
	defer source.Close()
	var served bytes.Buffer
	if _, err := served.ReadFrom(source); err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !bytes.Equal(served.Bytes(), card) {
		t.Fatal("the stored card is not the file that was uploaded")
	}

	var coverRole string
	var coverAsset uuid.UUID
	err = pool.QueryRow(context.Background(), `
		select media.role, media.asset_id
		  from assets asset
		  join asset_media media on media.id = asset.cover_media_id
		 where asset.id = $1
	`, created.ID).Scan(&coverRole, &coverAsset)
	if err != nil {
		t.Fatalf("read cover media: %v", err)
	}
	if coverRole != string(MediaAvatar) || coverAsset != created.ID {
		t.Fatalf("cover = %s on asset %s", coverRole, coverAsset)
	}

}

// pngCardFile puts a card in a text chunk of a picture, the way an exporter does.
func pngCardFile(t *testing.T, body string) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for x := range 8 {
		for y := range 4 {
			picture.Set(x, y, color.RGBA{R: 90, G: 60, B: 120, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatalf("encode card picture: %v", err)
	}
	file := encoded.Bytes()
	end := len(file) - 12
	return slices.Concat(file[:end], textChunk("chara", body), file[end:])
}

func textChunk(keyword, body string) []byte {
	data := slices.Concat([]byte(keyword), []byte{0}, []byte(body))
	var chunk bytes.Buffer
	_ = binary.Write(&chunk, binary.BigEndian, uint32(len(data)))
	chunk.WriteString("tEXt")
	chunk.Write(data)
	_ = binary.Write(&chunk, binary.BigEndian, crc32.ChecksumIEEE(append([]byte("tEXt"), data...)))
	return chunk.Bytes()
}
