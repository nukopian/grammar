# grammar

[![test](https://github.com/nukopian/grammar/actions/workflows/test.yml/badge.svg)](https://github.com/nukopian/grammar/actions/workflows/test.yml)

Pull structured facts out of messy text, such as job titles and pay, with
Raku-style grammars and synonym lists, in Go.

Some parts of a job posting are small languages of their own: pay
(`£45k – £55,000 per annum`), hours (`37.5 hrs/wk`), titles
(`Snr SWE – Payments (WFH)`). A single regex for any of them soon becomes
unreadable, splitting it into pieces means hand-written glue to put the
pieces back together, and even then "SWE", "Software Developer" and
"Software Engineer" come out as three different things.

## Why this exists

I'd never written a grammar, or used Rakudo, though I knew of both. I came
to them through a practical need. Language models extract structured data
from job postings very well, but at a cost for every posting, and much of
what they extract (pay, titles, working arrangements) follows rules. I
remembered that grammars were one of Raku's biggest selling points, and
wondered whether rules could do that part locally.

For the reasons behind
[`scalar`](https://github.com/nukopian/scalar#rationale), I wanted that in
Go without the usual cost. Go's parser generators, such as Participle, are
faster, about one and a half times as fast on a pay string. But each
grammar there is Go code, with types and struct tags, and often a lexer to
design first. That's a lot of schlepping just to find out whether rules
can do a job at all. `grammar` borrows from where the ideas are best
worked out:

- Raku's grammars, roles and protos, checked against Rakudo itself.
- Synonym rules in the format Elasticsearch and Solr use, so word lists
  can come from a synonym store.

Rules read like the text they match, the phrases are data anyone can edit,
and changing a rule means changing a line. It's for getting from "could
rules do this?" to a measured answer quickly: its pay grammar went from
first sketch to agreeing with a language model on every posting tried, in
a day. It's also for keeping the rules readable once they work. At about
9 µs a parse, it gets through the 134,000 postings it was built for in
seconds. Where production volumes need more, a rule set that has settled
can be ported.

It's also where everything came together for me: a new language, a new
kind of tool, and a new way of working, building it with Claude.

This package does the structured parts locally, with three ideas that
work together:

- **Grammars** say where each kind of thing goes: a seniority, then a title,
  then optionally a team, then a working arrangement in brackets. They're
  written in Raku's grammar syntax, which reads much as you'd describe the
  text in words, or built in Go.
- **Word lists** say what counts as each kind of thing, and what it means:
  every synonym maps to a canonical value, so "SWE", "Software Developer"
  and "software  engineer" all come out as "Software Engineer". They take
  synonym rules in the format Elasticsearch and Solr use (`pm => product
  manager, project manager`), so they can be filled from a synonym store.
- **Typed results:** `grammar.To` fills a Go struct straight from a match,
  each capture filling the field of the same name, and actions turn what
  matched into any other value you need.

```go
titles := grammar.NewWordList("titles", grammar.FoldCase(), grammar.LooseSpacing())
titles.AddRules("Software Engineer, Software Developer, SWE, Developer")
// ... and lists for seniority and arrangements, the same way

g := grammar.MustCompile(`
grammar Title {
    token TOP         { [ <seniority> \s+ ]? <title> [ \s* <.dash> \s* <team> ]? [ \s* '(' <arrangement> ')' ]? }
    token seniority   { @seniority }
    token title       { @titles }
    token arrangement { @arrangements }
    token dash        { '-' | '–' | '|' }
    token team        { \w+ [ ' ' \w+ ]* }
}`, seniority, titles, arrangements)

type Title struct{ Seniority, Title, Team, Arrangement string }
t, err := grammar.To[Title](g.Parse("Snr SWE – Payments (WFH)"))
// {Seniority:Senior Title:Software Engineer Team:Payments Arrangement:Remote}
```

The whole example, with all three lists, is
[Titles](docs/EXAMPLES.md#titles) in the examples, and runs as a test.

Two more things set it apart:

- **Roles** let grammars share rules: one library of money and period
  tokens for pay, bonus and allowance grammars, or one lexicon for titles
  and requirements. See [Roles](docs/REFERENCE.md#roles).
- **It behaves as Raku does, checked against Raku itself.** The tests run
  dozens of grammars through both this package and Rakudo, and compare the
  results. You don't need to know Raku to use it, but if you do,
  nothing should surprise you. See
  [Testing against Raku](docs/REFERENCE.md#testing-against-raku).

**Status: experimental.** It works and is well tested, and a pay grammar
agreed with a language model's extraction on all 16 postings tried so far,
but it hasn't yet met real volumes, and the API may still change. What it
doesn't do yet is listed, with what each would take, in
[Gaps and possible additions](docs/REFERENCE.md#gaps-and-possible-additions).

## A first grammar

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
[tables in the reference](docs/REFERENCE.md#writing-a-grammar) say what
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
all work on it. [The match](docs/REFERENCE.md#the-match), in the
reference, has the details, and the examples'
[Match tree](docs/EXAMPLES.md#match-tree) prints a real one.

## Documentation

- [Reference](docs/REFERENCE.md): each feature in depth: writing grammars,
  word lists, roles, matches, parsing, errors, testing against Raku, and
  what's not done yet. If you know regexes, start with
  [What you might assume](docs/REFERENCE.md#what-you-might-assume): a
  grammar looks like a regex, but doesn't always behave like one.
- [Examples](docs/EXAMPLES.md): one working example for every function
  and method, with the output it prints.
- [The process](https://github.com/nukopian/scalar/blob/main/docs/PROCESS.md): how `grammar` and
  [`scalar`](https://github.com/nukopian/scalar) were made, as a
  partnership with Claude in which judgement stayed with Iain.

PDFs of the documents, for reading offline, are built on demand by
`scripts/readme-pdf.sh`.

## Licence

Copyright 2026 Iain Campbell. `grammar` is free software; you can
redistribute it and/or modify it under the terms of the Artistic License
2.0, as Rakudo is (`Artistic-2.0`); see [LICENSE](LICENSE). It uses the
`scalar` package, which is under the same terms as Perl itself; see
[its licence](https://github.com/nukopian/scalar#licence).

## Acknowledgements

The ideas here aren't new. `grammar` follows Raku's grammars, and is
tested against Rakudo to keep its behaviour faithful to theirs; the match
values it builds come from `scalar`, which borrows Perl's way with data.
Thanks to the Raku and Perl communities for decades of thought about
parsing text well.

`grammar` was conceived and directed by Iain Campbell (nukopian). He set
its rules, which are in [CLAUDE.md](CLAUDE.md): Raku as the baseline, checked against Rakudo itself; tests
first; speed changes kept only when measured; and features added when
real use called for them. He decided what to build and what not to, and
reviewed and committed each change. Most of the code, tests and
documentation were written by Claude, Anthropic's AI model, working to
that direction; nearly all its commits credit Claude in a
`Co-Authored-By` trailer. How the partnership worked is in Iain's essay,
[The process](https://github.com/nukopian/scalar/blob/main/docs/PROCESS.md).

`grammar` started inside [job-meta](https://github.com/nukopian/job-meta),
a proof of concept for extracting metadata from job postings, and was
extracted with its history, so some commit messages mention packages that
stayed behind.

