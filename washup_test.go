package grammar

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nukopian/scalar"
)

// compileErr compiles src, reporting a panic as an error too, so a test can
// tell a crash from a syntax error.
func compileErr(src string) (panicked any, err error) {
	defer func() { panicked = recover() }()
	_, err = Compile(src)
	return nil, err
}

// Left recursion is an error, not a fatal stack overflow: Compile reports
// it, and a grammar built in Go panics with a message on first use.
func TestLeftRecursion(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`grammar L1 { token TOP { <TOP>? a } }`, "TOP → TOP"},
		{`grammar L2 { token TOP { <x> }; token x { <ws> <y> }; token y { <TOP> b } }`, "TOP → x → y → TOP"},
		{`grammar L3 { token TOP { <x>* <TOP> }; token x { a } }`, "TOP → TOP"},
		{`grammar L4 { token TOP { <?before <TOP>> a } }`, "TOP → TOP"},
		{`grammar L5 { token TOP { [ a | <TOP> ] b } }`, "TOP → TOP"},
	} {
		p, err := compileErr(c.src)
		if p != nil || err == nil || !strings.Contains(err.Error(), "left-recursive") || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err %v, panic %v; want left-recursive %s", c.src, err, p, c.want)
		}
	}
	// Recursion after some text is fine.
	for _, src := range []string{
		`grammar R1 { token TOP { a <TOP>? } }`,
		`grammar R2 { token TOP { '(' <TOP>* ')' } }`,
		`grammar R3 { token TOP { <x> <TOP>? }; token x { a } }`,
	} {
		if p, err := compileErr(src); err != nil || p != nil {
			t.Errorf("%s: err %v, panic %v", src, err, p)
		}
	}
	if !MustCompile(`grammar R4 { token TOP { '(' <TOP>* ')' } }`).Parse("(()(()))").Defined() {
		t.Error("nested brackets didn't parse")
	}
}

// A grammar built in Go that recurses on the left panics with a message,
// which can be recovered, rather than overflowing the stack. Run in a child
// process, as a regression would kill the test binary.
func TestLeftRecursionBuilt(t *testing.T) {
	if os.Getenv("GRAMMAR_CHILD") == "1" {
		defer func() { fmt.Println("recovered:", recover()) }()
		New("L").Token("TOP", Seq(Opt(Call("TOP")), Lit("a"))).Parse("aaa")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLeftRecursionBuilt$", "-test.v")
	cmd.Env = append(os.Environ(), "GRAMMAR_CHILD=1")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "recovered: grammar L: rule TOP is left-recursive") {
		t.Fatalf("child: %v\n%.2000s", err, out)
	}
}

// Malformed sources are errors, never panics.
func TestCompileNeverPanics(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`grammar G { token TOP { <[a..`, "unterminated character class"},
		{`grammar G { token TOP { <[a..\`, "unterminated"},
		{`grammar G { token TOP { $<from>=\d+ } }`, `"from" is reserved`},
		{`grammar G { token TOP { (a) $<to>=b } }`, `"to" is reserved`},
		{`grammar G { token TOP { a ** 99999999999999999999 } }`, "too large"},
		{`grammar G { token TOP { a ** 3..1 } }`, "empty range"},
	} {
		p, err := compileErr(c.src)
		if p != nil || err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err %v, panic %v; want %q", c.src, err, p, c.want)
		}
	}
}

// One Capture node used twice in a rule captures twice.
func TestCaptureReused(t *testing.T) {
	d := Capture(Plus(Digit))
	g := New("G").Token("TOP", Seq(d, Lit("-"), d))
	m := g.Parse("12-34")
	if m.At("0.~").Str() != "12" || m.At("1.~").Str() != "34" {
		t.Errorf("got 0=%v 1=%v", m.At("0.~"), m.At("1.~"))
	}
	// Inside another capture's scope too.
	outer := New("G").Token("TOP", Capture(Seq(d, Lit("-"), d)))
	m = outer.Parse("12-34")
	if m.At("0.0.~").Str() != "12" || m.At("0.1.~").Str() != "34" {
		t.Errorf("nested: got %v %v", m.At("0.0.~"), m.At("0.1.~"))
	}
}

type ptrActs struct{ calls int }

func (a *ptrActs) Amount(m *scalar.SV) any { a.calls++; return "ptr" }

// Actions with pointer receivers work whether the value or a pointer is
// passed.
func TestActionsPointerReceivers(t *testing.T) {
	if got := pay.Parse("£5k", Actions(ptrActs{})).At("min.amount.made").Str(); got != "ptr" {
		t.Errorf("by value: %q", got)
	}
	a := &ptrActs{}
	if got := pay.Parse("£5k", Actions(a)).At("min.amount.made").Str(); got != "ptr" || a.calls != 1 {
		t.Errorf("by pointer: %q, %d calls", got, a.calls)
	}
	if !pay.Parse("£5k", Actions(nil)).Defined() {
		t.Error("nil actions")
	}
}

// An adverb part-way through a group covers the rest of it, later
// alternatives included, as in Raku.
func TestAdverbScope(t *testing.T) {
	g := MustCompile(`grammar A { token TOP { [ a :i b | c ] } }`)
	if !g.Parse("C").Defined() || !g.Parse("aB").Defined() || g.Parse("Ab").Defined() {
		t.Errorf("C %v, aB %v, Ab %v", g.Parse("C").Defined(), g.Parse("aB").Defined(), g.Parse("Ab").Defined())
	}
}

// Nested alternations in a token take time in proportion to their size,
// not doubling with each level.
func TestNestedAlternations(t *testing.T) {
	src := "x"
	for range 24 {
		src = "[ " + src + " | y ]"
	}
	g := MustCompile(`grammar A { token TOP { ` + src + ` } }`)
	start := time.Now()
	ok := g.Parse("x").Defined()
	if d := time.Since(start); !ok || d > time.Second {
		t.Errorf("matched %v in %v", ok, d)
	}
}

// Check's result is reused between parses, but not once the grammar, or a
// grammar it inherits from, changes.
func TestCheckCache(t *testing.T) {
	base := New("B").Token("TOP", Call("x")).Token("x", Lit("a"))
	ext := base.Extend("E")
	if !ext.Parse("a").Defined() {
		t.Fatal("didn't parse")
	}
	base.Token("x", Call("missing")) // changes what ext inherits
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "missing") {
			t.Errorf("recovered %v", r)
		}
	}()
	ext.Parse("a")
}

func TestReTimeout(t *testing.T) {
	if got := Re(`a`).(re).rx.MatchTimeout; got != scalar.RegexTimeout {
		t.Errorf("MatchTimeout %v, want scalar.RegexTimeout %v", got, scalar.RegexTimeout)
	}
}
