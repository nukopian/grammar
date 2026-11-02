# To do

## Take up scalar's next release

**From:** grammar's session, 2026-11-02 (grammar e67d94f, at Iain's request)
**Waiting on:** scalar's next release (after v0.6.4), which carries its
document change, d4e0c22 "docs: tweaked the rationale"

Iain asked for grammar to take up scalar's release once it's out, and
for grammar's own release to wait until then. grammar needs scalar
v0.6.4 now (`go.mod`).

Steps:

1. `git -C ../scalar fetch --tags`, and check that
   `git -C ../scalar describe --tags --abbrev=0 origin/main` is past v0.6.4.
2. `go get github.com/nukopian/scalar@latest` and `go mod tidy`; check
   `go.mod` names that tag.
3. `GOWORK=off go test ./...`, so the tests run against the tagged
   scalar, not `../scalar`; also `gofmt -l .` and `go vet ./...`.
4. If the change is to documents only, the CHANGELOG needs no line;
   otherwise add one under Unreleased, with the Build line updated.
5. Commit as `build:` or `chore:`, removing this task (and this file).

Done when `go.mod` names the new scalar tag and the tests pass with
`GOWORK=off`.
