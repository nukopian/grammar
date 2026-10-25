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

## Make FindAll's matches easier to range over from Go

**What:** consider a Go-shaped way to go through `FindAll`'s matches. It
returns a `*scalar.AV`, so a Go caller writes `.Items()` before ranging,
and then `.At("made")` on each. An iterator (`iter.Seq[*scalar.SV]`),
or a method giving each match's `made` value as a string, would read as
the rest of a Go program does.

**Why:** jdc-lab's `sections.All` lists every kind of section a
heading's phrases point to, which is this loop:

```go
for _, m := range finder.FindAll(key).Items() {
	kind := m.At("made").String()
	...
}
```

It works, and is short, but it's the one place jdc-lab's code reaches
into scalar, only to get strings back out of a list of matches.

**Files:** `grammar.go`, where `FindAll` is, a test, an
example in `example_parse_test.go`, `docs/REFERENCE.md`, `CHANGELOG.md`.

**Done when:** it's decided either way; if added, it's tested, has an
example, and the reference shows it beside `FindAll`.

## Keep the order a word list's canonical values were added in

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

## Mark each match with scalar's Autovivify(false)

**What:** once scalar releases `Autovivify` (on its `develop` since
5fd3f0dc, not yet tagged), raise grammar's scalar to that release and
set `Autovivify(false)` on each finished match in `options.result`
(`grammar.go`), so that reading a capture a match doesn't have creates
nothing. Then remove the "Reading a missing capture adds it to the
match" row from the reference's Differences from Raku, whose last
column still points at scalar's TODO.md.

**Why:** today `m.At("period.~")` on a match without a `period` leaves
an empty `period` behind, which `AtOK`, `Exists` and `MarshalJSON` then
find. Raku creates nothing on a read. `Lock` couldn't fix it, as a
locked hash panics on a missing key, which would break `At`, `Values`
and `To`. scalar's `Autovivify(false)` is Perl's `no autovivification`:
missing levels read as undef and create nothing, writes still create,
and values stored later take the setting. The scalar side was decided
with Iain on 2026-10-25 and tested against the CPAN pragma.

**Files:** `go.mod` and `go.sum` (the scalar release), `grammar.go`
(`options.result`; `noMatch` can stay a locked undef, which already
reads through quietly), a Go test that reads a missing capture and
checks the match is unchanged (`Equal` to a clone taken before, the
same `MarshalJSON`), a Raku test beside the others that does the same
and compares, `docs/REFERENCE.md` (the Differences row), and
`CHANGELOG.md`.

**Done when:** scalar is released with `Autovivify`, grammar uses it,
reading a missing capture leaves a match unchanged in Go and agrees
with Raku, the Differences row is gone, and the reference PDF reads
well.
