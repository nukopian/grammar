# Building only what a code assertion reads

**Status:** unreleased

**Origin:** roadmap 24, with a TODO task from scalar's session (scalar a97896c011e1244a5e478dfd2b2cadd6722002bd)

**Started:** 2026-11-02, in f9058f2b3862169c43abcbd45f3b82b9a4152aee

**Finished:** 2026-11-02

## The plan

From the roadmap:

**Serves:** speed

**Size:** M (3)

**What:** have a code assertion build only the captures its function
reads, not the whole match so far. Two ways to try, behind a switch:
lazy values from scalar (its roadmap 5), each capture built when it's
read; or, with no change to scalar, an `Assertion` that names the
captures it reads, failing loudly when the function reads another.

**Why:** each assertion builds a hash for every capture, though
`pay($/)` reads only `cur` and `period`: in `BenchmarkAssert`, `sofar`
makes 39% of the search's allocations, and garbage collection takes
most of its time. jdc-lab moved its pay rule into its grammar and
took the cost, 4–6% of its chunker's time (about 2 s over 28,000 ads),
for one copy of the rule; assertions should cost about what checking
afterwards does.

**Done when** `BenchmarkAssert` is measured before and after, with its
allocations; the way kept is the one that pays, and the other's
numbers are in the record; jdc-lab's chunker, rerun over every ad,
gives the same output in less time; and the reference's gap is gone
or says what's left.

From the TODO task, "Measure roadmap 24 both ways, and tell scalar which
pays":

**What:** do roadmap 24, building only what a code assertion reads,
measuring both of its ways, and tell scalar's session which pays: scalar
is waiting on it to decide whether lazy values (its roadmap 5) stay.

**What's known:** an experiment in scalar's session tried both ways on
`BenchmarkAssert`.

- Lazy captures: a prototype is on the local branch `r24-lazy-sofar`
  (not pushed), at 6e74c9dd16a5627cd89b37373e5cb1e5c1f46b50, using
  scalar's `Lazy` from scalar's local branch `r5-lazy-values`. It needs
  a `go mod edit -replace` pointing at a worktree of scalar with that
  branch checked out (scalar's own working copy stays on develop), and
  the replace isn't committed. `lazysofar.go` has
  the `lazySofar` switch and `buildLazy`, used by `sofar` with no
  actions. With it on, grammar's full suite passes with `-race`.
- Against eager captures on the same scalar: lazy captures −13% time,
  −9% bytes, −10% allocations. Named captures, estimated by building
  only `cur` and `period` eagerly: −21% time, −17% bytes, −21%
  allocations. `sofar` made 45% of the allocations, so neither reaches
  the ceiling: the top-level match hash is built at every assertion.
- Each lazy capture costs a few allocations of its own (the scalar, its
  lazy value and its closure), almost what a small capture costs to
  build.
- The builder must be made only on the lazy path: when the lazy
  closures capture a builder made before the branch, it escapes on the
  eager path too, which then allocates 3% more.

**Steps:**

1. Open roadmap 24's change record, with its plan word for word.
2. Build the named way properly, behind a switch, and measure it
   against eager and lazy captures, interleaved.
3. Keep the way that pays, with the other's numbers in the record.
4. Tell scalar's session, through scalar's `TODO.md` task "Decide
   whether lazy values stay (roadmap 5)", which way was kept and the
   numbers.

**Done when** roadmap 24's Done when is met, and scalar's task has
grammar's answer.

## What happened

- **Outcome:** the named way is kept. `Assertion(name, fn, reads...)`,
  and `Assert` and `AssertNot` the same, build only the captures named,
  with the match's `~`, `from` and `to`. `BenchmarkAssertReads` (the
  same search, naming `cur` and `period`) against `BenchmarkAssert`:
  1.88 ms to 1.43 ms, −24% time, −22% bytes, −28% allocations, on scalar
  v0.6.3 as released. jdc-lab's chunker, naming the reads of Pay's and
  Hours' assertions, gave output identical, byte for byte, to v0.5.0's
  over all 28,000 ads, in 37.05–37.69 s against 38.01–38.36 s (three
  runs each, interleaved), about 2% less. The reference's gap now says
  what's left: the match's own hash and text, made at each call.
- **Changes from the plan:** "failing loudly" came from scalar as it is.
  The match so far is locked with its own keys left restricted, so a key
  it hasn't got panics, as Perl's Hash::Util does, while what's below the
  named captures reads as undef as before. grammar adds which assertion
  it was and what it names to the panic. A named capture that hasn't
  matched is there as undef, or an empty array where the rule can
  capture it more than once, so that reading it doesn't panic. Nothing
  checks at compile time that the names are the rule's captures: a
  misspelt name makes the function's read of the right one panic, so a
  mistake can't pass in silence.
- **The switch** is `namedReads`. With it off, every capture is built.
  `TestNamedReadsChangeNothing` runs the Raku cases' assertions, now each
  naming its reads (`rakuCase.Reads`), with it on and off, comparing
  every result and failure report; a planted bug, leaving the first
  named capture out, failed it. `FuzzMatch` compares too, its assertion
  `f` naming `c`, and ran 60 seconds with nothing found.
- **Measurements:** interleaved, 10 runs each, all on scalar's
  `r5-lazy-values` branch (c0719949675e824b08aa29b2abe2cfeda1e5afb4),
  against eager captures on the same scalar:

  | `BenchmarkAssert` | time | bytes | allocations |
  |---|---|---|---|
  | eager, every capture | 1.879 ms | 2.401 MiB | 44.25k |
  | lazy captures | 1.639 ms, −12.7% | 2.192 MiB, −8.7% | 39.92k, −9.8% |
  | named reads | 1.439 ms, −23.4% | 1.875 MiB, −21.9% | 31.95k, −27.8% |

  On scalar v0.6.3, eager was 1.880 ms and named 1.426 ms, −24.2%; the
  branch's eager path was 1.875 ms, unchanged. `FindAll`, `FindRe`,
  `Parse`, `FindLines` and `Lookbehind` (8 runs each) were unchanged in
  time, bytes and allocations.
- **Tried and reverted:** lazy captures, the prototype on the local
  branch `r24-lazy-sofar`, each capture a `scalar.Lazy` built when the
  function reads it. It asks nothing of the function, but saved about
  half what naming does, as each lazy capture makes three allocations
  of its own (the scalar, its lazy value and its closure), nearly what a
  small capture costs to build; it covered only parses without actions;
  and it needs scalar's lazy values, which make every read in scalar
  check for one (+0.5% geomean on scalar's benchmarks). Given these
  numbers, scalar dropped lazy values, its roadmap 5, and the prototype
  branches in both repositories were deleted.
- **What's left:** the match's own hash and its text, built at each
  call, which most functions don't read; in the reference's
  [Engineering gaps](../REFERENCE.md#engineering).
