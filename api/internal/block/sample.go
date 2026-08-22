package block

import (
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// A sample is a glance, not a summary. Four things and sixty characters each
// is enough to recognise what is at stake and short enough to store beside a
// loss report on every asset.
const (
	sampleItems  = 4
	sampleRunes  = 60
	sampleJoiner = ": "
)

// Sample is a few of the things one element holds, so somebody deciding
// whether a loss matters sees entry names and greeting openings rather than a
// bare count.
type Sample struct {
	// Texts are openings and names, in the element's own order.
	Texts []string
	// Images are the media ids of the pictures the element points at.
	Images []uuid.UUID
	// Count is how many things the element holds in all.
	Count int
}

// TakeSample reads one glance across every element carrying a role. A
// repeatable role holds several, and what is at stake is all of them.
func TakeSample(contents []Content) Sample {
	var merged Sample
	for _, content := range contents {
		part := sampleOf(content)
		merged.Count += part.Count
		for _, text := range part.Texts {
			if len(merged.Texts) < sampleItems {
				merged.Texts = append(merged.Texts, text)
			}
		}
		for _, image := range part.Images {
			if len(merged.Images) < sampleItems {
				merged.Images = append(merged.Images, image)
			}
		}
	}
	return merged
}

func sampleOf(content Content) Sample {
	switch held := content.(type) {
	case Prose:
		if held.Text == "" {
			return Sample{}
		}
		return Sample{Texts: []string{opening(held.Text)}, Count: 1}
	case TextSet:
		return textSample(held.Texts, func(item TextItem) string {
			return firstWritten(item.Name, item.Text)
		})
	case DialogueSample:
		return textSample(held.Turns, func(turn DialogueTurn) string {
			if turn.Speaker == "" {
				return opening(turn.Text)
			}
			return opening(turn.Speaker + sampleJoiner + turn.Text)
		})
	case EntryTable:
		return textSample(held.Entries, func(entry Entry) string {
			if entry.Name != "" {
				return opening(entry.Name)
			}
			if len(entry.Keys) > 0 {
				return opening(strings.Join(entry.Keys, ", "))
			}
			return opening(entry.Text)
		})
	case FieldList:
		return textSample(held.Fields, func(field FieldItem) string {
			return firstWritten(field.Name, field.Value)
		})
	case LinkList:
		return textSample(held.Links, func(link LinkItem) string {
			return firstWritten(link.Label, link.URL)
		})
	case ImageSet:
		sample := Sample{Images: make([]uuid.UUID, 0, sampleItems), Count: len(held.Images)}
		for _, image := range held.Images {
			if len(sample.Images) == sampleItems {
				break
			}
			sample.Images = append(sample.Images, image.MediaID)
		}
		return sample
	case RecordList:
		return textSample(held.Records, func(item LumiaRecord) string {
			return opening(item.LumiaName)
		})
	default:
		return Sample{}
	}
}

func textSample[T any](items []T, describe func(T) string) Sample {
	sample := Sample{Texts: make([]string, 0, sampleItems), Count: len(items)}
	for _, item := range items {
		if len(sample.Texts) == sampleItems {
			break
		}
		if text := describe(item); text != "" {
			sample.Texts = append(sample.Texts, text)
		}
	}
	return sample
}

func firstWritten(name, fallback string) string {
	if name != "" {
		return opening(name)
	}
	return opening(fallback)
}

// opening is the start of a body, cut on a word where it can be.
func opening(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= sampleRunes {
		return text
	}
	cut := string(runes[:sampleRunes])
	if space := strings.LastIndexFunc(cut, unicode.IsSpace); space > 0 {
		cut = cut[:space]
	}
	return strings.TrimSpace(cut) + "…"
}
