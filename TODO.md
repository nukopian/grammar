# To do

## Fewer allocations in the matcher, measured on jdc-lab's facts

**From:** jdc-lab's session, 2026-10-31 (jdc-lab 8c9df97c5d9ab378637b3c95a2bb75b2d8193823)
**Waiting on:** Iain's go-ahead; jdc-lab has checked the commit

**What:** what's left of roadmap 9, whose plan, measurements and checks
are in its [change record](docs/changes/2026-10-31-r9-fewer-closures-in-the-matcher.md).
Steps 1 to 5 are done: `once` keeps its results in slots on the state,
and `seq` steps over atoms in a loop, both behind `fastPaths`.

**jdc-lab's check,** 2026-10-31, of 312894f9cd68206515f759a6123952602a50c502
against v0.4.2 (jdc-lab 07ca7b6 with its `TODO.md` task; `GOWORK=off`
for v0.4.2, the lab's `go.work` for the commit; each binary's build
info checked to name the grammar it was built with):

- `scripts/test-all.sh` passes, and `go test ./...` with `GOWORK=off`.
- `pipeline compare claude/before claude/after`: every stage unchanged.
- Every ad of the samples (25,954) through `cmd/chunk -json` each way:
  the outputs identical, byte for byte, so every fact is the same.
- `go test ./internal/facts ./internal/chunk -run '^$' -bench 'Samples$'
  -benchtime 3x -count 5`, over 2,000 ads, median of five:

  | jdc-lab benchmark | v0.4.2 | 312894f | Change |
  |---|---:|---:|---:|
  | `BenchmarkFindSamples`, time | 1.82 s | 1.11 s | 39% less |
  | allocated | 1.76 GB | 0.98 GB | 44% less |
  | allocations | 64.8 million | 16.7 million | 74% less |
  | `BenchmarkAdOfSamples`, time | 2.22 s | 1.39 s | 38% less |
  | allocated | 2.20 GB | 1.30 GB | 41% less |
  | allocations | 74.6 million | 24.5 million | 67% less |

- Every ad chunked with `cmd/chunk`, reading the run's ads included:
  45.9 s, then 33.9 s, 26% less.

The plan estimated the lab's facts about 1.5 times faster; they're 1.6
times faster, with a quarter of the allocations.

**Steps:**

6. Done: jdc-lab checked the commit and reported to Iain (above), who
   gives the go-ahead for a release. Put the lab's figures in the record.
7. Release, then look in jdc-lab's `TODO.md` for the task waiting on it.
   Finish the record, saying the release, and move 9 out of the
   roadmap's Now.

**Done when** jdc-lab finds every output unchanged, Iain has given the
go-ahead, and the release is out.
