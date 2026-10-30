# Change records

How each planned change to grammar went: the plan as it was written,
then what happened. The [CHANGELOG](../../CHANGELOG.md) says what
changed for users, and links here; a record keeps the thinking behind
it, for anyone looking back, including whoever plans the next change.

A change gets a record when it has a plan: a roadmap item that reaches
Now, or a task in `TODO.md` with steps. A planned change that's dropped
keeps its record, saying why, as dead ends are the history most worth
having. Small fixes don't need one.

## How a record is kept

- **It's opened when work starts.** The plan moves here from the
  roadmap or `TODO.md`, and the roadmap's Now item shrinks to a
  paragraph and a link. `TODO.md` keeps only the next steps for a
  session.
- **The plan isn't edited once work starts.** What changed goes under
  What happened, so the record shows how the plan held up.
- **What happened is added as the work goes,** and finished when the
  change ships or is dropped, in the same commit. When it ships, the
  status says "shipped in v0.4.0", and the release names its version.
- **It's named for the day work started, then, for a roadmap item,
  "r" and its number, then the change,** such as
  `2026-10-26-r2-limit-errors-that-say-where.md` for roadmap 2: dates
  never collide between sessions and sort in order, and a search for
  `r2` finds the record, where a bare 2 would match every date and
  figure. A record for a `TODO.md` task has no number, such as
  `2026-10-27-named-quantified-groups.md`. A roadmap item keeps its
  number as its name in commits and the CHANGELOG, and the record gives
  it in its header too.
- **It's public,** on `main` and in every release, like the rest of
  `docs/`, so nothing personal goes in it.

## The template

```markdown
# Title

**Status:** in progress | shipped in v0.4.0 | shipped in v0.2.4 | dropped

**Origin:** roadmap 7 | a TODO task from scalar's session (scalar <commit>)

**Started:** 2026-10-26, in <commit>

**Finished:** 2026-10-27

## The plan

The plan as it was when work started, word for word: Serves, What,
Why, What's known, Steps, Done when.

## What happened

- **Outcome:** against Done when.
- **Changes from the plan,** and why.
- **Measurements,** and how they were taken.
- **Tried and reverted,** with the numbers.
- **What's left,** linked to its gap or roadmap item.
```

## Records

Newest first.

- [A word list's value in a rule with more in it than the list](2026-10-30-list-value-beside-lookarounds.md),
  shipped in v0.4.1
- [Name the quantified group in $<k>=( … )?, as Raku does](2026-10-27-named-quantified-groups.md),
  shipped in v0.4.0
- [Match paths checked against the grammar](2026-10-27-r16-match-paths-checked-against-the-grammar.md)
  (roadmap 16), shipped in v0.4.0
- [Overlapping matches without the nested ones](2026-10-26-r14-overlapping-matches-without-nested.md)
  (roadmap 14), shipped in v0.3.2
- [Every phrase in a text, overlaps included](2026-10-26-r13-every-phrase-overlaps-included.md)
  (roadmap 13), shipped in v0.3.2
- [Rules whose canonical isn't a phrase](2026-10-26-r12-rules-whose-canonical-isnt-a-phrase.md)
  (roadmap 12), shipped in v0.3.1
- [Limit errors that say where](2026-10-26-r2-limit-errors-that-say-where.md)
  (roadmap 2), shipped in v0.2.4
