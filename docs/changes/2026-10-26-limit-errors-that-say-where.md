# Limit errors that say where

**Status:** shipped in v0.2.4

**Origin:** roadmap 2

**Started:** 2026-10-26, in 12e6de447aed5dc091ca8f0ab8a205941913fe1c

**Finished:** 2026-10-26

## The plan

**Serves:** debugging

**From:** grammar's session, 2026-10-26 (grammar 3ba32030ba9caac31997111e43035a285d322fc1)
**Waiting on:** nothing

**What:** `StepLimitError` and `DepthLimitError` name the rule that was
being matched when the limit was reached, and where in the text that
rule's match started, such as "gave up after 10000000 steps, in amount
from line 3, column 14".

**Why:** they name only the grammar and the limit, so finding the rule
that backtracks badly means guessing.

**What's known:** both errors are made in `node.go`: `tick` and
`deeper`, and `Quant.loop`'s own depth check. Matching passes
continuations, so a rule's frame is still on Go's stack while what
follows it runs; the innermost rule is the one whose body is matching,
not the last one called. The failure tracker that `ParseErr` uses
already keeps that right, leaving a rule before its continuation and
re-entering after (`call.match`), and the state can do the same with
one frame: a store or two per rule call. `ParseError`'s `Offset`, `Line`
and `Column`, and `lineCol`, are the pattern for the position. Raku has
no step or depth limit, so there's nothing to compare with Raku; the
differential tests only show that nothing else changed.

**Steps:**

1. Record `BenchmarkParse`, `BenchmarkFindAll` and `BenchmarkLookbehind`
   (`-count 10`) as the baseline.
2. Tests first: each limit, reached through `Parse` (the panic),
   `ParseErr` and `Find`, names the right rule and place, including a
   rule that has matched and handed on (the error names its caller),
   a proto's variant, and the fast paths on and off.
3. Keep the innermost rule and where it started in `state`, set in
   `call.match` and `matchProto` as the tracker does, and give both
   errors `Rule`, `Offset`, `Line` and `Column`, with the message
   above.
4. Measure again. If it costs more than noise, record the rule only
   while failures are tracked, so `ParseErr` and `SubparseErr` say where
   and the rest wait for item 1's trace, and say so in the reference.
5. The reference's StepLimit and DepthLimit sections, the two examples'
   output, `docs/EXAMPLES.md` (`go test -run TestReference -update`),
   and the gap removed. `FuzzMatch` for 60 seconds.

**Done when** both errors carry the rule and position, the cost is
measured (or the rule is recorded only while tracking failures), tests
cover both limits, and this item has moved to the CHANGELOG as
"(roadmap 2)".

## What happened

- **Outcome:** from `ParseErr` and `SubparseErr`, both errors name the
  rule that was matching and where its match started, such as "gave up
  after 100000 steps, in amount from line 1, column 9", with `Rule`,
  `Offset`, `Line` and `Column` fields named as `ParseError`'s are. From
  `Parse`, `Find` and the rest, the place is empty and the message is as
  before. That meets Done when through its fallback: the rule is
  recorded only while failures are tracked.
- **Changes from the plan:**
  - Step 3's rule kept in `state` for every match was measured at step 4
    and reverted, as the plan allowed. The fallback needed no new state
    at all: the failure tracker's stack of rules already has the
    innermost rule on top, kept right around continuations, so the
    errors read it.
  - For a proto, the variant's frame is pushed onto the tracker's stack
    while it's tried, so the error names `amount:sym<range>` rather than
    `amount`. A failure report names the outermost rule that started at
    a position, the proto's, so reports don't change; the Raku
    differential tests agree.
  - The fields are plain exported fields on each error, not a shared
    embedded struct, which godoc would hide.
  - `ExampleStepLimit` became a pay grammar whose runaway rule isn't
    `TOP`, to show what the place is for.
  - Step 5's gap was rewritten rather than removed, as half of it is
    still open: it's now "Only `ParseErr`'s limit errors say where",
    with the measurements, so the reverted approach isn't tried blind.
  - The fast paths needed no test of their own: `TestFastPathsKeepLimits`
    compares the errors' text with them on and off, and under `ParseErr`
    the loop fast path is off anyway.
- **Measurements:** `BenchmarkParse`, `BenchmarkFindAll` and
  `BenchmarkLookbehind`, run five times two (`-count 2`), alternating
  between a worktree of 12e6de447aed5dc091ca8f0ab8a205941913fe1c and the
  change, both with `GOWORK=off`; the median ns/op of ten runs, on the
  maintainer's Mac.

  | Benchmark | Before | Rule in `state` | Before (again) | From the tracker |
  |---|---|---|---|---|
  | `Parse` | 9,923 | 10,148 (+2.3%) | 9,924 | 9,986 (+0.6%) |
  | `FindAll` | 40,986 | 41,464 (+1.2%) | 41,340 | 40,958 (−0.9%) |
  | `Lookbehind/bounded` | 857,865 | 896,092 (+4.5%) | 856,142 | 859,749 (+0.4%) |
  | `Lookbehind/unbounded` | 40,680,500 | 43,212,400 (+6.2%) | 40,907,300 | 41,383,337 (+1.2%) |

- **Tried and reverted:** keeping the rule in `state` for every match,
  first restoring the caller's with a deferred closure, then with
  explicit stores on each return. Both cost the same, about 2.5% on
  `Parse` (measured against an earlier baseline, not alternated), so
  the defer wasn't the cost. The stores themselves and the larger
  closure are the likely cost, though that wasn't profiled.
- **Checks:** a planted bug, leaving out the caller's restore around the
  continuation, fails the "handed on" case in `TestLimitErrorsSayWhere`
  through `Parse`, `Find` and `ParseErr`. The Raku differential tests
  ran against Rakudo 2025.11 and pass; `FuzzMatch` ran for 60 seconds
  and found nothing.
- **What's left:** the place from `Parse`, `Find` and the rest, in the
  reference's gap
  [Only `ParseErr`'s limit errors say where](../REFERENCE.md#gaps-and-possible-additions),
  best done through roadmap 1, the trace.
