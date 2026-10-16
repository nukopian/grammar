package grammar

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// The fuzz tests run their seeds with go test, and search for failures
// with -fuzz:
//
//	go test ./internal/scalar/grammar -run '^$' -fuzz FuzzCompile -fuzztime 60s
//	go test ./internal/scalar/grammar -run '^$' -fuzz FuzzMatch -fuzztime 60s
//	go test ./internal/scalar/grammar -run '^$' -fuzz FuzzWords -fuzztime 60s

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
// FindAll finds first; skipping start positions changes nothing; and Values
// takes any match.
func FuzzMatch(f *testing.F) {
	for _, seed := range [][2]string{
		{`<a>+ % ',' <c>?`, "x,x,abc"},
		{`[ a | ab | abc ] .*? <b>`, "abc y z"},
		{`:i @w+ % <.ws>`, "AB a-b X"},
		{`( \d+ ) $<k>=[ k ]? <!before \w>`, "45k "},
		{`^^ <a> $$ \n?`, "x\nx"},
		{`[ <?after x> y ]* <<x>>`, "xyx"},
		{`:s a b`, "a  b"},
		{`<c> ** 2..3 %% '-'`, "ab-c-"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, body, text string) {
		// Short texts, and no unrestricted backtracking, which can take
		// exponential time by design.
		if len(body) > 200 || utf8.RuneCountInString(text) > 40 || strings.Contains(body, "regex") || strings.Contains(body, ":!r") {
			t.Skip()
		}
		g, err := Compile("grammar F { token TOP { "+body+" }; token a { x }; rule b { y z }; token c { <[a..c]>+ } }", fuzzList())
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
			// Text is read as Go reads it into runes: each byte that isn't
			// UTF-8 becomes U+FFFD.
			if read := string([]rune(text)); !sub && m.Defined() && m.At("~").Str() != read {
				t.Fatalf("Parse matched %q of %q", m.At("~").Str(), read)
			}
			Values(m)
		}
		all := g.FindAll(text)
		first := g.Find(text)
		if all.Length() > 0 {
			if d := first.Diff(all.Get(0)); d != "" {
				t.Fatalf("Find and FindAll's first differ: %s", d)
			}
		} else if first.Defined() {
			t.Fatalf("Find found %v, FindAll nothing", first)
		}
		_, without := findBoth(g, text) // restores skipStarts, even on a panic
		if d := all.Diff(without[1].AV()); d != "" {
			t.Fatalf("skipping start positions changed FindAll: %s", d)
		}
	})
}

// Synonym rules never panic, and every phrase in a list matches itself.
func FuzzWords(f *testing.F) {
	f.Add("Year, annum, yr\nhr, ph => Hour\npm => product manager, project manager\nc\\, d => cd", true, true)
	f.Add("a\\\\, b => c\n# comment\n=>\n, ,", false, true)
	f.Add("Front-end Developer, Frontend Developer", true, false)
	f.Fuzz(func(t *testing.T, rules string, fold, loose bool) {
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
			l.AddRules(rules) // errors are fine; panics aren't
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
	})
}
