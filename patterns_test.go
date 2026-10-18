package grammar

import (
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// A bad entry is an error when the list is made, naming it, where Raku
// fails only when it's matched.
func TestPatternListErrors(t *testing.T) {
	_, err := NewPatternList("p", `\d+k`, `a[`)
	if err == nil || !strings.Contains(err.Error(), `a[`) {
		t.Errorf("err = %v, want one naming a[", err)
	}
	if _, err := NewPatternList("p", `@units`); err == nil {
		t.Error("an entry using a word list compiled")
	}
	for src, want := range map[string]string{
		`grammar A { token TOP { <@missing> } }`:                          "no pattern list",
		`grammar B { token TOP { <@words> } }`:                            "word list",
		`grammar C { token TOP { <@pats> }; token x { <@pats> <nope> } }`: "<nope>",
	} {
		_, err := Compile(src, NewWordList("words").Add("a"), MustPatternList("pats", "a"))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", src, err, want)
		}
	}
	// An entry calling a rule the grammar doesn't have is caught by Check.
	if _, err := Compile(`grammar D { token TOP { <@calls> } }`, MustPatternList("calls", "<nope>")); err == nil || !strings.Contains(err.Error(), "<nope>") {
		t.Errorf("err = %v, want <nope> missing", err)
	}
}

// Find skips positions where no entry can start, and an entry that calls
// its own rule first is left recursion.
func TestPatternListAnalysis(t *testing.T) {
	g := MustCompile(`grammar F { token TOP { <@units> } }`, MustPatternList("units", "years?", "weeks?"))
	if got := g.FindAll("two years and a week").Map(func(m *scalar.SV) any { return m.At("~") }).Join(","); got != "years,week" {
		t.Errorf("FindAll = %q", got)
	}
	_, err := Compile(`grammar L { token TOP { <@r> } }`, MustPatternList("r", "<TOP> a"))
	if err == nil || !strings.Contains(err.Error(), "left-recursive") {
		t.Errorf("err = %v, want left recursion", err)
	}
}
