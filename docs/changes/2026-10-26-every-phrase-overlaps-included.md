# Every phrase in a text, overlaps included

**Status:** shipped, unreleased

**Origin:** roadmap 13

**Started:** 2026-10-26, in 675df5ba00d3828d4b37bbb6e439ba30e864b7e8

**Finished:** 2026-10-26

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

- **Outcome:** all of Done when. `FindAll(text, Overlap())` gives a
  match from each position where one starts; `TestOverlap` has Raku's
  answers for the heading, `a+` on "aaaa", empty matches and `Pos`, and
  shows the other methods ignore it. The phrasebook's new task, "Every
  match, overlaps included", gives the same answer in Raku (`:ov`),
  grammar and plain Go. `ExampleOverlap` weighs a heading by both its
  phrases, and the reference's Parsing section explains it.
- **Changes from the plan:**
  - Raku was asked about `:ex` for tokens and alternations as well as
    word lists, before the reference said they're the same as `:ov`:
    `[ a | ab | abc ]` under `:r`, and a grammar's token, give one match
    per position with either.
  - Step 6's trial found that `:ov`'s semantics, kept faithfully, aren't
    quite what classifying headings wants. A phrase wholly inside a
    longer one is found as well as one that reaches past its end. On
    `data/ingestion/ads-20261024.jsonl` (1,890 ads), passing `Overlap()`
    in jdc-lab's `sections.All` gave 9 more headings whose phrases
    disagree (such as "What you'll get in return": benefits and
    responsibilities), and changed the winning kind in 2 ads, both
    "Company Profile", from company to requirements, as "profile" inside
    "company profile" outranked it. Nothing else changed, and the lab's
    tests passed. grammar keeps Raku's semantics; the reference now says
    nested matches are found, and how to drop them (each match within
    another's `from` and `to`), and jdc-lab's task says the same.
- **Measurements:** none needed: `FindAll` without the option is
  unchanged, and with it, the cost is a search at each position, which
  is what's asked for.
- **Checks:** `FuzzMatch` now checks that with `Overlap` the matches
  start at increasing positions, each is what `Find` with `Pos` finds
  there, `FindAll`'s matches are among them, and skipping start
  positions changes nothing (`findBoth` and `TestStartsChangeNothing`
  compare it too). A planted bug, stepping two positions on under
  `Overlap`, failed the seeds ("Overlap's matches leave out FindAll's
  from 1"). It ran for 60 seconds and found nothing. The Raku
  differential tests ran against Rakudo 2025.11 and pass.
- **What's left:** `:ex`, in the reference's gaps as "Exhaustive
  matching"; jdc-lab trying `Overlap` in its sections, through its
  `go.work`, before grammar releases it, as a task in each `TODO.md`.
