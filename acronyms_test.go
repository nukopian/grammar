package grammar

import (
	"slices"
	"testing"
)

// ExactAcronyms keeps a folded list's acronyms in capitals: "IT" isn't
// "it" (found dogfooding internal/soc, which kept a second, unfolded list
// for its acronyms, 2026-10-17).
func TestExactAcronyms(t *testing.T) {
	l := NewWordList("q", FoldCase(), WholeWords(), Plurals(), ExactAcronyms()).
		AddAs("computing/IT", "computing", "IT").Add("NHS trust").Add("PM")
	g := MustCompile(`grammar Q { token TOP { @q } }`, l)
	for text, want := range map[string]string{
		"we use it daily": "",
		"IT support":      "IT",
		"It works":        "",
		"Computing":       "Computing",
		"an NHS Trust":    "NHS Trust",
		"an NHS TRUST":    "NHS TRUST",
		"an nhs trust":    "",
		"two PMs":         "PMs",
		"PMS":             "PMS",
		"evening pms":     "",
	} {
		if got := g.Find(text).At("~").Str(); got != want {
			t.Errorf("Find(%q) = %q, want %q", text, got, want)
		}
	}
	if got := l.Canonical("it"); got != nil {
		t.Errorf(`Canonical("it") = %q, want none`, got)
	}
	if got := l.Canonical("IT"); !slices.Equal(got, []string{"computing/IT"}) {
		t.Errorf(`Canonical("IT") = %q`, got)
	}
	if got := l.Canonical("nhs TRUST"); got != nil {
		t.Errorf(`Canonical("nhs TRUST") = %q, want none`, got)
	}
	// Without it, a folded list folds acronyms too, as before.
	plain := NewWordList("p", FoldCase(), WholeWords()).Add("IT")
	if MustCompile(`grammar P { token TOP { @p } }`, plain).Find("use it").At("~").Str() != "it" {
		t.Error("a list without ExactAcronyms stopped folding acronyms")
	}
}
