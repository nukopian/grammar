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
		g, err := Compile(c.Src, caseUses(c.Lists, c.Patterns)...)
		if err != nil {
			t.Errorf("%v\n%s", err, c.Src)
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

// A goal's closer that's missing is reported with where its opener was, as
// a title's bracket might be many words back.
func TestParseErrGoal(t *testing.T) {
	g := MustCompile(`grammar T {
    token TOP   { <words> [ ' ' '(' ~ ')' <quals> ]? }
    token words { [ \w+ ]+ % ' ' }
    token quals { [ \w+ ]+ % ', ' }
}`)
	for _, c := range []struct{ text, want string }{
		{"Senior Engineer (Remote, FTC", `grammar T: line 1, column 29: expected ', ' or ')' to close '(' from line 1, column 17, found end of text`},
		{"Senior Engineer (Remote; FTC)", `grammar T: line 1, column 24: expected ', ' or ')' to close '(' from line 1, column 17, found "; FTC)"`},
	} {
		_, err := g.ParseErr(c.text)
		if err == nil || err.Error() != c.want {
			t.Errorf("%q: %v\nwant %s", c.text, err, c.want)
		}
	}
	// After the closer, failures are reported as usual. (In a regex, as
	// what follows runs inside the closer's match; in a token, it runs
	// after the closer has committed.)
	g3 := MustCompile(`grammar A { regex TOP { '(' ~ ')' \w+ 'y' } }`)
	_, err3 := g3.ParseErr("(ab)z")
	if want := `grammar A: line 1, column 5: expected 'y', found "z"`; err3 == nil || err3.Error() != want {
		t.Errorf("%v\nwant %s", err3, want)
	}
	// Over lines.
	g2 := MustCompile(`grammar L { token TOP { 'x' '(' ~ ')' \s* } }`)
	_, err := g2.ParseErr("x(\n\n")
	if want := `grammar L: line 3, column 1: expected ')' to close '(' from line 1, column 2, found end of text`; err == nil || err.Error() != want {
		t.Errorf("%v\nwant %s", err, want)
	}
}

// The Go builder's Goal is the same as ~.
func TestGoalBuilder(t *testing.T) {
	g := New("G").Token("TOP", Goal(Lit("("), Lit(")"), Plus(Class(isWord))))
	if !g.Parse("(ab)").Defined() || g.Parse("(ab").Defined() {
		t.Error("Goal doesn't match as '(' ~ ')' \\w+")
	}
	_, err := g.ParseErr("(ab")
	if want := `grammar G: line 1, column 4: expected ')' to close '(' from line 1, column 1, found end of text`; err == nil || err.Error() != want {
		t.Errorf("%v\nwant %s", err, want)
	}
}

// A ~ missing its closer or its inner part says so, wherever the sequence
// ends: at a ], a ), a |, the end of the rule, or the > of a lookaround.
func TestGoalSyntaxErrors(t *testing.T) {
	for _, src := range []string{
		`grammar G { token TOP { '(' ~ ')' } }`,
		`grammar G { token TOP { [ '(' ~ ')' ] } }`,
		`grammar G { token TOP { '(' ~ ')' | x } }`,
		`grammar G { token TOP { <?before '(' ~ ')' > . } }`,
		`grammar G { token TOP { <?before '(' ~ > . } }`,
		`grammar G { token TOP { '(' ~ } }`,
	} {
		_, err := Compile(src)
		if err == nil || !strings.Contains(err.Error(), "~ needs") {
			t.Errorf("%s: %v, want a ~ error", src, err)
		}
	}
}

// Branches of a conjunction that match different text are reported where
// they part.
func TestParseErrConj(t *testing.T) {
	for _, c := range []struct{ src, text, want string }{
		{`grammar A { token TOP { [ \w+ & <[a..c]>+ ] } }`, "abcd", `grammar A: line 1, column 4: expected \w and <[a..c]> to match the same text, found "d"`},
		{`grammar B { token TOP { [ <:Lu> \w+ & <[A..Z a..z]>+ ] '!' } }`, "Zoë!", `grammar B: line 1, column 3: expected <:Lu> and <[A..Z a..z]> to match the same text, found "ë!"`},
		{`grammar C { token TOP { [ \w+ & \d+ ] } }`, "ab", `grammar C: line 1, column 1: expected \d, found "ab"`},
	} {
		_, err := MustCompile(c.src).ParseErr(c.text)
		if err == nil || err.Error() != c.want {
			t.Errorf("%s on %q: %v\nwant %s", c.src, c.text, err, c.want)
		}
	}
}

// payRange needs a whole range with its period, so most postings nearly
// match it somewhere.
var payRange = MustCompile(`
grammar Range {
    token TOP    { <min=money> <.sep> <max=money> <period> }
    token money  { '£' \s? <num> [:i k]? }
    token num    { \d ** 1..3 [ ',' \d ** 3 ]* }
    token sep    { \s* [ '-' | '–' | to ] \s* }
    token period { :i \s* [ pa | per \s+ [ annum | year ] ] }
}`)

func TestFindErr(t *testing.T) {
	for _, c := range []struct {
		g        *Grammar
		in, want string
		from     int
		opts     []Option
	}{
		// The attempts from "£45k" and "£55k" both stop at column 19; the
		// first got further past its start, so it's the one reported, not
		// both merged into "<period> or <sep>".
		{payRange, "Salary £45k - £55k, depending on experience",
			`grammar Range: line 1, column 19: expected <period>, found ", depending on exper" (trying from line 1, column 8)`, 7, nil},
		// Nowhere could a match start: the report is from the first position.
		{payRange, "Competitive salary",
			`grammar Range: line 1, column 1: expected <money>, found "Competitive salary" (trying from line 1, column 1)`, 0, nil},
		// Attempts that get equally far: the earlier is reported.
		{payRange, "£45 - x, £46 - y",
			`grammar Range: line 1, column 7: expected <money>, found "x, £46 - y" (trying from line 1, column 1)`, 0, nil},
		// Lines and columns count as ParseErr's do, for both places.
		{payRange, "Salary:\n£45k to\n£55k, DOE",
			`grammar Range: line 3, column 5: expected <period>, found ", DOE" (trying from line 2, column 1)`, 8, nil},
		// Attempts at the positions before " per" get nowhere, and an
		// attempt that starts later gets less far, so neither crowds out
		// the one that nearly matched.
		{MustCompile(`grammar P { token TOP { \s* per \s+ annum } }`), "45k per anum",
			`grammar P: line 1, column 9: expected 'annum', found "anum" (trying from line 1, column 4)`, 3, nil},
		// Pos is where the search starts, and past the end is reported there.
		{payRange, "£45k - £55k pa; £60k to £70k",
			`grammar Range: line 1, column 29: expected <period>, found end of text (trying from line 1, column 17)`, 16, []Option{Pos(15)}},
		{payRange, "£45k", `grammar Range: line 1, column 5: expected something else, found end of text (trying from line 1, column 5)`, 4, []Option{Pos(9)}},
	} {
		m, err := c.g.FindErr(c.in, c.opts...)
		var pe *ParseError
		if m.Defined() || !errors.As(err, &pe) {
			t.Errorf("%q: match %v, err %v", c.in, m, err)
			continue
		}
		if err.Error() != c.want || pe.From != c.from {
			t.Errorf("%q:\n got %v (from %d)\nwant %s (from %d)", c.in, err, pe.From, c.want, c.from)
		}
		all, err2 := c.g.FindAllErr(c.in, c.opts...)
		if all.Length() != 0 || err2 == nil || err2.Error() != err.Error() {
			t.Errorf("%q: FindAllErr gave %v, %v", c.in, all.Dump(), err2)
		}
	}
	// A match is returned as Find returns it, with no error, and ParseErr's
	// message doesn't say where it started.
	text := "Salary £45k - £55k pa, DOE"
	m, err := payRange.FindErr(text)
	if err != nil || m.Diff(payRange.Find(text)) != "" {
		t.Errorf("match: %v, %v", m, err)
	}
	all, err := payRange.FindAllErr(text + "; £60k to £70k per annum")
	if err != nil || all.Length() != 2 {
		t.Errorf("FindAllErr: %v, %v", all.Dump(), err)
	}
	if _, err := payRange.ParseErr(text); err == nil || strings.Contains(err.Error(), "trying") {
		t.Errorf("ParseErr: %v", err)
	}
}

// Running out of steps is an error from FindErr and FindAllErr, as from
// ParseErr, and a trace is printed once, by the search that found nothing.
func TestFindErrLimitsAndTrace(t *testing.T) {
	withStepLimit(t, 1000, func() {
		m, err := blowup.FindErr(nearMiss)
		var se *StepLimitError
		if m.Defined() || !errors.As(err, &se) {
			t.Errorf("FindErr: %v, %v", m, err)
		}
		all, err := blowup.FindAllErr(nearMiss)
		if all.Length() != 0 || !errors.As(err, &se) {
			t.Errorf("FindAllErr: %v, %v", all.Dump(), err)
		}
	})
	var b, b2 strings.Builder
	payRange.Find("£45k - x", Trace(&b))
	payRange.FindErr("£45k - x", Trace(&b2))
	if b.String() != b2.String() {
		t.Errorf("FindErr traced\n%s\nwant\n%s", b2.String(), b.String())
	}
}

// From is where the attempt reported started, for ParseErr as for FindErr,
// so Offset less From is always how far it got: Pos when it's given.
func TestParseErrFrom(t *testing.T) {
	for _, c := range []struct {
		text              string
		pos, from, offset int
		sub               bool
	}{
		{"Pay: £45k - x", 0, 0, 0, false},
		{"Pay: £45k - x", 5, 5, 12, false},
		{"Pay: £x", 5, 5, 6, true},
		{"Pay: £45k - x", 20, 13, 13, false}, // past the end: reported at the end
	} {
		parse := payErr.ParseErr
		if c.sub {
			parse = payErr.SubparseErr
		}
		_, err := parse(c.text, Pos(c.pos))
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("%q, Pos(%d): no ParseError: %v", c.text, c.pos, err)
			continue
		}
		if pe.From != c.from || pe.Offset != c.offset || strings.Contains(err.Error(), "trying") {
			t.Errorf("%q, Pos(%d): %v, From %d, Offset %d; want From %d, Offset %d", c.text, c.pos, err, pe.From, pe.Offset, c.from, c.offset)
		}
	}
}
