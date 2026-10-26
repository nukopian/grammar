# Overlapping matches without the nested ones

**Status:** in progress

**Origin:** roadmap 14

**Started:** 2026-10-26, in 946f127c677353e619f0d596e4dde8e5daadc505

**Finished:**

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
