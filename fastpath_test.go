package grammar

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// results is everything a grammar gives for a text, to compare with the
// fast paths on and off: what Parse, Subparse, Find and FindAll return,
// ParseErr's and SubparseErr's reports, and the matcher's steps, trying the
// grammar at every position.
func results(g *Grammar, text string) []string {
	show := func(v *scalar.SV) string { return v.Dump() }
	errText := func(_ *scalar.SV, err error) string {
		var se *StepLimitError
		var de *DepthLimitError
		if errors.As(err, &se) || errors.As(err, &de) {
			return "limit: " + err.Error()
		}
		return fmt.Sprint(err)
	}
	st := g.newState([]rune(nfc(text)))
	for from := 0; from <= len(st.in); from++ {
		g.runState(st, from, false, options{rule: "TOP"})
	}
	return []string{
		"Parse " + show(g.Parse(text)),
		"Subparse " + show(g.Subparse(text)),
		"Find " + show(g.Find(text)),
		"FindAll " + show(scalar.Value(g.FindAll(text))),
		"ParseErr " + errText(g.ParseErr(text)),
		"SubparseErr " + errText(g.SubparseErr(text)),
		fmt.Sprint("steps ", st.steps),
	}
}

// withoutFastPaths runs f with the fast paths off, and turns them back on,
// even if f panics.
func withoutFastPaths(f func()) {
	defer func() { fastPaths = true }()
	fastPaths = false
	f()
}

// The fast paths change nothing: not a match, a failure report, or the
// number of steps the matcher takes, which StepLimit counts. Checked over
// every Raku case and the package's test grammars, each input also searched
// for inside some surrounding text.
func TestFastPathsChangeNothing(t *testing.T) {
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
		"IT and it, an NHS Trust, nhs trust, PAY and pay, ITs", "£45,000 - £55,000 per annum")
	compared := 0
	for _, g := range grammars {
		for _, text := range texts {
			with := results(g, text)
			var without []string
			withoutFastPaths(func() { without = results(g, text) })
			for i := range with {
				if with[i] != without[i] {
					t.Errorf("%s on %q:\n with: %s\n without: %s", g.Name(), text, with[i], without[i])
				}
			}
			compared++
		}
	}
	if compared < 1000 {
		t.Fatalf("only %d comparisons", compared)
	}
}
