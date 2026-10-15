package grammar_test

import (
	"fmt"
	"unicode"

	"github.com/nukopian/job-meta/internal/scalar/grammar"
)

// Each example here builds a rule in Go and gives the same rule in Raku
// syntax in a comment. The two are interchangeable: Compile turns the Raku
// into these same nodes.

// Lit matches text exactly. In Raku syntax, letters and digits are literal
// as they stand, and anything else is quoted: '£', or "a\tb" for escapes.
func ExampleLit() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit)))
	// Raku: token TOP { '£' \d+ }

	fmt.Println(g.Subparse("£450 per week").At("~"))
	fmt.Println(g.Subparse("$450").Defined())
	// Output:
	// £450
	// false
}

// Class matches one character that a function accepts. Digit, Word, Space
// and AnyChar are the classes Raku writes \d, \w, \s and . (any character,
// newlines too).
func ExampleClass() {
	upper := grammar.Class(unicode.IsUpper)
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Plus(upper), grammar.Lit("-"), grammar.Plus(grammar.Digit)))
	// Raku, for ASCII capitals: token TOP { <[A..Z]>+ '-' \d+ }

	fmt.Println(g.Parse("REF-12").Defined(), g.Parse("ref-12").Defined())

	w := grammar.New("W").Token("TOP", grammar.Seq(grammar.Plus(grammar.Word), grammar.Space, grammar.AnyChar))
	// Raku: token TOP { \w+ \s . }
	fmt.Println(w.Parse("ab_1 !").Defined())
	// Output:
	// true false
	// true
}

// Re matches a Perl-style regex, the kind scalar's M takes, as one piece of
// a rule. It's for patterns you already have; whatever it matches first is
// final, as if it were a token. Raku syntax has no equivalent.
func ExampleRe() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Re(`[A-Z]{2,4}`), grammar.Lit("-"), grammar.Capture(grammar.Re(`\d+`))))
	fmt.Println(g.Parse("REF-1234").At("0.~"))
	// Output:
	// 1234
}

// Seq matches its parts one after another. In Raku syntax, that's just
// writing them in order.
func ExampleSeq() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Lit("per"), grammar.Plus(grammar.Space), grammar.Lit("hour")))
	// Raku: token TOP { per \s+ hour }

	fmt.Println(g.Parse("per   hour").Defined())
	// Output:
	// true
}

// Alt is Raku's |: of the alternatives that match, the one that matches the
// most text wins, whatever order they're written in. So hr, hour and hours
// can be listed in any order.
func ExampleAlt() {
	g := grammar.New("G").Token("TOP", grammar.Alt(grammar.Lit("hr"), grammar.Lit("hour"), grammar.Lit("hours")))
	// Raku: token TOP { [ hr | hour | hours ] }

	fmt.Println(g.Subparse("hours a week").At("~"))
	// Output:
	// hours
}

// First is Raku's ||: the first alternative that matches wins, even if a
// later one would match more. In a token, that choice is final, so here
// Parse fails on "hours": First chose "hour", leaving the "s" over.
func ExampleFirst() {
	g := grammar.New("G").Token("TOP", grammar.First(grammar.Lit("hour"), grammar.Lit("hours")))
	// Raku: token TOP { [ hour || hours ] }

	fmt.Println(g.Subparse("hours").At("~"))
	fmt.Println(g.Parse("hours").Defined())
	// Output:
	// hour
	// false
}

// Opt is ?: zero or one.
func ExampleOpt() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit), grammar.Opt(grammar.Lit("k"))))
	// Raku: token TOP { '£' \d+ k? }

	fmt.Println(g.Parse("£45").Defined(), g.Parse("£45k").Defined())
	// Output:
	// true true
}

// Star is *: zero or more, as many as possible.
func ExampleStar() {
	thousands := grammar.Seq(grammar.Lit(","), grammar.Repeat(grammar.Digit, 3, 3))
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Repeat(grammar.Digit, 1, 3), grammar.Star(thousands)))
	// Raku: token TOP { \d ** 1..3 [ ',' \d ** 3 ]* }

	fmt.Println(g.Parse("45").Defined(), g.Parse("45,000,000").Defined(), g.Parse("45,00").Defined())
	// Output:
	// true true false
}

// Plus is +: one or more, as many as possible.
func ExamplePlus() {
	g := grammar.New("G").Token("TOP", grammar.Plus(grammar.Digit))
	// Raku: token TOP { \d+ }

	fmt.Println(g.Subparse("2024 onwards").At("~"))
	fmt.Println(g.Parse("").Defined())
	// Output:
	// 2024
	// false
}

// Repeat is **: between min and max times, with -1 for no limit. In Raku,
// \d ** 4 is exactly four, \d ** 1..3 one to three, \d ** 2..* two or more.
func ExampleRepeat() {
	g := grammar.New("G").Token("TOP", grammar.Repeat(grammar.Digit, 1, 3))
	// Raku: token TOP { \d ** 1..3 }

	fmt.Println(g.Subparse("12345").At("~"))
	// Output:
	// 123
}

// Sep puts a separator between repeats, as Raku's % does: <tag>+ % ','
// is one or more tags with commas between. There mustn't be a comma at the
// end; SepTrailing allows one.
func ExampleQuant_Sep() {
	g := grammar.New("G").
		Token("TOP", grammar.Plus(grammar.Call("tag")).Sep(grammar.Seq(grammar.Lit(","), grammar.Star(grammar.Space)))).
		Token("tag", grammar.Plus(grammar.Word))
	// Raku: token TOP { <tag>+ % [ ',' \s* ] }; token tag { \w+ }

	fmt.Println(g.Parse("go, sql, aws").At("tag").AV().Length())
	fmt.Println(g.Parse("go, sql,").Defined())
	// Output:
	// 3
	// false
}

// SepTrailing is Raku's %%: separators between repeats, and optionally one
// at the end.
func ExampleQuant_SepTrailing() {
	g := grammar.New("G").
		Token("TOP", grammar.Plus(grammar.Call("tag")).SepTrailing(grammar.Lit(","))).
		Token("tag", grammar.Plus(grammar.Word))
	// Raku: token TOP { <tag>+ %% ',' }; token tag { \w+ }

	fmt.Println(g.Parse("go,sql,").At("tag").AV().Length())
	// Output:
	// 2
}

// Frugal makes a quantifier match as little as it can, as *? and +? do in
// Raku and Perl, taking more only when what follows fails. Here the frugal
// .*? stops at the first " - "; a greedy .* would run to the last.
//
// In a token, where nothing else gives back what it matched, a frugal
// quantifier still takes more when it needs to, so .*? is how a token skips
// ahead. That works through [ ... ] and $<name>=[ ... ], but a capture
// ( ... ) commits to its first match, as in Raku: (.*?) in a token matches
// nothing, and stays that way. Name the text with $<name>=[ .*? ] instead,
// or use a regex.
func ExampleQuant_Frugal() {
	g := grammar.New("G").Token("TOP", grammar.Seq(
		grammar.Named("role", grammar.Star(grammar.AnyChar).Frugal()),
		grammar.Lit(" - "),
		grammar.Named("rest", grammar.Star(grammar.AnyChar)),
	))
	// Raku: token TOP { $<role>=[.*?] ' - ' $<rest>=[.*] }

	m := g.Parse("Engineer - Payments - Remote")
	fmt.Printf("%q %q\n", m.At("role.~"), m.At("rest.~"))

	capture := grammar.New("G").Token("TOP", grammar.Seq(
		grammar.Capture(grammar.Star(grammar.AnyChar).Frugal()), grammar.Lit(" - "), grammar.Capture(grammar.Star(grammar.AnyChar))))
	// Raku: token TOP { (.*?) ' - ' (.*) }
	fmt.Println(capture.Parse("Engineer - Payments - Remote").Defined())
	// Output:
	// "Engineer" "Payments - Remote"
	// false
}

// Call matches another rule and captures its match under the rule's name:
// <num> in Raku syntax.
func ExampleCall() {
	g := grammar.New("G").
		Token("TOP", grammar.Seq(grammar.Call("num"), grammar.Lit("%"))).
		Token("num", grammar.Plus(grammar.Digit))
	// Raku: token TOP { <num> '%' }; token num { \d+ }

	fmt.Println(g.Parse("10%").At("num.~"))
	// Output:
	// 10
}

// Silent matches another rule without capturing it: <.name> in Raku
// syntax. Use it for rules that only hold the text together, such as
// separators and whitespace.
func ExampleSilent() {
	g := grammar.New("G").
		Token("TOP", grammar.Seq(grammar.Call("num"), grammar.Silent("ws"), grammar.Lit("days"))).
		Token("num", grammar.Plus(grammar.Digit))
	// Raku: token TOP { <num> <.ws> days }

	m := g.Parse("25 days")
	fmt.Println(m.At("num.~"), m.At("ws").Defined())
	// Output:
	// 25 false
}

// Alias captures a rule's match under another name too: <min=money> in
// Raku syntax. As in Raku, the match is under both names, so here it's
// under min and money. Two calls of money make money an array.
func ExampleAlias() {
	g := grammar.New("G").
		Token("TOP", grammar.Seq(grammar.Alias("min", "money"), grammar.Lit("-"), grammar.Alias("max", "money"))).
		Token("money", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit)))
	// Raku: token TOP { <min=money> '-' <max=money> }

	m := g.Parse("£30-£35")
	fmt.Println(m.At("min.~"), m.At("max.~"), m.At("money").AV().Length())
	// Output:
	// £30 £35 2
}

// AliasOnly captures a rule's match under another name only:
// <min=.money> in Raku syntax.
func ExampleAliasOnly() {
	g := grammar.New("G").
		Token("TOP", grammar.Seq(grammar.AliasOnly("min", "money"), grammar.Lit("+"))).
		Token("money", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit)))
	// Raku: token TOP { <min=.money> '+' }

	m := g.Parse("£700+")
	fmt.Println(m.At("min.~"), m.At("money").Defined())
	// Output:
	// £700 false
}

// Capture is ( ... ): a capture by position, numbered from 0 in the order
// the groups are written, and read with At("0"), At("1") and so on.
func ExampleCapture() {
	digits := func(n int) grammar.Node { return grammar.Repeat(grammar.Digit, n, n) }
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Capture(digits(4)), grammar.Lit("-"), grammar.Capture(digits(2))))
	// Raku: token TOP { (\d ** 4) '-' (\d ** 2) }

	m := g.Parse("2026-10")
	fmt.Println(m.At("0.~"), m.At("1.~"))
	// Output:
	// 2026 10
}

// Named captures the text of part of a rule under a name: $<year>=[ ... ]
// in Raku syntax. Named(name, Capture(...)), $<name>=( ... ), captures
// with a scope of its own, so positional captures inside it are its own.
func ExampleNamed() {
	digits := func(n int) grammar.Node { return grammar.Repeat(grammar.Digit, n, n) }
	g := grammar.New("G").Token("TOP", grammar.Seq(
		grammar.Named("year", digits(4)),
		grammar.Lit("-"),
		grammar.Named("rest", grammar.Capture(grammar.Seq(grammar.Capture(digits(2)), grammar.Lit("-"), grammar.Capture(digits(2))))),
	))
	// Raku: token TOP { $<year>=[ \d ** 4 ] '-' $<rest>=( (\d ** 2) '-' (\d ** 2) ) }

	m := g.Parse("2026-10-15")
	fmt.Println(m.At("year.~"), m.At("rest.~"), m.At("rest.0.~"), m.At("rest.1.~"))
	// Output:
	// 2026 10-15 10 15
}

// Before checks what comes next without matching it: <?before ...> in
// Raku syntax. Here the digits must be followed by k, but the k isn't part
// of the match.
func ExampleBefore() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Plus(grammar.Digit), grammar.Before(grammar.Lit("k"))))
	// Raku: token TOP { \d+ <?before k> }

	fmt.Println(g.Subparse("45k").At("~"))
	// Output:
	// 45
}

// NotBefore checks that something doesn't come next: <!before ...> in
// Raku syntax. Here it skips a percentage to find a plain number.
func ExampleNotBefore() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Plus(grammar.Digit), grammar.NotBefore(grammar.Lit("%"))))
	// Raku: token TOP { \d+ <!before '%'> }

	fmt.Println(g.Find("10% bonus and 25 days' holiday").At("~"))
	// Output:
	// 25
}

// After checks what came just before: <?after ...> in Raku syntax.
func ExampleAfter() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.After(grammar.Lit("£")), grammar.Plus(grammar.Digit)))
	// Raku: token TOP { <?after '£'> \d+ }

	fmt.Println(g.Find("Ref 123, pays £45").At("~"))
	// Output:
	// 45
}

// NotAfter checks that something didn't come just before: <!after ...> in
// Raku syntax. Here it finds a number that isn't money; WordStart stops it
// matching the 5 of £45.
func ExampleNotAfter() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.NotAfter(grammar.Lit("£")), grammar.WordStart, grammar.Plus(grammar.Digit)))
	// Raku: token TOP { <!after '£'> << \d+ }

	fmt.Println(g.Find("£45 for 3 days").At("~"))
	// Output:
	// 3
}

// IgnoreCase is :i: letters match in either case, for the part of the rule
// it covers. In Raku syntax, :i applies to the rest of the group it's in:
// [:i per annum] or, at the start of a rule, all of it.
func ExampleIgnoreCase() {
	g := grammar.New("G").Token("TOP", grammar.IgnoreCase(grammar.Lit("per annum")))
	// Raku: token TOP { :i 'per annum' }

	fmt.Println(g.Parse("PER ANNUM").Defined(), g.Parse("Per Annum").Defined())
	// Output:
	// true true
}

// Ratchet turns backtracking off (:r) or on (:!r) for part of a rule. A
// token is :r throughout, so Ratchet(..., false), :!r in Raku syntax, lets
// one part of a token backtrack, as a regex would.
func ExampleRatchet() {
	g := grammar.New("G").Token("TOP", grammar.Ratchet(grammar.Seq(grammar.Star(grammar.Digit), grammar.Lit("5")), false))
	// Raku: token TOP { :!r \d* 5 }

	fmt.Println(g.Parse("12345").Defined())
	// Output:
	// true
}

// Start and End match at the start and end of the text, as ^ and $ do in
// Raku. LineStart and LineEnd, ^^ and $$, match at the start and end of
// any line.
func ExampleStart() {
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Start, grammar.Plus(grammar.Digit)))
	// Raku: token TOP { ^ \d+ }

	fmt.Println(g.Find("12 days").Defined(), g.Find("Day 12").Defined())
	// Output:
	// true false
}

// LineStart is ^^: the start of any line. Here it finds the bulleted lines
// of a description, and only those.
func ExampleLineStart() {
	line := grammar.Plus(grammar.Class(func(r rune) bool { return r != '\n' }))
	g := grammar.New("G").Token("TOP", grammar.Seq(grammar.LineStart, grammar.Lit("- "), grammar.Capture(line)))
	// Raku: token TOP { ^^ '- ' (\N+) }

	text := "Benefits:\n- 25 days' holiday\n- Pension - 5%\nApply now"
	for _, m := range g.FindAll(text).All() {
		fmt.Println(m.At("0.~"))
	}
	// Output:
	// 25 days' holiday
	// Pension - 5%
}

// WordStart and WordEnd are << and >>: where a word starts and ends.
// Without them, "pa" is found inside "company".
func ExampleWordStart() {
	bare := grammar.New("G").Token("TOP", grammar.Lit("pa"))
	word := grammar.New("G").Token("TOP", grammar.Seq(grammar.WordStart, grammar.Lit("pa"), grammar.WordEnd))
	// Raku: token TOP { << pa >> }

	text := "a company that pays 30k pa"
	fmt.Println(bare.Find(text).At("from"), word.Find(text).At("from"))
	// Output:
	// 5 24
}
