# To do

## Fewer allocations in the matcher, measured on jdc-lab's facts

**From:** jdc-lab's session, 2026-10-31 (jdc-lab 8c9df97c5d9ab378637b3c95a2bb75b2d8193823)
**Waiting on:** nothing: jdc-lab's roadmap 43 has shipped, and its
benchmarks are this task's baseline (below)

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
