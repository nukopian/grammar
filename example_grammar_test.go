package grammar_test

import (
	"fmt"

	"github.com/nukopian/job-meta/internal/scalar"
	"github.com/nukopian/job-meta/internal/scalar/grammar"
)

// A grammar is a set of named rules. Parsing starts at TOP, and a rule
// calls another by writing its name in angle brackets. Here TOP calls
// <word> and <num>, and the match has one of each.
//
// Inside a rule, spaces and # comments mean nothing; letters and digits
// match themselves, and anything else must be quoted, as '-' is.
func ExampleCompile() {
	g, err := grammar.Compile(`
grammar Code {
    token TOP  { <word> '-' <num> }   # such as REF-1234
    token word { <[A..Z]>+ }          # one or more capital letters
    token num  { \d+ }
}`)
	if err != nil {
		fmt.Println(err)
		return
	}
	m := g.Parse("REF-1234")
	fmt.Println(m.At("word.~"), m.At("num.~"))

	// Errors say where.
	_, err = grammar.Compile(`grammar Bad { token TOP { a - b } }`)
	fmt.Println(err)
	// Output:
	// REF 1234
	// grammar: line 1, column 29: '-' must be quoted, as '-', or escaped, as \-
}

// One source can declare several grammars and roles; Compile returns the
// last. Later declarations can use earlier ones, here with is.
func ExampleCompile_several() {
	g, _ := grammar.Compile(`
grammar Base  { token TOP { <greeting> ', ' \w+ }; token greeting { hello } }
grammar Polite is Base { token greeting { good \s+ morning } }
`)
	fmt.Println(g.Name(), g.Parse("good morning, Ada").At("greeting.~"))
	// Output:
	// Polite good morning
}

// MustCompile is for grammars written into the program. As a package-level
// variable, a grammar with a mistake in it stops the program as it starts,
// rather than when it's first used.
func ExampleMustCompile() {
	// var Hours = grammar.MustCompile(`...`) at package level, usually.
	hours := grammar.MustCompile(`grammar Hours { token TOP { \d+ [ '.' \d+ ]? \s* hours } }`)
	fmt.Println(hours.Find("Working 37.5 hours a week").At("~"))
	// Output:
	// 37.5 hours
}

// New starts a grammar in Go, for building rules from data or code rather
// than writing them in Raku syntax. Token and Regex add rules to it.
func ExampleNew() {
	hours := grammar.New("Hours").
		Token("TOP", grammar.Seq(grammar.Call("num"), grammar.Opt(grammar.Space),
			grammar.IgnoreCase(grammar.Alt(grammar.Lit("hours"), grammar.Lit("hrs"), grammar.Lit("h"))))).
		Token("num", grammar.Seq(grammar.Plus(grammar.Digit), grammar.Opt(grammar.Seq(grammar.Lit("."), grammar.Plus(grammar.Digit)))))

	fmt.Println(hours.Find("Working 37.5 HRS per week").At("num.~"))
	// Output:
	// 37.5
}

// A token doesn't backtrack: each part keeps the first match it finds and
// never gives any of it back. Here \d* takes all the digits, 5 included,
// so the 5 after it has nothing left to match, and the token fails, even on
// "5" alone. Written as \d* <?after 5>, or [ \d* ] <?after 5>, it would
// work; or use Regex.
//
// That sounds like a limitation, but it's what you want almost always: it
// makes a grammar's behaviour easy to predict, and fast. Use token unless
// you need Regex.
func ExampleGrammar_Token() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Star(grammar.Digit), grammar.Lit("5")))
	// The same in Raku syntax: token TOP { \d* 5 }

	fmt.Println(g.Parse("12345").Defined())
	fmt.Println(g.Parse("5").Defined())
	// Output:
	// false
	// false
}

// A regex backtracks, as a Perl regex does: when what follows fails, \d*
// gives back a digit and tries again, so \d* 5 matches "12345".
func ExampleGrammar_Regex() {
	g := grammar.New("G").Regex("TOP", grammar.Seq(grammar.Star(grammar.Digit), grammar.Lit("5")))
	// The same in Raku syntax: regex TOP { \d* 5 }

	fmt.Println(g.Parse("12345").Defined())
	// Output:
	// true
}

// An extension inherits its parent's rules and can replace any of them. A
// replaced rule is used everywhere, even where the parent's own rules call
// it: here Pay's TOP and money call <amount> and <period> as before, but
// PayUS's money is the one they get.
func ExampleGrammar_Extend() {
	us := Pay.Extend("PayUS").Token("money", grammar.Seq(grammar.Lit("$"), grammar.Call("amount")))
	fmt.Println(us.Subparse("$60k - $70k per year").At("max.~"))

	// The same, in Raku syntax.
	us2 := grammar.MustCompile(`grammar PayUS is Pay { token money { '$' <amount> } }`, Pay)
	fmt.Println(us2.Subparse("$60k - $70k per year").At("max.~"))
	// Output:
	// $70k
	// $70k
}

// Extend with more grammars inherits from them all, as is A is B does.
// Rules are looked up in the new grammar, then its parents in the order
// given: here Money's and Period's rules work together, and PayUK
// overrides Period's <unit>, which Period's own <period> then calls.
func ExampleGrammar_Extend_several() {
	g := grammar.MustCompile(`
grammar Money  { token money  { '£' <num> }; token num { \d+ } }
grammar Period { token period { ' per ' <unit> }; token unit { year } }
grammar PayUK is Money is Period {
    token TOP  { <money> <period> }
    token unit { hour | day }
}`)
	fmt.Println(g.Parse("£18 per hour").At("period.unit.~"), g.Parse("£18 per year").Defined())

	// The same, in Go.
	money := grammar.New("Money").Token("money", grammar.Seq(grammar.Lit("£"), grammar.Call("num"))).
		Token("num", grammar.Plus(grammar.Digit))
	period := grammar.New("Period").Token("period", grammar.Seq(grammar.Lit(" per "), grammar.Call("unit"))).
		Token("unit", grammar.Lit("year"))
	payUK := money.Extend("PayUK", period).
		Token("TOP", grammar.Seq(grammar.Call("money"), grammar.Call("period"))).
		Token("unit", grammar.Alt(grammar.Lit("hour"), grammar.Lit("day")))
	fmt.Println(payUK.Parse("£500 per day").At("money.num.~"))
	// Output:
	// hour false
	// 500
}

// A proto is a family of rules that grows: each variant, named
// period:sym<…>, is one way to write a period, and an extension adds its
// own without changing the grammar it extends. A call to <period> tries
// them all, the extension's and its parents', and keeps the longest. Here
// PayUK adds weekly and daily rates to Pay's, and replaces Pay's hourly
// one, which now also takes "ph".
func ExampleGrammar_Proto() {
	g := grammar.MustCompile(`
grammar Pay {
    token TOP    { '£' $<amount>=[ \d+ ] \s* <period> }
    proto token period {*}
    token period:sym<annum> { :i [ per \s+ annum | 'p.a.' | pa ] }
    token period:sym<hour>  { :i per \s+ hour }
}
grammar PayUK is Pay {
    token period:sym<week> { :i [ per \s+ week | pw ] }
    token period:sym<day>  { :i per \s+ day }
    token period:sym<hour> { :i [ per \s+ hour | ph ] }
}`)
	acts := map[string]func(*scalar.SV) any{
		"period:sym<annum>": func(*scalar.SV) any { return "Year" },
		"period:sym<week>":  func(*scalar.SV) any { return "Week" },
		"period:sym<day>":   func(*scalar.SV) any { return "Day" },
		"period:sym<hour>":  func(*scalar.SV) any { return "Hour" },
	}
	for _, text := range []string{"£45000 per annum", "£450 pw", "£18 ph", "£150 per day"} {
		fmt.Println(g.Parse(text, grammar.Actions(acts)).At("period.made"))
	}
	// Output:
	// Year
	// Week
	// Hour
	// Day
}

// A role is a library of rules to share between grammars. Grammars take a
// role's rules as their own with does. A role's rules can call rules it
// doesn't have, here <amount>, for each grammar that uses it to provide.
func ExampleNewRole() {
	money := grammar.NewRole("Money").
		Token("money", grammar.Seq(grammar.Lit("£"), grammar.Call("amount")))

	whole := grammar.New("Whole").Does(money).
		Token("TOP", grammar.Call("money")).
		Token("amount", grammar.Plus(grammar.Digit))
	pence := grammar.New("Pence").Does(money).
		Token("TOP", grammar.Call("money")).
		Token("amount", grammar.Seq(grammar.Plus(grammar.Digit), grammar.Lit("."), grammar.Repeat(grammar.Digit, 2, 2)))

	fmt.Println(whole.Parse("£12").Defined(), whole.Parse("£12.50").Defined())
	fmt.Println(pence.Parse("£12").Defined(), pence.Parse("£12.50").Defined())
	// Output:
	// true false
	// false true
}

// Does composes roles into a grammar. The grammar's own rules win over the
// role's, wherever Does is called. Two roles with a rule of the same name
// are an error, unless the grammar defines that rule itself to choose.
//
// In Raku syntax: role Money { ... } and grammar G does Money { ... }.
func ExampleGrammar_Does() {
	g := grammar.MustCompile(`
role Units { token unit { hours | hrs } }
role Extra { token unit { h } }
grammar Hours does Units does Extra {
    token TOP  { \d+ \s* <unit> }
    token unit { hours | hrs | h }    # ours: needed, as both roles have one
}`)
	fmt.Println(g.Parse("8 h").At("unit.~"))

	_, err := grammar.Compile(`
role Units { token unit { hours } }
role Extra { token unit { h } }
grammar Hours does Units does Extra { token TOP { \d+ <unit> } }`)
	fmt.Println(err)
	// Output:
	// h
	// grammar: line 4, column 65: rule unit is in both Units and Extra; define it in Hours to choose
}

// Check reports a call to a rule that doesn't exist. Compile checks, but a
// grammar built in Go is only checked when it's first used, and then a
// missing rule panics, so call Check after building one from data.
func ExampleGrammar_Check() {
	g := grammar.New("G").Token("TOP", grammar.Call("missing"))
	fmt.Println(g.Check())
	// Output:
	// grammar G: rule TOP calls <missing>, which doesn't exist
}

func ExampleGrammar_Name() {
	fmt.Println(Pay.Name())
	// Output:
	// Pay
}

func ExampleGrammar_IsRole() {
	fmt.Println(Pay.IsRole(), grammar.NewRole("Money").IsRole())
	// Output:
	// false true
}

// In a rule, the spaces in the rule match spaces in the text: each becomes a
// call to <.ws>, which matches any amount of whitespace, so the rule reads
// like the text it matches. Here "up to £500 per day" matches with any
// spacing, but "upto" doesn't, as <.ws> won't match nothing between two
// words. See the README's "Spaces in rules" for exactly where a space counts.
func ExampleCompile_rule() {
	g := grammar.MustCompile(`
grammar Rate {
    rule  TOP   { up to <money> per <unit> }
    token money { '£' \d+ }
    token unit  { day | hour | week }
}`)
	for _, text := range []string{"up to £500 per day", "up  to £500   per day", "upto £500 per day"} {
		fmt.Println(g.Parse(text).At("money.~").Or("no match"))
	}
	// Output:
	// £500
	// £500
	// no match
}

// Unicode properties pick characters by what Unicode says they are, so a
// rule works in any language: <:Lu> is any capital, where <[A..Z]> is only
// the 26. Here, capitalised words in names from three scripts.
func Example_unicodeProperties() {
	g := grammar.MustCompile(`grammar Names { token TOP { <:Lu> <:Ll>+ } }`)
	ascii := grammar.MustCompile(`grammar ASCII { token TOP { <[A..Z]> <[a..z]>+ } }`)

	text := "Zoë Ölander, Жанна Петрова, 李 Wei"
	fmt.Println(g.FindAll(text).Map(func(m *scalar.SV) any { return m.At("~") }).Join(" | "))
	fmt.Println(ascii.FindAll(text).Map(func(m *scalar.SV) any { return m.At("~") }).Join(" | "))
	// Output:
	// Zoë | Ölander | Жанна | Петрова | Wei
	// Zo | Wei
}
