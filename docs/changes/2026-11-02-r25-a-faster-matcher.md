# A faster matcher

**Status:** shipped in v0.5.2

**Origin:** roadmap 25, from a session looking at how the ideas still on
the roadmap (rule parameters, longest-token matching, rejecting a text
that can't hold a match, stemming) could be fitted in more easily, and
what would make the matcher faster

**Started:** 2026-11-02, in 9c2e63db0d56073455d9d2d45bff6f6f10291056

**Finished:** 2026-11-02

## The plan

Benchmark, experiment, compare, and see how much faster the matcher can
be made, by the practice's rule for speculative speed-ups: branch from a
clean tree, record baseline benchmarks, profile first, put each change
behind a switch with a differential test and a planted bug to show the
test would notice, keep what pays, revert what doesn't, and record what
was reverted and its numbers.

The design discussion before it named the candidates, cheapest first:
the closures the matcher still makes; looking rules up by name through
the grammar and its parents at every call; making a state, and its
slots, for every search; and, as the start of a compiled form of a
grammar, somewhere to keep facts worked out ahead of time on the nodes
that need them, such as an alternation's branches' starts, which ideas
20 to 23 build on.

## What happened

- **Outcome:** across the package's benchmarks, 40% less time and 75%
  fewer allocations (geometric means, `benchstat`, 8 to 10 runs each, on
  an Apple M2). A pay parse (`BenchmarkParse`) went from 8.36 µs and 225
  allocations to 5.48 µs and 126; searching each line of a posting for
  facts (`BenchmarkFindLines`) from 1.14 ms to 0.39 ms, with 84% fewer
  allocations; `FindAll` on a posting from 39.7 µs to 28.0 µs; the
  sections grammar's unbounded lookbehind from 37.4 ms to 13.5 ms; and a
  search with a code assertion (`BenchmarkAssertReads`) from 1.44 ms to
  0.77 ms. Every step count is unchanged. Reworked against this checkout,
  jdc-lab's tests pass, its `sections` command writes the same output
  for 4,296 ads, and its benchmarks of finding facts in postings
  (`FindSamples`) and chunking them (`AdOfSamples`) take 54% and 53% less
  time, with 84% and 85% fewer allocations.
- **What was kept,** in the order it was done, each measured against the
  step before:
  - Under ratchet, `seq` steps over anything it would commit to in its
    loop, through `once` or `loop`, rather than with a closure for each;
    a rule call that commits makes no `finish` closure; and an
    alternation keeps its best branch so far rather than a list of
    candidates. Parse −15.5%, FindLines −28%, half FindLines'
    allocations. Behind `fastPaths`.
  - A table of the ASCII characters in a grammar's starts, worked out
    once, so `Find` doesn't look up a map and call each test at every
    position (`startsTable`); and `runState` making its call and
    continuation once a state, not at every position. FindAll −23%,
    FindLines −19%, word lists −25%.
  - `loop` for any greedy repeat, with no separator, of an item that
    commits, through `once`, not only a class or a literal; `backtrack`,
    outside ratchet, for a greedy repeat of a class or a literal, taking
    as many as it can and offering the continuation each count in turn,
    longest first; and a capture and its match made in one allocation.
    Parse −12%, the unbounded lookbehind −64%.
  - Finished states kept on the grammar, in a `sync.Pool`, for its next
    search (`done`), with `once`'s slots and the call `runState` makes.
    Parse −10%, FindLines −7%, FindLines' allocations −39%.
  - Rule names numbered when a call to them is made (`ruleID`), so the
    state keeps the rules it has found in a slice (`ruleOf`) rather than
    looking each up through the grammar and its parents. FindLines −9%.
  - An alternation skips a branch that can't start with the next
    character (`skipBranches`), from the branch's starts, worked out as
    `Find`'s are and kept on the state by the alternation's number. That
    needed `alt` to become a struct, to carry the number: the first place
    a fact worked out ahead of time hangs on a node. Measured in one
    binary with the switch on and off, FindLines −27%, Parse −8%,
    FindAll −2%.
- **Against Participle:** job-meta's comparison
  (`data/scratch/pcompare`), run again with this package in place of
  job-meta's frozen copy, does the same job both ways: pay parsed into a
  struct, through actions and `To`, and found in a posting. Parsing went
  from 16.4 µs with v0.5.1 to 14.3 µs, against Participle's 5.7 µs, and
  finding from 47.8 µs to 32.6 µs, against Participle's 11.6 µs. The
  README had said Participle was about one and a half times as fast, from
  a bare match (`BenchmarkParse`) beside Participle's whole job; on the
  same job it's about two and a half times, and the README now says so.
  Most of the difference is building the result as scalar data.
- **Changes from the plan:** a skipped branch takes no steps, so with
  `skipBranches` on a call can count fewer steps than with it off, as
  positions `Find` skips already took none. Its differential test,
  `TestBranchSkipsChangeNothing`, compares every result and failure
  report, and checks the steps are never more, and fewer somewhere. The
  other changes take exactly the steps they did.
- **Tests:** `TestFastPathsChangeNothing` and `TestFastPathsKeepLimits`
  cover the new fast paths; `TestStartsTableChangesNothing` checks the
  table against the starts for every ASCII character, for each test
  grammar and lookbehind; `TestStatesShared` searches one grammar from
  eight goroutines at once, with some searches stopped by `StepLimit`,
  under `-race` in CI. Each was shown to catch a planted bug: captures
  dropped in `seq`'s loop, a wrong bit in the table, `backtrack` never
  offering its minimum, a state's steps carried into its next search, a
  match with two names built twice, and branch starts that ignore `:i`.
  `FuzzMatch`, `FuzzCompile` and `FuzzWords` ran clean.
- **Measuring:** benchmarks of separate binaries moved by up to 20% on
  code that hadn't changed (`BenchmarkNFC`, `BenchmarkFindRe`), from how
  the code is laid out in each. Where a change's effect was small, it
  was measured in one binary with its switch on and off.
- **Tried and reverted:**
  - Making a lookbehind's continuation once a call, rather than once for
    each start it tries: the unbounded lookbehind 14% faster, but the
    bounded one, which rarely tries any start, made one allocation a
    position, 16 to 4,191 a search. Kept instead: made when the first
    start is tried.
  - Building only matches with two names through the builder's memo map,
    which every result makes and fills, with the captures gathered in a
    buffer on the stack: no gain in Parse or FindLines, and the other
    benchmarks moved by layout alone.
- **Measured for scalar:** with scalar's `Hash` making a hash's value
  scalars in one allocation, rather than one each, a pay parse took
  9.6% less time and 32% fewer allocations, and FindLines 2%. That's
  scalar's change to make, with its own switch and tests, so it was
  reverted here and left as a task in scalar's `TODO.md`.
- **Found at the wash-up:** each alternation's branch starts were kept on
  the state, in a slice by a number every alternation in the program was
  given as it was made, so a new state grew with all of them: after
  200,000 alternations made anywhere, a parse on a new state made 9.6 MB
  rather than 4.5 KB, and the pool makes new states after garbage
  collection. They're now kept on the alternation, for each grammar that
  uses it, at the generation they were worked out, which also shares
  them between states. `TestBranchStartsDontGrow` failed first, and
  `TestBranchStartsPerGrammar` checks an alternation shared by two
  grammars, with a planted bug (the grammar ignored) to show it would
  notice. No benchmark moved. Rule names are still numbered across the
  program, but they come from grammars' source, so a state's rules grow
  only with the names a program uses, at 8 bytes each.
- **Checked again at the wash-up,** with scalar v0.6.5: the comparison
  with Participle gave 13.7 µs to parse pay into a struct, against
  Participle's 5.7 µs, and 31.4 µs to find it, against 11.5 µs, so the
  README's "about two and a half times" stands. Its "When not to use it"
  still said 1.4 times, from the bare match, and now agrees.
- **What's left:** building the result is now most of a parse, about two
  thirds of `BenchmarkParse`'s time and nine tenths of its allocations,
  in scalar (see [Engineering](../REFERENCE.md#engineering)). A proto's
  variants could be skipped as alternation branches are. The branch
  starts are a first-character version of ideas 20 to 22's prefixes.
