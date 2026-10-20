# Changes

What changed in each release of grammar, newest first. Each release's
entry is taken from its signed tag message. Versions follow
[semantic versioning](https://semver.org): before v1, a breaking change
bumps the minor version.

Work collects under Unreleased on the `develop` branch, and becomes a
release's entry when `main` is moved to it and tagged (scalar's
[MAINTAINING.md](https://github.com/nukopian/scalar/blob/main/docs/MAINTAINING.md)
has the steps, for both repositories).

## Unreleased

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
