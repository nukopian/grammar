# To do

## Take up scalar's Hash, Array and Push that make their values in one allocation

**From:** scalar's session, 2026-11-02 (scalar 4d630bb090a48da47206190f0940f8b3f5015a9d and 05f11b932d9d9739c5ab1d9da5c58a4b1df617cc)
**Waiting on:** a scalar release after v0.6.3

**What:** raise `github.com/nukopian/scalar` in `go.mod` to the release
that has scalar's "Hash makes its plain values in one allocation", the
change asked for in scalar's `TODO.md` from this repository's session,
and "Array and Push make their plain values in one allocation", which
took it further the same day.

**Why:** each match is a `scalar.Hash("~", text, "from", n, "to", n)`.
Through `go.work`, against scalar v0.6.3, 8 runs each, interleaved:
`BenchmarkParse` 5.549 µs → 4.960 µs (−10.6%), 126 → 86 allocations;
`BenchmarkFindLines` −4.3%, `BenchmarkFindAll` −3.9%,
`BenchmarkAssert` −6.6%, with 21–32% fewer allocations in each. Half of
it was a spare `&SV{}` scalar `Hash` made for every value, which
grammar's experiment had also removed. scalar's
[record](https://github.com/nukopian/scalar/blob/develop/docs/changes/2026-11-02-hash-values-in-one-allocation.md)
has the rest.

`Array` and `Push`, given values that don't flatten, now make their new
scalars in one allocation too, and `Value` no longer boxes a float or a
large integer a second time. In scalar, an array of 20 floats builds
2.2 times as fast, with 3 allocations rather than 40. Grammar mostly
pushes one value at a time (`FindAll`'s results, a repeated capture in
`assert.go` and `grammar.go`), which a block doesn't help, so it gains
less: against 4d630bb090a48da47206190f0940f8b3f5015a9d, `BenchmarkFindAll`
was 4.7% faster, and `Parse`, `FindLines` and `Assert` didn't move.
`Split` pushes two strings for each piece (`grammar.go`, `pieces.Push`),
one allocation fewer each.

**Steps:** `go get github.com/nukopian/scalar@` the new version, `go mod
tidy`, then with `GOWORK=off` run the tests and grammar's benchmarks
against v0.6.3's numbers above, `BenchmarkFindAll` included. Grammar
has no benchmark for `Split`; add one if its gain is worth showing. Note the gain in the CHANGELOG, as faster parsing and
searching.

**Done when:** `go.mod` names the release, the tests pass with
`GOWORK=off`, and `BenchmarkParse` shows the gain against the released
scalar. Tell scalar's session, whose record waits on that check.
