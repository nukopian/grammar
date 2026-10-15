package grammar_test

import (
	"fmt"

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
