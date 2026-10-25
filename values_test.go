package grammar

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

var shapes = MustCompile(`
grammar Shapes {
    token TOP  { <name> ':' <.ws> [ <tag>+ % ',' ]? <.ws> ( \d+ )? }
    token name { <first> ' ' <last> }
    token first { \w+ }
    token last  { \w+ }
    token tag  { '#' \w+ }
}`)

func TestValues(t *testing.T) {
	m := shapes.Parse("Ada Lovelace: #maths,#poetry 1815")
	v := Values(m)
	for path, want := range map[string]string{
		"name.first": "Ada", // a hash of captures, all the way down
		"name.last":  "Lovelace",
		"tag.0":      "#maths", // arrays stay arrays, of reduced matches
		"tag.1":      "#poetry",
		"0":          "1815", // positional captures under their numbers
	} {
		if got := v.At(path).Str(); got != want {
			t.Errorf("%s: %q, want %q", path, got, want)
		}
	}
	for _, key := range reserved {
		if v.Exists(key) {
			t.Errorf("%q kept", key)
		}
	}

	// Missing captures are missing; quantified ones are empty arrays.
	v = Values(shapes.Parse("Ada Lovelace: "))
	if v.Exists("0") || v.At("tag").AV().Length() != 0 {
		t.Errorf("missing captures: %v", v)
	}

	// Undef stays undef.
	if Values(shapes.Parse("nope")).Defined() {
		t.Error("undef match")
	}
}

// A made value wins over captures and text, except an undefined one, as
// Raku's .made // ~$/.
func TestValuesMade(t *testing.T) {
	m := shapes.Parse("Ada Lovelace: #maths", Actions(map[string]func(*scalar.SV) any{
		"name": func(m *scalar.SV) any { return m.At("last.~").Str() + ", " + m.At("first.~").Str() },
		"tag":  func(*scalar.SV) any { return nil },
	}))
	v := Values(m)
	if got := v.At("name").Str(); got != "Lovelace, Ada" {
		t.Errorf("made: %q", got)
	}
	if got := v.At("tag.0").Str(); got != "#maths" {
		t.Errorf("undefined made: %q", got)
	}
	// TOP's own made value is the whole result.
	top := shapes.Parse("Ada Lovelace: ", Actions(map[string]func(*scalar.SV) any{
		"TOP": func(*scalar.SV) any { return 42 },
	}))
	if Values(top).Int() != 42 {
		t.Errorf("TOP made: %v", Values(top))
	}
}

func TestGrammarTo(t *testing.T) {
	type Name struct{ First, Last string }
	type Person struct {
		Name Name
		Tags []string `json:"tag"`
		Born int      `json:"0"`
	}
	p, err := To[Person](shapes.Parse("Ada Lovelace: #maths,#poetry 1815"))
	if err != nil || p.Name.Last != "Lovelace" || len(p.Tags) != 2 || p.Born != 1815 {
		t.Errorf("got %+v, %v", p, err)
	}

	_, err = To[Person](shapes.Parse("nope"))
	if !errors.Is(err, ErrNoMatch) {
		t.Errorf("no match: %v", err)
	}

	// scalar.To's errors come through: "Ada" isn't an int.
	type Wrong struct {
		Name struct{ First int }
	}
	_, err = To[Wrong](shapes.Parse("Ada Lovelace: "))
	var de *scalar.DecodeError
	if !errors.As(err, &de) {
		t.Errorf("decode error: %v", err)
	}
}

func TestGrammarToAll(t *testing.T) {
	l := NewWordList("sections", FoldCase(), WholeWords())
	if err := l.AddRules("Benefits, perk, perks\nDuties, duty, responsibilities"); err != nil {
		t.Fatal(err)
	}
	g := MustCompile(`grammar Headings { token TOP { @sections } }`, l)

	// Canonical values, in the order found, repeats kept.
	kinds, err := ToAll[string](g.FindAll("Duties, perks and responsibilities"))
	if want := []string{"Duties", "Benefits", "Duties"}; err != nil || !slices.Equal(kinds, want) {
		t.Errorf("got %q, %v, want %q", kinds, err, want)
	}

	// Structs, one per match.
	type Pay struct {
		Min, Max struct{ Amount struct{ Num string } }
	}
	pays, err := ToAll[Pay](pay.FindAll("£30k basic, £5k - £6k car allowance"))
	if err != nil || len(pays) != 2 || pays[0].Min.Amount.Num != "30" || pays[1].Max.Amount.Num != "6" {
		t.Errorf("got %+v, %v", pays, err)
	}

	// No match is an empty slice, not an error: FindAll found nothing.
	none, err := ToAll[string](g.FindAll("Salary negotiable"))
	if err != nil || len(none) != 0 {
		t.Errorf("no match: %q, %v", none, err)
	}

	// scalar.To's errors come through, naming the match by its index.
	_, err = ToAll[int](g.FindAll("perks"))
	var de *scalar.DecodeError
	if !errors.As(err, &de) || !strings.Contains(err.Error(), "[0]") {
		t.Errorf("decode error: %v", err)
	}
}
