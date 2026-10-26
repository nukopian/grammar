# To do

## Take up scalar v0.5.9

**From:** scalar's session, 2026-10-26 (scalar b7a35bd683f918a019feadf6b020a759af323d7b, tagged v0.5.9)
**Waiting on:** nothing

**What:** bump grammar's scalar from v0.5.7 to v0.5.9, as
docs/MAINTAINING.md's "Taking up a new scalar" says:

```
s=v0.5.9
cd ~/Git/github.com/nukopian/grammar
GOWORK=off go get "github.com/nukopian/scalar@$s"
GOWORK=off go mod tidy
GOWORK=off go test ./...
```

**Why:** Iain asked for grammar to be on the current scalar (2026-10-26),
although grammar needs nothing new from it, which the guide would
otherwise leave to job-meta alone. v0.5.8 changed only scalar's docs.
v0.5.9 makes scalar's `S` with a replacement string such as `"$1"` about
1.6 times as fast, adds `$+` and `$+{name}`, and gives nothing for a
reference to a group that doesn't exist, as Perl does. grammar's own code
calls `S` with no replacement string that refers to the match, and its
`Re` uses regexp2 directly, so grammar's behaviour shouldn't change.

**Files:** `go.mod`, `go.sum`, and `CHANGELOG.md` only if users would
notice, which they shouldn't.

**Done when:** the tests pass with `GOWORK=off`, gofmt and vet are clean,
and the bump is committed on `develop` as `build: scalar v0.5.9`, with
this task removed (and this file, if it's the last task) in the same
commit. CI on `develop` must pass, its Raku tests included.
