# To do

## Fewer allocations in the matcher, measured on jdc-lab's facts

**From:** jdc-lab's session, 2026-10-31 (jdc-lab 8c9df97c5d9ab378637b3c95a2bb75b2d8193823)
**Waiting on:** jdc-lab's check of the commit, and Iain's go-ahead

**What:** what's left of roadmap 9, whose plan, measurements and checks
are in its [change record](docs/changes/2026-10-31-r9-fewer-closures-in-the-matcher.md).
Steps 1 to 5 are done: `once` keeps its results in slots on the state,
and `seq` steps over atoms in a loop, both behind `fastPaths`.

**Steps:**

6. jdc-lab checks the commit named in the task added to its `TODO.md`,
   and reports to Iain, who gives the go-ahead for a release.
7. Release, then look in jdc-lab's `TODO.md` for the task waiting on it.
   Finish the record, saying the release, and move 9 out of the
   roadmap's Now.

**Done when** jdc-lab finds every output unchanged, Iain has given the
go-ahead, and the release is out.
