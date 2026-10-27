# TODO

## Check match paths against the grammar (roadmap 16)

**From:** grammar's session, 2026-10-27 (grammar cec0be91c537ee2f4def239fea83cc06b24471e2)
**Waiting on:** a scalar release that exports its path parser (scalar's
`TODO.md`, "Export the path parser, for grammar's CheckPath")

**What:** build `(*Grammar).CheckPath`, steps 1 to 6 of the plan in
[its change record](docs/changes/2026-10-27-match-paths-checked-against-the-grammar.md):
bump scalar to the release (checked with `GOWORK=off`), tests first,
then the walk, the differential check, the documents, and dogfooding
in jdc-lab.

**Why:** a misspelt capture in a path reads undef and fails silently;
the grammar knows its captures, so it can say so.

**Done when** the change record's Done when holds, and its What
happened is written.
