# grammar

[![test](https://github.com/nukopian/grammar/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/nukopian/grammar/actions/workflows/test.yml?query=branch%3Amain)

Pull structured facts out of messy text, such as job titles and pay, with
Raku-style grammars and synonym lists, in Go.

Some parts of a job posting are small languages of their own: pay
(`£45k – £55,000 per annum`), hours (`37.5 hrs/wk`), titles
(`Snr SWE – Payments (WFH)`). A single regex for any of them soon becomes
unreadable, splitting it into pieces means hand-written glue to put the
pieces back together, and even then "SWE", "Software Developer" and
"Software Engineer" come out as three different things.

## Install

```sh
go get github.com/nukopian/grammar
```

It needs Go 1.25 or later, and brings in `scalar`, `regexp2` for `Re`'s
Perl-style regexes, and `x/text` for reading text in NFC.

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

## What's in it

| For | Use | Raku |
|---|---|---|
| Writing a grammar | `Compile` and `MustCompile`, from Raku's syntax: `grammar`, `token`, `rule` and `regex`; or built in Go, with `New`, `Token`, `Regex`, `Seq`, `Alt`, `Lit`, `Class`, `Call` and the rest | `grammar G { token TOP { … } }` |
| Capturing | `<name>`, `<alias=name>`, `( … )`, `$<name>=[ … ]`; `Call`, `Alias`, `Capture`, `Named` | the same |
| Repeats and alternatives | `*`, `+`, `?`, `** m..n`, separators with `%` and `%%`, frugal `*?`; `\|` for the longest alternative, `\|\|` for the first; `Star`, `Plus`, `Repeat`, `Alt`, `First` | the same |
| Looking around, matching again | `<?before …>`, `<!after …>`, `$<name>`, `$0`; `Before`, `NotAfter`, `Backref` | the same |
| Synonyms | `NewWordList`, with `Add` or `AddRules` in the synonym format Elasticsearch and Solr use, and `FoldCase`, `LooseSpacing`, `JoinedSpacing`, `Plurals`, `WholeWords`, `ExactAcronyms` and `MapOnly`; `@list` in a rule, its canonical value as `made` | `@list`, without canonical values |
| Patterns from data | `NewPatternList`, `<@name>` | `<@array>` |
| Sharing rules | roles with `NewRole` and `Does`, inheritance with `Extend`, protos with `Proto` and `:sym<…>` | `role`, `does`, `is`, `proto token` |
| Parsing and searching | `Parse`, `Subparse`, `Find`, `FindAll`, `Split`; `Pos` and `Rule` to start elsewhere; `Overlap` for every match, overlaps included | `.parse`, `.subparse`, `m:g//`, `split(:v)`, `:ov` |
| Reading a match | ordinary `scalar` data: `At("min.amount.~")`, `Or`; `Caps` and `Chunks` | `$/`, `.caps`, `.chunks` |
| Values from a match | `Actions`, methods named after rules, whose results are `made`; `To[T]` into a Go struct, `ToAll[T]` for every match `FindAll` finds, and `Values` | action classes, `make` and `.made` |
| When text doesn't match | `ParseErr` and `SubparseErr`, saying where the parse got furthest and what it expected; `Check` for mistakes in a grammar; `StepLimit` and `DepthLimit` for runaway matches | |

For how each feature behaves, see the [reference](docs/REFERENCE.md); for
a working example of every function and method, see the
[examples](docs/EXAMPLES.md).

## When not to use it

- **For one pattern.** A regex does that. `grammar` is for text with
  structure: several pieces, each following rules, that you'd otherwise
  glue together by hand.
- **At high volumes, once a grammar has settled.** Go's parser generators
  are faster: Participle is about one and a half times as fast on a pay
  string. `grammar` is for finding out whether rules can do the job, and
  keeping them readable; port a settled rule set if you need the speed
  (see [Handing a grammar on](docs/REFERENCE.md#handing-a-grammar-on)).
- **For text that doesn't follow rules.** A language model, or comparing
  embeddings, does better with wording nobody anticipated.
- **When you need to watch a parse to debug it.** There's no trace yet:
  `ParseErr` says where a parse got furthest and what it expected, but
  not which rules it tried on the way.
- **When a panic is unacceptable.** `Parse`, `Find` and the rest panic
  when a match runs past `StepLimit` or `DepthLimit`, and `MustCompile`
  on a malformed grammar. `ParseErr`, `SubparseErr` and `Compile` return
  errors instead.

## Documentation

- [How-to guide](docs/HOWTO.md): a quick start, a first grammar for pay,
  recipes for common tasks, and the same job done with a regex and with a
  grammar.
- [Reference](docs/REFERENCE.md): each feature in depth: writing grammars,
  word lists, roles, matches, parsing, errors, testing against Raku, and
  what's not done yet. If you know regexes, start with
  [What you might assume](docs/REFERENCE.md#what-you-might-assume): a
  grammar looks like a regex, but doesn't always behave like one.
- [Examples](docs/EXAMPLES.md): one working example for every function
  and method, with the output it prints.
- [From Raku to Go](docs/RAKU.md): a phrasebook of tasks in Raku, with
  `grammar` and in plain Go, all three checked against each other.
- [The process](https://github.com/nukopian/scalar/blob/main/docs/PROCESS.md): how `grammar` and
  [`scalar`](https://github.com/nukopian/scalar) were made, as a
  partnership with Claude in which judgement stayed with Iain.
- [Changes](CHANGELOG.md): what each release changed, and what's waiting
  for the next.
- [Roadmap](ROADMAP.md): what's planned, and why.
- [Maintaining](docs/MAINTAINING.md): the guide to branches, releases,
  and the git and GitHub work done by hand.

PDFs of the documents, for reading offline, are built on demand by
`scripts/readme-pdf.sh`.

## Status

**Experimental.** It works and is well tested, against Rakudo itself and
by fuzzing, and a pay grammar agreed with a language model's extraction
on all 16 postings tried so far, but it hasn't yet met real volumes, and
the API may still change before 1.0. What's planned is in the
[roadmap](ROADMAP.md). What it doesn't do yet is listed, with what each
would take, in
[Gaps and possible additions](docs/REFERENCE.md#gaps-and-possible-additions).

## Security

What's checked, and how often, so you can judge for yourself:

- **Known vulnerabilities:** govulncheck checks what `grammar` calls, in
  its dependencies (`scalar` among them) and in the standard library, on
  every push
  ([test](https://github.com/nukopian/grammar/actions/workflows/test.yml))
  and every Monday against the latest release
  ([vulnerabilities](https://github.com/nukopian/grammar/actions/workflows/vuln.yml)).
  GitHub's Dependabot alerts watch the dependencies too.
  `golang.org/x/text` is held at v0.41.0 or later, the oldest without
  known vulnerabilities.
- **Code:** `go vet` and staticcheck; the tests with the race detector
  on Linux, and on macOS and Windows; and a run of each fuzz target,
  `FuzzCompile`, `FuzzMatch` and `FuzzWords`, on every push.
- **Secrets:** gitleaks scans the whole history on every push, and
  GitHub's secret scanning and push protection are on.
- **Provenance:** every commit and release tag is signed, and `main`,
  the latest release, is protected: signed commits, a linear history and
  CI passing, with no force-pushes. A published version is never changed;
  Go's checksum database records each one.
- **Dependencies:** `go.mod` and `go.sum` list them with their checksums,
  and tools such as govulncheck and pkg.go.dev read them directly. GitHub
  exports them as an SPDX SBOM, from Insights → Dependency graph → Export
  SBOM.

To report a vulnerability, see the
[security policy](https://github.com/nukopian/grammar/security/policy):
reports are made privately, through the Security tab.

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

