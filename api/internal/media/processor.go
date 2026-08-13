package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

var (
	ErrImageTooLarge    = errors.New("image dimensions exceed the safety limit")
	ErrUnsupportedImage = errors.New("image is not a supported raster format")
	ErrUnknownVariant   = errors.New("unknown media variant")
)

const (
	DerivativeVersion uint32 = 1
	DerivativeType           = "image/png"
)

type Limits struct {
	MaxDimension int
	MaxPixels    int64
}

func DefaultLimits() Limits {
	return Limits{MaxDimension: 16_384, MaxPixels: 40_000_000}
}

type Variant struct {
	Name      string
	MaxWidth  int
	MaxHeight int
	Blurred   bool
}

var variants = []Variant{
	{Name: "grid", MaxWidth: 640, MaxHeight: 640},
	{Name: "grid_blurred", MaxWidth: 640, MaxHeight: 640, Blurred: true},
	{Name: "detail", MaxWidth: 1600, MaxHeight: 1600},
	{Name: "detail_blurred", MaxWidth: 1600, MaxHeight: 1600, Blurred: true},
	{Name: "thumb", MaxWidth: 160, MaxHeight: 160},
	{Name: "thumb_blurred", MaxWidth: 160, MaxHeight: 160, Blurred: true},
}

func VariantNames() []string {
	names := make([]string, 0, len(variants))
	for _, variant := range variants {
		names = append(names, variant.Name)
	}
	return names
}

func VariantByName(name string) (Variant, bool) {
	for _, variant := range variants {
		if variant.Name == name {
			return variant, true
		}
	}
	return Variant{}, false
}

type Derivative struct {
	Variant string
	Bytes   []byte
}

type Prepared struct {
	Width       int
	Height      int
	Derivatives []Derivative
}

type Processor struct {
	limits Limits
}

func NewProcessor(limits Limits) *Processor {
	return &Processor{limits: limits}
}

func (p *Processor) Prepare(ctx context.Context, source io.Reader) (Prepared, error) {
	decoded, width, height, err := p.decode(ctx, source)
	if err != nil {
		return Prepared{}, err
	}
	prepared := Prepared{
		Width:       width,
		Height:      height,
		Derivatives: make([]Derivative, 0, len(variants)),
	}
	for _, variant := range variants {
		if err := ctx.Err(); err != nil {
			return Prepared{}, err
		}
		derivative, err := render(decoded, variant)
		if err != nil {
			return Prepared{}, fmt.Errorf("render %s variant: %w", variant.Name, err)
		}
		prepared.Derivatives = append(prepared.Derivatives, derivative)
	}
	return prepared, nil
}

func (p *Processor) Render(ctx context.Context, source io.Reader, name string) (Derivative, error) {
	variant, ok := VariantByName(name)
	if !ok {
		return Derivative{}, ErrUnknownVariant
	}
	decoded, _, _, err := p.decode(ctx, source)
	if err != nil {
		return Derivative{}, err
	}
	if err := ctx.Err(); err != nil {
		return Derivative{}, err
	}
	return render(decoded, variant)
}

func (p *Processor) decode(ctx context.Context, source io.Reader) (image.Image, int, int, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, err
	}
	encoded, err := io.ReadAll(source)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("read image: %w", err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(encoded))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("read image dimensions: %w", ErrUnsupportedImage)
	}
	if err := p.checkDimensions(config.Width, config.Height); err != nil {
		return nil, 0, 0, err
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, 0, err
	}
	decoded, _, err := image.Decode(bytes.NewReader(encoded))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", ErrUnsupportedImage)
	}
	size := decoded.Bounds().Size()
	if size.X != config.Width || size.Y != config.Height {
		return nil, 0, 0, fmt.Errorf("decoded dimensions changed from %dx%d to %dx%d: %w",
			config.Width, config.Height, size.X, size.Y, ErrUnsupportedImage)
	}
	return decoded, config.Width, config.Height, nil
}

func (p *Processor) checkDimensions(width, height int) error {
	if width <= 0 || height <= 0 || width > p.limits.MaxDimension || height > p.limits.MaxDimension {
		return fmt.Errorf("image is %dx%d: %w", width, height, ErrImageTooLarge)
	}
	if int64(width) > p.limits.MaxPixels/int64(height) {
		return fmt.Errorf("image is %dx%d: %w", width, height, ErrImageTooLarge)
	}
	return nil
}

func render(source image.Image, variant Variant) (Derivative, error) {
	sourceSize := source.Bounds().Size()
	width, height := boundedSize(sourceSize.X, sourceSize.Y, variant.MaxWidth, variant.MaxHeight)
	resized := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(resized, resized.Bounds(), source, source.Bounds(), draw.Over, nil)
	output := image.Image(resized)
	if variant.Blurred {
		output = obscure(resized)
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, output); err != nil {
		return Derivative{}, fmt.Errorf("encode PNG: %w", err)
	}
	return Derivative{Variant: variant.Name, Bytes: encoded.Bytes()}, nil
}

func boundedSize(width, height, maxWidth, maxHeight int) (int, int) {
	scale := math.Min(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
	if scale >= 1 {
		return width, height
	}
	return max(1, int(math.Round(float64(width)*scale))),
		max(1, int(math.Round(float64(height)*scale)))
}

func obscure(source image.Image) image.Image {
	size := source.Bounds().Size()
	width, height := boundedSize(size.X, size.Y, 24, 24)
	small := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), source, source.Bounds(), draw.Over, nil)
	blurred := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
	draw.BiLinear.Scale(blurred, blurred.Bounds(), small, small.Bounds(), draw.Src, nil)
	return blurred
}
