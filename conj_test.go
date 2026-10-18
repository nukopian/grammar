package grammar

import (
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// Conj is & built in Go: every node matches the same text.
func TestConjBuilder(t *testing.T) {
	g := New("G").Token("TOP", Conj(Plus(Word), Plus(Class(func(r rune) bool { return r >= 'a' && r <= 'c' }))))
	if !g.Parse("abc").Defined() || g.Parse("abcd").Defined() {
		t.Error("Conj doesn't match as [ \\\\w+ & <[a..c]>+ ]")
	}
}

// A rule that calls itself at the start of any branch of a conjunction is
// left-recursive, as every branch starts where the conjunction does.
func TestConjLeftRecursion(t *testing.T) {
	_, err := Compile(`grammar L { token TOP { [ a & <TOP> ] } }`)
	if err == nil || !strings.Contains(err.Error(), "left-recursive") {
		t.Errorf("err = %v, want left recursion", err)
	}
	// A conjunction can match empty text only if every branch can.
	_, err = Compile(`grammar L2 { token TOP { [ '' & a ] <TOP>? } }`)
	if err != nil {
		t.Errorf("err = %v, want none", err)
	}
}

// Find finds a conjunction where it starts, skipping positions as usual.
func TestConjFind(t *testing.T) {
	g := MustCompile(`grammar F { token TOP { [ <:Lu> \w+ & <[A..Z a..z]>+ ] } }`)
	if got := g.Find("the Senior role").At("~").Str(); got != "Senior" {
		t.Errorf("Find = %q", got)
	}
	if got := g.FindAll("Zoë and Wei").Map(func(m *scalar.SV) any { return m.At("~") }).Join(","); got != "Wei" {
		t.Errorf("FindAll = %q", got)
	}
}
