package format

import (
	"slices"
	"testing"
)

func TestAKindsExportedHeaderFieldsAreEveryOneItsWritersPutInAFile(t *testing.T) {
	older := writerDeclaration("card_old", fullCharacterGrades())
	older.Header = []HeaderField{HeaderName, HeaderCreditedAuthor}
	newer := writerDeclaration("card_new", fullCharacterGrades())
	newer.Header = []HeaderField{HeaderName, HeaderNickname}
	registry := registryOf(t, older, newer)

	fields := registry.ExportedHeaderFields("character")

	want := []HeaderField{HeaderName, HeaderCreditedAuthor, HeaderNickname}
	slices.Sort(want)
	if !slices.Equal(fields, want) {
		t.Fatalf("ExportedHeaderFields = %v, want %v", fields, want)
	}
}

func TestAFieldOneKindWritesIsNotExportedOnAnother(t *testing.T) {
	card := writerDeclaration("card", fullCharacterGrades())
	card.Header = []HeaderField{HeaderName}
	bundle := writerDeclaration("bundle", fullCharacterGrades())
	bundle.Kind = "theme"
	bundle.Header = []HeaderField{HeaderName, HeaderBlurb}
	registry := registryOf(t, card, bundle)

	if slices.Contains(registry.ExportedHeaderFields("character"), HeaderBlurb) {
		t.Error("the blurb is exported on a character, which writes no blurb")
	}
	if !slices.Contains(registry.ExportedHeaderFields("theme"), HeaderBlurb) {
		t.Error("the blurb is not exported on a theme, whose writer puts it in the file")
	}
}

func TestAWriterThatNamesAnUnknownHeaderFieldIsRefused(t *testing.T) {
	declaration := writerDeclaration("card", fullCharacterGrades())
	declaration.Header = []HeaderField{"favourite_colour"}

	err := ValidateDeclaration(declaration)

	if err == nil {
		t.Fatal("ValidateDeclaration accepted an unknown header field")
	}
}
