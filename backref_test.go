package grammar

import (
	"testing"

	"github.com/nukopian/scalar"
)

// Backref built in Go is $<name>.
func TestBackrefBuilder(t *testing.T) {
	g := New("Q").Token("TOP", Seq(Named("q", Class(func(r rune) bool { return r == '\'' || r == '"' })),
		Plus(Word), Backref("q")))
	if !g.Parse(`"ab"`).Defined() || g.Parse(`"ab'`).Defined() {
		t.Error(`Backref("q") doesn't match as $<q>`)
	}
}

// A backreference can match any text, so Find tries every position, and
// it may match nothing, for left recursion's sake.
func TestBackrefAnalysis(t *testing.T) {
	g := MustCompile(`grammar F { token TOP { $<d>=[\d] $<d> } }`)
	if got := g.FindAll("a 11 22 3 44").Map(func(m *scalar.SV) any { return m.At("~") }).Join(","); got != "11,22,44" {
		t.Errorf("FindAll = %q", got)
	}
	if _, err := Compile(`grammar L { token TOP { $<x>=[ a? ] $<x> <TOP> } }`); err == nil {
		t.Error("left recursion through a backreference that can be empty wasn't caught")
	}
}
