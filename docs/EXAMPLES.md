# grammar: examples

Every function and method in [grammar](../README.md), each with a working
example and the output it prints. For how the features fit together, see
the [reference](REFERENCE.md); for tasks, see the [how-to guide](HOWTO.md);
coming from Raku, see the [phrasebook](RAKU.md).

One example for every function and method, with the output it prints. The
examples are a guide to grammars as well as to the API: read them in order.
<!-- reference:start -->

Generated from the examples in `example_*_test.go`, which `go test` runs, so every
example here works and prints what it says. Edit the examples, not this section, then run
`go test . -run TestReference -update`.

- [A worked example](#reference-a-worked-example)
- [Writing grammars](#reference-writing-grammars)
- [Parsing and matches](#reference-parsing-and-matches)
- [Building rules in Go](#reference-building-rules-in-go)
- [Word lists](#reference-word-lists)

### Reference: A worked example

#### Overview

The whole idea in one example: find pay in a sentence, and read the figures the actions made from it.

```go
text := "Salary: £45k – £55,000 per annum, plus bonus."

m := Pay.Find(text, grammar.Actions(PayActions{}))
fmt.Println(m.At("~")) // the text of the whole match
fmt.Println(m.At("min.amount.made"), m.At("max.amount.made"))
fmt.Println(m.At("period.~"))
fmt.Println(m.At("from"), m.At("to")) // where in text it was
```

```
£45k – £55,000 per annum
45000 55000
 per annum
8 32
```

#### Match tree

A whole match, printed as YAML to show its shape. Every match is a hash with its text under "~" and its position under "from" and "to". Each \<rule> it called is a match of its own, under the rule's name, so the tree follows the grammar. \<.sep> has a dot, so it isn't captured.

\<low=n> captures under both low and n, as in Raku, and as n is captured twice, n is an array. low and n's first element are the same match, not copies, as in Raku; YAML shows that with an anchor, &2, and a reference back to it, \*2.

```go
g := grammar.MustCompile(`
grammar Range {
    token TOP { <low=n> <.sep> <high=n> }
    token n   { \d+ }
    token sep { \s* '-' \s* }
}`)
out, _ := g.Parse("18 - 21").ToYAML()
fmt.Print(string(out))
```

```
from: 0
high: &1
  from: 5
  to: 7
  "~": "21"
low: &2
  from: 0
  to: 2
  "~": "18"
"n":
  - *2
  - *1
to: 7
"~": 18 - 21
```

#### Titles

The workflow this package is for: synonym lists, such as you'd pull from a synonym store, a grammar that says where each kind of word goes, and grammar.To to hand back typed Go. Every spelling of a title, a seniority or a working arrangement comes out as its canonical value.

```go
seniority := grammar.NewWordList("seniority", grammar.FoldCase())
titles := grammar.NewWordList("titles", grammar.FoldCase(), grammar.LooseSpacing())
arrangements := grammar.NewWordList("arrangements", grammar.FoldCase(), grammar.LooseSpacing())
for l, rules := range map[*grammar.WordList]string{
	seniority:    "Senior, Snr, Sr\nLead\nJunior, Jnr, Jr",
	titles:       "Software Engineer, Software Developer, SWE, Developer\nProduct Manager, PM",
	arrangements: "Remote, WFH, home based\nHybrid\nOnsite, on-site, office based",
} {
	if err := l.AddRules(rules); err != nil {
		panic(err)
	}
}

g := grammar.MustCompile(`
grammar Title {
    token TOP         { [ <seniority> \s+ ]? <title> [ \s* <.dash> \s* <team> ]? [ \s* '(' <arrangement> ')' ]? }
    token seniority   { @seniority }
    token title       { @titles }
    token arrangement { @arrangements }
    token dash        { '-' | '–' | '|' }
    token team        { \w+ [ ' ' \w+ ]* }
}`, seniority, titles, arrangements)

// Capture names fill the fields of the same names, ignoring case: each
// word list's rule gives its canonical value, and team its text.
type Title struct {
	Seniority, Title, Team, Arrangement string
}
for _, text := range []string{
	"Snr SWE – Payments (WFH)",
	"product manager | Growth (office based)",
	"Lead Software Developer",
} {
	t, err := grammar.To[Title](g.Parse(text))
	fmt.Printf("%+v %v\n", t, err)
}
```

```
{Seniority:Senior Title:Software Engineer Team:Payments Arrangement:Remote} <nil>
{Seniority: Title:Product Manager Team:Growth Arrangement:Onsite} <nil>
{Seniority:Lead Title:Software Engineer Team: Arrangement:} <nil>
```

### Reference: Writing grammars

#### `Compile`

Compile compiles grammars and roles written in Raku syntax, and returns the last one declared:

A grammar is a set of named rules. Parsing starts at TOP, and a rule calls another by writing its name in angle brackets. Here TOP calls \<word> and \<num>, and the match has one of each.

Inside a rule, spaces and \# comments mean nothing; letters and digits match themselves, and anything else must be quoted, as '-' is.

```go
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
```

```
REF 1234
grammar: line 1, column 29: '-' must be quoted, as '-', or escaped, as \-
```

#### `Compile.several`

One source can declare several grammars and roles; Compile returns the last. Later declarations can use earlier ones, here with is.

```go
g, _ := grammar.Compile(`
grammar Base  { token TOP { <greeting> ', ' \w+ }; token greeting { hello } }
grammar Polite is Base { token greeting { good \s+ morning } }
`)
fmt.Println(g.Name(), g.Parse("good morning, Ada").At("greeting.~"))
```

```
Polite good morning
```

#### `MustCompile`

MustCompile is Compile, panicking if src doesn't compile.

MustCompile is for grammars written into the program. As a package-level variable, a grammar with a mistake in it stops the program as it starts, rather than when it's first used.

```go
// var Hours = grammar.MustCompile(`...`) at package level, usually.
hours := grammar.MustCompile(`grammar Hours { token TOP { \d+ [ '.' \d+ ]? \s* hours } }`)
fmt.Println(hours.Find("Working 37.5 hours a week").At("~"))
```

```
37.5 hours
```

#### `New`

New returns an empty grammar called name.

New starts a grammar in Go, for building rules from data or code rather than writing them in Raku syntax. Token and Regex add rules to it.

```go
hours := grammar.New("Hours").
	Token("TOP", grammar.Seq(grammar.Call("num"), grammar.Opt(grammar.Space),
		grammar.IgnoreCase(grammar.Alt(grammar.Lit("hours"), grammar.Lit("hrs"), grammar.Lit("h"))))).
	Token("num", grammar.Seq(grammar.Plus(grammar.Digit), grammar.Opt(grammar.Seq(grammar.Lit("."), grammar.Plus(grammar.Digit)))))

fmt.Println(hours.Find("Working 37.5 HRS per week").At("num.~"))
```

```
37.5
```

#### `Grammar.Token`

Token adds a rule that doesn't backtrack, as Raku's token does.

A token doesn't backtrack: each part keeps the first match it finds and never gives any of it back. Here \\d\* takes all the digits, 5 included, so the 5 after it has nothing left to match, and the token fails, even on "5" alone. Written as \\d\* \<?after 5>, or \[ \\d\* ] \<?after 5>, it would work; or use Regex.

That sounds like a limitation, but it's what you want almost always: it makes a grammar's behaviour easy to predict, and fast. Use token unless you need Regex.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Star(grammar.Digit), grammar.Lit("5")))
// The same in Raku syntax: token TOP { \d* 5 }

fmt.Println(g.Parse("12345").Defined())
fmt.Println(g.Parse("5").Defined())
```

```
false
false
```

#### `Grammar.Regex`

Regex adds a rule that backtracks, as Raku's regex does.

A regex backtracks, as a Perl regex does: when what follows fails, \\d\* gives back a digit and tries again, so \\d\* 5 matches "12345".

```go
g := grammar.New("G").Regex("TOP", grammar.Seq(grammar.Star(grammar.Digit), grammar.Lit("5")))
// The same in Raku syntax: regex TOP { \d* 5 }

fmt.Println(g.Parse("12345").Defined())
```

```
true
```

#### `Grammar.Extend`

Extend returns an empty grammar called name that inherits g's rules, as grammar name is g { ...

An extension inherits its parent's rules and can replace any of them. A replaced rule is used everywhere, even where the parent's own rules call it: here Pay's TOP and money call \<amount> and \<period> as before, but PayUS's money is the one they get.

```go
us := Pay.Extend("PayUS").Token("money", grammar.Seq(grammar.Lit("$"), grammar.Call("amount")))
fmt.Println(us.Subparse("$60k - $70k per year").At("max.~"))

// The same, in Raku syntax.
us2 := grammar.MustCompile(`grammar PayUS is Pay { token money { '$' <amount> } }`, Pay)
fmt.Println(us2.Subparse("$60k - $70k per year").At("max.~"))
```

```
$70k
$70k
```

#### `Grammar.Extend_several`

Extend with more grammars inherits from them all, as is A is B does. Rules are looked up in the new grammar, then its parents in the order given: here Money's and Period's rules work together, and PayUK overrides Period's \<unit>, which Period's own \<period> then calls.

```go
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
```

```
hour false
500
```

#### `Grammar.Proto`

Proto declares name a proto, as proto token name {*} does: a family of rules, the variants, each named name:sym<…> and added with Token or Regex, which extensions and roles can add to without changing the grammar that declares it.

A proto is a family of rules that grows: each variant, named period:sym\<…>, is one way to write a period, and an extension adds its own without changing the grammar it extends. A call to \<period> tries them all, the extension's and its parents', and keeps the longest. Here PayUK adds weekly and daily rates to Pay's, and replaces Pay's hourly one, which now also takes "ph".

```go
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
```

```
Year
Week
Hour
Day
```

#### `NewRole`

NewRole returns an empty role called name: a library of rules to compose into grammars with Does, as Raku's role is.

A role is a library of rules to share between grammars. Grammars take a role's rules as their own with does. A role's rules can call rules it doesn't have, here \<amount>, for each grammar that uses it to provide.

```go
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
```

```
true false
false true
```

#### `Grammar.Does`

Does composes roles into g, as grammar g does role { ...

Does composes roles into a grammar. The grammar's own rules win over the role's, wherever Does is called. Two roles with a rule of the same name are an error, unless the grammar defines that rule itself to choose.

In Raku syntax: role Money { ... } and grammar G does Money { ... }.

```go
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
```

```
h
grammar: line 4, column 65: rule unit is in both Units and Extra; define it in Hours to choose
```

#### `Grammar.Check`

Check reports the first call to a rule that doesn't exist, and the first left-recursive rule: one that can call itself again, directly or through other rules, before matching any text, and so would recurse for ever.

Check reports a call to a rule that doesn't exist. Compile checks, but a grammar built in Go is only checked when it's first used, and then a missing rule panics, so call Check after building one from data.

```go
g := grammar.New("G").Token("TOP", grammar.Call("missing"))
fmt.Println(g.Check())
```

```
grammar G: rule TOP calls <missing>, which doesn't exist
```

#### `Grammar.Name`

Name returns the grammar's name.

```go
fmt.Println(Pay.Name())
```

```
Pay
```

#### `Grammar.IsRole`

IsRole reports whether g is a role.

```go
fmt.Println(Pay.IsRole(), grammar.NewRole("Money").IsRole())
```

```
false true
```

#### `Compile.rule`

In a rule, the spaces in the rule match spaces in the text: each becomes a call to \<.ws>, which matches any amount of whitespace, so the rule reads like the text it matches. Here "up to £500 per day" matches with any spacing, but "upto" doesn't, as \<.ws> won't match nothing between two words. See "Spaces in rules" in docs/REFERENCE.md for exactly where a space counts.

```go
g := grammar.MustCompile(`
grammar Rate {
    rule  TOP   { up to <money> per <unit> }
    token money { '£' \d+ }
    token unit  { day | hour | week }
}`)
for _, text := range []string{"up to £500 per day", "up  to £500   per day", "upto £500 per day"} {
	fmt.Println(g.Parse(text).At("money.~").Or("no match"))
}
```

```
£500
£500
no match
```

#### Unicode properties

Unicode properties pick characters by what Unicode says they are, so a rule works in any language: \<:Lu> is any capital, where \<\[A..Z]> is only the 26. Here, capitalised words in names from three scripts.

```go
g := grammar.MustCompile(`grammar Names { token TOP { <:Lu> <:Ll>+ } }`)
ascii := grammar.MustCompile(`grammar ASCII { token TOP { <[A..Z]> <[a..z]>+ } }`)

text := "Zoë Ölander, Жанна Петрова, 李 Wei"
fmt.Println(g.FindAll(text).Map(func(m *scalar.SV) any { return m.At("~") }).Join(" | "))
fmt.Println(ascii.FindAll(text).Map(func(m *scalar.SV) any { return m.At("~") }).Join(" | "))
```

```
Zoë | Ölander | Жанна | Петрова | Wei
Zo | Wei
```

### Reference: Parsing and matches

#### `Grammar.Parse`

Parse matches the whole of text, as Raku's .parse does, and returns the match, or undef if text doesn't match.

Parse matches the whole text, or nothing: a match that would leave text over is no match. When it fails, the result is undef, so Defined, Or and the like work on it, and reading through it gives undef too.

```go
fmt.Println(Pay.Parse("£18.50 per hour").At("min.amount.num.~"))
fmt.Println(Pay.Parse("£18.50 per hour, negotiable").Defined()) // not all of it
fmt.Println(Pay.Parse("negotiable").At("min.~").Or("none"))
```

```
18.50
false
none
```

#### `Grammar.Subparse`

Subparse matches from the start of text, without needing to reach the end, as Raku's .subparse does.

Subparse matches from the start of the text, without needing to reach its end.

```go
m := Pay.Subparse("£18.50 per hour, negotiable")
fmt.Println(m.At("~"))
fmt.Println(Pay.Subparse("Pay: £18.50").Defined()) // it must start at the start
```

```
£18.50 per hour
false
```

#### `Grammar.Find`

Find returns the first match anywhere in text, or undef.

Find hunts for the first place in the text where the grammar matches, which is how to pull a fact out of free text.

```go
m := Pay.Find("The role pays £30k to £35k depending on experience.")
fmt.Println(m.At("~"), "at", m.At("from"))
```

```
£30k to £35k at 14
```

#### `Grammar.Find_noMatch`

When Find finds nothing, its undef can be read through, giving undef, and stays undef: reading it doesn't make it a match, as in Raku.

```go
m := Pay.Find("Salary negotiable")
fmt.Println(m.At("min.~").Or("none"))
fmt.Println(m.Defined())
```

```
none
false
```

#### `Grammar.FindAll`

FindAll returns every match in text, left to right and not overlapping, unless Overlap or Outermost is given.

FindAll finds every match, left to right, without overlapping.

```go
for _, m := range Pay.FindAll("£30k basic, £5k car allowance, £2k bonus").All() {
	fmt.Println(m.At("~"))
}
```

```
£30k
£5k
£2k
```

#### `Rule`

Rule starts the match at the named rule instead of TOP, as :rule does.

Rule starts the match at another rule instead of TOP, which is handy for testing one rule on its own, or for reusing part of a grammar.

```go
fmt.Println(Pay.Parse("12,500", grammar.Rule("num")).At("~"))
fmt.Println(Pay.Parse("12,500").Defined()) // TOP wants a £
```

```
12,500
false
```

#### `Pos`

Pos starts matching n characters into the text (of its NFC form, as grammars match it), as Raku's :pos does: Parse and Subparse match from there, Parse still to the end; Find, FindAll and Split search from there.

Pos starts part-way into the text, as Raku's :pos does, with the text before it still there to look back at, and positions still the whole text's.

```go
m := Pay.Subparse("Salary: £45k", grammar.Pos(8))
fmt.Println(m.At("~"), m.At("from"))
fmt.Println(Pay.Subparse("Salary: £45k").Defined()) // from the start, there's no £
```

```
£45k 8
false
```

#### `Overlap`

Overlap has FindAll find a match from each position where one starts, as Raku's :ov (:overlap) does, rather than going on from the end of the last one: in "in this role you will need", a list with "in this role you will" and "you will need" finds both.

Overlap finds a match from each place one starts, as Raku's :ov does, so a heading is weighed by every phrase in it, even one inside another.

```go
sections := grammar.NewWordList("sections", grammar.FoldCase(), grammar.WholeWords()).
	AddAs("responsibilities", "in this role you will", "duties").
	AddAs("requirements", "you will need", "skills")
g := grammar.MustCompile(`grammar Heading { token TOP { @sections } }`, sections)
heading := "In this role you will need"
made := func(m *scalar.SV) any { return m.At("made") }
fmt.Println(g.FindAll(heading).Map(made).Join(", "))
fmt.Println(g.FindAll(heading, grammar.Overlap()).Map(made).Join(", "))
```

```
responsibilities
responsibilities, requirements
```

#### `Outermost`

Outermost has FindAll find the matches Overlap finds, less each one wholly inside another, so it implies Overlap.

Outermost is Overlap without the matches inside another, so "profile" can't outweigh the "company profile" it's part of, while "you will need" still counts beside "in this role you will".

```go
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
```

```
Company Profile: company, requirements; company
In this role you will need: responsibilities, requirements; responsibilities, requirements
```

#### `Actions`

Actions has a's methods turn matches into values, as :actions does.

Actions turns matches into values. For each rule that matched, the method named after it is called with its match, and what it returns is stored in the match under "made". Actions run innermost first, so here Amount has run for each \<amount> before TOP's action reads their made values.

```go
m := Pay.Parse("£45k - £55,000", grammar.Actions(PayActions{}))
fmt.Println(m.At("min.amount.made"), m.At("max.amount.made"))
```

```
45000 55000
```

#### `Actions.map`

A map of functions, keyed by rule name, works as well as a type with methods, and suits actions built at run time.

```go
m := Pay.Parse("£45k - £55k per annum", grammar.Actions(map[string]func(*scalar.SV) any{
	"amount": PayActions{}.Amount,
	"period": func(m *scalar.SV) any { return strings.TrimSpace(m.At("~").Str()) },
}))
fmt.Println(m.At("max.amount.made"), m.At("period.made"))
```

```
55000 per annum
```

#### `Actions.to`

A match is ordinary scalar data, so once TOP's action has built the value you want, scalar.To makes it typed Go.

```go
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
```

```
{Min:45000 Max:55000} <nil>
```

#### Capture shapes

How captures are shaped. A capture that can match more than once, because it's quantified (\*, +, \*\*) or its name appears twice, is always an array, even when it matched once or not at all. Any other capture is a single match, or missing when it didn't match. The grammar decides this, not the text, so code reading a match never has to check which it got.

( ... ) captures by position, as "0", "1" and so on; \$\<name>=\[ ... ] captures the text it matched under a name.

```go
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
```

```
2 tags; code 42 ; bang !
0 tags; code 42 ; bang none
```

#### `Values`

Values reduces a match to the values in it, ready for `scalar.To`.

Values reduces a match to the values in it: each match becomes its made value if an action made one, a hash of its captures if it has any, and its text otherwise. That's usually the shape you want to read, or to pass to scalar.To.

```go
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
```

```
born: "1815"
name:
  first: Ada
  last: Lovelace
```

#### `To`

To reduces a match with Values and converts it to a T with `scalar.To`, so a grammar fills a struct directly:

To fills a struct from a match, by way of Values: capture names match field names, ignoring case, as in encoding/json, and a json tag can name one that doesn't, such as a positional capture or a rule called pay-range. A capture that didn't match leaves its field at its zero value. If the text didn't match at all, To returns ErrNoMatch.

```go
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
```

```
{Min:45000 Max:55000 Period:per annum} <nil>
{Min:18.5 Max:0 Period:} <nil>
grammar: no match true
```

#### `ToAll`

ToAll is To for FindAll: it reduces each match with Values and converts them to a []T, in order, so a word list's canonical values come back as Go strings:

ToAll is To for FindAll: each match becomes a T, in order, here each amount of money in a posting as the number its action made.

```go
acts := grammar.Actions(map[string]func(*scalar.SV) any{
	"money":  func(m *scalar.SV) any { return m.At("amount.made") },
	"amount": PayActions{}.Amount,
})
posting := "£30k basic, £5k car allowance, £2.5k bonus"
amounts, err := grammar.ToAll[float64](Pay.FindAll(posting, grammar.Rule("money"), acts))
fmt.Println(amounts, err)
```

```
[30000 5000 2500] <nil>
```

#### `Grammar.ParseErr`

ParseErr is Parse, but when the text doesn't match it also returns a *ParseError, saying where the parse got furthest and what it expected there.

ParseErr says why a parse failed: where in the text the grammar got furthest, and what it expected there. A rule that couldn't start there is named, \<money> here; one that got part of the way is reported by what it needed next. That's usually exactly where the mistake is, as everything before it matched.

```go
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
```

```
ok: £45k - £55,000 per annum
grammar Pay: line 1, column 8: expected <money>, found "55,000"
grammar Pay: line 1, column 14: expected \d, found end of text
grammar Pay: line 1, column 15: expected end of text, found ", DOE"
```

#### `Trace`

Trace writes a trace of the parse to w, in the style of Raku's Grammar::Tracer: each rule's name as it's entered, indented by depth, then "* MATCH" and the text it matched, or "* FAIL", at its caller's depth.

Trace prints each rule as it's tried, indented by depth, then what it matched, or FAIL, at its caller's depth, as Raku's Grammar::Tracer does. There's no separator, so \<sep> fails, and TOP goes on to \<period>.

```go
Pay.Parse("£45k per annum", grammar.Trace(os.Stdout))
```

```
TOP
|  money
|  |  amount
|  |  |  num
|  |  |  * MATCH "45"
|  |  * MATCH "45k"
|  * MATCH "£45k"
|  sep
|  * FAIL
|  period
|  * MATCH " per annum"
* MATCH "£45k per annum"
```

#### `Trace.failure`

Where ParseErr says only that \<money> was expected at "55,000", a trace shows the way there: the separator matched, then \<money> failed at once, with no £, so the range was given up, \<period> was tried and failed, and TOP matched "£45k", which isn't the whole text.

```go
Pay.Parse("£45k - 55,000", grammar.Trace(os.Stdout))
```

```
TOP
|  money
|  |  amount
|  |  |  num
|  |  |  * MATCH "45"
|  |  * MATCH "45k"
|  * MATCH "£45k"
|  sep
|  * MATCH " - "
|  money
|  * FAIL
|  period
|  * FAIL
* MATCH "£45k"
```

#### `Grammar.SubparseErr`

SubparseErr is Subparse, with a *ParseError when the text doesn't match, as ParseErr has.

SubparseErr is ParseErr for Subparse: the text must match from its start, but needn't match to its end.

```go
_, err := Pay.SubparseErr("Salary: £45k")
fmt.Println(err)
```

```
grammar Pay: line 1, column 1: expected <money>, found "Salary: £45k"
```

#### `ParseError.Error`

A ParseError's fields give the same facts as its message, for a program to use: to log which rule postings fail at, say, or to underline the place in an editor.

```go
_, err := Pay.ParseErr("£45k -\n55k")
var pe *grammar.ParseError
if errors.As(err, &pe) {
	fmt.Println(pe.Line, pe.Column, pe.Expected, pe.Found)
	fmt.Println(pe)
}
```

```
2 1 [<money>] 55k
grammar Pay: line 2, column 1: expected <money>, found "55k"
```

#### `Grammar.CheckPath`

CheckPath returns an error if path, as scalar's At takes it, can never be defined in a match of g, and nil if some text could define it.

CheckPath says whether a path can ever be defined in a match. A misspelt capture reads undef, as one that didn't match does, so a typo fails silently; CheckPath names the step that can't be there, and what could. It's for tests: check each path the program reads.

```go
for _, path := range []string{"min.amount.num.~", "period.~", "min.amont", "money.amount", "max.amount.k.made"} {
	fmt.Println(Pay.CheckPath(path))
}
```

```
<nil>
<nil>
grammar Pay: path "min.amont": rule money has no capture "amont" (it has: amount)
grammar Pay: path "money.amount": money is a list of matches, as it can capture more than once: index it, as money[0]
grammar Pay: path "max.amount.k.made": max.amount.k never has a made value: only a rule's match, or a word list's, has one
```

#### `DepthLimitError.Error`

Matching recurses as it goes, a level or so for each item matched, and past DepthLimit it stops with a \*DepthLimitError, which can be recovered, rather than running out of stack, which can't: Go would end the program. ParseErr returns it. Here the limit is low, to show it.

```go
g := grammar.MustCompile(`grammar Long { token TOP { <[a..z]>* } }`)
defer func(old int) { grammar.DepthLimit = old }(grammar.DepthLimit)
grammar.DepthLimit = 1_000

_, err := g.ParseErr(strings.Repeat("a", 600))
fmt.Println(err)
```

```
grammar Long: match too deep: gave up after 1000 levels, in TOP from line 1, column 1 (see DepthLimit)
```

#### `Backref`

Backref is \$<name>, or \$0 for the first positional capture: it matches the text the capture last matched, as Raku's does: a separator used again, as in \d+ \$<sep>=<[,.]> \d ** 3 - \d+ \$<sep> \d ** 3, or a closing quote.

Backref is \$\<name>: the text a capture matched, again. Here a pay range must use the same thousands separator in both figures, so a range that mixes them, likely two figures run together, isn't read as one.

```go
g := grammar.MustCompile(`
grammar SameSep {
    token TOP { \d+ $<sep>=<[,.]> \d ** 3 \s* '-' \s* \d+ $<sep> \d ** 3 }
}`)
for _, text := range []string{"45,000 - 55,000", "45,000 - 55.000"} {
	fmt.Printf("%-16q %v\n", text, g.Parse(text).Defined())
}
```

```
"45,000 - 55,000" true
"45,000 - 55.000" false
```

#### `Caps`

Caps iterates over m's captures in the order they come in the text, as Raku's .caps does: each capture's name and match.

Caps gives a match's captures in the order they come in the text, as Raku's .caps does. A match is a hash, which keeps no order between names; here, the qualifiers of a title are captured by kind, and Caps puts them back in the order the title has them.

```go
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
```

```
title       Senior Engineer
contract    Contract
arrangement Remote
contract    Temp
```

#### `Chunks`

Chunks iterates over m in pieces, as Raku's .chunks does: its captures, in Caps' order, with the text between them under "~", as a string.

Chunks is Caps with the text between the captures, under "~", so the chunks put the text back together: here, to mark up the parts of a pay string.

```go
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
```

```
[min: £45k] - [max: £55k][period:  per annum]
```

#### `StepLimit`

StepLimit stops a match that backtracks badly. Nested repeats in a regex rule, as in amount here, try every way of sharing the digits out before giving up, so each extra digit doubles the time: 30 of them would take minutes. The default limit stops it in about a second; this one is lower. ParseErr's error names the rule that was matching, and where its match started.

```go
g := grammar.MustCompile(`
grammar Pay {
    token TOP { 'Salary: ' <amount> }
    regex amount { [ \d+ ]+ ' per annum' }
}`)
defer func(old int) { grammar.StepLimit = old }(grammar.StepLimit)
grammar.StepLimit = 100_000

_, err := g.ParseErr("Salary: " + strings.Repeat("9", 30) + " pa")
fmt.Println(err)
```

```
grammar Pay: match too long: gave up after 100000 steps, in amount from line 1, column 9 (see StepLimit)
```

#### `StepLimitError.Error`

Parse, Find and the rest panic with a \*StepLimitError, as scalar's regexes panic past scalar.RegexTimeout. A program that reads texts it can't trust can recover it, and go on to the next text.

```go
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
```

```
"aaa" <nil>
"" grammar Blowup: match too long: gave up after 100000 steps (see StepLimit)
```

#### `Grammar.Split`

Split returns the text between the grammar's matches, with each match's text in place between them, as Raku's split with :v does: "a, b" split at commas is "a", ", ", "b".

Split gives the text between matches with the matches in place, as Raku's split with :v does: here, a paragraph split at headings run into it.

```go
g := grammar.MustCompile(`
grammar Inline {
    token TOP { <?after '.' \s+> [ 'What you will do' | 'How to apply' ] }
}`)
for _, piece := range g.Split("We're hiring. What you will do Lead a team. How to apply Email us.").All() {
	fmt.Printf("%q\n", piece)
}
```

```
"We're hiring. "
"What you will do"
" Lead a team. "
"How to apply"
" Email us."
```

### Reference: Building rules in Go

#### `Lit`

Lit matches s exactly, or ignoring case inside IgnoreCase.

Lit matches text exactly. In Raku syntax, letters and digits are literal as they stand, and anything else is quoted: '£', or "a\\tb" for escapes.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit)))
// Raku: token TOP { '£' \d+ }

fmt.Println(g.Subparse("£450 per week").At("~"))
fmt.Println(g.Subparse("$450").Defined())
```

```
£450
false
```

#### `Class`

Class matches one character for which in returns true.

Class matches one character that a function accepts. Digit, Word, Space and AnyChar are the classes Raku writes \\d, \\w, \\s and . (any character, newlines too).

```go
upper := grammar.Class(unicode.IsUpper)
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Plus(upper), grammar.Lit("-"), grammar.Plus(grammar.Digit)))
// Raku, for ASCII capitals: token TOP { <[A..Z]>+ '-' \d+ }

fmt.Println(g.Parse("REF-12").Defined(), g.Parse("ref-12").Defined())

w := grammar.New("W").Token("TOP", grammar.Seq(grammar.Plus(grammar.Word), grammar.Space, grammar.AnyChar))
// Raku: token TOP { \w+ \s . }
fmt.Println(w.Parse("ab_1 !").Defined())
```

```
true false
true
```

#### `Re`

Re matches pattern, a Perl-style regex (as scalar's M takes), at the current position.

Re matches a Perl-style regex, the kind scalar's M takes, as one piece of a rule. It's for patterns you already have; whatever it matches first is final, as if it were a token. Raku syntax has no equivalent.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Re(`[A-Z]{2,4}`), grammar.Lit("-"), grammar.Capture(grammar.Re(`\d+`))))
fmt.Println(g.Parse("REF-1234").At("0.~"))
```

```
1234
```

#### `Seq`

Seq matches each node in turn.

Seq matches its parts one after another. In Raku syntax, that's just writing them in order.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Lit("per"), grammar.Plus(grammar.Space), grammar.Lit("hour")))
// Raku: token TOP { per \s+ hour }

fmt.Println(g.Parse("per   hour").Defined())
```

```
true
```

#### `Alt`

Alt is Raku's |: the branch that matches the most text wins, and the earlier branch wins a tie.

Alt is Raku's |: of the alternatives that match, the one that matches the most text wins, whatever order they're written in. So hr, hour and hours can be listed in any order.

```go
g := grammar.New("G").Token("TOP", grammar.Alt(grammar.Lit("hr"), grammar.Lit("hour"), grammar.Lit("hours")))
// Raku: token TOP { [ hr | hour | hours ] }

fmt.Println(g.Subparse("hours a week").At("~"))
```

```
hours
```

#### `First`

First is Raku's ||: the first branch that matches wins.

First is Raku's ||: the first alternative that matches wins, even if a later one would match more. In a token, that choice is final, so here Parse fails on "hours": First chose "hour", leaving the "s" over.

```go
g := grammar.New("G").Token("TOP", grammar.First(grammar.Lit("hour"), grammar.Lit("hours")))
// Raku: token TOP { [ hour || hours ] }

fmt.Println(g.Subparse("hours").At("~"))
fmt.Println(g.Parse("hours").Defined())
```

```
hour
false
```

#### `Goal`

Goal is Raku's goal matching, open ~ close inner: open, then inner, then close, as Seq(open, inner, close) is, written with the brackets together, as in '(' ~ ')' <qualifiers>.

Goal is Raku's goal matching, '(' ~ ')' x: x between brackets, written with the brackets together. It matches as Seq(open, x, close) does; what it adds is that ParseErr names the opening bracket a missing closer was for, and where it was.

```go
quals := grammar.Seq(grammar.Plus(grammar.Class(unicode.IsLetter)),
	grammar.Star(grammar.Seq(grammar.Lit(", "), grammar.Plus(grammar.Class(unicode.IsLetter)))))
g := grammar.New("Quals").Token("TOP", grammar.Goal(grammar.Lit("("), grammar.Lit(")"), quals))
// Raku: token TOP { '(' ~ ')' [ <:L>+ ]+ % ', ' }

fmt.Println(g.Parse("(Remote, FTC)").At("~"))
_, err := g.ParseErr("(Remote, FTC")
fmt.Println(err)
```

```
(Remote, FTC)
grammar Quals: line 1, column 13: expected ', ' or ')' to close '(' from line 1, column 1, found end of text
```

#### `Conj`

Conj is Raku's & and &&: every node matches, over the same text, as in [ <:Lu> \S* ]+ % ' ' & @titles, a list phrase in title case.

Conj is & and &&: every node matches, over the same text. Here, an acronym from a list, but only where it's written in capitals.

```go
acronyms := grammar.NewWordList("acronyms", grammar.FoldCase(), grammar.WholeWords()).Add("SQL").Add("AWS")
g := grammar.New("Acronym").Token("TOP",
	grammar.Conj(grammar.Plus(grammar.Class(unicode.IsUpper)), grammar.Words(acronyms)))
// Raku: token TOP { <:Lu>+ & @acronyms }

fmt.Println(g.FindAll("SQL, sql, AWS and SQLITE").Map(func(m *scalar.SV) any { return m.At("~") }).Join(" "))
```

```
SQL AWS
```

#### `Sym`

Sym is <sym>: in a rule named name:sym<…>, it matches the text between the angle brackets, and captures it as sym, as Raku's does.

Proto and Sym are proto token and \<sym> built in Go: variants are rules named name:sym\<…>, and Sym matches that name, captured as sym.

```go
g := grammar.New("Units").
	Token("TOP", grammar.Plus(grammar.Call("unit")).Sep(grammar.Lit(", "))).
	Proto("unit").
	Token("unit:sym<year>", grammar.Seq(grammar.Sym(), grammar.Opt(grammar.Lit("s")))).
	Token("unit:sym<week>", grammar.Seq(grammar.Sym(), grammar.Opt(grammar.Lit("s"))))
// Raku: token TOP { <unit>+ % ', ' }; proto token unit {*}
//       token unit:sym<year> { <sym> s? }; token unit:sym<week> { <sym> s? }

m := g.Parse("years, week")
fmt.Println(m.At("unit.0.sym.~"), m.At("unit.1.sym.~"))
```

```
year week
```

#### `Opt`

Opt is n?.

Opt is ?: zero or one.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit), grammar.Opt(grammar.Lit("k"))))
// Raku: token TOP { '£' \d+ k? }

fmt.Println(g.Parse("£45").Defined(), g.Parse("£45k").Defined())
```

```
true true
```

#### `Star`

Star is n*.

Star is \*: zero or more, as many as possible.

```go
thousands := grammar.Seq(grammar.Lit(","), grammar.Repeat(grammar.Digit, 3, 3))
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Repeat(grammar.Digit, 1, 3), grammar.Star(thousands)))
// Raku: token TOP { \d ** 1..3 [ ',' \d ** 3 ]* }

fmt.Println(g.Parse("45").Defined(), g.Parse("45,000,000").Defined(), g.Parse("45,00").Defined())
```

```
true true false
```

#### `Plus`

Plus is n+.

Plus is +: one or more, as many as possible.

```go
g := grammar.New("G").Token("TOP", grammar.Plus(grammar.Digit))
// Raku: token TOP { \d+ }

fmt.Println(g.Subparse("2024 onwards").At("~"))
fmt.Println(g.Parse("").Defined())
```

```
2024
false
```

#### `Repeat`

Repeat is n ** min..max; a negative max is no limit, as in ** 2..*.

Repeat is \*\*: between min and max times, with -1 for no limit. In Raku, \\d \*\* 4 is exactly four, \\d \*\* 1..3 one to three, \\d \*\* 2..\* two or more.

```go
g := grammar.New("G").Token("TOP", grammar.Repeat(grammar.Digit, 1, 3))
// Raku: token TOP { \d ** 1..3 }

fmt.Println(g.Subparse("12345").At("~"))
```

```
123
```

#### `Quant.Sep`

Sep returns q with sep between items, as % does.

Sep puts a separator between repeats, as Raku's % does: \<tag>+ % ',' is one or more tags with commas between. There mustn't be a comma at the end; SepTrailing allows one.

```go
g := grammar.New("G").
	Token("TOP", grammar.Plus(grammar.Call("tag")).Sep(grammar.Seq(grammar.Lit(","), grammar.Star(grammar.Space)))).
	Token("tag", grammar.Plus(grammar.Word))
// Raku: token TOP { <tag>+ % [ ',' \s* ] }; token tag { \w+ }

fmt.Println(g.Parse("go, sql, aws").At("tag").AV().Length())
fmt.Println(g.Parse("go, sql,").Defined())
```

```
3
false
```

#### `Quant.SepTrailing`

SepTrailing returns q with sep between items and optionally after the last, as %% does.

SepTrailing is Raku's %%: separators between repeats, and optionally one at the end.

```go
g := grammar.New("G").
	Token("TOP", grammar.Plus(grammar.Call("tag")).SepTrailing(grammar.Lit(","))).
	Token("tag", grammar.Plus(grammar.Word))
// Raku: token TOP { <tag>+ %% ',' }; token tag { \w+ }

fmt.Println(g.Parse("go,sql,").At("tag").AV().Length())
```

```
2
```

#### `Quant.Frugal`

Frugal returns q matching as few items as it can, as *? and +? do.

Frugal makes a quantifier match as little as it can, as \*? and +? do in Raku and Perl, taking more only when what follows fails. Here the frugal .\*? stops at the first " - "; a greedy .\* would run to the last.

In a token, where nothing else gives back what it matched, a frugal quantifier still takes more when it needs to, so .\*? is how a token skips ahead. That works through \[ ... ] and \$\<name>=\[ ... ], but a capture ( ... ) commits to its first match, as in Raku: (.\*?) in a token matches nothing, and stays that way. Name the text with \$\<name>=\[ .\*? ] instead, or use a regex.

```go
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
```

```
"Engineer" "Payments - Remote"
false
```

#### `Call`

Call is <name>: it matches the rule name and captures it under that name.

Call matches another rule and captures its match under the rule's name: \<num> in Raku syntax.

```go
g := grammar.New("G").
	Token("TOP", grammar.Seq(grammar.Call("num"), grammar.Lit("%"))).
	Token("num", grammar.Plus(grammar.Digit))
// Raku: token TOP { <num> '%' }; token num { \d+ }

fmt.Println(g.Parse("10%").At("num.~"))
```

```
10
```

#### `Silent`

Silent is <.name>: it matches the rule name without capturing it.

Silent matches another rule without capturing it: \<.name> in Raku syntax. Use it for rules that only hold the text together, such as separators and whitespace.

```go
g := grammar.New("G").
	Token("TOP", grammar.Seq(grammar.Call("num"), grammar.Silent("ws"), grammar.Lit("days"))).
	Token("num", grammar.Plus(grammar.Digit))
// Raku: token TOP { <num> <.ws> days }

m := g.Parse("25 days")
fmt.Println(m.At("num.~"), m.At("ws").Defined())
```

```
25 false
```

#### `Alias`

Alias is <alias=name>.

Alias captures a rule's match under another name too: \<min=money> in Raku syntax. As in Raku, the match is under both names, so here it's under min and money. Two calls of money make money an array.

```go
g := grammar.New("G").
	Token("TOP", grammar.Seq(grammar.Alias("min", "money"), grammar.Lit("-"), grammar.Alias("max", "money"))).
	Token("money", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit)))
// Raku: token TOP { <min=money> '-' <max=money> }

m := g.Parse("£30-£35")
fmt.Println(m.At("min.~"), m.At("max.~"), m.At("money").AV().Length())
```

```
£30 £35 2
```

#### `AliasOnly`

AliasOnly is <alias=.name>, captured only as alias.

AliasOnly captures a rule's match under another name only: \<min=.money> in Raku syntax.

```go
g := grammar.New("G").
	Token("TOP", grammar.Seq(grammar.AliasOnly("min", "money"), grammar.Lit("+"))).
	Token("money", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit)))
// Raku: token TOP { <min=.money> '+' }

m := g.Parse("£700+")
fmt.Println(m.At("min.~"), m.At("money").Defined())
```

```
£700 false
```

#### `Capture`

Capture is ( n ): a positional capture, numbered from 0 in its scope.

Capture is ( ... ): a capture by position, numbered from 0 in the order the groups are written, and read with At("0"), At("1") and so on.

```go
digits := func(n int) grammar.Node { return grammar.Repeat(grammar.Digit, n, n) }
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Capture(digits(4)), grammar.Lit("-"), grammar.Capture(digits(2))))
// Raku: token TOP { (\d ** 4) '-' (\d ** 2) }

m := g.Parse("2026-10")
fmt.Println(m.At("0.~"), m.At("1.~"))
```

```
2026 10
```

#### `Named`

Named is \$<name>=n.

Named captures the text of part of a rule under a name: \$\<year>=\[ ... ] in Raku syntax. Named(name, Capture(...)), \$\<name>=( ... ), captures with a scope of its own, so positional captures inside it are its own.

```go
digits := func(n int) grammar.Node { return grammar.Repeat(grammar.Digit, n, n) }
g := grammar.New("G").Token("TOP", grammar.Seq(
	grammar.Named("year", digits(4)),
	grammar.Lit("-"),
	grammar.Named("rest", grammar.Capture(grammar.Seq(grammar.Capture(digits(2)), grammar.Lit("-"), grammar.Capture(digits(2))))),
))
// Raku: token TOP { $<year>=[ \d ** 4 ] '-' $<rest>=( (\d ** 2) '-' (\d ** 2) ) }

m := g.Parse("2026-10-15")
fmt.Println(m.At("year.~"), m.At("rest.~"), m.At("rest.0.~"), m.At("rest.1.~"))
```

```
2026 10-15 10 15
```

#### `Before`

Before is <?before n>, NotBefore <!before n>, After <?after n> and NotAfter <!after n>.

Before checks what comes next without matching it: \<?before ...> in Raku syntax. Here the digits must be followed by k, but the k isn't part of the match.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Plus(grammar.Digit), grammar.Before(grammar.Lit("k"))))
// Raku: token TOP { \d+ <?before k> }

fmt.Println(g.Subparse("45k").At("~"))
```

```
45
```

#### `NotBefore`

NotBefore checks that something doesn't come next: \<!before ...> in Raku syntax. Here it skips a percentage to find a plain number.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Plus(grammar.Digit), grammar.NotBefore(grammar.Lit("%"))))
// Raku: token TOP { \d+ <!before '%'> }

fmt.Println(g.Find("10% bonus and 25 days' holiday").At("~"))
```

```
25
```

#### `After`

After checks what came just before: \<?after ...> in Raku syntax.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.After(grammar.Lit("£")), grammar.Plus(grammar.Digit)))
// Raku: token TOP { <?after '£'> \d+ }

fmt.Println(g.Find("Ref 123, pays £45").At("~"))
```

```
45
```

#### `NotAfter`

NotAfter checks that something didn't come just before: \<!after ...> in Raku syntax. Here it finds a number that isn't money; WordStart stops it matching the 5 of £45.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.NotAfter(grammar.Lit("£")), grammar.WordStart, grammar.Plus(grammar.Digit)))
// Raku: token TOP { <!after '£'> << \d+ }

fmt.Println(g.Find("£45 for 3 days").At("~"))
```

```
3
```

#### `IgnoreCase`

IgnoreCase is :i over n.

IgnoreCase is :i: letters match in either case, for the part of the rule it covers. In Raku syntax, :i applies to the rest of the group it's in: \[:i per annum] or, at the start of a rule, all of it.

```go
g := grammar.New("G").Token("TOP", grammar.IgnoreCase(grammar.Lit("per annum")))
// Raku: token TOP { :i 'per annum' }

fmt.Println(g.Parse("PER ANNUM").Defined(), g.Parse("Per Annum").Defined())
```

```
true true
```

#### `Ratchet`

Ratchet is :r over n, or :!r when on is false.

Ratchet turns backtracking off (:r) or on (:!r) for part of a rule. A token is :r throughout, so Ratchet(..., false), :!r in Raku syntax, lets one part of a token backtrack, as a regex would.

```go
g := grammar.New("G").Token("TOP", grammar.Ratchet(grammar.Seq(grammar.Star(grammar.Digit), grammar.Lit("5")), false))
// Raku: token TOP { :!r \d* 5 }

fmt.Println(g.Parse("12345").Defined())
```

```
true
```

#### `Start`

Start and End match at the start and end of the text, as ^ and \$ do in Raku. LineStart and LineEnd, ^^ and \$\$, match at the start and end of any line.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Start, grammar.Plus(grammar.Digit)))
// Raku: token TOP { ^ \d+ }

fmt.Println(g.Find("12 days").Defined(), g.Find("Day 12").Defined())
```

```
true false
```

#### `LineStart`

LineStart is ^^: the start of any line. Here it finds the bulleted lines of a description, and only those.

```go
line := grammar.Plus(grammar.Class(func(r rune) bool { return r != '\n' }))
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.LineStart, grammar.Lit("- "), grammar.Capture(line)))
// Raku: token TOP { ^^ '- ' (\N+) }

text := "Benefits:\n- 25 days' holiday\n- Pension - 5%\nApply now"
for _, m := range g.FindAll(text).All() {
	fmt.Println(m.At("0.~"))
}
```

```
25 days' holiday
Pension - 5%
```

#### `WordStart`

WordStart and WordEnd are \<\< and >>: where a word starts and ends. Without them, "pa" is found inside "company".

```go
bare := grammar.New("G").Token("TOP", grammar.Lit("pa"))
word := grammar.New("G").Token("TOP", grammar.Seq(grammar.WordStart, grammar.Lit("pa"), grammar.WordEnd))
// Raku: token TOP { << pa >> }

text := "a company that pays 30k pa"
fmt.Println(bare.Find(text).At("from"), word.Find(text).At("from"))
```

```
5 24
```

### Reference: Word lists

#### `NewWordList`

NewWordList returns an empty list called name, which is how Raku syntax refers to it: @name.

A word list holds words and phrases, each with a canonical value, so that synonyms match as one thing. Pass it to Compile and match it with @name; a rule that's nothing but the list gets the canonical value of what it matched as its made value.

```go
titles := grammar.NewWordList("titles")
titles.Add("Software Engineer", "Software Developer", "SWE")
titles.Add("Data Engineer", "Data Platform Engineer")

g := grammar.MustCompile(`
grammar Title {
    token TOP   { <title> .* }      # a title, then anything
    token title { @titles }
}`, titles)

for _, text := range []string{"SWE (Payments)", "Data Platform Engineer, Remote"} {
	fmt.Println(g.Parse(text).At("title.made"))
}
```

```
Software Engineer
Data Engineer
```

#### `Words`

Words matches the longest phrase in l, as @name does.

Words is @name for grammars built in Go.

```go
titles := grammar.NewWordList("titles").Add("Software Engineer", "SWE")
g := grammar.New("Title").
	Token("TOP", grammar.Seq(grammar.Call("title"), grammar.Star(grammar.AnyChar))).
	Token("title", grammar.Words(titles))

fmt.Println(g.Parse("SWE II").At("title.made"))
```

```
Software Engineer
```

#### `Words.longestAndBoundaries`

A list matches the longest phrase it can, whatever order they were added in, and like Raku's, never gives back part of it. It doesn't look at word boundaries: "Java" matches the start of "JavaScript", unless the rule says the word must end there, with >>.

```go
langs := grammar.NewWordList("langs").Add("Java").Add("JavaScript")
longest := grammar.MustCompile(`grammar L { token TOP { @langs } }`, langs)
fmt.Println(longest.Subparse("JavaScript developer").At("~"))

java := grammar.NewWordList("java").Add("Java")
loose := grammar.MustCompile(`grammar L2 { token TOP { @java } }`, java)
whole := grammar.MustCompile(`grammar L3 { token TOP { @java >> } }`, java)
fmt.Println(loose.Subparse("JavaScript").Defined(), whole.Subparse("JavaScript").Defined())
```

```
JavaScript
true false
```

#### `WordList.Add`

Add adds canonical and each of synonyms to the list, all meaning canonical.

Add adds a canonical value and its synonyms. The canonical value is a phrase in the list too. Add returns the list, so calls can be chained.

```go
units := grammar.NewWordList("units").
	Add("Year", "annum", "pa").
	Add("Week", "wk")

fmt.Println(units.Canonical("annum"), units.Canonical("Year"), units.Canonical("wk"))
```

```
[Year] [Year] [Week]
```

#### `WordList.AddAs`

AddAs adds each of phrases to the list, all meaning value, without adding value itself as a phrase.

AddAs gives phrases a value that isn't itself a phrase, such as a code.

```go
occupations := grammar.NewWordList("occupations", grammar.FoldCase()).
	AddAs("2134", "Software engineer", "Software developer").
	AddAs("8211", "HGV driver")

fmt.Println(occupations.Canonical("software developer"), occupations.Canonical("HGV driver"))
fmt.Println(occupations.Canonical("2134"), occupations.Phrases())
```

```
[2134] [8211]
[] [hgv driver software developer software engineer]
```

#### `WordList.AddRule`

AddRule adds one synonym rule in the format Elasticsearch and Solr use:

AddRule takes one synonym rule in the format Elasticsearch and Solr use. A list of phrases means its first; phrases => phrases means the right side; a backslash escapes a comma or =>.

```go
l := grammar.NewWordList("units")
for _, rule := range []string{
	"Year, annum, yr",       // annum and yr mean Year
	"hr, ph => Hour",        // hr and ph mean Hour, as does Hour
	`£\, per annum => Year`, // the phrase "£, per annum"
} {
	if err := l.AddRule(rule); err != nil {
		fmt.Println(err)
	}
}
fmt.Println(l.Canonical("yr"), l.Canonical("ph"), l.Canonical("£, per annum"))

fmt.Println(l.AddRule("a => b => c"))
```

```
[Year] [Hour] [Year]
grammar: synonym rule "a => b => c" has more than one =>
```

#### `WordList.AddRules`

AddRules adds each line of rules as a rule, as AddRule does, and reports the first bad one.

AddRules takes many rules, one per line, as Elasticsearch's synonyms API returns them. Blank lines and lines starting with \# are skipped.

```go
units := grammar.NewWordList("units", grammar.FoldCase())
err := units.AddRules(`
# periods
Year, annum, yr, pa, p.a.
Week, wk, pw
hour, hr, ph => Hour
`)
if err != nil {
	fmt.Println(err)
	return
}
g := grammar.MustCompile(`
grammar Rate {
    token TOP  { '£' $<amount>=[ \d+ ] \s* [ per | a | '/' ]? \s* <unit> }
    token unit { @units }
}`, units)

for _, text := range []string{"£450 per WK", "£18/hr", "£40000 p.a."} {
	m := g.Parse(text)
	fmt.Println(m.At("amount.~"), m.At("unit.made"))
}
```

```
450 Week
18 Hour
40000 Year
```

#### `WordList.Canonical`

Canonical returns what phrase means: its canonical values, in the order they were added, or nil if it isn't in the list.

Canonical says what a phrase means: everything it means, in the order added, or nothing if it isn't in the list. It reads the phrase as the list matches text, so with FoldCase and LooseSpacing it ignores case and spacing too.

```go
titles := grammar.NewWordList("titles", grammar.FoldCase(), grammar.LooseSpacing()).
	Add("Front-end Developer", "Frontend Developer")

fmt.Println(titles.Canonical("FRONT END developer"))
fmt.Println(titles.Canonical("Back-end Developer") == nil)
```

```
[Front-end Developer]
true
```

#### `WordList.Ambiguous`

Ambiguous returns the phrases that mean more than one thing, with what each means.

Ambiguous lists the phrases that mean more than one thing. A match of one has the first meaning as its made value; Canonical gives the rest, for the caller to choose from context, or to pass on to something that can.

```go
titles := grammar.NewWordList("titles", grammar.FoldCase())
titles.Add("Product Manager", "PM")
titles.Add("Project Manager", "PM")

fmt.Println(titles.Ambiguous())

g := grammar.MustCompile(`grammar T { token TOP { <title> }; token title { @titles } }`, titles)
fmt.Println(g.Parse("PM").At("title.made"))
```

```
map[pm:[Product Manager Project Manager]]
Product Manager
```

#### `WordList.Phrases`

Phrases returns the list's phrases, normalised, in sorted order, without the plurals that Plurals made.

Phrases lists every phrase, as the list stores it: case folded with FoldCase, spacing collapsed with LooseSpacing.

```go
l := grammar.NewWordList("units", grammar.FoldCase()).Add("Year", "Annum", "p.a.")
fmt.Println(l.Phrases())
```

```
[annum p.a. year]
```

#### `WordList.Canonicals`

Canonicals returns the list's canonical values, each once, in the order they were first added: by Add, AddAs, AddRule or AddRules, so a rules file's order is kept, as data.

Canonicals gives the canonical values in the order they were first added, so the order of a rules file can be used as data: here, which section wins when a heading has phrases of two, such as "Desirable skills".

```go
sections := grammar.NewWordList("sections", grammar.FoldCase())
sections.AddRules(`Desirable, nice to have, ideally
Requirements, skills, essential
Benefits, perks, what we offer
Requirements, experience`)

rank := map[string]int{}
for i, s := range sections.Canonicals() {
	rank[s] = i
}
fmt.Println(sections.Canonicals())
fmt.Println(rank["Desirable"] < rank["Requirements"])
```

```
[Desirable Requirements Benefits]
true
```

#### `WordList.Len`

Len returns how many distinct phrases the list holds, not counting plurals that Plurals made.

Len counts distinct phrases: a synonym added twice counts once.

```go
l := grammar.NewWordList("units").Add("Year", "annum").Add("Year", "annum", "yr")
fmt.Println(l.Len())
```

```
3
```

#### `WordList.Name`

Name returns the list's name.

Name is how Raku syntax refers to the list: @units.

```go
fmt.Println(grammar.NewWordList("units").Name())
```

```
units
```

#### `FoldCase`

FoldCase matches a list's phrases ignoring case, wherever it's used.

FoldCase makes a list ignore case wherever it's used. Without it, a list ignores case only where the rule says :i, as in Raku.

```go
exact := grammar.NewWordList("exact").Add("Senior")
folded := grammar.NewWordList("folded", grammar.FoldCase()).Add("Senior")
g := grammar.MustCompile(`
grammar Seniority {
    token TOP    { @exact }
    token folded { @folded }
    token ci     { :i @exact }
}`, exact, folded)

fmt.Println(g.Parse("SENIOR").Defined())
fmt.Println(g.Parse("SENIOR", grammar.Rule("folded")).Defined())
fmt.Println(g.Parse("SENIOR", grammar.Rule("ci")).Defined())
```

```
false
true
true
```

#### `LooseSpacing`

LooseSpacing matches any run of spaces, hyphens, dashes and underscores in the text with any such run in a phrase, so "front-end" in the list matches "front end" and "front – end" in the text.

LooseSpacing makes any run of spaces, hyphens, dashes and underscores match any other, as job titles are written every which way. There must still be some spacing: "frontend" is a different phrase from "front-end", unless the list has JoinedSpacing too.

```go
l := grammar.NewWordList("titles", grammar.LooseSpacing()).Add("front-end developer")
g := grammar.MustCompile(`grammar T { token TOP { @titles } }`, l)

for _, text := range []string{"front end developer", "front – end developer", "front_end  developer", "frontend developer"} {
	fmt.Println(g.Parse(text).Defined())
}
```

```
true
true
true
false
```

#### `JoinedSpacing`

JoinedSpacing lets the text leave out the spacing in a phrase, so "on site" in the list matches "onsite" too, and "front-end" matches "frontend".

JoinedSpacing lets the text leave out a phrase's spacing, as postings write "on site", "on-site" and "onsite" for the same thing. Write the spaced form: there's no telling where a space would go in "onsite". With LooseSpacing too, one phrase covers every spelling, and the canonical value is the same for each.

```go
places := grammar.NewWordList("places", grammar.FoldCase(), grammar.LooseSpacing(),
	grammar.JoinedSpacing(), grammar.WholeWords()).
	Add("On-site", "on site", "office based")
g := grammar.MustCompile(`grammar P { token TOP { @places } }`, places)

for _, text := range []string{"Onsite", "on-site", "On Site", "office-based", "Officebased", "onsiteness"} {
	fmt.Printf("%-14q %v\n", text, g.Parse(text).At("made").Or("-"))
}
fmt.Println(places.Canonical("officebased"))
```

```
"Onsite"       On-site
"on-site"      On-site
"On Site"      On-site
"office-based" On-site
"Officebased"  On-site
"onsiteness"   -
[On-site]
```

#### `Plurals`

Plurals lets each phrase match its plural too, as postings write "Care Assistants" for a list's "Care Assistant": the canonical value is the same.

Plurals lets each phrase match its plural, with the same canonical value. Only the last word changes, by the regular English rules; the plurals don't show in Phrases, which lists what was added.

```go
roles := grammar.NewWordList("roles", grammar.FoldCase(), grammar.WholeWords(), grammar.Plurals()).
	Add("Care Assistant", "carer").Add("Secretary").Add("Coach")
g := grammar.MustCompile(`grammar R { token TOP { @roles } }`, roles)

for _, text := range []string{"Care Assistants", "carers", "secretaries", "coaches", "Coachs"} {
	fmt.Printf("%-18q %v\n", text, g.Parse(text).At("made").Or("-"))
}
fmt.Println(roles.Phrases())
```

```
"Care Assistants"  Care Assistant
"carers"           Care Assistant
"secretaries"      Secretary
"coaches"          Coach
"Coachs"           -
[care assistant carer coach secretary]
```

#### `WholeWords`

WholeWords matches a phrase only where it isn't part of a longer word: "java" doesn't match the start of "JavaScript", nor "lead" the start of "leadership".

WholeWords matches a phrase only where it isn't part of a longer word, which \<\< @list >> would do, without having to write the boundaries.

```go
levels := grammar.NewWordList("levels", grammar.FoldCase(), grammar.WholeWords()).
	Add("Lead", "team lead", "team leader").Add("Senior", "snr")
g := grammar.MustCompile(`grammar L { token TOP { @levels } }`, levels)

for _, title := range []string{"Team Leader (Care)", "Snr Engineer", "Leadership Trainer", "SNR-level role"} {
	fmt.Printf("%-20q %v\n", title, g.Find(title).At("made").Or("-"))
}
```

```
"Team Leader (Care)" Lead
"Snr Engineer"       Senior
"Leadership Trainer" -
"SNR-level role"     Senior
```

#### `WordLists`

WordLists makes a word list of each set of synonym rules in lists, named by its key and read as AddRules reads them, with opts for every list.

WordLists makes a list for each set of synonym rules, ready for Compile: one call instead of a loop per list. A rule ending with a comma carries on onto the next line.

```go
uses, err := grammar.WordLists(map[string]string{
	"units": `Year, annum, yr, pa,
	          p.a., per annum
	          Week, wk`,
	"levels": "Senior, snr, sr\nJunior, jnr, graduate",
}, grammar.FoldCase())
if err != nil {
	fmt.Println(err)
	return
}
g := grammar.MustCompile(`
grammar Rate {
    token TOP   { <level> \s+ <unit> }
    token level { @levels }
    token unit  { @units }
}`, uses...)
m := g.Parse("Snr per annum")
fmt.Println(m.At("level.made"), m.At("unit.made"))
```

```
Senior Year
```

#### `WordList.besideLookaround`

A rule that's a list beside lookarounds and anchors still gets the list's canonical value, looked up in what the list alone matched. Here "Contract:" is a label, not a pattern, so the rule won't take a pattern with a colon after it. The whitespace a rule matches after the list, as after "Temp to Perm", is in the match but left out of the lookup.

```go
patterns := grammar.NewWordList("patterns", grammar.FoldCase(), grammar.WholeWords())
patterns.Add("contract", "fixed term")
patterns.Add("temp to perm", "temp-to-perm")
g := grammar.MustCompile(`grammar Pattern {
    rule TOP { @patterns <!before ':'> }
}`, patterns)

for _, m := range g.FindAll("Contract: Temp to Perm / Fixed term").All() {
	fmt.Printf("%q: %s\n", m.At("~"), m.At("made"))
}
```

```
"Temp to Perm ": temp to perm
"Fixed term": contract
```

#### `Named.wordList`

A named capture of a word list, \$\<name>=@list, has the canonical value as its made value, so a list inside a bigger rule needs no rule of its own, and grammar.To can read it straight into a struct.

```go
levels := grammar.NewWordList("levels", grammar.FoldCase()).Add("Senior", "snr", "sr")
g := grammar.MustCompile(`grammar T { token TOP { $<level>=@levels? \s* $<role>=[ \N+ ] } }`, levels)

for _, title := range []string{"Snr Software Engineer", "Software Engineer"} {
	t, _ := grammar.To[struct{ Level, Role string }](g.Parse(title))
	fmt.Printf("%+v\n", t)
}
```

```
{Level:Senior Role:Software Engineer}
{Level: Role:Software Engineer}
```

#### `Named.captureInside`

\$\<name>=\[ ... ] keeps only the text, so a capture inside it belongs to the enclosing rule, as in Raku: here level is beside title, not in it, and title.level reads undef. For the capture inside it, use \$\<name>=( ... ).

```go
levels := grammar.NewWordList("levels", grammar.FoldCase()).Add("Senior", "snr", "sr")
g := grammar.MustCompile(`grammar T { token TOP { $<title>=[ $<level>=@levels \s+ \w+ ] } }`, levels)

m := g.Parse("Snr Engineer")
fmt.Println(m.At("title.~"))
fmt.Println(m.At("level.made"))
fmt.Println(m.At("title.level.made").Or("undef"))
```

```
Snr Engineer
Senior
undef
```

#### `NewPatternList`

NewPatternList compiles patterns into a list called name, which is how Raku syntax refers to it: <@name>.

A pattern list holds patterns as data, each a regex in Raku syntax, for \<@name> to match, as Raku's \<@array> does: the longest match wins. Here, the ways a period is written, matched with their spacing and case.

```go
periods, err := grammar.NewPatternList("periods",
	`:i per \s* annum`, `:i 'p.a.'`, `:i pa >>`, `:i per \s* hour`)
if err != nil {
	fmt.Println(err)
	return
}
g := grammar.MustCompile(`grammar Pay { token TOP { '£' $<amount>=[ \d+ ] \s* $<period>=<@periods> } }`, periods)
for _, text := range []string{"£45000 per annum", "£45000 PA", "£18 per hour", "£45000 p.a."} {
	fmt.Println(g.Parse(text).At("period.~"))
}

_, err = grammar.NewPatternList("bad", `\d+ k`, `per [ annum`)
fmt.Println(err)
```

```
per annum
PA
per hour
p.a.
pattern list bad: pattern "per [ annum": grammar: line 1, column 12: expected ]
```

#### `MustPatternList`

MustPatternList is NewPatternList, panicking if a pattern doesn't compile.

MustPatternList is NewPatternList, panicking if a pattern doesn't compile, for lists written into the program.

```go
amounts := grammar.MustPatternList("amounts", `\d+ k`, `\d ** 1..3 [ ',' \d ** 3 ]+`, `\d+`)
g := grammar.MustCompile(`grammar A { token TOP { '£' <@amounts> } }`, amounts)
fmt.Println(g.FindAll("£45k, £45,000 or £45").Map(func(m *scalar.SV) any { return m.At("~") }).Join(" | "))
```

```
£45k | £45,000 | £45
```

#### `PatternList.Name`

Name returns the list's name.

Name is how Raku syntax refers to the list: \<@periods>.

```go
fmt.Println(grammar.MustPatternList("periods", `pa`).Name())
```

```
periods
```

#### `ExactAcronyms`

ExactAcronyms keeps a list's acronyms in capitals where it otherwise ignores case (FoldCase, or :i): "IT" in the list doesn't match "it" in the text, though "computing" matches "Computing".

ExactAcronyms keeps a folded list's acronyms in capitals, so "IT", the field, isn't "it", the word.

```go
l := grammar.NewWordList("fields", grammar.FoldCase(), grammar.WholeWords(), grammar.ExactAcronyms()).
	Add("IT").Add("computing")
g := grammar.MustCompile(`grammar F { token TOP { @fields } }`, l)
fmt.Println(g.FindAll("Is it IT or Computing?").Map(func(m *scalar.SV) any { return m.At("~") }).Join(", "))
```

```
IT, Computing
```

#### `MapOnly`

MapOnly makes a rule written phrases => values add the phrases meaning the values, as AddAs does, without making the values phrases too.

MapOnly keeps the value of a phrases => value rule out of the phrases, so a section called legal isn't found in "Legal Secretary".

```go
sections := grammar.NewWordList("sections", grammar.FoldCase(), grammar.WholeWords(), grammar.MapOnly())
err := sections.AddRules(`
benefits, perks, what we offer
legal notice, the legal bit => legal
`)
if err != nil {
	fmt.Println(err)
	return
}
g := grammar.MustCompile(`grammar Heading { token TOP { @sections } }`, sections)
for _, heading := range []string{"Legal Secretary", "Benefits", "Legal Notice"} {
	fmt.Printf("%s: %v\n", heading, g.Find(heading).At("made").Or("no section"))
}
fmt.Println(sections.Canonicals())
```

```
Legal Secretary: no section
Benefits: benefits
Legal Notice: legal
[benefits legal]
```

<!-- reference:end -->
