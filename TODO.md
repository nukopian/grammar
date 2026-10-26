# TODO

## Release Overlap once jdc-lab has tried it

**From:** grammar's session, 2026-10-26 (grammar da7bc9b9cdc732bbed40f74bf56f312d654d3c02)
**Waiting on:** jdc-lab's session trying `Overlap` in `internal/sections`, through its `go.work` (its `TODO.md` task "Try grammar's Overlap in sections.All, before it's released")

**What:** release `Overlap()` (roadmap 13), shipped on `develop` and
held at Iain's request so jdc-lab can test it first. When jdc-lab's
session has tried it, it changes this task's Waiting on to "nothing"
and says what it found, in a `docs:` commit here. Then: if it found
nothing to change, release as MAINTAINING.md's steps say, as v0.3.2 (a
new option, nothing breaks), asking Iain before tagging; if it did, make
the change first, with a test, and update the change record.

**Why:** Iain asked for jdc-lab to test before a release
(2026-10-26). A trial here found that `:ov` also finds a phrase inside a
longer one ("profile" in "Company Profile"), which jdc-lab may want
handled differently; better found before the API is published.

**Files:** `TODO.md` (this task, removed in the commit that does it),
then the release's.

**How to check it's done:** jdc-lab's task is gone from its `TODO.md`,
this one is gone from here, and v0.3.2 is tagged with the Overlap entry.
