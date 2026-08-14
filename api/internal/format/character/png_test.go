package character

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		for y := range 2 {
			picture.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		t.Fatalf("encode test PNG: %v", err)
	}
	return encoded.Bytes()
}

func pngChunk(kind string, data []byte) []byte {
	var chunk bytes.Buffer
	_ = binary.Write(&chunk, binary.BigEndian, uint32(len(data)))
	chunk.WriteString(kind)
	chunk.Write(data)
	_ = binary.Write(&chunk, binary.BigEndian, crc32.ChecksumIEEE(append([]byte(kind), data...)))
	return chunk.Bytes()
}
