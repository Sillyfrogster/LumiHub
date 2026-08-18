package block

// Layout is a block's arrangement, chosen from a small set Illarin defines. It
// assigns elements to named slots and arranges those slots, and does nothing
// else. It never changes element order, presence, visibility or data.
type Layout string

const (
	Single    Layout = "single"
	Duo       Layout = "duo"
	MainAside Layout = "main-aside"
	Trio      Layout = "trio"
	Stack2    Layout = "stack-2"
	Stack3    Layout = "stack-3"
)

// slots names every layout's arrangement. A slot count has to be finite and
// checkable, which is why a two-element and a three-element stack are separate
// presets rather than one stack with a number.
var slots = map[Layout][]Slot{
	Single:    {"main"},
	Duo:       {"left", "right"},
	MainAside: {"main", "aside"},
	Trio:      {"left", "middle", "right"},
	Stack2:    {"top", "bottom"},
	Stack3:    {"top", "middle", "bottom"},
}

var minimumWidths = map[Layout]Width{
	Single:    Third,
	Duo:       TwoThirds,
	MainAside: TwoThirds,
	Trio:      Full,
	Stack2:    Third,
	Stack3:    Third,
}

// Slots returns the layout's named slots in arrangement order.
func (l Layout) Slots() []Slot { return slots[l] }

// MinimumWidth returns the narrowest width that can hold this layout.
func (l Layout) MinimumWidth() Width { return minimumWidths[l] }

// Width is how much of the page a block occupies. It is the narrowest a block
// will render and never the widest, because the last block in a row absorbs
// whatever is left over.
type Width string

const (
	Full      Width = "full"
	TwoThirds Width = "two_thirds"
	Half      Width = "half"
	Third     Width = "third"
)

var widthColumns = map[Width]int{
	Full:      12,
	TwoThirds: 8,
	Half:      6,
	Third:     4,
}

// Columns returns this width as twelfths of the content column.
func (w Width) Columns() int { return widthColumns[w] }

func (w Width) label() string {
	switch w {
	case Full:
		return "full width"
	case TwoThirds:
		return "two thirds"
	case Half:
		return "half"
	case Third:
		return "a third"
	default:
		return string(w)
	}
}
