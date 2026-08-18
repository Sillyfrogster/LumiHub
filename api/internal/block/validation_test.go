package block

import (
	"encoding/json"
	"strconv"
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

func TestEverySavePathSharesTheCollectionLimit(t *testing.T) {
	texts := make([]TextItem, MaxCollectionItems+1)
	holder := Block{
		ID: uuid.New(), Definition: Messages, Layout: Stack2, Width: Full,
		Elements: []Element{{
			ID: uuid.New(), Type: TypeTextSet, Role: RoleGreetings, Slot: "top",
			Options: Options{Display: DisplayRich}, Content: TextSet{Texts: texts},
		}},
	}
	err := ValidateStructure(holder)
	if err == nil || !strings.Contains(err.Error(), "5001") || !strings.Contains(err.Error(), "5000") {
		t.Fatalf("limit error = %v, want the actual and allowed counts", err)
	}
}

func TestEverySavePathSharesThePayloadLimit(t *testing.T) {
	text := strings.Repeat("x", MaxPayloadBytes)
	element := Element{
		Type: TypeProse, Role: RoleDescription, Content: Prose{Text: text},
	}
	encoded, err := json.Marshal(element.Content)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	err = ValidateContentLimits([]Element{element})
	if err == nil || !strings.Contains(err.Error(), strconv.Itoa(len(encoded))) ||
		!strings.Contains(err.Error(), strconv.Itoa(MaxPayloadBytes)) {
		t.Fatalf("limit error = %v, want actual %d and limit %d", err, len(encoded), MaxPayloadBytes)
	}
}
