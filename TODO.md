# To do

## Measure roadmap 24 both ways, and tell scalar which pays

**From:** scalar's session, 2026-11-02 (scalar a97896c011e1244a5e478dfd2b2cadd6722002bd)
**Waiting on:** nothing

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
