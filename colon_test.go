package grammar

import (
	"strings"
	"testing"
)

// : is Ratchet for one atom: in a token, where atoms commit anyway, it
// changes nothing.
func TestColonInToken(t *testing.T) {
	with := MustCompile(`grammar W { token TOP { [ a | ab ]: b } }`)
	without := MustCompile(`grammar O { token TOP { [ a | ab ] b } }`)
	for _, text := range []string{"abb", "ab"} {
		if with.Parse(text).Defined() != without.Parse(text).Defined() {
			t.Errorf("%q: : changed a token", text)
		}
	}
}

// A : that ends a group, or comes before |, still commits the atom
// before it.
func TestColonPlaces(t *testing.T) {
	for src, text := range map[string]string{
		`grammar P1 { regex TOP { [ [ a | ab ]: ] b } }`:  "ab",
		`grammar P2 { regex TOP { [ a | ab ]: | x } }`:    "ab",
		`grammar P3 { regex TOP { [ \w+: ] [ b | x ] } }`: "aab",
	} {
		g, err := Compile(src)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		_ = g.Parse(text)
	}
}

// A : with no atom before it is an error, as in Raku, and so are :: and
// :::, which Rakudo doesn't support either.
func TestColonErrors(t *testing.T) {
	for src, want := range map[string]string{
		`grammar A { regex TOP { : b } }`:          "no atom before",
		`grammar B { regex TOP { [ : b ] } }`:      "no atom before",
		`grammar C { regex TOP { a | : b } }`:      "no atom before",
		`grammar D { regex TOP { a :: b | a c } }`: "aren't supported",
		`grammar E { regex TOP { a ::: b } }`:      "aren't supported",
	} {
		_, err := Compile(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", src, err, want)
		}
	}
}
