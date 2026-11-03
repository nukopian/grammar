# To do

## Keep each rule's starts, not only the last one searched for

**From:** jdc-lab's session, 2026-11-03 (jdc-lab 3baebf1d19d5d0bf6a9a6aee30b7e8a7066b11ed, work uncommitted)
**Waiting on:** nothing

**What:** `startsOf` (`starts.go`) caches a grammar's starts for one
rule: `startsEntry` holds the last rule asked for. A program that
searches one grammar for several rules in turn misses the cache at
almost every search, and works the starts out afresh each time. Keep
each rule's starts instead, in a map on the entry, as long as the
generation holds.

**Why:** jdc-lab's headings stage searches its `Schedule` grammar for
six rules in turn (`rota`, `days`, `shift`, `clock`, `points`, `span`),
for every line that might be a shift time. In its profile, `startsOf`
was 38% of reading an ad's headings, and 1.2 GB of the boilerplate
stage's garbage over 28,000 ads. With the fix below in this working
copy, through jdc-lab's `go.work`, 8 runs each, interleaved, on an
Apple M2:

| jdc-lab benchmark | before | after |
|---|---|---|
| `headings.BenchmarkReadSamples` | 412 ms, 2.48M allocs | 195 ms (−53%), 796k allocs (−68%) |
| `boilerplate.BenchmarkSamples` | 3.82 s, 14.6M allocs | 3.26 s (−15%), 10.6M allocs (−27%) |
| `facts.BenchmarkFindSamples` | 631 ms | 626 ms (no change: one rule a grammar) |

**What's known:** the fix is in this working copy, uncommitted, and
`go test -race .` passes with it. The entry is never changed once
stored: a new rule's starts go into a copy of the map, stored whole, so
concurrent searches stay as safe as they are now; two searches that
miss at once may each store a copy without the other's rule, which only
costs working one out again. A grammar's rules are few, so the map stays
small. The diff:

```go
// startsEntry caches a grammar's starts for each rule searched for,
// until any grammar changes. It's never changed once stored: a rule's
// starts are added to a copy.
type startsEntry struct {
	gen   uint64
	rules map[string]ruleStartsOf
}

// ruleStartsOf is what startsOf returns for a rule.
type ruleStartsOf struct {
	s     *starts
	empty bool
}

func (g *Grammar) startsOf(rule string) (*starts, bool) {
	gen := generation.Load()
	e := g.starts.Load()
	if e != nil && e.gen != gen {
		e = nil
	}
	if e != nil {
		if r, ok := e.rules[rule]; ok {
			return r.s, r.empty
		}
	}
	f := &startFinder{g: g, rules: map[string]*ruleStarts{}}
	s := &starts{runes: map[rune]bool{}}
	empty := f.add(&call{name: rule}, false, s)
	s.table()
	rules := map[string]ruleStartsOf{rule: {s, empty}}
	if e != nil {
		for k, r := range e.rules {
			rules[k] = r
		}
	}
	g.starts.Store(&startsEntry{gen, rules})
	return s, empty
}
```

**Steps:**

1. A test that searching one grammar for two rules in turn works each
   rule's starts out once (count calls to `startFinder.add` for the
   rule's call, or the entries stored), failing against the code before.
2. A test that a change to a grammar (a new generation) still drops
   every rule's starts, as `TestStatesShared`'s concurrency checks run
   under `-race` in CI.
3. A benchmark that alternates rules on one grammar, such as
   `Find(text, Rule("a"))` then `Rule("b")`, measured before and after.
4. CHANGELOG line under Unreleased, then a release, and a task in
   jdc-lab's `TODO.md` to take it up.

**Done when:** a grammar keeps each rule's starts, the tests above pass
and fail without it, and a release carries it.
