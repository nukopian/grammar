package grammar

import (
	"slices"
	"strings"
	"testing"
)

// The order rules are looked up in, as Raku's C3 gives it.
func TestLinearisation(t *testing.T) {
	g, err := Compile(`
grammar Base { }
grammar L is Base { }
grammar R is Base { }
grammar Bottom is L is R { }`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, gg := range g.mro {
		names = append(names, gg.name)
	}
	if want := []string{"Bottom", "L", "R", "Base", "Grammar"}; !slices.Equal(names, want) {
		t.Errorf("order = %v, want %v", names, want)
	}
}

// The errors Raku gives, at compile time.
func TestInheritanceErrors(t *testing.T) {
	for src, want := range map[string]string{
		`grammar Base { }; grammar L is Base { }; grammar P is Base is L { }`: "can't order",
		`grammar A { }; grammar Q is A is A { }`:                              "already has parent A",
		`role Ro { }; grammar G is Ro { }`:                                    "is a role",
	} {
		_, err := Compile(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", src, err, want)
		}
	}
}

// Extend with more parents is "is A is B", built in Go; the same errors
// panic, as building grammars in Go does.
func TestExtendMany(t *testing.T) {
	a := New("A").Token("x", Lit("a")).Token("y", Lit("a"))
	b := New("B").Token("x", Lit("b")).Token("z", Lit("b"))
	c := a.Extend("C", b).Token("TOP", Seq(Call("x"), Call("y"), Call("z")))
	if !c.Parse("aab").Defined() || c.Parse("bab").Defined() {
		t.Error("C doesn't look up A before B")
	}
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "already has parent A") {
			t.Errorf("panic = %q", r)
		}
	}()
	a.Extend("D", a)
}

// A rule in any parent counts for Check, and overriding works through
// every parent: a rule in a parent calls the child's version.
func TestInheritCheckAndOverride(t *testing.T) {
	g, err := Compile(`
grammar Money { token money { '£' <num> }; token num { \d+ } }
grammar Period { token period { ' per ' <unit> }; token unit { year } }
grammar Pay is Money is Period { token TOP { <money> <period> }; token unit { hour } }`)
	if err != nil {
		t.Fatal(err)
	}
	if !g.Parse("£18 per hour").Defined() || g.Parse("£18 per year").Defined() {
		t.Error("Period's <unit> didn't call Pay's")
	}
	if _, err := Compile(`grammar M { token m { <nope> } }; grammar N { }; grammar O is M is N { token TOP { <m> } }`); err == nil {
		t.Error("Check missed a call to a missing rule in a parent")
	}
}
