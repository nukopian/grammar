package grammar

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nukopian/scalar"
)

// findBoth runs Find, and FindAll alone, with Overlap and with Outermost,
// with and without skipping positions where no match can start.
func findBoth(g *Grammar, text string) (with, without [4]*scalar.SV) {
	defer func() { skipStarts = true }()
	for _, skip := range []bool{true, false} {
		skipStarts = skip
		r := [4]*scalar.SV{g.Find(text), scalar.Value(g.FindAll(text)), scalar.Value(g.FindAll(text, Overlap())),
			scalar.Value(g.FindAll(text, Outermost()))}
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
		g, err := Compile(c.Src, c.uses()...)
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
			for i, name := range []string{"Find", "FindAll", "FindAll with Overlap", "FindAll with Outermost"} {
				if d := with[i].Diff(without[i]); d != "" {
					t.Errorf("%s.%s(%q): %s", g.Name(), name, text, d)
				}
			}
		}
	}
}

func slicesClone(s []string) []string { return append([]string(nil), s...) }

// The starts' table of ASCII characters says what asking the starts does,
// for every character, for every Raku case's grammar and the package's test
// grammars, and for each lookbehind in them.
func TestStartsTableChangesNothing(t *testing.T) {
	grammars := []*Grammar{pay, payErr, shapes}
	var texts []string
	for _, c := range rakuCases {
		g, err := Compile(c.Src, c.uses()...)
		if err != nil {
			t.Fatal(err)
		}
		grammars = append(grammars, g)
		texts = append(texts, c.Inputs...)
	}
	same := func(name string, s *starts) {
		if !s.tabled {
			t.Fatalf("%s: no table", name)
		}
		slow := *s
		slow.tabled = false
		for r := rune(0); r < 128; r++ {
			if s.has(r) != slow.has(r) {
				t.Errorf("%s: %q: table says %v", name, r, s.has(r))
			}
		}
	}
	lookbehinds := 0
	for _, g := range grammars {
		s, _ := g.startsOf("TOP")
		same(g.Name(), s)
		for _, text := range texts {
			g.FindAll(text) // works out its lookbehinds' starts
		}
		g.afters.Range(func(_, v any) bool {
			same(g.Name()+" lookbehind", v.(*lookInfo).s)
			lookbehinds++
			return true
		})
	}
	if lookbehinds == 0 {
		t.Fatal("no lookbehinds checked")
	}
}

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
	// A regex terminal is asked where it next matches, unless it can match
	// empty text or uses \G, when it could start anywhere.
	for _, c := range []struct {
		pattern    string
		everywhere bool
	}{{`\d+`, false}, {`\d*`, true}, {`\G\d`, true}, {`(?=\d)`, true}} {
		s, empty := New("R").Token("TOP", Re(c.pattern)).startsOf("TOP")
		if got := empty || s.any; got != c.everywhere || !got && len(s.res) != 1 {
			t.Errorf("Re(%s): everywhere %v, %d regexes; want everywhere %v", c.pattern, got, len(s.res), c.everywhere)
		}
	}
}

// Grammars that start with a regex terminal skip the positions before
// where it next matches, and find what they found without skipping: alone,
// beside a literal, under :i, after a lookahead or optional spaces, with
// lookbehind in the pattern, in a called rule, and where they can't skip.
func TestReStartsChangeNothing(t *testing.T) {
	grammars := []*Grammar{
		New("R1").Token("TOP", Re(`£\d[\d,]*(?:k\b)?`)),
		New("R2").Token("TOP", Alt(Re(`\d+k`), Lit("£"))),
		New("R3").Token("TOP", IgnoreCase(Re(`up to £\d+`))),
		New("R4").Token("TOP", Seq(Before(Lit("£")), Re(`£\d+`))),
		New("R5").Token("TOP", Seq(Star(Lit(" ")), Re(`\d+`), Lit("k"))),
		New("R6").Token("TOP", Re(`(?<=£)\d+`)),
		New("R7").Token("TOP", Seq(Call("num"), Lit("k"))).Token("num", Re(`\d+`)),
		New("R8").Token("TOP", Seq(Re(`\d+`), Opt(Re(`k`)))),
		New("R9").Token("TOP", Re(`\G£\d`)),
		New("R10").Token("TOP", Re(`\d*k`)),
		New("R11").Regex("TOP", Seq(Re(`\d+`), Lit("0k"))),
	}
	texts := []string{
		"Salary: £45k – £55,000 per annum, plus £5k bonus", "", "£", "45k", "x£1 £ 2 £3,000k",
		"UP TO £50 or up to £60", "Up To £7", "  45k and 100k", "£1£2£3", "1000k", "10k, 20k\n30k",
	}
	for _, g := range grammars {
		for _, text := range texts {
			with, without := findBoth(g, text)
			for i, name := range []string{"Find", "FindAll", "FindAll with Overlap", "FindAll with Outermost"} {
				if d := with[i].Diff(without[i]); d != "" {
					t.Errorf("%s.%s(%q): %s", g.Name(), name, text, d)
				}
			}
			skipStarts = false
			split := g.Split(text)
			skipStarts = true
			if d := split.Diff(g.Split(text)); d != "" {
				t.Errorf("%s.Split(%q): %s", g.Name(), text, d)
			}
		}
	}
}

// A search skips the positions before a regex terminal's next match: on a
// long text with three figures, the matcher is tried at just those.
func TestReStartsSkip(t *testing.T) {
	g := New("R").Token("TOP", Re(`£\d+`))
	text := strings.Repeat("no figures here. ", 50) + "£1 £2 and £3"
	in := []rune(text)
	sc := g.scanner("TOP", in)
	var tried []int
	for at := 0; at <= len(in); at++ {
		if sc.can(at) {
			tried = append(tried, at)
		}
	}
	if want := []int{850, 853, 860}; !slices.Equal(tried, want) {
		t.Errorf("tried %v, want %v", tried, want)
	}
}

// A regex whose search over the rest of the text runs past RegexTimeout,
// as (?:a|aa)+b does over a run of a's, stops skipping, and every position
// is tried, as without it, rather than the search panicking.
func TestReStartsTimeout(t *testing.T) {
	old := scalar.RegexTimeout
	scalar.RegexTimeout = time.Millisecond
	defer func() { scalar.RegexTimeout = old }()
	g := New("S").Token("TOP", Re(`(?:a|aa)+b|c`))
	in := []rune(strings.Repeat("a", 40) + " c")
	sc := g.scanner("TOP", in)
	for at := 0; at <= len(in); at++ {
		if !sc.can(at) {
			t.Fatalf("position %d skipped after the search timed out", at)
		}
	}
	if !sc.stuck[0] {
		t.Error("the search didn't time out: lengthen the run of a's")
	}
}

// Skipping an alternation's branches that can't start with the next
// character changes no result, not even a failure report, and never takes
// more steps: checked over every Raku case and the package's test grammars,
// each input also searched for inside some surrounding text. Somewhere, it
// must take fewer, or it skipped nothing.
func TestBranchSkipsChangeNothing(t *testing.T) {
	defer func() { skipBranches = true }()
	acronyms := NewWordList("acro", FoldCase(), WholeWords(), Plurals(), ExactAcronyms()).Add("IT").Add("NHS trust").Add("pay")
	grammars := []*Grammar{pay, payErr, shapes, MustCompile(`grammar Acro { token TOP { @acro | IT \d } }`, acronyms)}
	var texts []string
	for _, c := range rakuCases {
		g, err := Compile(c.Src, c.uses()...)
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
		"IT and it, an NHS Trust, nhs trust, PAY and pay, ITs, IT4", "£45,000 - £55,000 per annum")
	fewer := 0
	for _, g := range grammars {
		for _, text := range texts {
			skipBranches = true
			with := results(g, text)
			skipBranches = false
			without := results(g, text)
			last := len(with) - 1 // the steps
			for i := range last {
				if with[i] != without[i] {
					t.Errorf("%s on %q:\n with: %s\n without: %s", g.Name(), text, with[i], without[i])
				}
			}
			var stepsWith, stepsWithout int
			fmt.Sscanf(with[last], "steps %d", &stepsWith)
			fmt.Sscanf(without[last], "steps %d", &stepsWithout)
			if stepsWith > stepsWithout {
				t.Errorf("%s on %q: %d steps skipping branches, %d without", g.Name(), text, stepsWith, stepsWithout)
			}
			if stepsWith < stepsWithout {
				fewer++
			}
		}
	}
	if fewer == 0 {
		t.Fatal("no branch was ever skipped")
	}
	t.Logf("fewer steps for %d texts", fewer)
}

// TestBranchStartsDontGrow checks that a search's state doesn't grow with
// the alternations made elsewhere in the program: they were once kept on
// the state by a number counted across every grammar, so that after
// 200,000 alternations, a new state took 9.6 MB.
func TestBranchStartsDontGrow(t *testing.T) {
	for range 100000 {
		Alt(Lit("x"), Lit("y"))
	}
	g := MustCompile(`grammar G { token TOP { a | b } }`)
	runtime.GC()
	runtime.GC() // empties g's states
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	g.Parse("a")
	runtime.ReadMemStats(&after)
	if n := after.TotalAlloc - before.TotalAlloc; n > 100000 {
		t.Errorf("a parse on a new state made %d bytes", n)
	}
}

// TestBranchStartsPerGrammar checks that an alternation shared by two
// grammars skips branches by what each grammar's rules start with.
func TestBranchStartsPerGrammar(t *testing.T) {
	shared := Alt(Seq(Call("x"), Lit("!")), Lit("b"))
	a := New("A").Token("x", Lit("a")).Token("TOP", shared)
	b := New("B").Token("x", Lit("b")).Token("TOP", shared)
	for range 2 {
		for _, c := range []struct {
			g    *Grammar
			text string
			want bool
		}{{a, "a!", true}, {b, "b!", true}, {a, "b", true}, {b, "a!", false}} {
			if got := c.g.Parse(c.text).Defined(); got != c.want {
				t.Errorf("%s on %q: %v, want %v", c.g.Name(), c.text, got, c.want)
			}
		}
	}
}
