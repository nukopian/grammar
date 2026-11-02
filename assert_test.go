package grammar

import (
	"errors"
	"fmt"
	"slices"
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

// An assertion that names the captures it reads is given those, with the
// match's text and position: one that hasn't matched is undef, or an empty
// array where it can capture more than once, and what's below them reads
// as ever. With actions as well, and in the builder's Assert.
func TestAssertReads(t *testing.T) {
	src := `grammar Pay {
    token TOP { <cur>? <min=num> [ '-' <cur>? <max=num> ]? <period>? <?{ pay($/) }> }
    token cur { '£' }
    token num { \d+ }
    token period { ' ph' }
}`
	var seen []string
	pay := func(m *scalar.SV) bool {
		seen = append(seen, fmt.Sprintf("%s %d-%d cur %d period %v %v",
			m.At("~").Str(), m.At("from").Int(), m.At("to").Int(),
			len(m.At("cur").AV().Items()), m.At("period").Defined(), m.At("period.missing").Defined()))
		return len(m.At("cur").AV().Items()) > 0 || m.At("period").Defined()
	}
	acts := Actions(map[string]func(*scalar.SV) any{"num": func(m *scalar.SV) any { return 1 }})
	var want []string
	for _, opts := range [][]Option{nil, {acts}} {
		for _, reads := range [][]string{nil, {"cur", "period"}} {
			seen = nil
			g := MustCompile(src, Assertion("pay", pay, reads...))
			all := g.FindAll("5-12 and £5-£12, 12 ph", opts...)
			if all.Length() != 2 {
				t.Errorf("reads %v: %d matches", reads, all.Length())
			}
			if want == nil {
				want = seen
			} else if !slices.Equal(seen, want) {
				t.Errorf("reads %v, actions %v: the assertion saw\n%s\nwant\n%s", reads, opts != nil,
					strings.Join(seen, "\n"), strings.Join(want, "\n"))
			}
		}
	}
	b := New("Range").
		Token("TOP", Seq(Alias("min", "num"), Lit("-"), Alias("max", "num"), Assert("ordered", ordered, "min", "max"))).
		Token("num", Plus(Class(isDigit)))
	if !b.Parse("5-12").Defined() || b.Parse("12-5").Defined() {
		t.Error("Assert with reads")
	}
}

// The match so far has no made value of its own, as its rule hasn't
// finished, and reads it as undef whether the assertion names its reads
// or not: made isn't a capture to name.
func TestAssertReadsMade(t *testing.T) {
	for _, reads := range [][]string{nil, {"n"}} {
		g := MustCompile(`grammar G { token TOP { <n=num> <?{ unmade($/) }> }; token num { \d+ } }`,
			Assertion("unmade", func(m *scalar.SV) bool { return !m.At("made").Defined() }, reads...))
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("reads %v: %v", reads, r)
				}
			}()
			if !g.Parse("12").Defined() {
				t.Errorf("reads %v: no match", reads)
			}
		}()
	}
}

// Reading a capture an assertion doesn't name panics, saying which
// assertion and what it names, and so does writing to the match.
func TestAssertReadsUnnamed(t *testing.T) {
	for _, c := range []struct {
		fn   func(*scalar.SV) bool
		want string
	}{
		{func(m *scalar.SV) bool { return m.At("max").Defined() },
			"grammar: ordered($/) reads a capture it doesn't name (it names min): scalar: Attempt to access disallowed key 'max' in a restricted hash"},
		{func(m *scalar.SV) bool { m.At("min").Set(1); return true }, "Modification of a read-only value attempted"},
	} {
		g := MustCompile(`grammar Range { token TOP { <min=num> '-' <max=num> <?{ ordered($/) }> }; token num { \d+ } }`,
			Assertion("ordered", c.fn, "min"))
		func() {
			defer func() {
				if r := fmt.Sprint(recover()); !strings.Contains(r, c.want) {
					t.Errorf("panicked with %s\nwant %s", r, c.want)
				}
			}()
			g.Parse("5-12")
		}()
	}
}

// Naming the captures an assertion reads changes no match or failure
// report: checked over the Raku cases, with each named and with none.
func TestNamedReadsChangeNothing(t *testing.T) {
	compared := 0
	for _, c := range rakuCases {
		if c.Reads == nil {
			continue
		}
		g, err := Compile(c.Src, c.uses()...)
		if err != nil {
			t.Fatal(err)
		}
		for _, in := range c.Inputs {
			for _, text := range []string{in, "xx " + in + " ;" + in} {
				with := results(g, text)
				var without []string
				withoutNamedReads(func() { without = results(g, text) })
				if !slices.Equal(with, without) {
					t.Errorf("%s on %q: naming reads changed\n%s\nto\n%s", c.Src, text,
						strings.Join(without, "\n"), strings.Join(with, "\n"))
				}
				compared++
			}
		}
	}
	if compared == 0 {
		t.Fatal("nothing compared")
	}
}

// withoutNamedReads runs f building every capture for every assertion,
// and turns naming back on, even if f panics.
func withoutNamedReads(f func()) {
	defer func() { namedReads = true }()
	namedReads = false
	f()
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
func BenchmarkAssert(b *testing.B) { benchmarkAssert(b) }

// BenchmarkAssertReads is BenchmarkAssert with the assertion naming the
// captures it reads.
func BenchmarkAssertReads(b *testing.B) { benchmarkAssert(b, "cur", "period") }

func benchmarkAssert(b *testing.B, reads ...string) {
	g := MustCompile(`grammar Pay {
    token TOP    { <cur>? \s? <min=num> [ <.sep> <cur>? \s? <max=num> ]? <period>? <?{ pay($/) }> }
    token cur    { '£' | '$' | '€' }
    token num    { \d+ [ ',' \d ** 3 ]* [ '.' \d+ ]? }
    token sep    { \s* [ '-' | :i to ] \s* }
    token period { \s* :i [ 'p.a.' | per \s+ annum | per \s+ hour | ph | pa ] >> }
}`, Assertion("pay", func(m *scalar.SV) bool {
		return len(m.At("cur").AV().Items()) > 0 || m.At("period").Defined()
	}, reads...))
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
