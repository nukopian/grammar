# To do

## Take up scalar v0.6.0

**From:** scalar's session, 2026-10-26 (scalar 5596f10cd4161e5c18a5ee93f12c3bcaf6abb547)
**Waiting on:** nothing

**What:** move grammar from scalar v0.5.9 to v0.6.0, released
2026-10-26, and say so in grammar's CHANGELOG. Then release grammar if
its own rules call for it, so that job-meta can take both.

**Why:** v0.6.0 keeps the digits of a number read as text when a
`float64` can't hold them, so long decimals and big ids from JSON, YAML
or a dump are written back exactly; a whole number beyond `int64`
prints as an integer; `%` and `++` follow Perl further (`1e19 % 7` is
`3`, `"007"++` is `"008"`); and `To` and `Decode` fill `*big.Rat` and
the like from a number. Some output changes, which is why it's a minor
version: see scalar's
[v0.6.0 notes](https://github.com/nukopian/scalar/releases/tag/v0.6.0).

**Files:** `go.mod`, `go.sum`, `CHANGELOG.md`; anything whose output
shows a number beyond `int64` or a long decimal, or uses `%` or `++` on
strings of digits.

**How to check it's done:** without the workspace, so that the check is
against the published module:

```
GOWORK=off go get github.com/nukopian/scalar@v0.6.0
GOWORK=off go mod tidy
GOWORK=off go test ./...
```

grammar's tests already passed against scalar's develop, through
`go.work`, before the release, so a failure here means something
differs between the published module and that checkout.
