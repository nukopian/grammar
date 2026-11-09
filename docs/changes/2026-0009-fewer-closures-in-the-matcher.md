# 2026-0009. Fewer closures in the matcher

**Status:** shipped in v0.4.3

**Origin:** roadmap 2026-0009, taken up by a TODO task from jdc-lab's session
(jdc-lab 8c9df97c5d9ab378637b3c95a2bb75b2d8193823)

**Started:** 2026-10-31, in 4cf91879b7b641e4808be710d37408e2cd4bae15

**Finished:** 2026-10-31

## The plan

**Serves:** speed

**Size:** S (2)

The roadmap's item:

> Commit to a rule body or a group without making a closure, taking a pay
> parse closer to Participle's 6 µs from about 9 µs, with each new fast
> path checked on and off as the existing ones are.

The task, word for word:

**What:** take up roadmap 9 (Fewer closures in the matcher) as an
experiment, with jdc-lab's fact grammars as the measure: stop `once`
allocating on every call, and perhaps let `seq` step over literals and
classes without a closure, each behind `fastPaths`. Commit to `develop`
as usual, but don't release until jdc-lab has tested it and Iain gives
the go-ahead.

**Why:** in jdc-lab, chunking an ad costs far more than every stage
before it: about 200 ads a second, where splitting an ad into its
sections runs at over 1,500. Profiled, most of it is the lab's fact
grammars (pay, hours, work pattern, schedule, postcode), each run with
`FindAll(line, Outermost())` on every line of an ad, and most of that
is the matcher allocating. The lab expects five times as many ads soon.
This is dogfooding at work: the lab found the cost, and the fix belongs
here.

**What's known:** jdc-lab's roadmap 43 (jdc-lab
d4c7c75) made its own side cheaper first: a grammar is no longer run
over a line that can't hold a match (62% of lines hold no fact at all),
and each line is read once per ad. Chunking went from 9.3 s to 2.2 s
for 2,000 ads. What's left is the matcher, with grammar v0.4.2, on the
lab's benchmarks over 2,000 job ads (median of five, `-benchtime 3x`):

| jdc-lab benchmark | Time | Allocated | Allocations |
|---|---:|---:|---:|
| `internal/facts` `BenchmarkFindSamples` | 1.76 s | 1.76 GB | 64.8 million |
| `internal/chunk` `BenchmarkAdOfSamples` | 2.16 s | 2.20 GB | 74.6 million |

The ads aren't public, so grammar needs a benchmark of its own (step 1).
Before roadmap 43, when every grammar ran over every line, the lab
measured the following, and the matcher's share should be much the
same in the lines that are still read:

| Measure | Per pass |
|---|---:|
| time | 5.4 s |
| allocated | 5.7 GB |
| allocations | 210 million (about 27 bytes each) |
| `runtime.mallocgc`, of `FindAll`'s CPU | 5.6 s of 10.0 s, about 56% |

Counted with temporary counters, since reverted:

| Call | Count per pass | Allocations each |
|---|---:|---:|
| `once` | 50.8 million | 3: `end`, `caps`, and the closure (1.1, 1.2 and 3.5 GB in the profile) |
| of them, from `alt.match` | 15.8 million | |
| from `call.run`, a ratchet rule's body | 11.4 million | |
| from `step` | 23.6 million | |
| `seq.from` steps | 26.5 million | 1: the closure (6.1 GB) |
| of them, over a `lit` or `class` | 4.1 million | |
| over an `anchor` | 2.0 million | |

So `once` makes about 150 of the 210 million allocations. Its calls
nest strictly: each returns before its caller goes on, and its
continuation only records the first match and returns true. No node
keeps a continuation after its `match` returns (nothing stores a
`cont`, there are no goroutines and no memoising), and a `state` is
one match attempt, used by one goroutine. So the match `state` could
keep a stack of result slots, each with its continuation made once,
taken on entry to `once` and given back on return, and `once` would
allocate only when the stack grows past its deepest yet.

`lit` and `class` already have `at`, matching without a continuation,
so `seq.from` could step over a run of them, and `anchor`s, in a loop.
Each `step` raises the depth that `DepthLimit` counts and holds it while
the rest of the sequence matches, so the loop must raise it once for
each node it steps over, and lower it as many times when the rest
returns, or a limit would trip later than it does now. This saves at
most about 6 million allocations, an eighth of `once`'s, so it's second.

Allocation is about 56% of `FindAll`'s time, so removing about 70% of
the allocations might make the lab's facts about 1.5 times faster. That's
an estimate from the profile, not a measurement: measure before keeping
anything, as the practice says.

**Steps:**

1. Add a benchmark close to the lab's use, with `b.ReportAllocs()`:
   `FindAll(line, Outermost())` over many short lines, few of them with a
   match, through grammars with alternations, ratchet rules and word
   lists, such as the test `pay` grammar and a word list. Record it,
   `BenchmarkFindAll` and `BenchmarkParse` with `-benchmem -count 10`,
   and a CPU and memory profile, as the baseline.
2. Open roadmap 9's change record, and move 9 to Now.
3. Give `once` reusable result slots on the `state`, behind
   `fastPaths`. Check `TestFastPathsChangeNothing` (matches, failure
   reports and step counts the same with the fast paths on and off), the
   fuzz tests, `Trace`'s tests, and `StepLimit` and `DepthLimit`.
4. Measure against the baseline, with `benchstat`. Keep it if it pays.
5. Then, perhaps, the `seq` loop over `lit`, `class` and `anchor`, the
   same way, with a test that a `DepthLimit` trips at the same place.
6. Commit to `develop`, then add a task to jdc-lab's `TODO.md`, pushed
   at once, naming the commit to test. jdc-lab will run against it
   through its `go.work`:
   - `scripts/test-all.sh`, and `go test ./...` with `GOWORK=off`
     against v0.4.2 for comparison;
   - `go run ./cmd/pipeline run claude/before` at v0.4.2 and
     `claude/after` with the change, and `pipeline compare` showing
     every stage unchanged;
   - every ad of the run through `cmd/chunk -json` with each, and the
     output identical, byte for byte: every fact the same;
   - `go test ./internal/facts ./internal/chunk -run '^$' -bench
     'Samples$' -benchtime 3x -count 5`, before and after, and the time
     to chunk every ad.
   It reports to Iain, who gives the go-ahead for a release.
7. Release, then look in jdc-lab's `TODO.md` for the task waiting on it.

**Done when** the lab's facts allocate measurably less and run
measurably faster, with figures in roadmap 9's record; the fast-path,
fuzz, trace and limit tests pass with the fast paths on and off; jdc-lab
finds every output unchanged; and Iain has given the go-ahead and the
release is out.

## What happened

- **Outcome:** done. The lab's facts allocate a quarter of what they
  did and run 1.6 times as fast, where the plan estimated 1.5; every
  output of the lab's is unchanged; the fast-path, fuzz, trace and limit
  tests pass with the fast paths on and off; Iain gave the go-ahead, and
  it shipped in v0.4.3.
- **Baseline** (step 1): `BenchmarkFindLines`, added for this, runs
  `FindAll(line, Outermost())` over 140 short lines of a posting, 80
  facts among them, through a grammar of alternations, ratchet rules and
  a word list. With `-benchmem -count 10` on an Apple M2, Go 1.26.10:

  | Benchmark | Time | Allocated | Allocations |
  |---|---:|---:|---:|
  | `FindLines` | 1.870 ms ± 2% | 2.094 MiB | 78,440 |
  | `Parse` | 10.45 µs ± 6% | 14.19 KiB | 351 |
  | `FindAll` | 41.61 µs ± 4% | 40.36 KiB | 599 |

  Profiled, `FindLines` allocates 33% of its bytes in `once` (79% with
  what it calls), 28% in `seq.from` and 17% in `call.run`, and
  `runtime.mallocgc` is 60% of `FindAll`'s CPU: the lab's profile, in
  small.
- **`once`'s slots** (step 3): the state keeps a stack of result slots,
  each with its continuation made once; `once` takes the next on entry
  and gives it back on return, cleared, and allocates only when the stack
  grows past its deepest yet. Against the baseline, with benchstat
  (n=10, p=0.000 for each):

  | Benchmark | Time | Allocations |
  |---|---:|---:|
  | `FindLines` | 1.870 → 1.205 ms, −35.5% | 78,440 → 28,420, −63.8% |
  | `Parse` | 10.45 → 8.79 µs, −15.9% | 351 → 234, −33.3% |
  | `FindAll` | 41.61 → 38.84 µs, −6.7% | 599 → 410, −31.6% |

  Kept.
- **`seq` stepping over atoms** (step 5): `seq.from` steps over a run of
  literals, classes and anchors in a loop, raising the depth before each
  one, as `step` does, and lowering it by as many when the rest returns.
  `anchor` gained an `at`, as `lit` and `class` have. Against `once`'s
  slots alone: `FindLines` 1.205 → 1.126 ms (−6.6%), 28,420 → 26,280
  allocations (−7.5%); `Parse` 8.79 → 8.33 µs (−5.3%); `FindAll` no
  change in time (p=0.39), 3.2% fewer allocations. Less than `once`'s,
  as the plan expected, but measurable. Kept. Against the baseline, both
  together: `FindLines` −39.8% in time and −66.5% in allocations, `Parse`
  −20.4% and −35.9%, `FindAll` −7.0% and −33.7%.
- **Checks:** `TestFastPathsChangeNothing`, `TestFastPathsKeepLimits`,
  the Raku tests (Rakudo 2025.11, none skipped), `Trace`'s tests and
  the limit tests pass with the fast paths on and off; so do the race
  detector, `GOWORK=off`, 60 seconds of `FuzzMatch` and 30 of
  `FuzzCompile`. One 30-second run of `FuzzWords` stopped with "context
  deadline exceeded" and wrote no failing input; four more runs, two
  with this change and two without, passed, so it's taken for the
  fuzzer's own flake. jdc-lab's `go test ./...` passes against this
  checkout through its `go.work`.
- **Planted bugs:** a slot's continuation that drops the captures fails
  `TestFastPathsChangeNothing` on the pay grammar. In the `seq` loop, not
  raising the depth gives 19 differences in `TestFastPathsKeepLimits`,
  and lowering it before the rest of the sequence matches gives 9; the
  test gained two grammars with runs of atoms for this, without which the
  second gives 4. Every `once` sharing one slot changed nothing: an inner
  `once` always finishes, and clears the slot, before the outer one's
  continuation can fill it. The stack is kept all the same, as it doesn't
  lean on that finer ordering.
- **Changes from the plan:** none in what was built. benchstat was
  installed partway through step 1.
- **jdc-lab's check** (step 6), 2026-10-31, of
  312894f9cd68206515f759a6123952602a50c502 against v0.4.2 (jdc-lab
  07ca7b6; `GOWORK=off` for v0.4.2, the lab's `go.work` for the commit;
  each binary's build info checked to name the grammar it was built
  with): `scripts/test-all.sh` and `go test ./...` pass;
  `pipeline compare` shows every stage unchanged; all 25,954 sample ads
  through `cmd/chunk -json` give identical output, byte for byte. Over
  2,000 ads, median of five, `-benchtime 3x`:

  | jdc-lab benchmark | v0.4.2 | 312894f | Change |
  |---|---:|---:|---:|
  | `BenchmarkFindSamples`, time | 1.82 s | 1.11 s | 39% less |
  | allocated | 1.76 GB | 0.98 GB | 44% less |
  | allocations | 64.8 million | 16.7 million | 74% less |
  | `BenchmarkAdOfSamples`, time | 2.22 s | 1.39 s | 38% less |
  | allocated | 2.20 GB | 1.30 GB | 41% less |
  | allocations | 74.6 million | 24.5 million | 67% less |

  Chunking every ad, reading the run's ads included, went from 45.9 s
  to 33.9 s, 26% less.
- **What's left,** in the reference's gap "The matcher still makes
  closures for some steps": `FindAll` makes a new state, and so new
  slots, for every text, which jdc-lab's line-by-line search pays for
  each line; a sequence's steps over anything but an atom; and the match
  nodes rule calls make that backtracking throws away.
