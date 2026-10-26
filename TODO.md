# TODO

## Release Overlap once jdc-lab has tried it

**From:** grammar's session, 2026-10-26 (grammar da7bc9b9cdc732bbed40f74bf56f312d654d3c02)
**Waiting on:** nothing: jdc-lab tried it on 2026-10-26 (below)

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

**What jdc-lab found** (2026-10-26, on grammar 91301e43f3ee58d108141dc1c74fea01ec956a7b through
`go.work`): **nothing to change before the release.** `sections.All`
calls `finder.FindAll(key, grammar.Overlap())` and drops each match
wholly inside a longer one, as the reference's Overlapping matches
suggests. Over both samples (6,167 distinct ads, 7,469 distinct
headings), compared with a run without `Overlap`:

- exactly 6 headings (22 uses) change kind, all rightly, from
  responsibilities to requirements: "To succeed in this role, you will
  need the following:" (14 uses), "To join us and be successful in this
  role you will need:" (4), and four like them;
- "Company Profile" stays company: the nested "profile" is dropped;
- nothing else moves: no heading newly mapped or unmapped, and the only
  new rows in `disagree.tsv` are those 6.

jdc-lab also suggested an option to drop the nested matches: roadmap
idea 14, not blocking this release.

**How to check it's done:** jdc-lab's task is gone from its `TODO.md`,
this one is gone from here, and v0.3.2 is tagged with the Overlap entry.
