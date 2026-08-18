package block

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func imageBlock(size ItemSize) Block {
	return Block{
		ID: uuid.New(), Definition: Gallery, Layout: Single, Width: Half,
		Elements: []Element{{
			ID: uuid.New(), Type: TypeImageSet, Role: RoleGallery, Slot: "main",
			Options: Options{ItemSize: size}, Content: ImageSet{Images: []ImageItem{}},
		}},
	}
}

func TestItemSizeControlsTheImagesInsideAnElement(t *testing.T) {
	if err := ValidateStructure(imageBlock(ItemLarge)); err != nil {
		t.Errorf("a gallery drawing large images was refused: %v", err)
	}

	err := ValidateStructure(imageBlock("120px"))
	if err == nil {
		t.Fatalf("an element declared a measurement of its own")
	}
	if !strings.Contains(err.Error(), "small") {
		t.Errorf("refusal = %q, want it to name the sizes on offer", err)
	}

	if err := ValidateStructure(imageBlock("")); err == nil {
		t.Errorf("a gallery saved without saying how large its images are")
	}
}

func TestOnlyImagesTakeAnItemSize(t *testing.T) {
	holder := Block{
		ID: uuid.New(), Definition: Usage, Layout: Single, Width: Half,
		Elements: []Element{{
			ID: uuid.New(), Type: TypeProse, Slot: "main",
			Options: Options{Display: DisplayRich, ItemSize: ItemLarge},
			Content: Prose{Text: "Run it warm."},
		}},
	}
	if err := ValidateStructure(holder); err == nil {
		t.Errorf("prose took an image size")
	}
}
