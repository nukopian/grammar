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
- gitleaks scans the whole history for secrets: in CI on every push, and
  by hand (`gitleaks git --redact .`) before a repository goes public,
  before a tag, and after any history rewrite, before it's pushed. A leak
  stops the work like a failing test: once history is published, it
  can't be taken back.

## Documentation keeps up with the code

A feature isn't done until its documentation is.

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
  changed. The wash-up before a release also checks `ROADMAP.md`: what
  shipped is out of it, Now is what's really being worked on, and the
  ideas are pruned.
- **Dogfooding.** grammar is used in job-meta's programs (pay, hours,
  titles, sections and occupation coding). Note where it fought you, and
  fix the frequent friction. Check the fix by reworking those programs
  with their output unchanged.

## Planned work

- **`ROADMAP.md` holds what grammar means to do:** features and
  outcomes, not session tasks, each saying what it serves. **Now** is
  what's being worked on, each item a paragraph and a link to its
  change record, which holds the full plan in `TODO.md`'s task format,
  and at most three items, so it stays a queue, not a
  fire-hose (Iain, 2026-10-25): another starts only when one ships or
  goes back to Next; **Next** is decided but not started (what, why, done
  when); **Later** is meant to be done, but not yet planned, in a line
  or two; **Ideas** might be done, or never, kept so they aren't lost
  (Iain, 2026-10-26): a numbered heading, the date it was added, and a
  line or two, no Serves line until promoted, newest last. Most useful
  first in the others. The CHANGELOG is the past, the README the
  present, and the roadmap the future.
- **Every roadmap item has a number,** in its heading (`### 7. Title`,
  never a Markdown numbered list, which renumbers itself): the next one
  when it's added, in whichever group, kept as it moves, and never used
  again. The opening says which comes next. A shipped item's CHANGELOG
  line ends "(roadmap 7)". Each wash-up lists the ideas by number, and
  Iain keeps, promotes (number kept, Serves added) or drops each
  (deleted; the commit message names its number). When Iain mentions an
  idea in passing, offer in one line to add it ("add as idea 12?"), and
  add it in a `docs:` commit.
- **A planned change keeps a record** in `docs/changes/` (Iain,
  2026-10-26): opened when work starts on a roadmap item or a `TODO.md`
  task with steps, named `2026-10-26-short-title.md` for that day,
  holding the plan word for word, then what happened: the outcome,
  changes from the plan and why, measurements, what was tried and
  reverted, and what's left. The plan isn't edited once work starts.
  The record is finished in the commit that ships or drops the change,
  the CHANGELOG line links to it, and a dropped change keeps its record.
  `docs/changes/README.md` has the template.
- **The roadmap is public, on `main`,** and in every release, so people
  following grammar can see what's coming. What's kept out of the README
  is kept out of it too. Its PDF is built with the others.
- **The roadmap is kept current:** an item leaves when it ships, in the
  commit that adds it to the CHANGELOG, and a change that ships, starts
  or drops a feature updates it in the same commit.
- **`TODO.md` comes first:** its tasks take priority over the roadmap's
  items. When a session takes up an item, its next steps go into
  `TODO.md`, or are done at once.
- **At the start of a session, check for `TODO.md`** at the top of the
  repository, on `develop`. If it's there, read it, ask Iain whether each
  task should still be done, and do the ones he confirms. Remove each
  task in the commit that completes it, so the history shows which
  commit did which, and delete the file with the last one. A task he
  drops is removed too, saying why in the commit message.
- **Before each commit, `git pull --ff-only` and re-read `TODO.md`.**
  Sessions in the other repositories add tasks here, mid-session too, and
  a commit that completes a task must remove it.
- **Work planned for a later session goes into `TODO.md`,** committed on
  `develop`, with enough to do it cold: what, why, the files involved,
  and how to check it's done. Not only in Claude's memory or the chat,
  which Iain can't review and a fresh session may not have.
- **Each task opens with where it came from and what it waits on,** so
  a session can see at a glance which tasks it can do now, and whom to
  tell when one is done:

  ```markdown
  ## Mark each match with scalar's Autovivify(false)

  **From:** scalar's session, 2026-10-25 (scalar 5fd3f0dd76c34de50c011d0389719abf85dcc0f2)
  **Waiting on:** a scalar release with Autovivify
  ```

  `From` names the repository whose session added it, the date, and the
  commit that prompted it. If that work isn't committed yet, it gives the
  last commit and adds "work uncommitted". `Waiting on` is "nothing" when
  the task can be done now.
- **A task added to another repository's `TODO.md`** (scalar's and
  jdc-lab's, both on `develop`) **is committed and pushed there at
  once,** after a `git pull --ff-only`, as its own `docs:` commit (Iain,
  2026-10-25). `TODO.md` changes only through commits: an uncommitted
  task blocks that repository's pulls when incoming commits touch the
  file, and is invisible to the session that finishes it. A code fix
  made there to get past a bug may still stay uncommitted.
- **A release's last step** is to look in the other repositories'
  `TODO.md` for tasks waiting on it, and tell Iain which can now be done.
  Nothing else tells their sessions it has landed.
- **`TODO.md` never reaches `main`.** `main` is a fast-forward of
  `develop`, so a release waits until the file is gone: each task done,
  or, to outlast the release, a feature moved word for word into
  `ROADMAP.md`, or a decision awaiting Iain to a Claude Doc. There are
  no GitHub issues for planned work (Iain, 2026-10-25). The release steps in
  [MAINTAINING.md](docs/MAINTAINING.md) check for it.

Why: planned work survives between sessions in the open, where Iain
reviews it like any other change, and a release never carries an
unfinished list, while the roadmap shows anyone following grammar what's
planned. scalar's CLAUDE.md has the same rule.

## Commits

- Conventional commits, such as `feat: ...`, `fix: ...`, `docs: ...`,
  `test: ...` or `ci: ...`. Say why as well as what.
- Claude commits and pushes changes once they're done and checked, and
  Iain reviews them. Commits are signed.
- Work is committed and pushed to `develop`. `main` is the latest release
  and moves only at a release, by a fast-forward to `develop`, with the
  tag on it; it's protected, so a push to it needs CI passing. Add what
  changed for users to `CHANGELOG.md` under Unreleased in the same
  commit, and update `ROADMAP.md` there if it ships, starts or drops a
  feature. Unreleased is never blank: a release leaves `Nothing yet.`
  under it, and the first change after replaces that line;
  `TestChangelogUnreleased` checks both.
  [MAINTAINING.md](docs/MAINTAINING.md) has the release steps and the git
  and GitHub work done by hand.
- Every commit ends with a `Co-Authored-By` trailer for the Claude model
  that worked on it, currently
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Keep the README's Acknowledgements accurate: credit goes where the work
  was done.
