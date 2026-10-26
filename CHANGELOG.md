# Changes

What changed in each release of grammar, newest first. Each release's
entry is taken from its signed tag message. Versions follow
[semantic versioning](https://semver.org): before v1, a breaking change
bumps the minor version.

Work collects under Unreleased on the `develop` branch, and becomes a
release's entry when `main` is moved to it and tagged
([docs/MAINTAINING.md](docs/MAINTAINING.md) has the steps).

## Unreleased

- Build: needs scalar v0.5.9, up from v0.5.7. grammar's behaviour doesn't
  change, but code of yours that calls scalar's `S` with a replacement
  string such as `"$1"` runs about 1.6 times as fast, can use `$+` and
  `$+{name}`, and gets nothing for a group that doesn't exist, as in Perl.

## v0.2.1 (2026-10-25)

Documentation only: a roadmap

No change in behaviour or in the API: no Go code changed. Needs scalar
v0.5.7, as before.

- Docs: a [roadmap](ROADMAP.md) says what grammar means to do next, and
  why: what's being worked on (at most three items, and none yet), what's
  decided (a parse trace, limit errors that say where, and code
  assertions), and ideas worth keeping. The README and the reference's
  gaps link to it.

## v0.2.0 (2026-10-25)

Reading a match changes nothing, as in Raku

A breaking change, hence v0.2.0: writing into a no-match now panics, and
code that relied on a read leaving an empty capture in a match will find
it missing. Two new functions come from grammar's first use in jdc-lab:
`ToAll`, `To` for `FindAll`, and `WordList.Canonicals`. Needs scalar
v0.5.7.

- New: `WordList.Canonicals()` gives a list's canonical values, each once,
  in the order they were first added, by `Add`, `AddAs`, `AddRule` or
  `AddRules`. A rules file's order can then be used as data, such as
  which value wins when a text has phrases of two, without reading the
  file a second time: `Phrases()` is sorted, and nothing gave the values.
- New: `ToAll[T]`, `To` for `FindAll`, reduces each match and gives a
  `[]T` in the order found, so a word list's canonical values come back as
  Go strings: `grammar.ToAll[string](g.FindAll(text))`. Before, a caller
  ranged over `FindAll(...).Items()` and read `m.At("made").String()`
  from each match.
- Changed: the undef that `Parse`, `Subparse`, `Find`, `ParseErr` and
  `SubparseErr` return for no match is locked, so reading it stays undef,
  as in Raku. Before, reading through it with `At` autovivified it into a
  hash. `m.At("made")` and then `m.Defined()` reported a match, and
  `m.String()` gave `HASH(0x…)`. Writing into a no-match now panics
  (`scalar: Not a HASH reference`). `Find` has an example showing it.
- Changed: reading a match creates nothing, as in Raku. Each match has
  scalar's new `Autovivify(false)`, so a capture it hasn't got, such as
  `At("period.~")`, reads as undef and isn't added: `AtOK`, `Exists` and
  the match's JSON no longer find an empty `period` after the read. This
  needs scalar v0.5.7, which go.mod now asks for.
- Docs: "Differences from Raku" says that `Subparse` returns undef when
  nothing matches, where Raku's `.subparse` returns a failed `Match`
  that's defined but false.

## v0.1.6 (2026-10-22)

A weekly vulnerability scan, and a Security section in the README

None of the changes is to the code or the API.

- Security: govulncheck also runs every Monday on `main`, the latest
  release, so a vulnerability published after it is found, not only at
  the next push. GitHub's Dependabot alerts, secret scanning and push
  protection are on. The README has a Security section saying what's
  checked and how often, and how to report a vulnerability.
- The release steps check every check run on the commit before pushing
  `main`, and say what to do about a check stuck "in progress" or a
  `main` refused after its tag went up, both of which happened at
  grammar v0.1.5.

## v0.1.5 (2026-10-22)

The CI badge shows main's status

None of the changes is to the code or the API.

- The README's CI badge names `main`, so it shows the latest release's
  status: without a branch, GitHub's badge said "no status".

## v0.1.4 (2026-10-22)

Planned work goes in TODO.md on develop

None of the changes is to the code or the API.

- Planned work for a later session is listed in a `TODO.md` on
  `develop`, checked at the start of each session, and gone before a
  release, so it never reaches `main`. CLAUDE.md has the rules, and the
  release steps check for it.

## v0.1.3 (2026-10-22)

grammar's own maintainer's guide

None of the changes is to the code or the API.

- Docs: [docs/MAINTAINING.md](docs/MAINTAINING.md), grammar's own
  maintainer's guide, in place of pointing to scalar's. It adds what's
  particular to grammar: testing against the tagged scalar with
  `GOWORK=off`, checking the Raku tests ran, taking up a new scalar, and
  comparing job-meta's output before and after a release.

## v0.1.2 (2026-10-20)

A folded phrase matches however its accents are spelt

None of the changes is to the API, and the docs are split and extended
as scalar's are.

- Fix: a word list that ignores case, with FoldCase or under `:i`,
  matches a phrase however its accents are spelt. A few capitals with an
  accent have no composed form where their lower case does: Y with a ring
  above is `Y` and U+030A, and in lower case it's `ẙ`, one character. The
  list held the folded phrase out of NFC, so Phrases listed a phrase
  Canonical couldn't find, and `ẙ` didn't match `Y̊`. Found by FuzzWords;
  no change in speed. A literal under `:i` still folds a character at a
  time, as the reference's differences say.
- Docs, split and extended as scalar's are: the reference and examples
  move to `docs/`; a how-to guide (`docs/HOWTO.md`); a phrasebook from
  Raku to grammar to plain Go (`docs/RAKU.md`), whose every row
  TestMappingMatchesRaku runs in all three; "What you might assume", for
  readers who know regexes; Differences from Raku as a section of its
  own; and the README gains Install, What's in it, When not to use it and
  Status, with v0.1.1's speed figures. It links
  [The process](https://github.com/nukopian/scalar/blob/main/docs/PROCESS.md),
  the essay on how grammar and scalar were made.
- CI scans the whole history for secrets with gitleaks on every push, and
  fetches scalar publicly: the deploy key is gone.
- This change log, and a `develop` branch for work: `main` is the latest
  release, and moves only at the next.
- The repository is public, from 2026-10-20.

## v0.1.1 (2026-10-19)

Two fast paths in the matcher. None of the changes is to the API.

- Inside a token, an atom that can match only one way (a literal, class,
  anchor, regex or backreference) is matched without the closure that
  committing to a first match needs, and a greedy repeat of a class or
  literal runs as a plain loop. A pay parse goes from 12.5 to 9.3 µs and
  from 532 to 359 allocations; FindAll on a posting is 8% faster. Both
  sit behind fastPaths, with tests comparing every result, failure report
  and step count, also under tight StepLimit and DepthLimit, and
  FuzzMatch checks them too.
- Requires scalar v0.5.1, whose HV.Set makes no placeholder for a new
  key: 351 allocations per pay parse.
- The README's gaps record the experiments: what was kept, what was tried
  and reverted, and what wasn't pursued, with their numbers. A
  git-ignored go.work joins ../scalar for local work.

## v0.1.0 (2026-10-19)

grammar as a project of its own: Raku-style grammars, synonym word lists
and roles for Go, extracted from job-meta with its history (54 commits
from 15 October). It requires scalar v0.5.0 and Go 1.25 or later. CI tests
Go 1.25 and the newest on Linux, with the Raku differential tests run
against Rakudo 2025.11, as well as macOS, Windows and fuzzing.
