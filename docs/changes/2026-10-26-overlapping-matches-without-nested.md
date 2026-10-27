# Overlapping matches without the nested ones

**Status:** shipped in v0.3.2

**Origin:** roadmap 14

**Started:** 2026-10-26, in 946f127c677353e619f0d596e4dde8e5daadc505

**Finished:** 2026-10-26

## The plan

**Serves:** word lists

**From:** grammar's session, 2026-10-26 (grammar 946f127c677353e619f0d596e4dde8e5daadc505)
**Waiting on:** nothing

**What:** an `Outermost()` option for `FindAll`: the matches `Overlap()`
finds, less each one wholly inside another, so "you will need" in "in
this role you will need" is kept, but "profile" in "Company Profile"
isn't. It implies `Overlap()`, so it's one option to write (Iain,
2026-10-26). Built before `Overlap` is released, so jdc-lab takes both
in one release.

**Why:** classifying text by its phrases with `Overlap` needs the
nested matches dropped, or "profile" outranks the "company profile"
it's part of. jdc-lab wrote that itself, a dozen lines and a direct
import of scalar to read each match's `from` and `to`; everyone using
`Overlap` so would write the same.

**What's known:** jdc-lab's `outermost` drops a match when another
contains it and is longer. `Overlap` finds at most one match from each
position, in order of where they start, so that's the same as: a match
is nested when it ends no later than the furthest end of the matches
before it. So `FindAll` can drop them as it goes, keeping that furthest
end, with no second pass. An empty match at another's end is inside it,
by the same rule, and dropped. Raku has no such adverb, so it goes
under where grammar goes beyond Raku; without it, `FindAll` and
`Overlap` are unchanged, and still match Raku.

**Steps:**

1. Tests first: the heading keeps both its phrases, "Company Profile"
   keeps only "company profile", a phrase inside a phrase inside
   another, a chain of overlaps none nested ("a b", "b c", "c d"),
   empty matches, `Pos`; `Outermost` alone is the same as with
   `Overlap`; the other methods ignore it.
2. The option, and `FindAll` keeping the furthest end under it.
3. `FuzzMatch` checks that with `Outermost`, the matches are those of
   `Overlap` that aren't inside another, by jdc-lab's rule written out
   plainly, and that skipping start positions changes nothing; a
   planted bug to show the check notices; 60 seconds.
4. `ExampleOutermost`, with headings, and `docs/EXAMPLES.md`
   regenerated; the reference's Overlapping matches, and its list of
   what grammar adds to Raku; the README; the CHANGELOG, and this item
   out of the roadmap.
5. Dogfood: replace jdc-lab's `outermost` with the option, against this
   checkout, and check its output is unchanged on both samples; put
   jdc-lab back, and update its take-up task, and grammar's release
   task, to say both options come in v0.3.2.

**Done when** `Outermost` gives `Overlap`'s matches less the nested
ones, tested, fuzzed and documented, jdc-lab's own version is shown to
give the same output, and this item has moved to the CHANGELOG as
"(roadmap 14)".

## What happened

- **Outcome:** all of Done when. `FindAll(text, Outermost())` gives
  `Overlap()`'s matches less those inside another, dropped as they're
  found by keeping the furthest end so far. `TestOutermost` covers
  step 1's cases, and `ExampleOutermost` shows "Company Profile" and
  the heading with each option.
- **Changes from the plan:**
  - Two of the test's expected answers were wrong when first written,
    and the test said so: from `Pos(1)`, `a*` on "aaaa" also matches
    empty at the end, which is inside "aaa" and dropped; and the chain
    case needed a list of its own, as "a b c d" in the same list made
    every other match nested.
  - The reference says how to write it in Raku, `:ov` and then a `grep`
    with jdc-lab's rule, after Raku ran it and gave the same answers
    for both headings; and `Outermost` joined the Differences from Raku,
    as an addition.
- **Measurements:** none needed: without the option nothing changes,
  and with it, `FindAll` keeps one integer and builds no match it
  drops.
- **Checks:** `FuzzMatch` compares `Outermost` with `Overlap`'s
  matches filtered by jdc-lab's rule, written out plainly, and with
  start skipping off. A planted bug, keeping a match that ends where an
  earlier one does ("profile" in "Company Profile"), failed the first
  seed; 60 seconds found nothing. The Raku differential tests ran
  against Rakudo 2025.11 and pass. In jdc-lab, against this checkout,
  `sections.All` with `Outermost()` in place of its own `outermost`
  helper, and without its direct import of scalar, wrote identical
  files for both samples, `ads-20261024.jsonl` and `ads-20261025.jsonl`,
  and the lab's tests passed; its working tree was put back exactly.
- **What's left:** nothing here. jdc-lab drops its helper when it
  takes up the release, which its `TODO.md` will say.
- **jdc-lab's trial,** before the release (on grammar
  a15a10ad1ed91a097b9764b402190cc8e914d688, through `go.work`): with
  `Outermost()` in place of its helper, the lab's tests passed, and over
  both samples every section file was byte for byte the same as with
  the helper. Released with roadmap 13 in v0.3.2.
