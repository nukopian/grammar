package grammar

import (
	"strings"
	"testing"
)

// A rule that recurses after a Re that matches no text in context is left
// recursion, and Check must say so: matching it would overflow the stack
// and kill the program. Each of these regexes matches nothing on an empty
// string, but matches empty text somewhere.
func TestCheckLeftRecursionThroughRe(t *testing.T) {
	for _, pattern := range []string{
		`(?=x)`, `(?!y)`, `(?<=a)`, `(?<!a)`, `\b`, `\B\b`, `x?\b`, `(?:\b|q)`,
		`(?=(x))`, `(?=[)])`, `(?=\))`, `\Gq?\b`,
	} {
		g := New("G").Token("TOP", Seq(Re(pattern), Call("TOP")))
		err := g.Check()
		if err == nil || !strings.Contains(err.Error(), "left-recursive") {
			t.Errorf("Re(%q) then <TOP>: Check() = %v, want left recursion", pattern, err)
		}
	}
}

// Regexes that always take text, assertions or not, are fine before a
// recursive call.
func TestCheckRightRecursionAfterRe(t *testing.T) {
	for _, pattern := range []string{
		`\w+\b`, `(?<=,)\s*x`, `[(?=]`, `\(?=`, `a(?=b)`, `[\b]`, `\^`,
	} {
		g := New("G").Token("TOP", Seq(Re(pattern), Opt(Call("TOP"))))
		if err := g.Check(); err != nil {
			t.Errorf("Re(%q) then <TOP>?: Check() = %v, want nil", pattern, err)
		}
	}
}

func TestStripAssertions(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`(?=x)`, ``},
		{`a(?!b)c`, `ac`},
		{`(?<=a(b)c)d`, `d`},
		{`(?<name>x)`, `(?<name>x)`}, // a named group, not a lookbehind
		{`(?=[)])x`, `x`},
		{`(?=\))x`, `x`},
		{`\bword\B`, `word`},
		{`^a$`, `a`},
		{`[\b^$]`, `[\b^$]`}, // inside a class, these are characters
		{`[]a]\b`, `[]a]`},
		{`[^]a]\b`, `[^]a]`},
		{`\\b`, `\\b`}, // an escaped backslash, then b
		{`\Ax\z\Z\G`, `x`},
	} {
		if got := stripAssertions(c.in); got != c.want {
			t.Errorf("stripAssertions(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// Parsing with such a grammar panics with Check's error, as any grammar
// built in Go that fails Check does, instead of overflowing the stack.
func TestParseLeftRecursionThroughRe(t *testing.T) {
	g := New("G").Token("TOP", Seq(Re(`(?=x)`), Call("TOP")))
	defer func() {
		if r, _ := recover().(string); !strings.Contains(r, "left-recursive") {
			t.Errorf("panicked with %q, want left recursion", r)
		}
	}()
	g.Parse("x")
}
