# grammar: how-to guide

How to get things done with [grammar](../README.md): a quick start, then
recipes for common tasks, then the same job done with a regex and with a
grammar, side by side. For how each feature behaves, see the
[reference](REFERENCE.md), starting with
[what you might assume](REFERENCE.md#what-you-might-assume) if you know
regexes; for one example of every function and method, see the
[examples](EXAMPLES.md). Coming from Raku? The [phrasebook](RAKU.md) shows
tasks in Raku, with grammar and in plain Go.

- [Quick start](#quick-start)
- [Recipes](#recipes)
  - [Every pay figure in a posting](#every-pay-figure-in-a-posting)
  - [Word lists from a synonym store](#word-lists-from-a-synonym-store)
  - [The sections of a description](#the-sections-of-a-description)
  - [Sharing rules between grammars](#sharing-rules-between-grammars)
  - [When a grammar won't match](#when-a-grammar-wont-match)
- [A regex or a grammar?](#a-regex-or-a-grammar)

## Quick start

A grammar is a set of named rules that call each other. Here's one for
pay:

```go
var Pay = grammar.MustCompile(`
grammar Pay {
    token TOP    { <min=money> [ <.sep> <max=money> ]? <period>? }
    token money  { '£' \s? <amount> }
    token amount { <num> [ $<k>=[:i k] ]? }
    token num    { \d ** 1..3 [ ',' \d ** 3 ]* [ '.' \d+ ]? }
    token sep    { \s* [ '-' | '–' | to ] \s* }
    token period { :i \s* [ 'p.a.' | pa | per \s+ [ annum | year | hour | day ] ] }
}`)
```

Parsing starts at `TOP`, which reads: a `<money>`, captured as `min`; then
optionally a separator and another `<money>`, captured as `max`; then
optionally a `<period>`. Each rule is described in the same way, and the
[tables in the reference](REFERENCE.md#writing-a-grammar) say what
each piece means.

An action is a Go method named after a rule. It's called with that rule's
match, and what it returns is stored in the match as `made`:

```go
type PayActions struct{}

// Amount is called for each <amount>: "45,000" is 45000, and "45k" is too.
func (PayActions) Amount(m *scalar.SV) any {
	n := m.At("num.~").S(`,`, "", "gr").Num() // num.~ is the text <num> matched
	if m.At("k").Defined() {
		n *= 1000
	}
	return n
}

m := Pay.Find("Salary: £45k – £55,000 per annum, plus bonus.", grammar.Actions(PayActions{}))
m.At("~")                // "£45k – £55,000 per annum": the text matched
m.At("min.amount.made")  // 45000
m.At("max.amount.made")  // 55000
m.At("period.~")         // " per annum"
```

### What a match looks like

A match is a tree that follows the grammar: each rule that matched is a
match of its own, under the name it was captured as. Every match has its
text under `~`, and each also has `from` and `to`, its position, left out
here:

```yaml
"~": £45k – £55,000 per annum   # the text TOP matched
min:                            # <min=money>
  "~": £45k
  amount:                       # <amount>, inside money
    "~": 45k
    num: { "~": "45" }
    k:   { "~": k }             # $<k>=[ k ]: only there when it matched
    made: 45000                 # what the Amount action returned
max:                            # <max=money>
  "~": £55,000
  amount:
    "~": 55,000
    num: { "~": "55,000" }
    made: 55000
money: [ …, … ]                 # the same two matches: money is captured twice, so it's an array
period:
  "~": " per annum"
```

So `m.At("min.amount.made")` walks down the tree to a value, and
`m.At("period.~")` reads "the text of the period capture". A capture that
didn't match is missing, and reads as undef, so `Or` and `Defined` work on
it. The match is ordinary `scalar` data, so `At`, `Or`, `To` and the rest
all work on it. [The match](REFERENCE.md#the-match), in the
reference, has the details, and the examples'
[Match tree](EXAMPLES.md#match-tree) prints a real one.

## Recipes

Each recipe runs as written, with the `Pay` grammar from the quick start
where it's used, and prints what its comments say.

### Every pay figure in a posting

`FindAll` finds every match in a text, left to right, and actions turn
each one into numbers, so `To` can fill a struct from it:

```go
type Range struct {
	Min, Max float64
	Period   string
}
acts := grammar.Actions(map[string]func(*scalar.SV) any{
	"amount": func(m *scalar.SV) any {
		n := m.At("num.~").S(`,`, "", "gr").Num()
		if m.At("k").Defined() {
			n *= 1000
		}
		return n
	},
	"money":  func(m *scalar.SV) any { return m.At("amount.made") }, // a money is its amount
	"period": func(m *scalar.SV) any { return strings.TrimSpace(m.At("~").Str()) },
})

posting := "We pay £45k – £55,000 per annum, plus a £5k bonus. Weekend shifts: £18.50 per hour."
for _, m := range Pay.FindAll(posting, acts).All() {
	p, err := grammar.To[Range](m)
	fmt.Printf("%-26q %+v %v\n", m.At("~").Str(), p, err)
}
// "£45k – £55,000 per annum" {Min:45000 Max:55000 Period:per annum} <nil>
// "£5k"                      {Min:5000 Max:0 Period:} <nil>
// "£18.50 per hour"          {Min:18.5 Max:0 Period:per hour} <nil>
```

The bonus is found too: it's pay-shaped, and `FindAll` finds every match.
To tell a bonus from a salary, give the grammar the words around it, as
the role recipe below does.

### Word lists from a synonym store

`WordLists` makes a word list of each set of synonym rules, in the format
Elasticsearch and Solr use, so the lists can come from wherever synonyms
are kept, rather than from code:

```go
// as they might come from a synonym store, one set of rules per list
stored := map[string]string{
	"units":     "Year, annum, pa, p.a.\nWeek, wk, pw\nHour, hr, ph",
	"seniority": "Senior, Snr, Sr\nJunior, Jnr, Jr",
}
uses, err := grammar.WordLists(stored, grammar.FoldCase())
if err != nil {
	return err
}
g, err := grammar.Compile(`
grammar Rate {
    token TOP  { '£' $<amount>=[ \d+ ] \s* [ per | a | '/' ]? \s* <unit> }
    token unit { @units }
}`, uses...)
if err != nil {
	return err
}
for _, text := range []string{"£450 per WK", "£18/hr", "£40000 p.a."} {
	m := g.Parse(text)
	fmt.Println(m.At("amount.~"), m.At("unit.made")) // made: the canonical value
}
// 450 Week
// 18 Hour
// 40000 Year
```

A list the grammar doesn't use, `seniority` here, is simply not called.
For titles, the same way, see [Titles](EXAMPLES.md#titles) in the
examples.

### The sections of a description

`Split` gives the text between a grammar's matches, with each match's
text in place between them, so a grammar of headings cuts a description
into sections:

```go
headings := grammar.MustCompile(`
grammar Headings {
    token TOP { <?after [ ^ | '.' \s+ ]> <heading> ':' }
    token heading { :i [ 'What you will do' | 'About you' | 'How to apply' ] }
}`)
text := "We're hiring. What you will do: lead a team. About you: you like Go. How to apply: email us."
pieces := headings.Split(text).StrSlice()
fmt.Printf("%q\n", pieces[0]) // before the first heading
for i := 1; i+1 < len(pieces); i += 2 {
	fmt.Printf("%-20s %q\n", strings.TrimSuffix(pieces[i], ":"), strings.TrimSpace(pieces[i+1]))
}
// "We're hiring. "
// What you will do     "lead a team."
// About you            "you like Go."
// How to apply         "email us."
```

The pieces alternate, text then heading, and there's always text before
the first heading and after the last, even if it's empty. The lookbehind,
`<?after …>`, means a heading counts only at the start of the text or
after a sentence. For a match's captures, rather than its text, use
`FindAll`.

### Sharing rules between grammars

A role is a library of rules. Compile it once, and pass it to each
grammar that uses it, so money is written once for both salary and bonus:

```go
money := grammar.MustCompile(`
role Money {
    token money  { '£' \s? <amount> }
    token amount { \d+ [ ',' \d ** 3 ]* $<k>=[:i k]? }
}`)
salary := grammar.MustCompile(`
grammar Salary does Money {
    token TOP { <min=money> \s* [ '-' | to ] \s* <max=money> }
}`, money)
bonus := grammar.MustCompile(`
grammar Bonus does Money {
    token TOP { :i bonus \s+ [ of | 'up to' ] \s+ <money> }
}`, money)

fmt.Println(salary.Find("Pay: £45k - £55,000 a year").At("max.~")) // £55,000
fmt.Println(bonus.Find("plus a bonus of £5k").At("money.~"))       // £5k
fmt.Println(bonus.Find("and a bonus up to £8k").At("money.~"))     // £8k
```

A grammar can also override a role's rule, by defining its own, and
inherit from another grammar with `is`. See [Roles](REFERENCE.md#roles).

### When a grammar won't match

`ParseErr` says where the parse got furthest, and what it expected
there, which is usually where the grammar and the text part company:

```go
for _, text := range []string{"£45k - 55,000", "£45k - £55,00", "£45k - £55k pa, DOE"} {
	_, err := Pay.ParseErr(text)
	fmt.Println(err)
}
// grammar Pay: line 1, column 8: expected <money>, found "55,000"
// grammar Pay: line 1, column 14: expected \d, found end of text
// grammar Pay: line 1, column 15: expected end of text, found ", DOE"
```

Then:

- **Check what Raku does.** A grammar here is Raku's syntax, so Rakudo
  can say whether it would match:
  `raku -e 'grammar Pay { … }; say Pay.parse("£45k - 55,000")'`.
  If Raku agrees, the grammar needs changing; if it doesn't, it may be a
  difference listed in [Differences from Raku](REFERENCE.md#differences-from-raku).
- **Check the assumptions.** Most surprises are a token not
  backtracking, or a space that means nothing: see
  [What you might assume](REFERENCE.md#what-you-might-assume).
- **Check a grammar built in Go.** `Compile` checks a grammar for
  mistakes, but one built in Go is only checked when first used. `Check`
  finds them sooner:
  ```go
  g := grammar.New("G").Token("TOP", grammar.Call("money"))
  fmt.Println(g.Check()) // grammar G: rule TOP calls <money>, which doesn't exist
  ```
- **A match that runs away** stops at `StepLimit` or `DepthLimit`, which
  `ParseErr` returns as an error. See
  [Runaway matches](REFERENCE.md#runaway-matches).

There's no trace of a parse yet, showing each rule tried; it's in the
[gaps](REFERENCE.md#gaps-and-possible-additions).

## A regex or a grammar?

The same job, pay from a short text, with a regex and with the `Pay`
grammar. Both give the same answers on these inputs:

```go
var payRe = regexp.MustCompile(`£\s?(\d{1,3}(?:,\d{3})*(?:\.\d+)?)([kK])?(?:\s*(?:-|–|to)\s*£\s?(\d{1,3}(?:,\d{3})*(?:\.\d+)?)([kK])?)?(?:\s*((?i:p\.a\.|pa|per\s+(?:annum|year|hour|day))))?`)

amount := func(num, k string) float64 {
	n, _ := strconv.ParseFloat(strings.ReplaceAll(num, ",", ""), 64)
	if k != "" {
		n *= 1000
	}
	return n
}
sm := payRe.FindStringSubmatch("£45k – £55,000 per annum")
fmt.Println(amount(sm[1], sm[2]), amount(sm[3], sm[4]), sm[5]) // 45000 55000 per annum
```

| Text | Regex | Grammar |
|---|---|---|
| `£45k – £55,000 per annum` | 45000–55000, per annum | 45000–55000, per annum |
| `£18.50 per hour` | 18.5, per hour | 18.5, per hour |
| `£30,000 to £35,000 pa` | 30000–35000, pa | 30000–35000, pa |
| `from £21k` | 21000 | 21000 |

The regex is shorter, and for one pattern that's what to use. The grammar
names each part, `<money>`, `<amount>`, `<period>`, so a change is made to
one readable rule, and a part is used again rather than copied, as the
regex repeats the amount pattern for the second figure. The captures come
back as a tree, `min.amount.~`, not numbered groups, and actions and `To`
turn them into values. When a text doesn't match, `ParseErr` says where
and why; the regex just doesn't match. See
[When not to use it](../README.md#when-not-to-use-it) for when the regex
is the better choice.
