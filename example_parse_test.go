package grammar_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nukopian/job-meta/internal/scalar"
	"github.com/nukopian/job-meta/internal/scalar/grammar"
)

// Parse matches the whole text, or nothing: a match that would leave text
// over is no match. When it fails, the result is undef, so Defined, Or and
// the like work on it, and reading through it gives undef too.
func ExampleGrammar_Parse() {
	fmt.Println(Pay.Parse("£18.50 per hour").At("min.amount.num.~"))
	fmt.Println(Pay.Parse("£18.50 per hour, negotiable").Defined()) // not all of it
	fmt.Println(Pay.Parse("negotiable").At("min.~").Or("none"))
	// Output:
	// 18.50
	// false
	// none
}

// Subparse matches from the start of the text, without needing to reach
// its end.
func ExampleGrammar_Subparse() {
	m := Pay.Subparse("£18.50 per hour, negotiable")
	fmt.Println(m.At("~"))
	fmt.Println(Pay.Subparse("Pay: £18.50").Defined()) // it must start at the start
	// Output:
	// £18.50 per hour
	// false
}

// Find hunts for the first place in the text where the grammar matches,
// which is how to pull a fact out of free text.
func ExampleGrammar_Find() {
	m := Pay.Find("The role pays £30k to £35k depending on experience.")
	fmt.Println(m.At("~"), "at", m.At("from"))
	// Output:
	// £30k to £35k at 14
}

// FindAll finds every match, left to right, without overlapping.
func ExampleGrammar_FindAll() {
	for _, m := range Pay.FindAll("£30k basic, £5k car allowance, £2k bonus").All() {
		fmt.Println(m.At("~"))
	}
	// Output:
	// £30k
	// £5k
	// £2k
}

// Rule starts the match at another rule instead of TOP, which is handy for
// testing one rule on its own, or for reusing part of a grammar.
func ExampleRule() {
	fmt.Println(Pay.Parse("12,500", grammar.Rule("num")).At("~"))
	fmt.Println(Pay.Parse("12,500").Defined()) // TOP wants a £
	// Output:
	// 12,500
	// false
}

// Actions turns matches into values. For each rule that matched, the method
// named after it is called with its match, and what it returns is stored in
// the match under "made". Actions run innermost first, so here Amount has
// run for each <amount> before TOP's action reads their made values.
func ExampleActions() {
	m := Pay.Parse("£45k - £55,000", grammar.Actions(PayActions{}))
	fmt.Println(m.At("min.amount.made"), m.At("max.amount.made"))
	// Output:
	// 45000 55000
}

// A map of functions, keyed by rule name, works as well as a type with
// methods, and suits actions built at run time.
func ExampleActions_map() {
	m := Pay.Parse("£45k - £55k per annum", grammar.Actions(map[string]func(*scalar.SV) any{
		"amount": PayActions{}.Amount,
		"period": func(m *scalar.SV) any { return strings.TrimSpace(m.At("~").Str()) },
	}))
	fmt.Println(m.At("max.amount.made"), m.At("period.made"))
	// Output:
	// 55000 per annum
}

// A match is ordinary scalar data, so once TOP's action has built the value
// you want, scalar.To makes it typed Go.
func ExampleActions_to() {
	type Range struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
	}
	m := Pay.Parse("£45k - £55k", grammar.Actions(map[string]func(*scalar.SV) any{
		"amount": PayActions{}.Amount,
		"TOP": func(m *scalar.SV) any {
			min := m.At("min.amount.made")
			return scalar.Hash("min", min, "max", m.At("max.amount.made").Or(min))
		},
	}))
	r, err := scalar.To[Range](m.At("made"))
	fmt.Printf("%+v %v\n", r, err)
	// Output:
	// {Min:45000 Max:55000} <nil>
}

// How captures are shaped. A capture that can match more than once, because
// it's quantified (*, +, **) or its name appears twice, is always an array,
// even when it matched once or not at all. Any other capture is a single
// match, or missing when it didn't match. The grammar decides this, not the
// text, so code reading a match never has to check which it got.
//
// ( ... ) captures by position, as "0", "1" and so on; $<name>=[ ... ]
// captures the text it matched under a name.
func Example_captureShapes() {
	g := grammar.MustCompile(`
grammar Shapes {
    token TOP  { <tag>* <.ws> $<code>=[ \d+ ] ( '!' )? }
    token tag  { '#' \w+ <.ws> }
}`)
	for _, text := range []string{"#a #b 42!", "42"} {
		m := g.Parse(text)
		fmt.Println(m.At("tag").AV().Length(), "tags;",
			"code", m.At("code.~"), ";",
			"bang", m.At("0.~").Or("none"))
	}
	// Output:
	// 2 tags; code 42 ; bang !
	// 0 tags; code 42 ; bang none
}

// Values reduces a match to the values in it: each match becomes its made
// value if an action made one, a hash of its captures if it has any, and its
// text otherwise. That's usually the shape you want to read, or to pass to
// scalar.To.
func ExampleValues() {
	g := grammar.MustCompile(`
grammar Person {
    token TOP   { <name> ',' <.ws> <born> }
    token name  { <first> ' ' <last> }
    token first { \w+ }
    token last  { \w+ }
    token born  { \d ** 4 }
}`)
	out, _ := grammar.Values(g.Parse("Ada Lovelace, 1815")).ToYAML()
	fmt.Print(string(out))
	// Output:
	// born: "1815"
	// name:
	//   first: Ada
	//   last: Lovelace
}

// To fills a struct from a match, by way of Values: capture names match
// field names, ignoring case, as in encoding/json, and a json tag can name
// one that doesn't, such as a positional capture or a rule called
// pay-range. A capture that didn't match leaves its field at its zero value.
// If the text didn't match at all, To returns ErrNoMatch.
func ExampleTo() {
	type Range struct {
		Min    float64 `json:"min"`
		Max    float64 `json:"max"`
		Period string
	}
	acts := grammar.Actions(map[string]func(*scalar.SV) any{
		"money":  func(m *scalar.SV) any { return m.At("amount.made") }, // a money is its amount
		"amount": PayActions{}.Amount,
		"period": func(m *scalar.SV) any { return strings.TrimSpace(m.At("~").Str()) },
	})

	r, err := grammar.To[Range](Pay.Parse("£45k - £55,000 per annum", acts))
	fmt.Printf("%+v %v\n", r, err)

	r, err = grammar.To[Range](Pay.Parse("£18.50", acts))
	fmt.Printf("%+v %v\n", r, err)

	_, err = grammar.To[Range](Pay.Parse("negotiable", acts))
	fmt.Println(err, errors.Is(err, grammar.ErrNoMatch))
	// Output:
	// {Min:45000 Max:55000 Period:per annum} <nil>
	// {Min:18.5 Max:0 Period:} <nil>
	// grammar: no match true
}

// ParseErr says why a parse failed: where in the text the grammar got
// furthest, and what it expected there. A rule that couldn't start there is
// named, <money> here; one that got part of the way is reported by what it
// needed next. That's usually exactly where the mistake is, as everything
// before it matched.
func ExampleGrammar_ParseErr() {
	for _, text := range []string{
		"£45k - £55,000 per annum",
		"£45k - 55,000",       // the second figure has no £
		"£45k - £55,00",       // a thousands group cut short
		"£45k - £55k pa, DOE", // more after the pay
	} {
		m, err := Pay.ParseErr(text)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Println("ok:", m.At("~"))
	}
	// Output:
	// ok: £45k - £55,000 per annum
	// grammar Pay: line 1, column 8: expected <money>, found "55,000"
	// grammar Pay: line 1, column 14: expected \d, found end of text
	// grammar Pay: line 1, column 15: expected end of text, found ", DOE"
}

// SubparseErr is ParseErr for Subparse: the text must match from its
// start, but needn't match to its end.
func ExampleGrammar_SubparseErr() {
	_, err := Pay.SubparseErr("Salary: £45k")
	fmt.Println(err)
	// Output:
	// grammar Pay: line 1, column 1: expected <money>, found "Salary: £45k"
}

// A ParseError's fields give the same facts as its message, for a program
// to use: to log which rule postings fail at, say, or to underline the
// place in an editor.
func ExampleParseError_Error() {
	_, err := Pay.ParseErr("£45k -\n55k")
	var pe *grammar.ParseError
	if errors.As(err, &pe) {
		fmt.Println(pe.Line, pe.Column, pe.Expected, pe.Found)
		fmt.Println(pe)
	}
	// Output:
	// 2 1 [<money>] 55k
	// grammar Pay: line 2, column 1: expected <money>, found "55k"
}

// StepLimit stops a match that backtracks badly. Nested repeats in a regex
// rule, as here, try every way of sharing the a's out before giving up, so
// each extra a doubles the time: 30 of them would take minutes. The
// default limit stops it in about a second; this one is lower.
func ExampleStepLimit() {
	g := grammar.MustCompile(`grammar Blowup { regex TOP { [ a+ ]+ $ } }`)
	grammar.StepLimit = 100_000
	defer func() { grammar.StepLimit = 10_000_000 }()

	_, err := g.ParseErr(strings.Repeat("a", 30) + "b")
	fmt.Println(err)
	// Output:
	// grammar Blowup: gave up after 100000 steps (see StepLimit)
}

// Parse, Find and the rest panic with a *StepLimitError, as scalar's regexes
// panic past scalar.RegexTimeout. A program that reads texts it can't trust
// can recover it, and go on to the next text.
func ExampleStepLimitError_Error() {
	g := grammar.MustCompile(`grammar Blowup { regex TOP { [ a+ ]+ $ } }`)
	grammar.StepLimit = 100_000
	defer func() { grammar.StepLimit = 10_000_000 }()

	find := func(text string) (m *scalar.SV, err error) {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(*grammar.StepLimitError)
				if !ok {
					panic(r) // some other problem
				}
				m, err = &scalar.SV{}, e
			}
		}()
		return g.Find(text), nil
	}
	for _, text := range []string{"aaa", strings.Repeat("a", 30) + "b"} {
		m, err := find(text)
		fmt.Printf("%q %v\n", m.At("~"), err)
	}
	// Output:
	// "aaa" <nil>
	// "" grammar Blowup: gave up after 100000 steps (see StepLimit)
}

// Split gives the text between matches with the matches in place, as Raku's
// split with :v does: here, a paragraph split at headings run into it.
func ExampleGrammar_Split() {
	g := grammar.MustCompile(`
grammar Inline {
    token TOP { <?after '.' \s+> [ 'What you will do' | 'How to apply' ] }
}`)
	for _, piece := range g.Split("We're hiring. What you will do Lead a team. How to apply Email us.").All() {
		fmt.Printf("%q\n", piece)
	}
	// Output:
	// "We're hiring. "
	// "What you will do"
	// " Lead a team. "
	// "How to apply"
	// " Email us."
}
