package grammar

import (
	"errors"
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
