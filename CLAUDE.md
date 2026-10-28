# Working on grammar

@../claude-practice/PRACTICE.md

The shared practice above, from
[nukopian/claude-practice](https://github.com/nukopian/claude-practice),
covers planned work, the roadmap, change records, branches, commits,
releases, wash-ups, documents and secrets. These are grammar's own rules
on top of it; where they differ, these win. grammar is developed by Iain
Campbell with Claude (see the README's Acknowledgements), and keeps in
step with scalar (`../scalar/CLAUDE.md`), so the repositories read alike
and work moves between them easily.

## What grammar is for

It pulls structured facts out of messy text, such as job titles, pay and
working hours, with Raku-style grammars and synonym word lists, in Go.
It was built for job postings, where a language model extracts metadata
well but at a cost for every posting, and much of what it extracts
follows rules. Matches come back as scalar values, so `At`, `Or` and `To`
work on them, and `To` hands them on to typed Go.

Prefer clear, forgiving APIs and good error messages over speed, unless a
measurement shows a real cost.
## Raku is the baseline

- **Where Raku has a feature, match Raku's behaviour.** When the Go and
  Raku results differ, Raku is right, unless the difference is listed
  under "Differences from Raku" in the reference. Raise any new difference
  with Iain before building it, and document it there, with when it
  matters.
- **Check what Raku does before writing an expected output.** Ask it
  directly, such as
  `raku -e 'grammar G { token TOP { a .*? b } }; say G.parse("aab")'`,
  then add the case to `rakuCases` in `raku_test.go`. Rakudo 2025.11 is
  the reference build.
- **Where grammar goes beyond Raku**, as with word lists' canonical
  values, `FoldCase`, `JoinedSpacing` and `Plurals`, or `Re` taking
  Perl-style regexes, say so in the reference, and keep a Raku-compatible
  form matching as Raku does.
- **Naming follows Go's convention,** as in scalar: Go names for the API
  (`Compile`, `FindAll`, `ParseErr`), Raku's spelling inside the grammar
  syntax.

Why: grammar brings Raku's grammars to Go. Faithfulness to Raku makes it
predictable for anyone who knows Raku, and the documented differences
make it honest for everyone else.
## Tests

- `TestMatchesRaku` and `TestCapsMatchRaku` run the same cases through
  Rakudo and compare: the match trees, and the captures in order.
  `TestMappingMatchesRaku` runs the phrasebook's tasks in Raku, grammar
  and plain Go. CI fails if any of them skipped or Raku wasn't compared. They skip without `raku` on the
  PATH; say so when you're relying on one that skipped. Any change to how
  matching behaves starts with a case there, and Raku saying what's right.
- Fuzz targets: `FuzzCompile`, `FuzzMatch` and `FuzzWords`. Run the
  relevant one after changing the code it covers, for example
  `go test -run '^$' -fuzz '^FuzzMatch$' -fuzztime 60s`. A failure it
  finds is written to `testdata/fuzz`: commit it, so it's checked from
  then on.
- Speed-ups sit behind a switch, as `skipStarts` does for `Find` and
  `fastPaths` for the matcher, with a test that runs with it on and off
  and compares the results, as `FuzzMatch`, `starts_test.go` and
  `fastpath_test.go` do, and a planted bug to show the comparison would
  notice a difference.
- grammar supports Go 1.25 and later, as `go.mod` says, as scalar does.
  Don't use newer standard-library features unless raising the minimum is
  a deliberate decision.
- grammar requires a tagged version of scalar. A git-ignored `go.work`
  joins `../scalar`, so local changes to it show up at once, as in
  job-meta. Check with `GOWORK=off` before committing a scalar bump or
  tagging grammar, so the tests run against the scalar `go.mod` asks for,
  not your local one; that's also how to test on Go 1.25, as `go.work`
  names the Go that made it.
- Run `gofmt -l .`, `go vet ./...` and `go test ./...` before every
  commit. CI (`.github/workflows/test.yml`) runs the same on every push:
  on Linux with the race detector, Rakudo, govulncheck and staticcheck,
  failing unless the Raku tests ran; on macOS and Windows with `-short`;
  and 30 seconds of each fuzz target.
- govulncheck also runs every Monday on `main`
  (`.github/workflows/vuln.yml`), so a vulnerability published after a
  release is found, and GitHub's Dependabot alerts, secret scanning and
  push protection are on. The README's Security section tells users
  this: keep it true when any of these change.
  [MAINTAINING.md](docs/MAINTAINING.md#security-checks) has the commands.

## Documents

- The documents: `README.md` (what grammar is, a synopsis, why it exists,
  what's in it and when not to use it), `docs/HOWTO.md` (a quick start,
  recipes, and a regex beside a grammar), `docs/REFERENCE.md` (each
  feature in depth, what you might assume, testing against Raku, and the
  gaps), `docs/EXAMPLES.md` (one generated example per function) and
  `docs/RAKU.md` (a Raku → grammar → plain Go phrasebook, whose every row
  `TestMappingMatchesRaku` runs in all three and compares), as scalar's
  are laid out, with `CHANGELOG.md` (what each release changed),
  `ROADMAP.md` (what grammar means to do next, and why),
  `docs/changes/` (a record of each planned change: the plan, and what
  happened) and `docs/MAINTAINING.md` (branches, releases, and the git
  and GitHub work done by hand) beside them. The how-to's code is compiled and run, with its
  output compared, whenever it changes.
- Every exported function and method has an example in the right
  `example_*_test.go` file, which `TestEveryFeatureHasExample` enforces.
  `docs/EXAMPLES.md` is generated from those examples: edit the
  examples, then run `go test -run TestReference -update`.
- Examples use job-posting text, such as pay, hours and titles, as the
  postings put it, unless an example needs something neutral.
- "Gaps and possible additions" in the reference lists what grammar doesn't
  do yet, when you'd need it, what it would take, and the effort, for a
  future reader with only the package as context. Keep it current: add a
  gap when one's found, and remove it when it's closed.
- `internal/docref` is a copy of scalar's, kept identical, byte for byte.
  Change it in one repository and copy the change to the other in the
  same session. job-meta has a copy too, but its code has been frozen
  since 2026-10-26, so it's left as it is.
- Compile Markdown snippets to check them, using a temporary test file
  that you delete afterwards.
- The PDFs (see the practice) are never committed here: module
  downloads would carry them.

Why: the documents are how people learn the package, its author
included, and gaps found by reading them have led to real features.

## How changes are made here

- **Benchmarks:** `BenchmarkParse`, `BenchmarkFindAll` and the others.
  Reverted ideas and their numbers go in the reference's gaps.
- **Wash-ups** re-run the Raku differential tests to show nothing else
  changed.
- **Dogfooding:** grammar is used in jdc-lab's `internal/sections`, and
  was in job-meta's programs until job-meta's code was frozen on
  2026-10-26, at grammar v0.3.0. A fix is checked by reworking jdc-lab's
  code against this checkout, its output unchanged.
- **The other repositories** whose `TODO.md` this one hands tasks to,
  and checks at a release, are scalar's and jdc-lab's, both on `develop`.
- **`TestChangelogUnreleased`** checks the CHANGELOG's Unreleased
  section. [MAINTAINING.md](docs/MAINTAINING.md) has the release steps
  and the git and GitHub work done by hand.
