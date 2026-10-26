# TODO

## Take up scalar v0.6.0

**From:** scalar's session, 2026-10-26 (scalar 5596f10cd4161e5c18a5ee93f12c3bcaf6abb547)
**Waiting on:** nothing

**What:** move grammar from scalar v0.5.9 to v0.6.0, released
2026-10-26, and say so in grammar's CHANGELOG. Then release grammar if
its own rules call for it, so that job-meta can take both.

**Why:** v0.6.0 keeps the digits of a number read as text when a
`float64` can't hold them, so long decimals and big ids from JSON, YAML
or a dump are written back exactly; a whole number beyond `int64`
prints as an integer; `%` and `++` follow Perl further (`1e19 % 7` is
`3`, `"007"++` is `"008"`); and `To` and `Decode` fill `*big.Rat` and
the like from a number. Some output changes, which is why it's a minor
version: see scalar's
[v0.6.0 notes](https://github.com/nukopian/scalar/releases/tag/v0.6.0).

**Files:** `go.mod`, `go.sum`, `CHANGELOG.md`; anything whose output
shows a number beyond `int64` or a long decimal, or uses `%` or `++` on
strings of digits.

**How to check it's done:** without the workspace, so that the check is
against the published module:

```
GOWORK=off go get github.com/nukopian/scalar@v0.6.0
GOWORK=off go mod tidy
GOWORK=off go test ./...
```

grammar's tests already passed against scalar's develop, through
`go.work`, before the release, so a failure here means something
differs between the published module and that checkout.

## Guard CHANGELOG's Unreleased with a test, as scalar does

**From:** scalar's session, 2026-10-26 (scalar 8b53b312060f15cedf1f260d6e1c1a8af081f6e9)
**Waiting on:** nothing

**What:** add a test that fails if `CHANGELOG.md`'s Unreleased section
is blank, or holds both `Nothing yet.` and entries; it holds either
exactly `Nothing yet.` or entries. scalar's
[`changelog_test.go`](https://github.com/nukopian/scalar/blob/develop/changelog_test.go)
has `TestChangelogUnreleased`, and `TestCheckUnreleased`, which shows
the check catches each slip and passes what a release and a first change
leave; it can be copied as it is. Name the test in `CLAUDE.md`'s commit
rules and in `docs/MAINTAINING.md`'s step 2, as scalar's now do.

**Why:** scalar's v0.6.0 release left Unreleased blank, where every
earlier release had left `Nothing yet.`: its MAINTAINING.md said "add an
empty Unreleased", and nothing checked it. grammar's step 2 already says
`Nothing yet.`, and its CHANGELOG has it, so this is a guard, not a fix:
CI would then catch the slip instead of Iain.

**Files:** a new `changelog_test.go`, `CLAUDE.md`, `docs/MAINTAINING.md`.

**How to check it's done:** the test passes on today's `CHANGELOG.md`,
fails with Unreleased blanked or with an entry added above
`Nothing yet.`, and CI runs it.
