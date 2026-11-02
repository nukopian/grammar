# To do

## Take up scalar's Hash that makes its values in one allocation

**From:** scalar's session, 2026-11-02 (scalar 4d630bb090a48da47206190f0940f8b3f5015a9d)
**Waiting on:** a scalar release after v0.6.3

**What:** raise `github.com/nukopian/scalar` in `go.mod` to the release
that has scalar's "Hash makes its plain values in one allocation", the
change asked for in scalar's `TODO.md` from this repository's session.

**Why:** each match is a `scalar.Hash("~", text, "from", n, "to", n)`.
Through `go.work`, against scalar v0.6.3, 8 runs each, interleaved:
`BenchmarkParse` 5.549 µs → 4.960 µs (−10.6%), 126 → 86 allocations;
`BenchmarkFindLines` −4.3%, `BenchmarkFindAll` −3.9%,
`BenchmarkAssert` −6.6%, with 21–32% fewer allocations in each. Half of
it was a spare `&SV{}` scalar `Hash` made for every value, which
grammar's experiment had also removed. scalar's
[record](https://github.com/nukopian/scalar/blob/develop/docs/changes/2026-11-02-hash-values-in-one-allocation.md)
has the rest.

**Steps:** `go get github.com/nukopian/scalar@` the new version, `go mod
tidy`, then with `GOWORK=off` run the tests and grammar's benchmarks
against v0.6.3's numbers above. Note the gain in the CHANGELOG, as
faster parsing.

**Done when:** `go.mod` names the release, the tests pass with
`GOWORK=off`, and `BenchmarkParse` shows the gain against the released
scalar. Tell scalar's session, whose record waits on that check.
