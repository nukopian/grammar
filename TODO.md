# To do

Work planned for the next session. CLAUDE.md's
[Planned work](CLAUDE.md#planned-work) has the rules: ask whether each
task should still be done, remove it in the commit that completes it, and
delete this file with the last one. It must be gone before a release.

These come from dogfooding: grammar's first use in jdc-lab
(github.com/nukopian/jdc-lab, `internal/sections`, 2026-10-25), where a
word list of heading phrases, loaded with `AddRules`, and a one-rule
grammar (`token TOP { @sections }`) find which section of a job ad each
heading starts. The list's options did their job well: `Plurals` found
"Main Duties" from "duty", `WholeWords` kept short phrases from matching
inside longer words, and `Ambiguous` and `Canonical` check the phrase
file in jdc-lab's tests. None of what follows is a bug; each is friction worth
deciding on.

## Keep the order a word list's canonical values were added in

**From:** grammar's session, dogfooding in jdc-lab, 2026-10-25 (grammar aa5f6a4f6e083a2001f09d2582308356162e32ea)
**Waiting on:** nothing

**What:** let a word list say its canonical values in the order they were
first added, such as a `Canonicals() []string` method. Today `Phrases()`
returns the phrases sorted, and nothing gives the canonical values, so
the order of a list's rules is lost once it's loaded.

**Why:** jdc-lab's phrase file gives each section kind a rule, and
when a heading has phrases of two kinds, the kind whose rule comes
first wins. That ranking is the file's own order, kept as data, but the
word list can't give it back, so jdc-lab reads the file a second time
itself, taking the first phrase of each line that isn't indented or a
comment, to rank the kinds. That duplicates `AddRules`' own reading of
the format, and would go wrong if the two ever disagreed about it
(continued lines, say).

**Files:** `words.go`, `words_test.go`, `example_words_test.go`,
`docs/REFERENCE.md` (Word lists), `CHANGELOG.md`.

**Done when:** the method exists, gives values in the order added (for
`Add`, `AddAs`, `AddRule` and `AddRules` alike, each once), is tested and
has an example, and the reference lists it with `Phrases` and
`Canonical`.

## Three more Planned work rules, agreed with Iain

**From:** jdc-lab's session, 2026-10-25 (jdc-lab 5e5f6be33efaf551697bda1e87c5a4d8d0f7c3f6)
**Waiting on:** nothing

**What:** add three rules to CLAUDE.md's Planned work, worded to match
it, and to docs/MAINTAINING.md where it covers TODO.md and releases.
jdc-lab 5e5f6be33efaf551697bda1e87c5a4d8d0f7c3f6 has its version.

1. A task a session adds to another repository's TODO.md is committed
   and pushed there at once, after `git pull --ff-only`, as its own
   `docs:` commit (on `develop` for grammar and scalar, `main` for
   jdc-lab). TODO.md changes only through commits. A code fix made in
   another working copy to get past a bug may still stay uncommitted.
2. If the work that prompted a task isn't committed yet, From gives the
   last commit and adds "work uncommitted".
3. A release's last step: look in the other repositories' TODO.md for
   tasks waiting on it, and tell Iain which can now be done.

**Why:** an uncommitted task in another repository's TODO.md makes that
repository's `git pull --ff-only` refuse to run when the incoming commits
touch TODO.md, and the session that finishes the task never sees it in a
commit. And nothing tells a session that a release it's waiting on has
landed.

**Done when:** CLAUDE.md and MAINTAINING.md have the three rules, and
this task is removed in the same commit.
