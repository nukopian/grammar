package grammar

import (
	"strconv"
	"strings"
	"testing"
)

// traceOf parses text with g, traced, and returns the trace.
func traceOf(g *Grammar, text string, opts ...Option) string {
	var b strings.Builder
	g.Parse(text, append(opts, Trace(&b))...)
	return b.String()
}

// The traces are those Raku's Grammar::Tracer prints for the same grammars
// and texts (Rakudo 2025.11, without its colours), except where the
// comments say why not.
func TestTrace(t *testing.T) {
	cases := []struct {
		name, src, text, want string
		sub                   bool
	}{
		{"match", `grammar G {
			token TOP { <num> <.ws> <unit> }
			token num { \d+ }
			token unit { "hour" | "day" }
		}`, "12 hour", `TOP
|  num
|  * MATCH "12"
|  unit
|  * MATCH "hour"
* MATCH "12 hour"
`, false},
		{"fail", `grammar G {
			token TOP { <num> <.ws> <unit> }
			token num { \d+ }
			token unit { "hour" | "day" }
		}`, "12 week", `TOP
|  num
|  * MATCH "12"
|  unit
|  * FAIL
* FAIL
`, false},
		// Backtracking into a rule that has matched prints nothing more.
		{"backtrack into", `grammar B {
			regex TOP { <a> ab }
			regex a { a+ }
		}`, "aaab", `TOP
|  a
|  * MATCH "aaa"
* MATCH "aaab"
`, false},
		// Backtracking into a rule that tries again: the rules it calls are
		// traced at its own depth, as Raku's tracer, whose depth is back to
		// the caller's, traces them; found by the wash-up for v0.4.2.
		{"retry", `grammar B {
			regex TOP { <a> 'y' }
			regex a { <x> <y> || <x> }
			token x { 'x' }
			token y { 'y' }
		}`, "xy", `TOP
|  a
|  |  x
|  |  * MATCH "x"
|  |  y
|  |  * MATCH "y"
|  * MATCH "xy"
|  x
|  * MATCH "x"
* MATCH "xy"
`, false},
		// Built-in rules aren't traced; a grammar's own ws is.
		{"builtins", `grammar Bi { token TOP { <alpha> <digit> <alnum>+ } }`,
			"a1bc", "TOP\n* MATCH \"a1bc\"\n", false},
		{"own ws", `grammar W {
			rule TOP { <word>+ }
			token word { \w+ }
			token ws { \h* }
		}`, "ab cd", `TOP
|  word
|  * MATCH "ab"
|  word
|  * FAIL
|  ws
|  * MATCH " "
* MATCH "ab "
`, false},
		// Calls inside lookarounds are traced.
		{"lookarounds", `grammar L {
			token TOP { <?before <d>> <d> <!after x> }
			token d { \d }
		}`, "5", `TOP
|  d
|  * MATCH "5"
|  d
|  * MATCH "5"
* MATCH "5"
`, false},
		{"optional", `grammar S {
			token TOP { <x>? 'y' }
			token x { 'x' }
		}`, "y", `TOP
|  x
|  * FAIL
* MATCH "y"
`, true},
		// Raku's tracer, wrapping each rule, stops Raku deciding | by
		// longest token, so it tries <h>, matches "hour", and stops. Alt
		// matches each branch to find the longest, so both are traced.
		{"alternation", `grammar U {
			token TOP { <unit> }
			token unit { <h> | <hours> }
			token h { 'hour' }
			token hours { 'hours' }
		}`, "hours", `TOP
|  unit
|  |  h
|  |  * MATCH "hour"
|  |  hours
|  |  * MATCH "hours"
|  * MATCH "hours"
* MATCH "hours"
`, false},
		// Raku calls only the variant its longest-token matching picks; a
		// proto here matches every variant to find the longest.
		{"proto", `grammar P {
			token TOP { <amount> }
			proto token amount {*}
			token amount:sym<range> { <n> '-' <n> }
			token amount:sym<one> { <n> }
			token n { \d+ }
		}`, "10-12", `TOP
|  amount
|  |  amount:sym<range>
|  |  |  n
|  |  |  * MATCH "10"
|  |  |  n
|  |  |  * MATCH "12"
|  |  * MATCH "10-12"
|  |  amount:sym<one>
|  |  |  n
|  |  |  * MATCH "10"
|  |  * MATCH "10"
|  * MATCH "10-12"
* MATCH "10-12"
`, false},
	}
	for _, c := range cases {
		g := MustCompile(c.src)
		var b strings.Builder
		if c.sub {
			g.Subparse(c.text, Trace(&b))
		} else {
			g.Parse(c.text, Trace(&b))
		}
		if got := b.String(); got != c.want {
			t.Errorf("%s: got\n%s\nwant\n%s", c.name, got, c.want)
		}
	}
}

// The text is cut to 60 characters, less three for each level, and quoted
// as Raku quotes a string.
func TestTraceText(t *testing.T) {
	g := MustCompile(`grammar Q { token TOP { .+ } }`)
	got := traceOf(g, "a$b@c%d&e{f}g\\h\"i'j\tk\x01l\x7fm£n\u00a0o\u200bp")
	want := "TOP\n* MATCH \"a\\$b\\@c\\%d\\&e\\{f}g\\\\h\\\"i'j\\tk\\x[1]l\\x[7F]m£n\u00a0o\u200bp\"\n"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	long := strings.Repeat("a", 80)
	if got, want := traceOf(g, long), "TOP\n* MATCH \""+long[:60]+"\"\n"; got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	for _, c := range []struct{ in, want string }{
		{"\r", `"\r"`}, {"\x00", `"\0"`}, {"\x1b", `"\x[1B]"`}, {"\u0085", `"\x[85]"`},
		{"\u009f", `"\x[9F]"`}, {"\u00ad", "\"\u00ad\""}, {"\u0301a", `"\x[301]a"`},
		{"a\u0301", "\"a\u0301\""}, {"\a", `"\x[7]"`}, {"\b", `"\b"`}, {"\f", `"\x[C]"`},
		{"\v", `"\x[B]"`}, {"\n", `"\n"`},
	} {
		if got := rakuQuote([]rune(c.in)); got != c.want {
			t.Errorf("rakuQuote(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

// At depth 20 and past, there's no room for the text.
func TestTraceDepth(t *testing.T) {
	var src strings.Builder
	src.WriteString("grammar D { token TOP { <r0> }\n")
	for i := range 21 {
		if i == 20 {
			src.WriteString("token r20 { 'xxxxx' }\n")
		} else {
			src.WriteString("token r" + strconv.Itoa(i) + " { <r" + strconv.Itoa(i+1) + "> }\n")
		}
	}
	src.WriteString("}")
	got := strings.Split(traceOf(MustCompile(src.String()), "xxxxx"), "\n")
	pad := func(n int) string { return strings.Repeat("|  ", n) }
	for _, w := range []string{pad(21) + "* MATCH", pad(20) + "* MATCH", pad(19) + `* MATCH "xxx"`, `* MATCH "xxxxx"`} {
		found := false
		for _, l := range got {
			if l == w {
				found = true
			}
		}
		if !found {
			t.Errorf("no line %q in\n%s", w, strings.Join(got, "\n"))
		}
	}
}

// Find traces each position it tries, and none it can rule out.
func TestTraceFind(t *testing.T) {
	g := MustCompile(`grammar F { token TOP { <num> 'h' } token num { \d+ } }`)
	var b strings.Builder
	g.Find("a 12h", Trace(&b))
	want := `TOP
|  num
|  * MATCH "12"
* MATCH "12h"
`
	if b.String() != want {
		t.Errorf("got\n%s\nwant\n%s", b.String(), want)
	}
	b.Reset()
	g.FindAll("1 2h", Trace(&b))
	want = `TOP
|  num
|  * MATCH "1"
* FAIL
TOP
|  num
|  * MATCH "2"
* MATCH "2h"
`
	if b.String() != want {
		t.Errorf("got\n%s\nwant\n%s", b.String(), want)
	}
}

// ParseErr traces too, and tracing changes neither its result nor its
// error.
func TestTraceParseErr(t *testing.T) {
	g := MustCompile(`grammar E { token TOP { <num> 'h' } token num { \d+ } }`)
	var b strings.Builder
	m, err := g.ParseErr("12x", Trace(&b))
	m2, err2 := g.ParseErr("12x")
	if m.Defined() || err == nil || err.Error() != err2.Error() || m2.Defined() {
		t.Errorf("traced: %v, %v; untraced: %v, %v", m, err, m2, err2)
	}
	if want := "TOP\n|  num\n|  * MATCH \"12\"\n* FAIL\n"; b.String() != want {
		t.Errorf("got\n%s\nwant\n%s", b.String(), want)
	}
}
