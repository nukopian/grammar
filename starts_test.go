package grammar

import (
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// findBoth runs Find and FindAll with and without skipping positions where
// no match can start.
func findBoth(g *Grammar, text string) (with, without [2]*scalar.SV) {
	defer func() { skipStarts = true }()
	for _, skip := range []bool{true, false} {
		skipStarts = skip
		r := [2]*scalar.SV{g.Find(text), scalar.Value(g.FindAll(text))}
		if skip {
			with = r
		} else {
			without = r
		}
	}
	return with, without
}

// Skipping positions never changes what Find and FindAll find: checked over
// every Raku case, each input searched for inside some surrounding text.
func TestStartsChangeNothing(t *testing.T) {
	acronyms := NewWordList("acro", FoldCase(), WholeWords(), Plurals(), ExactAcronyms()).Add("IT").Add("NHS trust").Add("pay")
	grammars := []*Grammar{pay, payErr, shapes, MustCompile(`grammar Acro { token TOP { @acro } }`, acronyms)}
	var texts []string
	for _, c := range rakuCases {
		g, err := Compile(c.Src, caseUses(c.Lists, c.Patterns)...)
		if err != nil {
			t.Fatal(err)
		}
		grammars = append(grammars, g)
		texts = append(texts, c.Inputs...)
	}
	for _, in := range slicesClone(texts) {
		texts = append(texts, "xx "+in+" ;"+in, strings.ToUpper(in)+"\n"+in)
	}
	texts = append(texts, "Salary: £45k – £55,000 per annum, plus £5k bonus", "", "££", "£", "£ 1",
		"IT and it, an NHS Trust, nhs trust, PAY and pay, ITs")
	for _, g := range grammars {
		for _, text := range texts {
			with, without := findBoth(g, text)
			for i, name := range []string{"Find", "FindAll"} {
				if d := with[i].Diff(without[i]); d != "" {
					t.Errorf("%s.%s(%q): %s", g.Name(), name, text, d)
				}
			}
		}
	}
}

func slicesClone(s []string) []string { return append([]string(nil), s...) }

// What a grammar can start with.
func TestStarts(t *testing.T) {
	units := NewWordList("units").Add("day").Add("hour")
	folded := NewWordList("folded", FoldCase()).Add("Week")
	for _, c := range []struct {
		src        string
		yes, no    string // characters a match can and can't start with
		everywhere bool   // no positions can be skipped
	}{
		{src: `grammar S1 { token TOP { '£' \d+ } }`, yes: "£", no: "$1a "},
		{src: `grammar S2 { token TOP { <money> }; token money { '£' | '$' } }`, yes: "£$", no: "1"},
		{src: `grammar S3 { token TOP { :i per } }`, yes: "pP", no: "e"},
		{src: `grammar S4 { token TOP { \d+ } }`, yes: "05٣", no: "a"},
		{src: `grammar S5 { token TOP { a? b } }`, yes: "ab", no: "c"},
		{src: `grammar S6 { token TOP { <?before a> . } }`, yes: "ab", everywhere: false},
		{src: `grammar S7 { token TOP { @units } }`, yes: "dh", no: "Dw"},
		{src: `grammar S8 { token TOP { :i @units } }`, yes: "dDhH", no: "w"},
		{src: `grammar S9 { token TOP { @folded } }`, yes: "wW", no: "d"},
		{src: `grammar S10 { token TOP { a* } }`, everywhere: true},
		{src: `grammar S11 { token TOP { ^ a } }`, yes: "a", no: "b"},
		{src: `grammar S12 { token TOP { [ x? ]+ % ',' y } }`, yes: "x,y", no: "z"},
		{src: `grammar S13 { rule TOP { a b } }`, yes: "a", no: " b"},
		{src: `grammar S14 { token TOP { <[a..c]> | <-[\d]> } }`, yes: "abz ", no: "5"},
	} {
		g, err := Compile(c.src, units, folded)
		if err != nil {
			t.Fatal(err)
		}
		s, empty := g.startsOf("TOP")
		if c.everywhere {
			if !empty && !s.any {
				t.Errorf("%s: can skip positions", c.src)
			}
			continue
		}
		if empty || s.any {
			t.Errorf("%s: can't skip positions", c.src)
			continue
		}
		for _, r := range c.yes {
			if !s.has(r) {
				t.Errorf("%s: can't start with %q", c.src, r)
			}
		}
		for _, r := range c.no {
			if s.has(r) {
				t.Errorf("%s: can start with %q", c.src, r)
			}
		}
	}
	re := New("R").Token("TOP", Re(`\d+`))
	if s, empty := re.startsOf("TOP"); !s.any && !empty {
		t.Error("Re: skipped positions")
	}
}
