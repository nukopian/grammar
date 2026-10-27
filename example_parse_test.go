package grammar_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nukopian/grammar"
	"github.com/nukopian/scalar"
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

// When Find finds nothing, its undef can be read through, giving undef,
// and stays undef: reading it doesn't make it a match, as in Raku.
func ExampleGrammar_Find_noMatch() {
	m := Pay.Find("Salary negotiable")
	fmt.Println(m.At("min.~").Or("none"))
	fmt.Println(m.Defined())
	// Output:
	// none
	// false
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

// Pos starts part-way into the text, as Raku's :pos does, with the text
// before it still there to look back at, and positions still the whole
// text's.
func ExamplePos() {
	m := Pay.Subparse("Salary: £45k", grammar.Pos(8))
	fmt.Println(m.At("~"), m.At("from"))
	fmt.Println(Pay.Subparse("Salary: £45k").Defined()) // from the start, there's no £
	// Output:
	// £45k 8
	// false
}

// Overlap finds a match from each place one starts, as Raku's :ov does, so
// a heading is weighed by every phrase in it, even one inside another.
func ExampleOverlap() {
	sections := grammar.NewWordList("sections", grammar.FoldCase(), grammar.WholeWords()).
		AddAs("responsibilities", "in this role you will", "duties").
		AddAs("requirements", "you will need", "skills")
	g := grammar.MustCompile(`grammar Heading { token TOP { @sections } }`, sections)
	heading := "In this role you will need"
	made := func(m *scalar.SV) any { return m.At("made") }
	fmt.Println(g.FindAll(heading).Map(made).Join(", "))
	fmt.Println(g.FindAll(heading, grammar.Overlap()).Map(made).Join(", "))
	// Output:
	// responsibilities
	// responsibilities, requirements
}

// Outermost is Overlap without the matches inside another, so "profile"
// can't outweigh the "company profile" it's part of, while "you will need"
// still counts beside "in this role you will".
func ExampleOutermost() {
	sections := grammar.NewWordList("sections", grammar.FoldCase(), grammar.WholeWords()).
		AddAs("responsibilities", "in this role you will").
		AddAs("requirements", "you will need", "profile").
		AddAs("company", "company profile")
	g := grammar.MustCompile(`grammar Heading { token TOP { @sections } }`, sections)
	made := func(m *scalar.SV) any { return m.At("made") }
	for _, heading := range []string{"Company Profile", "In this role you will need"} {
		fmt.Printf("%s: %s; %s\n", heading,
			g.FindAll(heading, grammar.Overlap()).Map(made).Join(", "),
			g.FindAll(heading, grammar.Outermost()).Map(made).Join(", "))
	}
	// Output:
	// Company Profile: company, requirements; company
	// In this role you will need: responsibilities, requirements; responsibilities, requirements
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

// ToAll is To for FindAll: each match becomes a T, in order, here each
// amount of money in a posting as the number its action made.
func ExampleToAll() {
	acts := grammar.Actions(map[string]func(*scalar.SV) any{
		"money":  func(m *scalar.SV) any { return m.At("amount.made") },
		"amount": PayActions{}.Amount,
	})
	posting := "£30k basic, £5k car allowance, £2.5k bonus"
	amounts, err := grammar.ToAll[float64](Pay.FindAll(posting, grammar.Rule("money"), acts))
	fmt.Println(amounts, err)
	// Output:
	// [30000 5000 2500] <nil>
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

// CheckPath says whether a path can ever be defined in a match. A misspelt
// capture reads undef, as one that didn't match does, so a typo fails
// silently; CheckPath names the step that can't be there, and what could.
// It's for tests: check each path the program reads.
func ExampleGrammar_CheckPath() {
	for _, path := range []string{"min.amount.num.~", "period.~", "min.amont", "money.amount", "max.amount.k.made"} {
		fmt.Println(Pay.CheckPath(path))
	}
	// Output:
	// <nil>
	// <nil>
	// grammar Pay: path "min.amont": rule money has no capture "amont" (it has: amount)
	// grammar Pay: path "money.amount": money is a list of matches, as it can capture more than once: index it, as money[0]
	// grammar Pay: path "max.amount.k.made": max.amount.k never has a made value: only a rule's match, or a word list's, has one
}

// Matching recurses as it goes, a level or so for each item matched, and
// past DepthLimit it stops with a *DepthLimitError, which can be
// recovered, rather than running out of stack, which can't: Go would end
// the program. ParseErr returns it. Here the limit is low, to show it.
func ExampleDepthLimitError_Error() {
	g := grammar.MustCompile(`grammar Long { token TOP { <[a..z]>* } }`)
	defer func(old int) { grammar.DepthLimit = old }(grammar.DepthLimit)
	grammar.DepthLimit = 1_000

	_, err := g.ParseErr(strings.Repeat("a", 600))
	fmt.Println(err)
	// Output:
	// grammar Long: match too deep: gave up after 1000 levels, in TOP from line 1, column 1 (see DepthLimit)
}

// Backref is $<name>: the text a capture matched, again. Here a pay range
// must use the same thousands separator in both figures, so a range that
// mixes them, likely two figures run together, isn't read as one.
func ExampleBackref() {
	g := grammar.MustCompile(`
grammar SameSep {
    token TOP { \d+ $<sep>=<[,.]> \d ** 3 \s* '-' \s* \d+ $<sep> \d ** 3 }
}`)
	for _, text := range []string{"45,000 - 55,000", "45,000 - 55.000"} {
		fmt.Printf("%-16q %v\n", text, g.Parse(text).Defined())
	}
	// Output:
	// "45,000 - 55,000" true
	// "45,000 - 55.000" false
}

// Caps gives a match's captures in the order they come in the text, as
// Raku's .caps does. A match is a hash, which keeps no order between names;
// here, the qualifiers of a title are captured by kind, and Caps puts them
// back in the order the title has them.
func ExampleCaps() {
	g := grammar.MustCompile(`
grammar Title {
    token TOP         { $<title>=[ <-[(]>+? ] ' (' [ <arrangement> | <contract> ]+ % ', ' ')' }
    token arrangement { :i remote | hybrid | onsite }
    token contract    { :i contract | permanent | temp }
}`)
	m := g.Parse("Senior Engineer (Contract, Remote, Temp)")
	for name, c := range grammar.Caps(m) {
		fmt.Printf("%-11s %s\n", name, c.At("~"))
	}
	// Output:
	// title       Senior Engineer
	// contract    Contract
	// arrangement Remote
	// contract    Temp
}

// Chunks is Caps with the text between the captures, under "~", so the
// chunks put the text back together: here, to mark up the parts of a pay
// string.
func ExampleChunks() {
	m := Pay.Parse("£45k - £55k per annum")
	var b strings.Builder
	for name, c := range grammar.Chunks(m) {
		switch name {
		case "~":
			b.WriteString(c.Str())
		case "money":
			// money is min and max again, under a second name
		default:
			fmt.Fprintf(&b, "[%s: %s]", name, c.At("~"))
		}
	}
	fmt.Println(b.String())
	// Output:
	// [min: £45k] - [max: £55k][period:  per annum]
}

// StepLimit stops a match that backtracks badly. Nested repeats in a regex
// rule, as in amount here, try every way of sharing the digits out before
// giving up, so each extra digit doubles the time: 30 of them would take
// minutes. The default limit stops it in about a second; this one is
// lower. ParseErr's error names the rule that was matching, and where its
// match started.
func ExampleStepLimit() {
	g := grammar.MustCompile(`
grammar Pay {
    token TOP { 'Salary: ' <amount> }
    regex amount { [ \d+ ]+ ' per annum' }
}`)
	defer func(old int) { grammar.StepLimit = old }(grammar.StepLimit)
	grammar.StepLimit = 100_000

	_, err := g.ParseErr("Salary: " + strings.Repeat("9", 30) + " pa")
	fmt.Println(err)
	// Output:
	// grammar Pay: match too long: gave up after 100000 steps, in amount from line 1, column 9 (see StepLimit)
}

// Parse, Find and the rest panic with a *StepLimitError, as scalar's regexes
// panic past scalar.RegexTimeout. A program that reads texts it can't trust
// can recover it, and go on to the next text.
func ExampleStepLimitError_Error() {
	g := grammar.MustCompile(`grammar Blowup { regex TOP { [ a+ ]+ $ } }`)
	defer func(old int) { grammar.StepLimit = old }(grammar.StepLimit)
	grammar.StepLimit = 100_000

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
	// "" grammar Blowup: match too long: gave up after 100000 steps (see StepLimit)
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
