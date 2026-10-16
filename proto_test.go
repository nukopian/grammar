package grammar

import (
	"strings"
	"testing"

	"github.com/nukopian/job-meta/internal/scalar"
)

// Proto and variants built in Go, with Sym for <sym>.
func TestProtoBuilder(t *testing.T) {
	g := New("Pay").
		Token("TOP", Call("period")).
		Proto("period").
		Token("period:sym<annum>", IgnoreCase(Lit("per annum"))).
		Token("period:sym<hour>", Seq(Lit("per "), Sym()))
	m := g.Parse("per hour")
	if got := m.At("period.sym.~").Str(); got != "hour" {
		t.Errorf("period.sym = %q, want hour", got)
	}
	if !g.Parse("PER ANNUM").Defined() || g.Parse("per week").Defined() {
		t.Error("variants don't match as declared")
	}
	ext := g.Extend("PayUK").Token("period:sym<pw>", Lit("pw"))
	if !ext.Parse("pw").Defined() || !ext.Parse("per hour").Defined() {
		t.Error("an extension's variants aren't added to its parent's")
	}
}

// The variant's action runs, not the proto's: as a method named after the
// proto and the sym, PeriodSymAnnum, or by its full name in a map.
func TestProtoActions(t *testing.T) {
	g := MustCompile(`grammar A {
    token TOP { <period> }
    proto token period {*}
    token period:sym<annum> { 'per annum' }
    token period:sym<hour>  { 'per hour' }
    token period:sym<+>     { '+' }
}`)
	m := g.Parse("per annum", Actions(protoActions{}))
	if got := m.At("period.made").Str(); got != "Year" {
		t.Errorf("method: made = %q, want Year", got)
	}
	if m := g.Parse("per hour", Actions(protoActions{})); m.At("period.made").Defined() {
		t.Errorf("the proto's action ran: made = %v", m.At("period.made"))
	}
	acts := map[string]func(*scalar.SV) any{
		"period:sym<hour>": func(*scalar.SV) any { return "Hour" },
		"period:sym<+>":    func(*scalar.SV) any { return "Plus" },
	}
	if got := g.Parse("per hour", Actions(acts)).At("period.made").Str(); got != "Hour" {
		t.Errorf("map: made = %q, want Hour", got)
	}
	if got := g.Parse("+", Actions(acts)).At("period.made").Str(); got != "Plus" {
		t.Errorf("map, symbol sym: made = %q, want Plus", got)
	}
}

type protoActions struct{}

func (protoActions) PeriodSymAnnum(*scalar.SV) any { return "Year" }
func (protoActions) Period(*scalar.SV) any         { return "proto" }

// Mistakes are errors when the grammar compiles.
func TestProtoErrors(t *testing.T) {
	for src, want := range map[string]string{
		`grammar A { token TOP { <sym> } }`:                                               "<sym>",
		`grammar B { proto token x { a } }`:                                               "{*}",
		`grammar C { token TOP { <y> }; token y:sym<a> { a } }`:                           "<y>",
		`grammar D { token x:sym<a> { a }; token x:sym<a> { b } }`:                        "twice",
		`grammar E { proto token x {*}; token TOP { <x> }; token x { a } }`:               "declared twice",
		`grammar F { token TOP { <x:sym<z>> }; proto token x {*}; token x:sym<a> { a } }`: "calls the proto, not the variant",
	} {
		_, err := Compile(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", src, err, want)
		}
	}
}

// Find skips positions where no variant can start, and left recursion
// through a variant is caught.
func TestProtoAnalysis(t *testing.T) {
	g := MustCompile(`grammar F { token TOP { <u> }; proto token u {*}; token u:sym<y> { year }; token u:sym<w> { week } }`)
	if got := g.FindAll("a year or a week").Map(func(m *scalar.SV) any { return m.At("~") }).Join(","); got != "year,week" {
		t.Errorf("FindAll = %q", got)
	}
	_, err := Compile(`grammar L { token TOP { <x> }; proto token x {*}; token x:sym<a> { <x> a } }`)
	if err == nil || !strings.Contains(err.Error(), "left-recursive") {
		t.Errorf("left recursion through a variant: %v", err)
	}
}

// Of variants matching the same length, the first declared wins: this
// package's stand-in for Raku's longest-token rules, as for |.
func TestProtoTie(t *testing.T) {
	g := MustCompile(`grammar T { token TOP { <x> }; proto token x {*}
    token x:sym<one> { $<one>=[ a ] b }
    token x:sym<two> { a $<two>=[ . ] } }`)
	if m := g.Parse("ab"); !m.At("x.one").Defined() || m.At("x.two").Defined() {
		t.Errorf("x = %v, want the first declared, x:sym<one>", m.At("x"))
	}
}
