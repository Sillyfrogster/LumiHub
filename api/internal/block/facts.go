package block

import (
	"fmt"
	"strconv"
)

// Facts are what a structured element can say about its own data, such as how
// many entries a book holds and how many of them are switched on. They are
// worked out on the way out and never stored, so a count can never drift from
// what it counts.
//
// Nothing here measures tokens. Illarin has no tokenizer, and a number that
// depended on one would be a guess a creator could act on.
func (e Element) Facts() []string {
	switch content := e.Content.(type) {
	case EntryTable:
		return e.bookFacts(content)
	case ImageSet:
		return e.itemFacts(len(content.Images))
	case TextSet:
		return e.itemFacts(len(content.Texts))
	case DialogueSample:
		return e.itemFacts(len(content.Turns))
	case FieldList:
		return e.itemFacts(len(content.Fields))
	case LinkList:
		return e.itemFacts(len(content.Links))
	default:
		return nil
	}
}

// bookFacts says how large a book is and how much of it is live. The second
// fact is left out where every entry is on, because a count that always
// repeats the first one says nothing.
func (e Element) bookFacts(book EntryTable) []string {
	facts := e.itemFacts(len(book.Entries))
	if len(facts) == 0 {
		return nil
	}
	on := 0
	for _, entry := range book.Entries {
		if entry.Enabled {
			on++
		}
	}
	if on == len(book.Entries) {
		return facts
	}
	return append(facts, fmt.Sprintf("%s switched on", number(on)))
}

func (e Element) itemFacts(count int) []string {
	if count == 0 {
		return nil
	}
	singular, plural := e.itemNoun()
	noun := plural
	if count == 1 {
		noun = singular
	}
	return []string{fmt.Sprintf("%s %s", number(count), noun)}
}

// itemNoun is what one item inside this element is called. The role names it
// where the element has one, because a reader knows greetings and expressions
// by those words and not by the element type underneath them.
func (e Element) itemNoun() (string, string) {
	switch e.Role {
	case RoleGreetings:
		return "greeting", "greetings"
	case RoleGroupGreetings:
		return "group-only greeting", "group-only greetings"
	case RoleExpressions:
		return "expression", "expressions"
	case RoleLorebookEntries:
		return "entry", "entries"
	}
	switch e.Type {
	case TypeImageSet:
		return "image", "images"
	case TypeDialogueSample:
		return "turn", "turns"
	case TypeFieldList:
		return "detail", "details"
	case TypeLinkList:
		return "link", "links"
	case TypeEntryTable:
		return "entry", "entries"
	default:
		return "item", "items"
	}
}

// number writes a count the way a person reads it, so a book of 1004 entries
// says 1,004. Counts are never negative, so there is no sign to carry.
func number(count int) string {
	digits := strconv.Itoa(count)
	for cut := len(digits) - 3; cut > 0; cut -= 3 {
		digits = digits[:cut] + "," + digits[cut:]
	}
	return digits
}
