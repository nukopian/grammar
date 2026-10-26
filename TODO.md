# TODO

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
