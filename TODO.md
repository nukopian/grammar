# TODO

## Check match paths against the grammar (roadmap 16)

**From:** grammar's session, 2026-10-27 (grammar cec0be91c537ee2f4def239fea83cc06b24471e2)
**Waiting on:** nothing: scalar v0.6.1 exports the parser (scalar's
session, 2026-10-27, scalar 823987d8c02d4d73ec56952160d6efa86cd582d2)

**What scalar shipped:** not the plan's `ParsePath`, but
`scalar.AtSteps(path string) ([]scalar.AtStep, error)`. An `AtStep` has
`Kind`, `Key` and `Index`, and nothing else. `Kind` is `scalar.StepKey`
(a name, `{key}` or `["key"]`, with its `Key`), `scalar.StepIndex`
(`[0]` or `[-1]`, with its `Index`) or `scalar.StepEither` (a number
after a dot, as in `content.0`, with both: `Key` is the digits as
written, so `007`, and `Index` is 7). It is `At`'s own parser, which
`At`, `AtOK`, `DeleteAt`, `CheckPath` and templates share. A malformed
path gives exactly `scalar.CheckPath`'s error, offset and all. The
steps are a fresh slice each call. scalar's
`docs/changes/2026-10-27-export-the-path-parser.md` has the record, and
its reference documents it beside `CheckPath`.

**What:** build `(*Grammar).CheckPath`, steps 1 to 6 of the plan in
[its change record](docs/changes/2026-10-27-match-paths-checked-against-the-grammar.md):
bump scalar to the release (checked with `GOWORK=off`), tests first,
then the walk, the differential check, the documents, and dogfooding
in jdc-lab.

**Why:** a misspelt capture in a path reads undef and fails silently;
the grammar knows its captures, so it can say so.

**Done when** the change record's Done when holds, and its What
happened is written.
