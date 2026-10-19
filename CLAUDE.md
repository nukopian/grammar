# Working on grammar

These are the working rules for this repository. grammar is developed by
Iain Campbell with Claude (see the README's Acknowledgements), and they
follow scalar's (`../scalar/CLAUDE.md`) as far as they apply, so that the
repositories read alike and work moves between them easily. Each rule
comes with the reason for it, and the why behind them all is Iain's essay,
[The process](https://github.com/nukopian/scalar/blob/main/docs/PROCESS.md).

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
  Rakudo and compare: the match trees, and the captures in order. They skip without `raku` on the
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
- scalar is private, so CI reads it with a deploy key: read-only, added to
  scalar under Settings → Deploy keys as "grammar CI: read scalar", with
  its private half in grammar's `SCALAR_DEPLOY_KEY` secret. To replace it,
  make a new ed25519 pair, add the public half to scalar and remove the
  old one, then set the secret again with
  `gh secret set SCALAR_DEPLOY_KEY --repo nukopian/grammar < key`.

## Documentation keeps up with the code

A feature isn't done until its documentation is.

- The documents: `README.md` (what grammar is, a synopsis, why it exists,
  what's in it and when not to use it), `docs/HOWTO.md` (a quick start,
  recipes, and a regex beside a grammar), `docs/REFERENCE.md` (each
  feature in depth, what you might assume, testing against Raku, and the
  gaps) and `docs/EXAMPLES.md` (one generated example per function), as
  scalar's are laid out. The how-to's code is compiled and run, with its
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
  Change it in one repository and copy the change to the others (scalar
  and job-meta) in the same session.
- Compile Markdown snippets to check them, using a temporary test file
  that you delete afterwards.
- Every link in the documents must resolve, including the `#anchor`s.
- PDFs of the documents are for reading offline, and git-ignored:
  `scripts/readme-pdf.sh` builds them (pandoc and Prince). Don't commit
  them; module downloads would carry them.

Why: the documents are how people learn the package, its author
included, and gaps found by reading them have led to real features.

## How changes are made

- **Measure, don't predict.** For a speculative performance change:
  record baseline benchmarks (`BenchmarkParse`, `BenchmarkFindAll` and the
  others), and profile first, because the profile's share sets a ceiling
  on the gain. Keep what pays, revert what doesn't, and record reverted
  ideas and their numbers in the reference's gaps so they aren't retried
  blind.
- **Wash-ups.** After a run of features, review them together for
  crashes, memory, correctness, inefficiency and slop. Reproduce each
  finding before reporting it, rank the findings, and write failing tests
  before fixes. Re-run the Raku differential tests to show nothing else
  changed.
- **Dogfooding.** grammar is used in job-meta's programs (pay, hours,
  titles, sections and occupation coding). Note where it fought you, and
  fix the frequent friction. Check the fix by reworking those programs
  with their output unchanged.

## Commits

- Conventional commits, such as `feat: ...`, `fix: ...`, `docs: ...`,
  `test: ...` or `ci: ...`. Say why as well as what.
- Claude commits and pushes changes once they're done and checked, and
  Iain reviews them. Commits are signed.
- Every commit ends with a `Co-Authored-By` trailer for the Claude model
  that worked on it, currently
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Keep the README's Acknowledgements accurate: credit goes where the work
  was done.
