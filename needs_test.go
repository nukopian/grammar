package grammar

import (
	"strings"
	"testing"
	"unicode"

	"github.com/nukopian/scalar"
)

// searches runs every search a text's needs can cut short: Find, FindAll
// alone, with Overlap and with Outermost, and Split; and, with reports,
// FindErr's and FindAllErr's reports, which try past the needs.
func searches(g *Grammar, text string, reports bool) []*scalar.SV {
	r := []*scalar.SV{g.Find(text), scalar.Value(g.FindAll(text)), scalar.Value(g.FindAll(text, Overlap())),
		scalar.Value(g.FindAll(text, Outermost())), scalar.Value(g.Split(text))}
	if !reports {
		return r
	}
	report := func(err error) *scalar.SV {
		if err == nil {
			return scalar.Value("")
		}
		return scalar.Value(err.Error())
	}
	_, findErr := g.FindErr(text)
	_, findAllErr := g.FindAllErr(text)
	return append(r, report(findErr), report(findAllErr))
}

var searchNames = []string{"Find", "FindAll", "FindAll with Overlap", "FindAll with Outermost", "Split", "FindErr", "FindAllErr"}

// withoutNeeds runs f with skipNeeds off, restoring it even on a panic.
func withoutNeeds(f func()) {
	defer func() { skipNeeds = true }()
	skipNeeds = false
	f()
}

// Rejecting texts by their needs never changes what a search finds, nor
// what FindErr says: checked over every Raku case and the package's test
// grammars, each input searched for inside some surrounding text, and
// with its digits, its letters, its punctuation or its spaces taken out. The reports, slow to
// make, are compared for the package's test grammars.
func TestNeedsChangeNothing(t *testing.T) {
	acronyms := NewWordList("acro", FoldCase(), WholeWords(), Plurals(), ExactAcronyms()).Add("IT").Add("NHS trust").Add("pay")
	grammars := []*Grammar{pay, payErr, shapes, MustCompile(`grammar Acro { token TOP { @acro } }`, acronyms)}
	reported := len(grammars)
	var texts []string
	for _, c := range rakuCases {
		g, err := Compile(c.Src, c.uses()...)
		if err != nil {
			t.Fatal(err)
		}
		grammars = append(grammars, g)
		texts = append(texts, c.Inputs...)
	}
	inputs := slicesClone(texts)
	for _, in := range inputs {
		texts = append(texts, "xx "+in+" ;"+in, strings.ToUpper(in)+"\n"+in)
	}
	texts = append(texts, "Salary: £45k – £55,000 per annum, plus £5k bonus", "", "££", "£", "£ 1",
		"IT and it, an NHS Trust, nhs trust, PAY and pay, ITs")
	for _, in := range append(inputs, "Salary: £45k – £55,000 per annum, plus £5k bonus") {
		for _, out := range []func(rune) bool{unicode.IsDigit, unicode.IsLetter, unicode.IsPunct, unicode.IsSpace} {
			texts = append(texts, strings.Map(func(r rune) rune {
				if out(r) {
					return -1
				}
				return r
			}, in))
		}
	}
	rejected := 0
	for n, g := range grammars {
		needs := g.needsOf("TOP")
		for _, text := range texts {
			with := searches(g, text, n < reported)
			var without []*scalar.SV
			withoutNeeds(func() { without = searches(g, text, n < reported) })
			for i, name := range searchNames[:len(with)] {
				if d := with[i].Diff(without[i]); d != "" {
					t.Errorf("%s.%s(%q): %s", g.Name(), name, text, d)
				}
			}
			if needs != nil && lastNeeded(needs, []rune(nfc(text))) < 0 {
				rejected++
			}
		}
	}
	if rejected < 100 {
		t.Fatalf("only %d texts rejected by their needs: too few to show anything", rejected)
	}
}

// What every match of a grammar needs, and when it can't tell.
func TestNeeds(t *testing.T) {
	cases := []struct {
		src       string
		has, hasn string // characters in the needs, and not; has "" means none
	}{
		{`grammar G { token TOP { '£' \s? \d+ } }`, "£", "1a "},
		{`grammar G { token TOP { '£'? \s? \d+ [ :i k ]? } }`, "0123456789", "£ka "},
		{`grammar G { token TOP { <cur>? \s? <num> } token cur { '£' | '$' } token num { \d+ } }`, "05", "£$ "},
		{`grammar G { token TOP { :i 'p.a.' } }`, ".", "pPaA"},
		{`grammar G { token TOP { :i pa } }`, "pP", "x."}, // a tie: the first
		{`grammar G { token TOP { [ mon | tue ] } }`, "mt", "MTx"},
		{`grammar G { token TOP { :i [ mon | tue ] } }`, "mtMT", "x"},
		{`grammar G { token TOP { \d ** 2..3 | <[a..c]>+ } }`, "1ab", "dx"},
		{`grammar G { token TOP { <[A..Z]> ** 1..2 \d } }`, "5", "A"},
		{`grammar G { token TOP { [ a | b ] && \d } }`, "ab", "1"},
		{`grammar G { token TOP { '(' ~ ')' \d+ } }`, "(", ")"},
		{`grammar G { token TOP { $<x>=[ \d+ ] } }`, "1", "x"},
		{`grammar G { token TOP { [ \d+ ]: } }`, "1", "x"},
		{`grammar G { token TOP { [ x \d ]+ % ',' } }`, "x", ","},
		// A rule reached again while it's being worked out needs nothing.
		{`grammar G { token TOP { '(' <TOP>? ')' | \d } }`, "(1", ")x"},
		// None: optional, empty, a lookaround, a backref, a regex, a branch
		// with none, an optional call, an unknown rule, a backref.
		{`grammar G { token TOP { \d? } }`, "", ""},
		{`grammar G { token TOP { \d* } }`, "", ""},
		{`grammar G { token TOP { '' } }`, "", ""},
		{`grammar G { token TOP { <?before \d> } }`, "", ""},
		{`grammar G { token TOP { <?after \d> } }`, "", ""},
		{`grammar G { token TOP { a | \d? } }`, "", ""},
		{`grammar G { token TOP { <x>? } token x { a } }`, "", ""},
		{`grammar G { token TOP { $<x>=[ a? ] $<x> } }`, "", ""},
	}
	for _, c := range cases {
		g := MustCompile(c.src)
		needs := g.needsOf("TOP")
		if c.has == "" {
			if needs != nil {
				t.Errorf("%s: needs %v, want none", c.src, needs)
			}
			continue
		}
		if needs == nil {
			t.Errorf("%s: no needs", c.src)
			continue
		}
		for _, r := range c.has {
			if !needs.has(r) {
				t.Errorf("%s: needs leave out %q", c.src, r)
			}
		}
		for _, r := range c.hasn {
			if needs.has(r) {
				t.Errorf("%s: needs take in %q", c.src, r)
			}
		}
	}
	// A regex terminal, which isn't in Raku's syntax, and a call to a rule
	// that isn't there, which Compile refuses, need nothing.
	if needs := New("R").Token("TOP", Seq(Re(`\d+`))).needsOf("TOP"); needs != nil {
		t.Errorf("Re: needs %v, want none", needs)
	}
	if needs := New("U").Token("TOP", Call("nowhere")).needsOf("TOP"); needs != nil {
		t.Errorf("an unknown rule: needs %v, want none", needs)
	}
}

// A word list needs the first characters of its phrases, in every case
// under FoldCase.
func TestNeedsWords(t *testing.T) {
	exact := NewWordList("w").Add("perm", "permanent").Add("temp")
	fold := NewWordList("w", FoldCase()).Add("perm").Add("temp")
	joined := NewWordList("w", JoinedSpacing()).Add("full time")
	for _, c := range []struct {
		l         *WordList
		has, hasn string
	}{
		{exact, "pt", "PTx"},
		{fold, "ptPT", "x"},
		{joined, "f", "x "},
	} {
		needs := MustCompile(`grammar G { token TOP { @w } }`, c.l).needsOf("TOP")
		if needs == nil {
			t.Fatalf("%v: no needs", c.l)
		}
		for _, r := range c.has {
			if !needs.has(r) {
				t.Errorf("%q: needs leave out %q", c.has, r)
			}
		}
		for _, r := range c.hasn {
			if needs.has(r) {
				t.Errorf("%q: needs take in %q", c.has, r)
			}
		}
	}
}

// A text with none of a grammar's needs is rejected without trying the
// grammar at any position: a trace of the search is empty. One with a
// needed character is searched only up to it.
func TestNeedsTryNothing(t *testing.T) {
	g := MustCompile(`grammar Pay { token TOP { '£'? \s? \d+ } }`)
	var trace strings.Builder
	if m := g.Find("We are looking for an experienced care assistant", Trace(&trace)); m.Defined() {
		t.Fatalf("found %v", m)
	}
	if trace.Len() > 0 {
		t.Errorf("a text with no digit was searched:\n%s", trace.String())
	}
	trace.Reset()
	g.FindAll("a 1 b c d e", Trace(&trace))
	tries := strings.Count(trace.String(), "TOP\n")
	trace.Reset()
	withoutNeeds(func() { g.FindAll("a 1 b c d e", Trace(&trace)) })
	if all := strings.Count(trace.String(), "TOP\n"); tries == 0 || tries >= all {
		t.Errorf("tried TOP %d times with needs, %d without", tries, all)
	}
	// A text with no digit is still reported from the attempt that got
	// furthest, from the £, rather than from the first position, as it
	// would be if the report tried only the positions the needs allow.
	_, err := g.FindErr("x £ y")
	if err == nil || !strings.Contains(err.Error(), "column 5: expected \\d") {
		t.Errorf("FindErr: %v, want the attempt from the £, at column 5", err)
	}
}

// Needs are worked out once for each rule, and again once a grammar changes.
func TestNeedsCached(t *testing.T) {
	l := NewWordList("w").Add("perm")
	g := MustCompile(`grammar G { token TOP { @w } }`, l)
	a := g.searchFor("TOP").needs
	if b := g.searchFor("TOP").needs; a != b {
		t.Error("needs worked out again")
	}
	l.Add("temp")
	b := g.searchFor("TOP").needs
	if !b.has('t') {
		t.Error("needs kept after the word list changed")
	}
	if !g.Find("a temp role").Defined() {
		t.Error("a phrase added after a search isn't found")
	}
}

// A needed character that NFC composes, as e and a combining acute make é,
// is found in a text that has it decomposed, as the matcher reads the text
// in NFC; rejecting a text before reading it mustn't miss it.
func TestNeedsNFC(t *testing.T) {
	g := MustCompile(`grammar Cafe { token TOP { caf 'é' } }`)
	if needs := g.needsOf("TOP"); needs == nil || !needs.has('é') {
		t.Fatalf("needs %v, want é", needs)
	}
	for _, text := range []string{"a café here", "café"} {
		if !g.Find(text).Defined() || g.FindAll(text).Length() != 1 || g.Split(text).Length() != 3 {
			t.Errorf("%q: not found", text)
		}
	}
}
