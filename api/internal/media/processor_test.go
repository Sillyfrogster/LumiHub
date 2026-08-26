package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"slices"
	"testing"
)

func TestNamedVariantsAreTheRelaunchSet(t *testing.T) {
	want := []string{
		"grid", "grid_blurred",
		"detail", "detail_blurred",
		"thumb", "thumb_blurred",
	}
	if got := VariantNames(); !slices.Equal(got, want) {
		t.Fatalf("variants = %v, want %v", got, want)
	}
	if _, ok := VariantByName("og"); ok {
		t.Fatal("og was accepted as an ordinary media variant")
	}
}

func TestVariantBoundsWithoutCroppingOrUpscaling(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 1200, 600))
	encoded := encodePNG(t, source)

	prepared, err := NewProcessor(DefaultLimits()).Prepare(
		context.Background(), bytes.NewReader(encoded),
	)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if prepared.Width != 1200 || prepared.Height != 600 {
		t.Fatalf("native dimensions = %dx%d, want 1200x600", prepared.Width, prepared.Height)
	}

	grid := derivativeNamed(t, prepared.Derivatives, "grid")
	decoded, err := png.Decode(bytes.NewReader(grid.Bytes))
	if err != nil {
		t.Fatalf("decode grid derivative: %v", err)
	}
	if got := decoded.Bounds().Size(); got.X != 640 || got.Y != 320 {
		t.Fatalf("grid dimensions = %dx%d, want 640x320", got.X, got.Y)
	}

	small := image.NewRGBA(image.Rect(0, 0, 80, 40))
	prepared, err = NewProcessor(DefaultLimits()).Prepare(
		context.Background(), bytes.NewReader(encodePNG(t, small)),
	)
	if err != nil {
		t.Fatalf("Prepare small image: %v", err)
	}
	thumb := derivativeNamed(t, prepared.Derivatives, "thumb")
	decoded, err = png.Decode(bytes.NewReader(thumb.Bytes))
	if err != nil {
		t.Fatalf("decode thumb derivative: %v", err)
	}
	if got := decoded.Bounds().Size(); got.X != 80 || got.Y != 40 {
		t.Fatalf("small thumb dimensions = %dx%d, want native 80x40", got.X, got.Y)
	}
}

func TestOversizedHeaderIsRejectedBeforePixelDecode(t *testing.T) {
	limits := DefaultLimits()
	_, err := NewProcessor(limits).Prepare(
		context.Background(), bytes.NewReader(pngHeader(40_000, 40_000)),
	)
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("Prepare error = %v, want ErrImageTooLarge", err)
	}
}

func TestBlurredCounterpartDoesNotCarryClearPixels(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 640, 320))
	for y := range 320 {
		for x := range 640 {
			if x < 320 {
				source.Set(x, y, color.Black)
			} else {
				source.Set(x, y, color.White)
			}
		}
	}
	prepared, err := NewProcessor(DefaultLimits()).Prepare(
		context.Background(), bytes.NewReader(encodePNG(t, source)),
	)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	clear := derivativeNamed(t, prepared.Derivatives, "grid")
	blurred := derivativeNamed(t, prepared.Derivatives, "grid_blurred")
	if bytes.Equal(clear.Bytes, blurred.Bytes) {
		t.Fatal("blurred derivative is byte-identical to the clear derivative")
	}
}

func TestEncoderCanBeReplaced(t *testing.T) {
	encoder := &recordingEncoder{}
	processor := NewProcessorWithEncoder(DefaultLimits(), encoder)
	prepared, err := processor.Prepare(
		context.Background(), bytes.NewReader(encodePNG(t, image.NewRGBA(image.Rect(0, 0, 2, 2)))),
	)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if encoder.calls != len(VariantNames()) {
		t.Fatalf("encoder calls = %d, want %d", encoder.calls, len(VariantNames()))
	}
	for _, derivative := range prepared.Derivatives {
		if string(derivative.Bytes) != "replacement encoding" {
			t.Fatalf("%s bytes = %q", derivative.Variant, derivative.Bytes)
		}
	}
	if processor.DerivativeType() != "image/example" {
		t.Fatalf("derivative type = %q", processor.DerivativeType())
	}
}

func TestOGIsASeparateComposedPreview(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 100, 200))
	for y := range 200 {
		for x := range 100 {
			source.Set(x, y, color.RGBA{R: 12, G: 34, B: 56, A: 255})
		}
	}
	processor := NewProcessor(DefaultLimits())
	preview, err := processor.ComposeSocialPreview(
		context.Background(), bytes.NewReader(encodePNG(t, source)), "og",
	)
	if err != nil {
		t.Fatalf("ComposeSocialPreview: %v", err)
	}
	if preview.Variant != "og" {
		t.Fatalf("preview variant = %q, want og", preview.Variant)
	}
	decoded, err := png.Decode(bytes.NewReader(preview.Bytes))
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if got := decoded.Bounds().Size(); got.X != 1200 || got.Y != 630 {
		t.Fatalf("preview dimensions = %dx%d, want 1200x630", got.X, got.Y)
	}
	if got := color.RGBAModel.Convert(decoded.At(0, 0)).(color.RGBA); got != previewField {
		t.Fatalf("preview corner = %#v, want the carbon field", got)
	}
	if _, ordinary := VariantByName("og"); ordinary {
		t.Fatal("composed og preview entered the ordinary variant set")
	}
}

type recordingEncoder struct {
	calls int
}

func (e *recordingEncoder) MediaType() string { return "image/example" }

func (e *recordingEncoder) Encode(w io.Writer, _ image.Image) error {
	e.calls++
	_, err := io.WriteString(w, "replacement encoding")
	return err
}

func derivativeNamed(t *testing.T, derivatives []Derivative, name string) Derivative {
	t.Helper()
	for _, derivative := range derivatives {
		if derivative.Variant == name {
			return derivative
		}
	}
	t.Fatalf("derivative %q is missing", name)
	return Derivative{}
}

func encodePNG(t *testing.T, source image.Image) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return encoded.Bytes()
}

func pngHeader(width, height uint32) []byte {
	var out bytes.Buffer
	out.Write([]byte("\x89PNG\r\n\x1a\n"))
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], width)
	binary.BigEndian.PutUint32(data[4:8], height)
	data[8] = 8
	data[9] = 6
	writePNGChunk(&out, "IHDR", data)
	writePNGChunk(&out, "IEND", nil)
	return out.Bytes()
}

func writePNGChunk(out *bytes.Buffer, kind string, data []byte) {
	_ = binary.Write(out, binary.BigEndian, uint32(len(data)))
	out.WriteString(kind)
	out.Write(data)
	checksum := crc32.NewIEEE()
	_, _ = checksum.Write([]byte(kind))
	_, _ = checksum.Write(data)
	_ = binary.Write(out, binary.BigEndian, checksum.Sum32())
}

func TestSocialPreviewOfFlaggedWorkIsBlurred(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for y := range 400 {
		for x := range 400 {
			if x < 200 {
				source.Set(x, y, color.Black)
			} else {
				source.Set(x, y, color.White)
			}
		}
	}
	encoded := encodePNG(t, source)
	processor := NewProcessor(DefaultLimits())

	clear, err := processor.ComposeSocialPreview(
		context.Background(), bytes.NewReader(encoded), "og",
	)
	if err != nil {
		t.Fatalf("compose og: %v", err)
	}
	blurred, err := processor.ComposeSocialPreview(
		context.Background(), bytes.NewReader(encoded), "og_blurred",
	)
	if err != nil {
		t.Fatalf("compose og_blurred: %v", err)
	}

	if blurred.Variant != "og_blurred" {
		t.Fatalf("preview variant = %q, want og_blurred", blurred.Variant)
	}
	if bytes.Equal(clear.Bytes, blurred.Bytes) {
		t.Fatal("the blurred social preview is byte-identical to the clear one")
	}
	decoded, err := png.Decode(bytes.NewReader(blurred.Bytes))
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if got := decoded.Bounds().Size(); got.X != 1200 || got.Y != 630 {
		t.Fatalf("preview dimensions = %dx%d, want 1200x630", got.X, got.Y)
	}
	if _, ordinary := VariantByName("og_blurred"); ordinary {
		t.Fatal("the blurred composed preview entered the ordinary variant set")
	}
	if _, ok := SocialPreviewByName("grid"); ok {
		t.Fatal("an ordinary variant was accepted as a social preview")
	}
}
