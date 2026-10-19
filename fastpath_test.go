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

// The fast paths reach StepLimit and DepthLimit exactly where the slow
// paths do: the same error, after the same number of steps, or the same
// result where neither is reached. Long runs of digits and spaces make
// repeats that loop takes hundreds of items at a time.
func TestFastPathsKeepLimits(t *testing.T) {
	defer func(s, d int) { StepLimit, DepthLimit = s, d }(StepLimit, DepthLimit)
	long := strings.Repeat("9", 400)
	texts := []string{
		"£45,000 - £55,000 per annum", "£" + long + " per year", "£1" + strings.Repeat(" ", 300) + "per hour",
		long, strings.Repeat("ab", 200), "",
	}
	grammars := []*Grammar{
		pay, payErr,
		MustCompile(`grammar D { token TOP { \d+ } }`),
		MustCompile(`grammar S { token TOP { \d ** 2..* \s* x? } }`),
		MustCompile(`grammar L { token TOP { [ab]+ | 'ab'* } }`),
		MustCompile(`grammar R { rule TOP { \d+ \d+ } }`),
	}
	compared, stepLimited, depthLimited := 0, 0, 0
	for _, limits := range [][2]int{{0, 0}, {3, 0}, {50, 0}, {400, 0}, {0, 10}, {0, 100}, {0, 600}, {200, 300}} {
		StepLimit, DepthLimit = limits[0], limits[1]
		for _, g := range grammars {
			for _, text := range texts {
				with := limitResults(g, text)
				var without []string
				withoutFastPaths(func() { without = limitResults(g, text) })
				for i := range with {
					if with[i] != without[i] {
						t.Errorf("limits %v, %s on %.40q:\n with: %s\n without: %s", limits, g.Name(), text, with[i], without[i])
					}
					stepLimited += strings.Count(with[i], "see StepLimit")
					depthLimited += strings.Count(with[i], "see DepthLimit")
				}
				compared++
			}
		}
	}
	if compared != 8*6*6 {
		t.Fatalf("compared %d", compared)
	}
	// Both limits must be reached, or this checks nothing about them.
	if stepLimited == 0 || depthLimited == 0 {
		t.Fatalf("StepLimit reached %d times, DepthLimit %d", stepLimited, depthLimited)
	}
	t.Logf("StepLimit reached %d times, DepthLimit %d", stepLimited, depthLimited)
}

// limitResults is Subparse's and Parse's results for text, or the panic
// each gives, which is how they report a limit, and the steps a run at
// every position takes before it stops.
func limitResults(g *Grammar, text string) []string {
	try := func(f func() string) (s string) {
		defer func() {
			if r := recover(); r != nil {
				s = fmt.Sprint("panic: ", r)
			}
		}()
		return f()
	}
	steps := try(func() string {
		st := g.newState([]rune(nfc(text)))
		defer func() {
			if r := recover(); r != nil {
				// Not st.depth: unwinding a panic runs iter's deferred
				// shallower, which loop doesn't have, and the state is
				// discarded anyway.
				panic(fmt.Sprint(r, " after ", st.steps, " steps"))
			}
		}()
		for from := 0; from <= len(st.in); from++ {
			g.runState(st, from, false, options{rule: "TOP"})
		}
		return fmt.Sprint("steps ", st.steps)
	})
	return []string{
		try(func() string { return "Subparse " + g.Subparse(text).Dump() }),
		try(func() string { return "Parse " + g.Parse(text).Dump() }),
		steps,
	}
}
