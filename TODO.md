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
