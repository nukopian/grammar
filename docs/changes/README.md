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

- **It's opened when its item first has a question to settle, or when
  work starts.** When work starts, the plan moves here from the
  roadmap or `TODO.md`, and the roadmap's Now item shrinks to a
  paragraph and a link. `TODO.md` keeps only the next steps for a
  session.
- **The plan isn't edited once work starts.** What changed goes under
  What happened, so the record shows how the plan held up.
- **What happened is added as the work goes,** and finished when the
  change ships or is dropped, in the same commit. When it ships, the
  status says "shipped in v0.4.0", and the release names its version.
- **It's named for its id, then the change,** such as
  `2026-0002-limit-errors-that-say-where.md` for roadmap 2026-0002. A
  roadmap item's id is its own, kept as its name in commits and the
  CHANGELOG; a `TODO.md` task with steps takes the next id from the
  roadmap's opening, in the same sequence, though it never appears
  there. Ids sort in the order items were added; the **Started:** line
  gives the date. Records opened before 2026-11-09 were named for the
  day work started and "r" and the item's number, and were renamed to
  their ids, a task's taking the next id in the order they were opened.
- **Its questions are settled before work starts,** numbered D1, D2
  and on under Decisions: unchecked while open, checked once settled,
  with the answer, the date and why, and summed up in a **Decisions:**
  line under the status.
- **It's public,** on `main` and in every release, like the rest of
  `docs/`, so nothing personal goes in it.

## The template

```markdown
# 2026-0007. Title

**Status:** open | in progress | unreleased | shipped in v0.4.0 | dropped

**Decisions:** D1 settled: the answer, in a few words. D2 open: the question, in a few words. | None.

**Origin:** roadmap 2026-0007 | a TODO task from scalar's session (scalar <commit>)

**Depends on:** 2026-0003, if anything

**Started:** 2026-10-26, in <commit>

**Finished:** 2026-10-27

## Decisions

- [x] **D1.** The question? The answer, on 2026-10-26, and why.
- [ ] **D2.** A question still open? The options, and the
  recommendation.

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

- [Rejecting a text that can't hold a match](2026-0023-rejecting-a-text-that-cant-hold-a-match.md)
  (roadmap 2026-0023), shipped in v0.5.5
- [Keep each rule's starts, not only the last one searched for](2026-0030-keep-each-rules-starts.md),
  shipped in v0.5.3
- [A faster matcher](2026-0025-a-faster-matcher.md)
  (roadmap 2026-0025), shipped in v0.5.2
- [Building only what a code assertion reads](2026-0024-building-only-what-a-code-assertion-reads.md)
  (roadmap 2026-0024), shipped in v0.5.1
- [Code assertions](2026-0003-code-assertions.md)
  (roadmap 2026-0003), shipped in v0.5.0
- [Find skipping positions for regex terminals](2026-0010-find-skipping-positions-for-regex-terminals.md)
  (roadmap 2026-0010), shipped in v0.4.4
- [Failure reports from Find and FindAll](2026-0004-failure-reports-from-find-and-findall.md)
  (roadmap 2026-0004), shipped in v0.4.4
- [Fewer closures in the matcher](2026-0009-fewer-closures-in-the-matcher.md)
  (roadmap 2026-0009), shipped in v0.4.3
- [A trace of a parse](2026-0001-a-trace-of-a-parse.md)
  (roadmap 2026-0001), shipped in v0.4.2
- [A word list's value in a rule with more in it than the list](2026-0029-list-value-beside-lookarounds.md),
  shipped in v0.4.1
- [Name the quantified group in $<k>=( … )?, as Raku does](2026-0028-named-quantified-groups.md),
  shipped in v0.4.0
- [Match paths checked against the grammar](2026-0016-match-paths-checked-against-the-grammar.md)
  (roadmap 2026-0016), shipped in v0.4.0
- [Overlapping matches without the nested ones](2026-0014-overlapping-matches-without-nested.md)
  (roadmap 2026-0014), shipped in v0.3.2
- [Every phrase in a text, overlaps included](2026-0013-every-phrase-overlaps-included.md)
  (roadmap 2026-0013), shipped in v0.3.2
- [Rules whose canonical isn't a phrase](2026-0012-rules-whose-canonical-isnt-a-phrase.md)
  (roadmap 2026-0012), shipped in v0.3.1
- [Limit errors that say where](2026-0002-limit-errors-that-say-where.md)
  (roadmap 2026-0002), shipped in v0.2.4
