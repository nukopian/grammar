package grammar

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/nukopian/scalar"
)

// The fuzz tests run their seeds with go test, and search for failures
// with -fuzz:
//
//	go test -run '^$' -fuzz FuzzCompile -fuzztime 60s
//	go test -run '^$' -fuzz FuzzMatch -fuzztime 60s
//	go test -run '^$' -fuzz FuzzWords -fuzztime 60s

// Compile never panics, however mangled its source: it returns an error.
func FuzzCompile(f *testing.F) {
	for _, c := range rakuCases {
		f.Add(c.Src)
	}
	for _, src := range []string{`grammar G { token TOP { <[a..`, `grammar G { rule TOP { a ** 3..1 } }`, `role R {`} {
		f.Add(src)
	}
	f.Fuzz(func(t *testing.T, src string) {
		Compile(src, fuzzList()) // a panic fails the test
	})
}

// fuzzList is a word list for fuzzed grammars to use, as @w.
func fuzzList() *WordList {
	return NewWordList("w", FoldCase(), LooseSpacing()).Add("ab", "a-b", "abc").Add("x")
}

// Whatever grammar and text, matching keeps its promises: no panics; Parse
// and ParseErr agree; a parse matches all of the text; Find finds what
// FindAll finds first; skipping start positions changes nothing; tracing
// changes nothing, and closes every rule it opens; and Values takes any
// match, and CheckPath every path in one.
// fuzzPatterns is a pattern list for FuzzMatch's grammars, <@pats>.
var fuzzPatterns = MustPatternList("pats", `x+`, `<c> y`, `'z z'`, `(a) $0?`)

// fuzzMatchSeeds are FuzzMatch's starting rule bodies and texts, one or
// more for each kind of syntax; TestFuzzMatchSeedsCompile keeps them
// compiling, as FuzzMatch passes over a body that doesn't.
var fuzzMatchSeeds = [][2]string{
	{`<a>+ % ',' <c>?`, "x,x,abc"},
	{`[ a | ab | abc ] .*? <b>`, "abc y z"},
	{`:i @w+ % <.ws>`, "AB a-b X"},
	{`( \d+ ) $<k>=[ k ]? <!before \w>`, "45k "},
	{`^^ <a> $$ \n?`, "x\nx"},
	{`[ <?after x> y ]* <<x>>`, "xyx"},
	{`<c> <?after <[a..c]> ** 2> \s* <?after <c> \s?> a`, "abc a"},
	{`:s a b`, "a  b"},
	{`<c> ** 2..3 %% '-'`, "ab-c-"},
	// The later additions: conjunctions, goal matching, backreferences,
	// : after an atom, Unicode properties, the proto <p> and the
	// pattern list <@pats> in the grammar below.
	{`<c> & <[a..c]>+`, "abc"},
	{`<a> && \w || y`, "x"},
	{`'(' ~ ')' <c>`, "(ab)"},
	{`$<q>=<[xy]> <c>? $<q>`, "xabx"},
	{`$<x> = [ a ] $<x>`, "aa"},
	{`(\w) $0+`, "aaa"},
	{`[ a | ab ]: b`, "abb"},
	{`<:L>+ <:!L>? <:Lu + [_]>*`, "Zoë1A_"},
	{`<p>+ % ','`, "x,xyab,x"},
	{`:i <@pats>+ % ' '`, "XX z z abY"},
}

// fuzzGrammar is FuzzMatch's grammar around a fuzzed rule body.
func fuzzGrammar(body string) (*Grammar, error) {
	return Compile("grammar F { token TOP { "+body+" }; token a { x }; rule b { y z }; token c { <[a..c]>+ }; "+
		"proto token p {*}; token p:sym<x> { <sym> }; token p:sym<xy> { xy <c>? } }", fuzzList(), fuzzPatterns)
}

func TestFuzzMatchSeedsCompile(t *testing.T) {
	for _, seed := range fuzzMatchSeeds {
		if _, err := fuzzGrammar(seed[0]); err != nil {
			t.Errorf("%s: %v", seed[0], err)
		}
	}
}

func FuzzMatch(f *testing.F) {
	for _, seed := range fuzzMatchSeeds {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, body, text string) {
		// Short texts, and no unrestricted backtracking, which can take
		// exponential time by design.
		if len(body) > 200 || utf8.RuneCountInString(text) > 40 || strings.Contains(body, "regex") || strings.Contains(body, ":!r") {
			t.Skip()
		}
		g, err := fuzzGrammar(body)
		if err != nil {
			return
		}
		// Running past StepLimit or DepthLimit is a panic by design, as for
		// '' ** 2111111; nothing else is.
		defer func() {
			switch r := recover().(type) {
			case nil, *StepLimitError, *DepthLimitError:
			default:
				panic(r)
			}
		}()
		for _, sub := range []bool{false, true} {
			parse, parseErr := g.Parse, g.ParseErr
			if sub {
				parse, parseErr = g.Subparse, g.SubparseErr
			}
			m := parse(text)
			m2, err := parseErr(text)
			if d := m.Diff(m2); d != "" || m.Defined() != (err == nil) {
				t.Fatalf("Parse and ParseErr disagree (sub %v): %s, err %v", sub, d, err)
			}
			var trace strings.Builder
			if d := m.Diff(parse(text, Trace(&trace))); d != "" {
				t.Fatalf("tracing changed the match (sub %v): %s", sub, d)
			}
			checkTrace(t, trace.String())
			// Text is read as Go reads it into runes, each byte that isn't
			// UTF-8 becoming U+FFFD, and then in NFC.
			if read := nfc(text); !sub && m.Defined() && m.At("~").Str() != read {
				t.Fatalf("Parse matched %q of %q", m.At("~").Str(), read)
			}
			Values(m)
			if m.Defined() {
				checkRealPaths(t, g, m) // CheckPath takes every path a match has
			}
		}
		all := g.FindAll(text)
		var trace strings.Builder
		if d := all.Diff(g.FindAll(text, Trace(&trace))); d != "" {
			t.Fatalf("tracing changed FindAll's matches: %s", d)
		}
		checkTrace(t, trace.String())
		first := g.Find(text)
		if all.Length() > 0 {
			if d := first.Diff(all.Get(0)); d != "" {
				t.Fatalf("Find and FindAll's first differ: %s", d)
			}
		} else if first.Defined() {
			t.Fatalf("Find found %v, FindAll nothing", first)
		}
		// With Overlap, a match from each place one starts: what Find
		// finds from there, with FindAll's matches among them.
		over := g.FindAll(text, Overlap())
		last, i := -1, 0
		for _, m := range over.All() {
			from := int(m.At("from").Int())
			if from <= last {
				t.Fatalf("Overlap's matches start at %d after %d", from, last)
			}
			last = from
			if d := m.Diff(g.Find(text, Pos(from))); d != "" {
				t.Fatalf("Overlap's match at %d isn't Find's from there: %s", from, d)
			}
			if i < all.Length() && from == int(all.Get(i).At("from").Int()) {
				if d := m.Diff(all.Get(i)); d != "" {
					t.Fatalf("Overlap's match at %d isn't FindAll's: %s", from, d)
				}
				i++
			}
		}
		if i != all.Length() {
			t.Fatalf("Overlap's matches leave out FindAll's from %v", all.Get(i).At("from"))
		}
		// With Outermost, Overlap's matches less each inside a longer one,
		// as jdc-lab first wrote it.
		outer := g.FindAll(text, Outermost())
		span := func(m *scalar.SV) (int64, int64) { return m.At("from").Int(), m.At("to").Int() }
		want := over.Grep(func(m *scalar.SV) bool {
			from, to := span(m)
			return !slices.ContainsFunc(over.Items(), func(n *scalar.SV) bool {
				f, t := span(n)
				return f <= from && to <= t && t-f > to-from
			})
		})
		if d := outer.Diff(want); d != "" {
			t.Fatalf("Outermost isn't Overlap less the nested matches: %s", d)
		}
		_, without := findBoth(g, text) // restores skipStarts, even on a panic
		if d := all.Diff(without[1].AV()); d != "" {
			t.Fatalf("skipping start positions changed FindAll: %s", d)
		}
		if d := over.Diff(without[2].AV()); d != "" {
			t.Fatalf("skipping start positions changed FindAll with Overlap: %s", d)
		}
		if d := outer.Diff(without[3].AV()); d != "" {
			t.Fatalf("skipping start positions changed FindAll with Outermost: %s", d)
		}
		// Narrowing lookbehinds' starts changes nothing either.
		var unbounded *scalar.AV
		withoutBounds(func() { unbounded = g.FindAll(text) })
		if d := all.Diff(unbounded); d != "" {
			t.Fatalf("narrowing lookbehinds changed FindAll: %s", d)
		}
		// So do the matcher's fast paths.
		var slow *scalar.AV
		var slowParse *scalar.SV
		withoutFastPaths(func() { slow, slowParse = g.FindAll(text), g.Subparse(text) })
		if d := all.Diff(slow); d != "" {
			t.Fatalf("the fast paths changed FindAll: %s", d)
		}
		if d := g.Subparse(text).Diff(slowParse); d != "" {
			t.Fatalf("the fast paths changed Subparse: %s", d)
		}
	})
}

// Synonym rules never panic, and every phrase in a list matches itself.
func FuzzWords(f *testing.F) {
	f.Add("Year, annum, yr\nhr, ph => Hour\npm => product manager, project manager\nc\\, d => cd", true, true)
	f.Add("a\\\\, b => c\n# comment\n=>\n, ,", false, true)
	f.Add("Front-end Developer, Frontend Developer", true, false)
	f.Add("legal notice, the legal bit => legal\nlegal, law => legal", true, true)
	f.Fuzz(func(t *testing.T, rules string, fold, loose bool) {
		// A phrase with a long run of spacing makes JoinedSpacing's search
		// slow, by design, until StepLimit stops it; a low limit keeps
		// each input quick, and running past it is a panic by design.
		defer func(old int) { StepLimit = old }(StepLimit)
		StepLimit = 200_000
		defer func() {
			switch r := recover().(type) {
			case nil, *StepLimitError, *DepthLimitError:
			default:
				panic(r)
			}
		}()
		for _, extra := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			joined, plural := extra[0], extra[1]
			var opts []ListOption
			if fold {
				opts = append(opts, FoldCase())
			}
			if loose {
				opts = append(opts, LooseSpacing())
			}
			if joined {
				opts = append(opts, JoinedSpacing())
			}
			if plural {
				opts = append(opts, Plurals())
			}
			l := NewWordList("w", opts...)
			err := l.AddRules(rules) // errors are fine; panics aren't
			mapped := NewWordList("w", append(opts, MapOnly())...)
			errMapped := mapped.AddRules(rules)
			if err == nil && errMapped == nil {
				// MapOnly only takes phrases and meanings away.
				for _, phrase := range mapped.Phrases() {
					for _, c := range mapped.Canonical(phrase) {
						if !slices.Contains(l.Canonical(phrase), c) {
							t.Fatalf("with MapOnly, %q means %q, which it doesn't without", phrase, c)
						}
					}
				}
				for _, c := range mapped.Canonicals() {
					if !slices.Contains(l.Canonicals(), c) {
						t.Fatalf("with MapOnly, %q is a canonical value, which it isn't without", c)
					}
				}
			}
			for _, l := range []*WordList{l, mapped} {
				g := MustCompile(`grammar W { token TOP { @w } }`, l)
				for _, phrase := range l.Phrases() {
					texts := []string{phrase}
					if joined && utf8.ValidString(phrase) {
						// Run together, it still matches, and means something.
						// (Joining the bytes of invalid UTF-8 could make a
						// character that was never in the phrase. And where a
						// word starts with a bare combining mark, joining can
						// put two marks together, which NFC then reorders, so
						// the text is no longer the phrase without its spaces:
						// those are left out.)
						if run := strings.Join(strings.FieldsFunc(phrase, isListSpace), ""); nfc(run) == run {
							texts = append(texts, run)
						}
					}
					if p := pluralOf(phrase); plural && p != "" {
						texts = append(texts, p) // and its plural
					}
					for _, text := range texts {
						if text == "" {
							continue
						}
						if len(l.Canonical(text)) == 0 {
							t.Fatalf("%q (from phrase %q) has no meaning", text, phrase)
						}
						m := g.Parse(text)
						if !m.Defined() {
							t.Fatalf("%q (from phrase %q) doesn't match", text, phrase)
						}
						if len(l.Canonical(m.At("~").Str())) == 0 {
							t.Fatalf("%q matched as %q, which has no meaning", text, m.At("~").Str())
						}
					}
				}
				g.FindAll(rules)
			}
		}
	})
}

// checkTrace checks that a trace closes every rule it opens, once, at the
// depth it opened it, and goes no deeper than one level at a time.
func checkTrace(t *testing.T, trace string) {
	t.Helper()
	open := 0 // rules open, each one level deeper
	for i, l := range strings.Split(strings.TrimSuffix(trace, "\n"), "\n") {
		if l == "" {
			continue
		}
		rest, depth := l, 0
		for strings.HasPrefix(rest, "|  ") {
			rest, depth = rest[3:], depth+1
		}
		switch {
		case strings.HasPrefix(rest, "* MATCH"), rest == "* FAIL":
			if depth != open-1 {
				t.Fatalf("line %d closes depth %d with %d open:\n%s", i+1, depth, open, trace)
			}
			open--
		default:
			if depth != open {
				t.Fatalf("line %d opens %q at depth %d with %d open:\n%s", i+1, rest, depth, open, trace)
			}
			open++
		}
	}
	if open != 0 {
		t.Fatalf("%d rules left open:\n%s", open, trace)
	}
}
