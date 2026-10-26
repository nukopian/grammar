# Every phrase in a text, overlaps included

**Status:** in progress

**Origin:** roadmap 13

**Started:** 2026-10-26, in 675df5ba00d3828d4b37bbb6e439ba30e864b7e8

**Finished:**

## The plan

**Serves:** word lists

**From:** grammar's session, 2026-10-26 (grammar 675df5ba00d3828d4b37bbb6e439ba30e864b7e8)
**Waiting on:** nothing

**What:** an `Overlap()` option for `FindAll`, as Raku's `:ov`
(`:overlap`): a match from each position where one starts, not only
those after the previous match's end. In "in this role you will need",
a list with "in this role you will" and "you will need" finds both.

**Why:** jdc-lab classifies headings by the phrases in them, and
`FindAll`'s non-overlapping matches hide some: "you will need"
(requirements) is lost inside "in this role you will"
(responsibilities), so the heading can't be weighed by both. It's one
heading in 7,000 ads so far, but it grows with the lab's word lists.

**What's known:** Raku 2025.11, asked directly:
`"in this role you will need".match(/ << @w >> /, :ov)` gives "in this
role you will", "role" and "you will need" with "role" in the list, one
match from each starting position, the one a search would find there;
`:ex` gives the same for a word list, as a list is one atom and gives
nothing back. `"aaaa".match(/ a+ /, :ov)` is "aaaa aaa aa a" (`:ex` adds
the shorter ones at each start, for a backtracking regex only, not with
`:r`); an empty match counts at each position ("ab" with `x?`: 0, 1, 2);
and `:c(2)`, grammar's `Pos(2)`, starts the search there. `FindAll`
(`grammar.go`) moves to `max(m.to, from+1)` after a match; under
`Overlap` it moves to `from+1`, and `canStart`'s skipping still applies.
Raku's `.parse`, `.subparse` and `.split` take no `:ov`, and `Find`'s
first match is the same either way, so the other methods ignore the
option. `:ex` is left as a gap: it adds nothing for word lists, and for
a grammar it needs every way each rule can match, not the first.

**Steps:**

1. Tests first: under `Overlap`, the heading's phrases, `a+` on "aaaa",
   the empty matches, and `Pos`, each as Raku gives them; without it,
   nothing changes; the other methods ignore it.
2. The option, and `FindAll` stepping one position on under it.
3. A row in the phrasebook (`docs/RAKU.md`), with Raku's `:ov`, grammar
   and plain Go, so `TestMappingMatchesRaku` compares all three.
4. `FuzzMatch` checks that with `Overlap`, the matches start at
   increasing positions, each is what `Find` with `Pos` finds there,
   `FindAll`'s matches are among them, and skipping start positions
   changes nothing; run it for 60 seconds, with a planted bug to show
   the checks notice.
5. `ExampleOverlap`, with headings as postings write them, and
   `docs/EXAMPLES.md` regenerated; the reference's Parsing section, and
   `:ex` as a gap; the CHANGELOG, and this item out of the roadmap.
6. Dogfood: rework jdc-lab's headings against this checkout and check
   that the heading is weighed by both phrases and nothing else
   changes; leave jdc-lab a task to try it through its `go.work`, and
   hold the release until it has.

**Done when** `FindAll` with `Overlap` matches Raku's `:ov` in the
tests and the phrasebook, it's fuzzed and documented, jdc-lab has a
task to try it, and this item has moved to the CHANGELOG as
"(roadmap 13)".

## What happened
