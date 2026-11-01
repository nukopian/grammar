# Failure reports from Find and FindAll

**Status:** shipped in v0.4.4

**Origin:** roadmap 4

**Started:** 2026-11-01, in ae1eeb0e0066d69bccfc5f4e3f3e0966ef412e6f

**Finished:** 2026-11-01

## The plan

**Serves:** debugging

**Size:** S (2)

**What:** a way to ask `Find` or `FindAll` why nothing was found: the
furthest failure across every start position, as `ParseErr` gives for
one string, with positions that fail at once left out as noise.

**Why:** with postings, the question is usually why pay wasn't found
anywhere in the text, not why one string didn't parse, and today the
only answer is to cut out the likely span by hand and run `ParseErr` on
it. Beside the trace (1) it answers both "what happened" and "where did
it get furthest", for little work.

**What's known:**

- `ParseErr`'s failure tracker (`tracker` in `node.go`) already records
  the furthest position and what was expected there, keeping `<ws>` and
  lookarounds quiet. `Find` and `FindAll` call `runState` once for each
  start position that `canStart` allows, sharing one state.
- "Furthest" can't mean the furthest position in the text, as it does
  for one parse. Tried on a scratch grammar,
  `<money> <.sep> <money> <period>`, over "Salary £45k - £55k, depending
  on experience": the start at "£45k" and the one at "£55k" both fail at
  offset 18, the first expecting `<period>`, the second `<sep>`. By
  position they tie, and the report would merge them into `expected
  <period> or <sep>`. So each start is ranked by how far it got past
  where it started, ties going to the earlier start: here the first,
  expecting `<period>`, which is the real problem. A start that gets
  nowhere ranks last, so positions that fail on their first item drop
  out without a special rule; `canStart` already skips most of them.
- A text where no start is possible ("Competitive salary") runs no
  start at all, so there's nothing tracked. The report then comes from
  one start at the first position, `Pos` if given: "expected <money>".
- The tracker turns the fast paths off, so a tracked search is slower.
  Running it only once the plain search has found nothing leaves `Find`
  and `FindAll` as they are, and costs a second pass only when there's
  no match, where it's wanted.
- Raku has nothing like it, so this goes beyond Raku; the matches
  returned are `Find`'s and `FindAll`'s, which the Raku tests cover.

**Steps:**

1. `FindErr(text, opts...) (*scalar.SV, error)`: `Find`, and when it
   finds nothing, a `*ParseError` from a tracked search ranked as above.
   Step and depth limits come back as errors, as from `ParseErr`.
2. `FindAllErr(text, opts...) (*scalar.AV, error)`: `FindAll`, with the
   same report when the array is empty, and no error otherwise.
3. Say in the report where the best attempt started, as a `From` field
   on `ParseError` (characters, as `Offset` is) and in its message, so
   it can be told from `ParseErr`'s; `ParseErr`'s message is unchanged.
4. Tests: the ranking (the tie above), no start possible, `Pos`, limits,
   matches identical to `Find`'s and `FindAll`'s, including in
   `FuzzMatch`.
5. Examples on posting text for both methods; a section in the
   reference beside `ParseErr`'s, saying it goes beyond Raku; remove
   the gap; the CHANGELOG; `docs/EXAMPLES.md` regenerated.
6. `BenchmarkFindAll` before and after.

**Done when** the report names the furthest position, the rules
running there and what they expected, as `ParseErr`'s does; positions
that fail on their first item don't crowd it out; `BenchmarkFindAll`
shows no cost when it isn't asked for; it has an example on posting
text and a place in the reference, which says it goes beyond Raku; and
its gap is gone.

## What happened

- **Outcome:** every point in Done when holds. `FindErr` and
  `FindAllErr` report the attempt that got furthest past its start, with
  `ParseErr`'s position, rules and expectations, and where it started;
  attempts that fail at once never win over one that got somewhere;
  `BenchmarkFindAll` is unchanged (below); both have examples on posting
  text; the reference has a section beside `ParseErr`'s and a row in
  Differences from Raku saying they go beyond it; the gap is gone.
- **Changes from the plan:** none in substance. `ParseError` keeps where
  the start was as line and column too, unexported, for the message,
  which ends "(trying from line 1, column 9)". `ParseErr`'s message is
  unchanged, and its `From` is 0. The step and depth limits' recovery,
  written out in `parseErr`, became `limitErr`, shared by the three.
- **Tests:** `TestFindErr` covers the tie that motivated the ranking,
  no possible start, `Pos` (inside and past the end), lines, and an
  attempt that starts later and gets less far; `TestFindErrLimitsAndTrace`
  the limits and a trace printed once. `FindErr` and `FindAllErr` were
  added to `TestFastPathsChangeNothing`'s results, `TestNoMatchStaysUndef`
  and `FuzzMatch`, which checks they find what `Find` and `FindAll` find.
  Two planted bugs show the ranking is pinned: `>=` in place of `>`
  (the later of equal attempts) and ranking by position in the text each
  fail `TestFindErr`. 60 seconds of `FuzzMatch` (1.76 million runs) found
  nothing.
- **Measurements:** `BenchmarkFindAll` and `BenchmarkFindLines`, ten
  runs each before and after, on an Apple M2: 38.31 µs against 38.22 µs
  (p=0.58) and 1.120 ms against 1.119 ms (p=0.85), allocations
  identical. Nothing changed on that path, as the report is made by a
  second search only when the first finds nothing.
- **What's left:** nothing for this item. A failed search costs two
  passes under `FindErr`, the second with the fast paths off; that's
  paid only when nothing is found, which is when the report is wanted.
- **The wash-up found** that `ParseErr` and `SubparseErr` left `From` at
  0 under `Pos`, so Offset less From wasn't how far the parse got, as
  the field's comment said. They set it to `Pos` now
  (`TestParseErrFrom`). It also found that `whyNotFound`'s comment
  blamed ranking by position for merging attempts, which pooling their
  failures would do; ranking by position favours late starts. The
  wash-up checked that a tracked search takes exactly the steps an
  untracked one does, over every Raku case, so `FindErr` can't hit
  `StepLimit` where `Find` didn't.
