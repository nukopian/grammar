package grammar

import (
	"errors"
	"strings"
	"testing"
)

func TestParseErr(t *testing.T) {
	for _, c := range []struct {
		in, want  string
		line, col int
	}{
		// A rule that couldn't start is named, not its first character.
		{"45k", `grammar Pay: line 1, column 1: expected <money>, found "45k"`, 1, 1},
		// After a separator, another <money> was due.
		{"£45k - x", `grammar Pay: line 1, column 8: expected <money>, found "x"`, 1, 8},
		// Part-way through a rule: what was due where it got furthest.
		{"£45,00", `grammar Pay: line 1, column 7: expected <digit>, found end of text`, 1, 7},
		// Parse must reach the end: what could have gone on, or the end.
		{"£45k!", `grammar Pay: line 1, column 5: expected <sep>, <period> or end of text, found "!"`, 1, 5},
		// Lines and columns count characters.
		{"£45k\n- £x", `grammar Pay: line 2, column 4: expected <amount>, found "x"`, 2, 4},
	} {
		m, err := payErr.ParseErr(c.in)
		var pe *ParseError
		if m.Defined() || !errors.As(err, &pe) {
			t.Errorf("%q: match %v, err %v", c.in, m, err)
			continue
		}
		if err.Error() != c.want || pe.Line != c.line || pe.Column != c.col {
			t.Errorf("%q:\n got %v (line %d, column %d)\nwant %s", c.in, err, pe.Line, pe.Column, c.want)
		}
	}
	// A match is returned as Parse returns it, with no error.
	m, err := payErr.ParseErr("£45k - £55k")
	if err != nil || m.At("max.~").Str() != "£55k" {
		t.Errorf("match: %v, %v", m, err)
	}
}

// payErr is pay with a rule a report can name for the digits after a comma.
var payErr = MustCompile(`
grammar Pay {
    token TOP    { <min=money> [ <.sep> <max=money> ]? <period>? }
    token money  { '£' \s? <amount> }
    token amount { <num> [ $<k>=[:i k] ]? }
    token num    { \d ** 1..3 [ ',' <digit> ** 3 ]* [ '.' \d+ ]? }
    token sep    { \s* [ '-' | '–' | to ] \s* }
    token period { :i \s* [ 'p.a.' | pa | per \s+ [ annum | year | hour | day ] ] }
}`)

func TestSubparseErr(t *testing.T) {
	_, err := payErr.SubparseErr("Pay: £45k")
	if err == nil || !strings.Contains(err.Error(), `column 1: expected <money>, found "Pay: £45k"`) {
		t.Errorf("got %v", err)
	}
	m, err := payErr.SubparseErr("£45k, negotiable")
	if err != nil || m.At("~").Str() != "£45k" {
		t.Errorf("match: %v, %v", m, err)
	}
}

// What a report says about each kind of thing it expected.
func TestParseErrDescriptions(t *testing.T) {
	units := NewWordList("units").Add("day")
	for _, c := range []struct{ src, in, want string }{
		{`grammar D1 { token TOP { a \d } }`, "ax", `expected \d`},
		{`grammar D2 { token TOP { a <[x..z]> } }`, "aa", `expected <[x..z]>`},
		{`grammar D3 { token TOP { a <-[\n]> } }`, "a\n", `expected <-[\n]>, found end of line`},
		{`grammar D4 { token TOP { a @units } }`, "ax", `expected @units`},
		{`grammar D5 { token TOP { a . } }`, "a", `expected any character, found end of text`},
		{`grammar D6 { token TOP { a $ } }`, "ab", `expected end of text, found "b"`},
		{`grammar D7 { token TOP { a ^^ b } }`, "ab", `expected start of line`},
		{`grammar D8 { token TOP { a << b } }`, "ab", `expected start of word`},
		// Lookarounds don't report their own failures.
		{`grammar D9 { token TOP { a <!before b> . } }`, "ab", `column 2: expected not 'b'`},
		{`grammar D12 { token TOP { a <?before \d> . } }`, "ab", `column 2: expected \d`},
		{`grammar D10 { rule TOP { a b } }`, "a c", `column 3: expected 'b'`},
		{`grammar D11 { token TOP { :i 'abc' } }`, "abd", `column 1: expected 'abc' (ignoring case)`},
		{`grammar D13 { token TOP { :i abc } }`, "abd", `column 1: expected 'abc' (ignoring case)`},
		// A bare word is one literal, so a misspelt word is reported whole,
		// and :i is said once for a list that's all under it.
		{`grammar D15 { token TOP { :i per \s+ [ annum | year | hour | day ] } }`, "per anum",
			`column 5: expected 'annum', 'year', 'hour' or 'day' (ignoring case), found "anum"`},
		// One more \d in \d+ isn't reported, but a literal that could follow is.
		{`grammar D14 { token TOP { \d+ [ k ]? '!' } }`, "45x", `column 3: expected 'k' or '!', found "x"`},
	} {
		g, err := Compile(c.src, units)
		if err != nil {
			t.Fatal(err)
		}
		_, err = g.ParseErr(c.in)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s on %q: %v, want %s", c.src, c.in, err, c.want)
		}
	}
	re := New("R").Token("TOP", Seq(Lit("a"), Re(`\d+`)))
	if _, err := re.ParseErr("ax"); err == nil || !strings.Contains(err.Error(), `expected /\d+/`) {
		t.Errorf("Re: %v", err)
	}
}

// Ordinary parsing doesn't track failures, and gives the same results.
func TestParseErrSameMatches(t *testing.T) {
	for _, c := range rakuCases {
		g, err := Compile(c.Src, caseLists(c.Lists)...)
		if err != nil {
			continue
		}
		for _, in := range c.Inputs {
			parse, sub := g.Parse, g.ParseErr
			if c.Sub {
				parse, sub = g.Subparse, g.SubparseErr
			}
			m, err := sub(in)
			if d := m.Diff(parse(in)); d != "" || (err == nil) != m.Defined() {
				t.Errorf("%s on %q: %s, err %v", g.Name(), in, d, err)
			}
		}
	}
}

// <!> never matches, and says nothing about what the text needed, so a
// failure report leaves it out, though it marks how far the parse got.
func TestParseErrNever(t *testing.T) {
	for _, c := range []struct{ src, text, want string }{
		{`grammar N1 { token TOP { a <!> } }`, "a", "grammar N1: line 1, column 2: expected something else, found end of text"},
		{`grammar N2 { token TOP { a [ <!> | b ] } }`, "ac", `grammar N2: line 1, column 2: expected 'b', found "c"`},
	} {
		g := MustCompile(c.src)
		_, err := g.ParseErr(c.text)
		if want := c.want; err == nil || err.Error() != want {
			t.Errorf("%s: %v, want %s", g.Name(), err, want)
		}
	}
}
