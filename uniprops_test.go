package grammar

import (
	"strings"
	"testing"
)

// An unknown property is an error, where Raku matches nothing: a typo
// would otherwise make a rule silently never match.
func TestUnknownProperty(t *testing.T) {
	for _, src := range []string{
		`grammar U { token TOP { <:Bogus> } }`,
		`grammar U { token TOP { <:Script<Klingon>> } }`,
		`grammar U { token TOP { <[a] + :Nope> } }`,
	} {
		_, err := Compile(src)
		if err == nil || !strings.Contains(err.Error(), "no Unicode") {
			t.Errorf("%s: %v, want no such Unicode property or script", src, err)
		}
	}
}

// Names match loosely, as Unicode's do: case, spaces, hyphens and
// underscores don't matter.
func TestPropertyNamesLoose(t *testing.T) {
	for _, name := range []string{"Lu", "lu", "LU", "Uppercase_Letter", "uppercaseletter", "Uppercase-Letter", "UPPERCASE_LETTER"} {
		g, err := Compile(`grammar U { token TOP { <:` + name + `> } }`)
		if err != nil {
			t.Errorf("<:%s>: %v", name, err)
			continue
		}
		if !g.Parse("Ж").Defined() || g.Parse("ж").Defined() {
			t.Errorf("<:%s> doesn't match as Lu", name)
		}
	}
}

// Find skips positions where a property can't start a match, as it does
// for other classes, and under :i, still doesn't fold a property's case.
func TestPropertyStarts(t *testing.T) {
	g := MustCompile(`grammar S { token TOP { <:Lu> <:Ll>+ }; token ci { :i <:Lu> <:Ll>+ } }`)
	if got := g.Find("abc Def").At("~").Str(); got != "Def" {
		t.Errorf("Find = %q", got)
	}
	if got := g.Find("abc Def", Rule("ci")).At("~").Str(); got != "Def" {
		t.Errorf("Find :i = %q", got)
	}
}
