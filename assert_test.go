package grammar

import (
	"errors"
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// ordered checks a pay range's minimum is at most its maximum.
func ordered(m *scalar.SV) bool { return m.At("min.~").Int() <= m.At("max.~").Int() }

// The builder's Assert and AssertNot match as <?{ }> and <!{ }> do.
func TestAssertBuilder(t *testing.T) {
	range_ := func(check Node) *Grammar {
		return New("Range").
			Token("TOP", Seq(Alias("min", "num"), Lit("-"), Alias("max", "num"), check)).
			Token("num", Plus(Class(isDigit)))
	}
	for _, c := range []struct {
		g    *Grammar
		in   string
		want bool
	}{
		{range_(Assert("ordered", ordered)), "5-12", true},
		{range_(Assert("ordered", ordered)), "12-5", false},
		{range_(AssertNot("ordered", ordered)), "5-12", false},
		{range_(AssertNot("ordered", ordered)), "12-5", true},
	} {
		if got := c.g.Parse(c.in).Defined(); got != c.want {
			t.Errorf("%q: matched %v, want %v", c.in, got, c.want)
		}
	}
}

// Code assertions' mistakes are compile errors that say what to do.
func TestAssertCompileErrors(t *testing.T) {
	check := Assertion("ordered", ordered)
	for _, c := range []struct{ src, want string }{
		{`grammar G { token TOP { <?{ missing($/) }> } }`,
			`line 1, column 25: there's no function missing for <?{ missing($/) }>: pass grammar.Assertion("missing", fn) to Compile`},
		{`grammar G { token TOP { <!{ $<min> <= $<max> }> } }`,
			`line 1, column 29: a code assertion here can only call a Go function with the match, as <?{ name($/) }>`},
		{`grammar G { token TOP { <?{ ordered() }> } }`, `column 29: a code assertion here can only call`},
		{`grammar G { token TOP { <?{ ordered($/); }> } }`, `column 29: a code assertion here can only call`},
		{`grammar G { token TOP { <?{ ordered($/) } } }`, `column 43: expected >`},
	} {
		_, err := Compile(c.src, check)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s:\n got %v\nwant %s", c.src, err, c.want)
		}
	}
	// Spaces are allowed where Raku allows them.
	if _, err := Compile(`grammar G { token TOP { <?{ordered( $/ )}> <!{  ordered($/)  }> } }`, check); err != nil {
		t.Error(err)
	}
}

// The match an assertion is given is read-only, so it can't change the
// match the parse returns, which shares its captures.
func TestAssertReadOnly(t *testing.T) {
	g := MustCompile(`grammar G { token TOP { <num> <?{ meddle($/) }> }; token num { \d+ } }`,
		Assertion("meddle", func(m *scalar.SV) (ok bool) {
			defer func() {
				if recover() == nil {
					t.Error("the match so far could be written")
				}
				ok = true
			}()
			m.At("num.~").Set("changed")
			return true
		}))
	// With actions and without, as the match so far is made differently:
	// a copy of what the result shares, or built for the assertion alone.
	acts := Actions(map[string]func(*scalar.SV) any{"num": func(m *scalar.SV) any { return 1 }})
	for _, opts := range [][]Option{nil, {acts}} {
		m := g.Parse("12", opts...)
		if m.At("num.~").Str() != "12" {
			t.Errorf("num is %q", m.At("num.~").Str())
		}
		// The result is the caller's to change, as ever.
		m.At("num").Set(scalar.Hash("mine", 1))
		if m.At("num.mine").Int() != 1 {
			t.Error("the result is read-only")
		}
	}
}

// An action runs once for each match, however often assertions read it,
// and the result has the value it made.
func TestAssertActionsOnce(t *testing.T) {
	calls := 0
	acts := map[string]func(*scalar.SV) any{"num": func(m *scalar.SV) any {
		calls++
		return m.At("~").Int() * 10
	}}
	g := MustCompile(`grammar G { token TOP { <num> <?{ a($/) }> <?{ a($/) }> '-' <?{ a($/) }> } token num { \d+ } }`,
		Assertion("a", func(m *scalar.SV) bool { return m.At("num.made").Int() == 120 }))
	m := g.Parse("12-", Actions(acts))
	if !m.Defined() || m.At("num.made").Int() != 120 || calls != 1 {
		t.Errorf("match %v, num.made %v, %d calls", m.Defined(), m.At("num.made"), calls)
	}
}

// A failed assertion is reported as the grammar writes it.
func TestAssertParseErr(t *testing.T) {
	g := MustCompile(`grammar Range { token TOP { <min=num> '-' <max=num> <?{ ordered($/) }> }; token num { \d+ } }`,
		Assertion("ordered", ordered))
	_, err := g.ParseErr("12-5")
	var pe *ParseError
	want := `grammar Range: line 1, column 5: expected <?{ ordered($/) }>, found end of text`
	if !errors.As(err, &pe) || err.Error() != want {
		t.Errorf("got %v\nwant %s", err, want)
	}
}

// BenchmarkAssert searches lines of the kind job postings state facts in
// with a pay grammar whose figures are pay only with a currency or a
// period, checked by a code assertion, as jdc-lab's is: most figures in
// such lines are something else, so the check is reached and fails often.
func BenchmarkAssert(b *testing.B) {
	g := MustCompile(`grammar Pay {
    token TOP    { <cur>? \s? <min=num> [ <.sep> <cur>? \s? <max=num> ]? <period>? <?{ pay($/) }> }
    token cur    { '£' | '$' | '€' }
    token num    { \d+ [ ',' \d ** 3 ]* [ '.' \d+ ]? }
    token sep    { \s* [ '-' | :i to ] \s* }
    token period { \s* :i [ 'p.a.' | per \s+ annum | per \s+ hour | ph | pa ] >> }
}`, Assertion("pay", func(m *scalar.SV) bool {
		return len(m.At("cur").AV().Items()) > 0 || m.At("period").Defined()
	}))
	text := strings.Repeat("Ref 225682916. Hours: 37.5 per week, 8 - 4, Mon-Fri. 2 - 3 years' experience. ", 20) +
		"Salary: £45,000 - £55,000 p.a. and 12.50 ph at weekends."
	for b.Loop() {
		if n := g.FindAll(text, Outermost()).Length(); n != 2 {
			b.Fatalf("%d matches", n)
		}
	}
}

// A rule or group marks where it started only for an assertion of its
// own: one inside a lookaround is the lookaround's, which marks itself, as
// is one inside a group or a called rule.
func TestAssertMarks(t *testing.T) {
	yes := Assertion("yes", func(*scalar.SV) bool { return true })
	g := MustCompile(`grammar G {
    token TOP  { <look> <group> <own> }
    token look { <?before <?{ yes($/) }>> \w }
    token group { ( \w <?{ yes($/) }> ) }
    token own  { \w <?{ yes($/) }> }
}`, yes)
	for rule, want := range map[string]bool{"TOP": false, "look": false, "group": false, "own": true} {
		if got := g.lookup(rule).code; got != want {
			t.Errorf("%s marks where it started: %v, want %v", rule, got, want)
		}
	}
	if !g.Parse("abc").Defined() {
		t.Error("no match")
	}
}
