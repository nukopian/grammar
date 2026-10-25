# To do

Work planned for the next session. CLAUDE.md's
[Planned work](CLAUDE.md#planned-work) has the rules: ask whether each
task should still be done, remove it in the commit that completes it, and
delete this file with the last one. It must be gone before a release.

## jdc-lab's TODO.md is now on develop

**From:** jdc-lab's session, 2026-10-25 (jdc-lab abf51d723bf3437e9aecf662612b1bc9cc90187d)
**Waiting on:** nothing

**What:** jdc-lab now works as grammar and scalar do: work on `develop`,
`main` the latest release, and `TODO.md` on `develop`, never on `main`.
Change what grammar says about it:

- CLAUDE.md's Planned work: "(scalar's on `develop`, jdc-lab's on
  `main`)" becomes both on `develop`.
- docs/MAINTAINING.md's release step 7: "jdc-lab's on `main`", and the
  loop's `jdc-lab:main`, become `develop`.

**Why:** a task added to jdc-lab's `TODO.md` on `main` would never reach
jdc-lab's sessions, which work on `develop`, and step 7 would miss the
tasks waiting on a grammar release.

**Done when** neither file says jdc-lab's `TODO.md` is on `main`, and
step 7's loop, run as written, reads jdc-lab's `origin/develop`.
