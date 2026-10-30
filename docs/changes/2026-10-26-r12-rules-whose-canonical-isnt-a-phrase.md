# Rules whose canonical isn't a phrase

**Status:** shipped in v0.3.1

**Origin:** roadmap 12

**Started:** 2026-10-26, in a67788361c9280bef0163c6c1d19b4a79c21363c

**Finished:** 2026-10-26

## The plan

**Serves:** word lists

**From:** grammar's session, 2026-10-26 (grammar a67788361c9280bef0163c6c1d19b4a79c21363c)
**Waiting on:** nothing

**What:** a `MapOnly()` list option, under which a rule written
`phrases => values` adds the phrases on the left meaning each value on
the right, as `AddAs` does, without making the values phrases too. Rules
without `=>`, and `Add` and `AddAs`, are unchanged.

**Why:** `a, b => c` makes `c` a phrase as well as a value, and
`AddRule`'s doc doesn't say so. jdc-lab needed a section, "legal", whose
name in a heading usually names a job ("Legal Secretary"), so it reads
its rules into one list, finds the `=>` rules with a regex, and rebuilds
them into another with `AddAs`. Making `=>` mean this for every list was
considered and turned down: it would quietly stop rule files matching
what they match now. An option changes nothing for lists without it,
and `WordLists` passes it to every list it makes.

**What's known:** `AddRule` (`words.go`) adds a `=>` rule with
`l.Add(canonical, lhs...)`, which adds the canonical as a phrase;
`AddAs` adds the phrases alone and still records the value in
`Canonicals`, so a rule file's order is kept as data. Options are set
when the list is made, so `AddRule` can read the option directly.
Under the option, a rule with nothing before `=>` would add nothing;
it should be an error, as nothing after `=>` is, rather than a rule
that does nothing. Word lists go beyond Raku, so there's nothing to
compare with Raku; the differential tests only show nothing else
changed.

**Steps:**

1. Tests first: under `MapOnly`, the values of a `=>` rule mean nothing
   and match nothing, and its phrases mean them; a rule without `=>` is
   unchanged; `pm => product manager, project manager` is still
   ambiguous; `Canonicals` keeps the rules' order; `Plurals` makes the
   phrases' plurals but not the values'; nothing before `=>` is an
   error; `WordLists` passes the option on; without it, nothing
   changes.
2. The option, and `AddRule` using `AddAs` under it.
3. `AddRule`'s doc says the values on the right are phrases too, unless
   the list has `MapOnly`.
4. `ExampleMapOnly`, with headings as postings write them, and
   `docs/EXAMPLES.md` regenerated.
5. `FuzzWords` builds each list with and without `MapOnly`, and checks
   that the option only takes phrases and meanings away; run it for 60
   seconds.
6. The reference's Loading synonyms and options table, the README's
   list of options, the CHANGELOG, and this item out of the roadmap.
7. Dogfood: rework jdc-lab's `internal/sections` to use the option,
   against this checkout, and check its tests and output are unchanged;
   leave jdc-lab a task to do it for real once this is released.

**Done when** `MapOnly` is in, tested, fuzzed and documented, jdc-lab's
rebuild is shown to be unnecessary with it, and this item has moved to
the CHANGELOG as "(roadmap 12)".

## What happened

- **Outcome:** all of Done when. Under `MapOnly`, a `=>` rule adds its
  phrases with `AddAs`; a rule with nothing before `=>` is an error
  ("has nothing before =>, so with MapOnly it would add nothing");
  `AddRule`'s doc says that, without the option, the values are phrases
  too. `TestMapOnly` covers step 1's cases, and `ExampleMapOnly` uses
  headings, as jdc-lab's sections do.
- **Changes from the plan:**
  - Step 1's `Plurals` case showed a mistake in the test, not the code:
    `Plurals` makes "perks" from "perk", not "perk" from "perks", so the
    test now looks up a phrase as written.
  - Step 6 also put `ExactAcronyms`, missing since it was added, beside
    `MapOnly` in the reference's row on what word lists add to Raku's.
  - Step 7 found the rebuild unnecessary twice over: jdc-lab's
    `internal/sections`, using `MapOnly` and reading its rules straight
    into its list, builds the same list as its rebuild (239 phrases, the
    same meanings, the kinds in the same order, checked by a temporary
    test), its tests pass, and `cmd/sections` writes the same files for
    `data/ingestion/ads-20261024.jsonl`. The rework was undone there, and
    left to jdc-lab, after the release, as a task in its `TODO.md`.
- **Measurements:** none needed: matching doesn't change, and the option
  is read only while rules are added.
- **Checks:** `FuzzWords` builds each list with and without `MapOnly`,
  checking that with it, each phrase means only what it does without,
  and each canonical value is one without, then matches every phrase of
  both. A planted bug, adding an extra phrase under `MapOnly`, failed
  it on the first seed. It ran for 60 seconds and found nothing. The
  Raku differential tests ran against Rakudo 2025.11 and pass, showing
  nothing else changed.
- **What's left:** nothing.
