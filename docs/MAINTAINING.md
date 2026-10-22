# Maintaining

How grammar is kept: its branches, releases, and the git and GitHub work
done by hand. It's for whoever maintains it, Iain and Claude now. Every
command can be pasted as it stands. The few that need a value say so on
the line before, and set it in a variable on their first line.

grammar is kept as [scalar](https://github.com/nukopian/scalar) is, and
this guide follows
[scalar's](https://github.com/nukopian/scalar/blob/main/docs/MAINTAINING.md)
step for step, with what's different for grammar: it depends on a tagged
scalar, its differential tests need Rakudo, and a release can change
what job-meta's programs print.

The repositories sit side by side. grammar uses scalar, and job-meta uses
both:

```
~/Git/github.com/nukopian/scalar
~/Git/github.com/nukopian/grammar
~/Git/github.com/nukopian/job-meta
```

## Branches

- `develop` is where work happens. Every commit goes there, and CI runs
  on every push to it.
- `main` is the latest release, and nothing else. It moves only when a
  release is made, by a fast-forward to `develop`, and the release's tag
  is on it. Visitors to GitHub see `main`, so its README and docs describe
  what `go get` installs.
- There are no feature branches, for now: changes are made one at a time.

`main` is protected on GitHub. Pushes to it need signed commits, a linear
history (no merge commits) and CI passing on every job, and it can't be
force-pushed or deleted. This holds for admins too. To see the settings:

```
gh api repos/nukopian/grammar/branches/main/protection --jq '{signatures: .required_signatures.enabled, linear: .required_linear_history.enabled, checks: .required_status_checks.contexts, force_push: .allow_force_pushes.enabled, admins: .enforce_admins.enabled}'
```

For a fresh clone, or a checkout still on `main`, switch to `develop`:

```
cd ~/Git/github.com/nukopian/grammar
git fetch origin
git switch develop
git pull --ff-only
```

## The local scalar

grammar's `go.mod` asks for a tagged scalar. A git-ignored `go.work`
joins `../scalar` as well, so a change to the local scalar shows up in
grammar at once, before it's released. That's convenient for working
across both, but it means a plain `go test` here tests against the local
scalar, not the one `go.mod` names.

So `GOWORK=off` switches the workspace off, and tests grammar as anyone
who runs `go get` would get it. Use it before committing a scalar bump,
before tagging, and to test on the oldest Go `go.mod` allows, since
`go.work` names the Go that made it:

```
GOWORK=off go test ./...
```

A fresh clone has no `go.work`. To make one:

```
cd ~/Git/github.com/nukopian/grammar
go work init . ../scalar
```

## Every commit

Run these first, and check that each one succeeds. Don't pipe `go test`
through `tail` or `grep` to judge it: the pipe's exit status is the last
command's, and hides a failure.

```
gofmt -l .
go vet ./...
go test ./...
```

The Raku differential tests (`TestMatchesRaku`, `TestCapsMatchRaku` and
`TestMappingMatchesRaku`) skip without `raku` on the PATH, and CI fails
if they skip. To see that they ran here, with Rakudo 2025.11, the
reference build:

```
raku --version
go test -v -run 'Raku' . | grep -E -- '--- (PASS|FAIL|SKIP)'
```

That pipe only lists the results: judge pass or fail by the plain
`go test ./...` above.

When a fuzz target finds a failure, it writes the input to
`testdata/fuzz`. Commit it with the fix, so it's checked from then on.

`internal/docref` is identical in scalar, grammar and job-meta. A change
to it here is copied to the other two in the same session. To check the
copies match:

```
cd ~/Git/github.com/nukopian
diff -r grammar/internal/docref scalar/internal/docref
diff -r grammar/internal/docref job-meta/internal/docref
```

Commits are signed, and use conventional prefixes (`feat:`, `fix:`,
`docs:`, `test:`, `ci:`, `build:`, `refactor:`, `chore:`), saying why as
well as what. To check the signatures of the last ten (`G` is good, `N` is
none):

```
git log --format='%h %G? %s' -10
```

Add what changed for users to `CHANGELOG.md` under Unreleased, in the same
commit.

## Taking up a new scalar

When grammar needs something new in scalar, release scalar first (its
guide has the steps), then bump grammar to the new tag, without the
workspace. Set scalar's version on the first line:

```
s=v0.5.2
cd ~/Git/github.com/nukopian/grammar
GOWORK=off go get "github.com/nukopian/scalar@$s"
GOWORK=off go mod tidy
GOWORK=off go test ./...
```

Commit it as `build: scalar v0.5.2`, saying what grammar needed from it,
with a line under Unreleased in the change log if users would notice. A
scalar release that grammar doesn't need is left for job-meta alone;
bumping grammar for it would only make an unreleased change.

## Making a release

A release is a signed, annotated tag on `main`. Before v1, a breaking
change bumps the minor version (v0.1.2 to v0.2.0) and anything else the
patch (v0.1.2 to v0.1.3). Claude asks before tagging.

1. **Check `develop`:** CI is green on its last commit, the tests pass
   against the scalar `go.mod` asks for, and the history has no secrets.

   ```
   cd ~/Git/github.com/nukopian/grammar
   git switch develop
   git pull --ff-only
   gh run list --branch develop --limit 1
   grep nukopian/scalar go.mod
   GOWORK=off go test ./...
   gitleaks git --redact --no-banner .
   ```

   `go.mod` names a scalar tag, such as `v0.5.2`, and never a pseudo-version
   (`v0.5.3-0.2026...`) or a `replace` pointing at `../scalar`.

2. **Turn Unreleased into the release** in [CHANGELOG.md](../CHANGELOG.md):
   rename the heading to the version and date, as in
   `## v0.1.3 (2026-10-22)`, start the entry with a sentence that sums it
   up (it becomes the tag's title), and add an Unreleased above it saying
   `Nothing yet.`. Commit (`docs: changes for v0.1.3`), push, and wait for
   CI.

3. **Write the tag message from the entry,** and read it. The entry is
   also the GitHub release's notes, and the tag message is the same with
   the version in front. Set the version on the first line:

   ```
   v=v0.1.3
   awk -v h="## $v (" 'index($0, h) == 1 {on = 1; next} /^## / {on = 0} on' CHANGELOG.md | sed '/./,$!d' > "$TMPDIR/notes-$v.txt"
   sed "1s/^/$v: /" "$TMPDIR/notes-$v.txt" > "$TMPDIR/tag-$v.txt"
   cat "$TMPDIR/tag-$v.txt"
   ```

4. **Move `main` and tag it.** The merge is a fast-forward, so `main` gets
   exactly the commits CI tested on `develop`, with their signatures. Set
   the version on the first line:

   ```
   v=v0.1.3
   git switch main
   git pull --ff-only
   git merge --ff-only develop
   git tag -s "$v" -F "$TMPDIR/tag-$v.txt"
   git push origin main "$v"
   gh release create "$v" --verify-tag --title "$(git tag -l --format='%(contents:subject)' "$v")" --notes-file "$TMPDIR/notes-$v.txt"
   git switch develop
   ```

   The release is the page GitHub shows for the tag, and the newest is
   marked Latest in the repository's sidebar. `go get` reads only the
   tag, so a tag without a release still installs, but GitHub would show
   an older version as the latest. grammar's tags had no releases until
   2026-10-20, when they were added. `--verify-tag` refuses to make a tag
   of its own if the push didn't happen.

   If the merge says it can't fast-forward, `main` has a commit `develop`
   doesn't. Stop and find out why; don't make a merge commit.

5. **Check that the proxy serves it.** The first fetch makes the Go module
   proxy and checksum database record the tag, for good:

   ```
   v=v0.1.3
   GOPRIVATE= GOPROXY=https://proxy.golang.org go list -m "github.com/nukopian/grammar@$v"
   ```

6. **Bump job-meta,** with `GOWORK=off`, so that the check is against the
   published module and not the local checkout. A grammar release can
   change what job-meta's programs find, so save their output before the
   bump and compare it after. They read the postings in `data/in` and
   `data/out`, which are local. Set the version on the first line:

   ```
   v=v0.1.3
   cd ~/Git/github.com/nukopian/job-meta
   mkdir -p "$TMPDIR/bump-$v/before" "$TMPDIR/bump-$v/after"
   GOWORK=off go run ./cmd/grammarrules -out "$TMPDIR/bump-$v/sections" > "$TMPDIR/bump-$v/before/grammarrules.txt"
   mv "$TMPDIR/bump-$v/sections" "$TMPDIR/bump-$v/before/sections"
   GOWORK=off go run ./cmd/titleeval > "$TMPDIR/bump-$v/before/titleeval.txt"
   GOWORK=off go run ./cmd/payeval > "$TMPDIR/bump-$v/before/payeval.txt"
   GOWORK=off go get "github.com/nukopian/grammar@$v"
   GOWORK=off go mod tidy
   GOWORK=off go test ./...
   ```

   Then run the same three again into `after`, with the same version on
   the first line, and compare:

   ```
   v=v0.1.3
   cd ~/Git/github.com/nukopian/job-meta
   GOWORK=off go run ./cmd/grammarrules -out "$TMPDIR/bump-$v/sections" > "$TMPDIR/bump-$v/after/grammarrules.txt"
   mv "$TMPDIR/bump-$v/sections" "$TMPDIR/bump-$v/after/sections"
   GOWORK=off go run ./cmd/titleeval > "$TMPDIR/bump-$v/after/titleeval.txt"
   GOWORK=off go run ./cmd/payeval > "$TMPDIR/bump-$v/after/payeval.txt"
   diff -r "$TMPDIR/bump-$v/before" "$TMPDIR/bump-$v/after"
   ```

   `grammarrules` writes into the same folder both times, and it's moved
   aside after each run, because the path is printed on every line of
   its table. The three programs print the same thing on every run, so
   no difference is the usual result. A difference the release meant to
   make goes in the commit message, with the numbers; one it didn't
   mean to make is a bug in the release, fixed with a new one. Commit as
   `build: grammar v0.1.3, ...`, with what the release brings after the
   comma.

## When something goes wrong

- **A bad commit pushed to `develop`:** fix it in a new commit, or revert
  it with `git revert`, and push. Don't force-push: Iain reviews what's
  pushed, and a rewrite moves it under him.
- **A mistake in a pushed commit message:** leave it, and correct it in
  the next release's change log.
- **A bad release:** never move or delete a published tag. The proxy and
  checksum database keep it whatever happens to the repository, and
  anyone's `go.sum` records its hash, job-meta's included. Release a
  fixed version instead. To steer people off the bad one, add a `retract`
  directive to `go.mod` in the fix (`retract v0.1.3 // why`), so `go get`
  skips it and `go list -m -u` warns about it.
- **A commit made on `main` by mistake,** before it's pushed: move it to
  `develop`, and put `main` back. Do this only if `main` hasn't been pushed
  since:

  ```
  git switch develop
  git cherry-pick main
  git branch -f main origin/main
  ```

- **A push to `main` refused:** the protection did its job. Push to
  `develop` instead, or, at a release, wait for CI to pass on the commit.
- **The Raku tests skip, or CI's "Raku tests ran" step fails:** Rakudo is
  missing or broken, so nothing was compared with Raku. Locally, check
  `raku --version` says 2025.11. In CI, the Rakudo step downloads that
  release and checks its SHA-256; if rakudo.org moves it, the step fails
  rather than testing against something else.

## Rewriting history

Rarely, history has to change: a file that shouldn't have been committed,
or a package moving out. grammar's own history was rewritten once, when
it left job-meta for this repository; v0.1.0 is its first tag. A rewrite
after a tag is published would leave that tag pointing at commits the
repository no longer has, so it shouldn't happen now.

If it must, it's done in a fresh clone, never in the working checkout,
with `git filter-repo` (`brew install git-filter-repo`). filter-repo
drops signatures, so each commit is then signed again with
`git commit-tree -S`, keeping its author, committer and dates, and the
commit ids that messages quote are rewritten to the new ones.

Before pushing, run gitleaks on the rewritten history, and compare the
final tree with the old one (`git diff old-head new-head` shows nothing
but what was meant to change). Push with `--force-with-lease`, naming the
full hash the remote is expected to be at, and the branch spelled out in
full (`refs/heads/main`), so that nothing else can be overwritten. Claude
writes out that command with the real hash, ready to paste. Lift `main`'s
protection for the push, and put it back straight after.

Moving a package into a repository of its own is described in
[scalar's guide](https://github.com/nukopian/scalar/blob/main/docs/MAINTAINING.md#moving-a-package-into-its-own-repository),
from grammar's own move.

## Visibility

grammar has been public since 2026-10-20. Its whole history is
published, so gitleaks scans all of it, in CI on every push and by hand
before each tag.

## Tools

`go` (the version `go.mod` asks for, or newer), `gh`, `gitleaks`,
Rakudo 2025.11 (as `raku`) for the differential tests, `git-filter-repo`,
and for the PDFs (`scripts/readme-pdf.sh`), `pandoc` and Prince.
