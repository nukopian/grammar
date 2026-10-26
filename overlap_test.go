package grammar

import (
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// overlapping is FindAll's matches as "text@from", joined with commas.
func overlapping(all *scalar.AV) string {
	var got []string
	for _, m := range all.All() {
		got = append(got, m.At("~").Str()+"@"+m.At("from").Str())
	}
	return strings.Join(got, ",")
}

// Overlap finds a match from each position where one starts, as Raku's
// :ov does; these are Raku 2025.11's answers (2026-10-26).
func TestOverlap(t *testing.T) {
	headings := NewWordList("headings", FoldCase(), WholeWords()).
		Add("responsibilities", "in this role you will").
		Add("requirements", "you will need").
		Add("role")
	h := MustCompile(`grammar H { token TOP { @headings } }`, headings)
	a := MustCompile(`grammar A { token TOP { a+ } }`)
	x := MustCompile(`grammar X { token TOP { x? } }`)
	for _, c := range []struct {
		name string
		got  *scalar.AV
		want string
	}{
		// A phrase inside another, and one starting inside it.
		{"heading", h.FindAll("In this role you will need", Overlap()), "In this role you will@0,role@8,you will need@13"},
		{"heading without", h.FindAll("In this role you will need"), "In this role you will@0"},
		// One match from each start: the longest, as a token takes it.
		{"a+", a.FindAll("aaaa", Overlap()), "aaaa@0,aaa@1,aa@2,a@3"},
		{"a+ from Pos", a.FindAll("aaaa", Overlap(), Pos(2)), "aa@2,a@3"},
		// An empty match at each position, the end included.
		{"empty", x.FindAll("ab", Overlap()), "@0,@1,@2"},
		{"none", a.FindAll("bbb", Overlap()), ""},
	} {
		if got := overlapping(c.got); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}

	// The other methods take no :ov in Raku, and ignore it here.
	text := "In this role you will need"
	if d := h.Find(text, Overlap()).Diff(h.Find(text)); d != "" {
		t.Errorf("Find: %s", d)
	}
	if got, want := h.Split(text, Overlap()).Join("|"), h.Split(text).Join("|"); got != want {
		t.Errorf("Split: %s, want %s", got, want)
	}
	if d := a.Parse("aaaa", Overlap()).Diff(a.Parse("aaaa")); d != "" {
		t.Errorf("Parse: %s", d)
	}
}
