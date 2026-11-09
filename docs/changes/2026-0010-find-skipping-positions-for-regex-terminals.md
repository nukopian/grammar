# 2026-0010. Find skipping positions for regex terminals

**Status:** shipped in v0.4.4

**Origin:** roadmap 2026-0010

**Started:** 2026-11-01, in 8678e28ae8f274bc4f9562a2674483fdbb3b1eee

**Finished:** 2026-11-01

## The plan

**Serves:** speed

**Size:** S (2)

**What:** `Find`, `FindAll` and `Split` skip the positions where a
grammar that starts with `Re` can't match, as they do for other
grammars, by asking regexp2 where the pattern next matches.

**Why:** a grammar whose first item is a regex terminal is tried at
every position, and pays a regex match at each. On a 4 KB posting,
`FindAll` with `Re(£\d[\d,]*(?:k\b)?)` takes 436 µs and 17,218
allocations, where `'£'` followed by the rest as `Re` takes 28 µs and
97: fifteen times as long, with nothing to say why.

**What's known:**

- The reference's gap suggested working out the pattern's first
  characters, or a hint from the caller. Either can be wrong (Go's
  `regexp/syntax` reads `\w`, `\d`, `\s` as ASCII where regexp2 reads
  them as Unicode; a hint can't be checked), and a wrong start set
  silently drops matches. Iain chose a third way, discussed on
  2026-11-01: regexp2's `FindRunesMatchStartingAt(text, from)` returns
  the first position at or after `from` where the pattern matches, so
  every position before it can be skipped, exactly, lookbehind included.
- Measured 2026-11-01 on an Apple M2: regexp2 finding each next match
  in that posting takes 8 µs and 15 allocations, so the `Re` grammar
  should come to about 36–40 µs, an estimate from the parts.
- `Re` compiles `\G(?:pattern)`, with and without `IgnoreCase`; the
  search needs `(?:pattern)` unanchored, with the same case setting as
  the terminal has where it's reached (`ci` in `startFinder.add`).
- A pattern containing `\G` means something else unanchored, so it
  skips nothing, as now. So does one that can match empty text (`re`'s
  `empty`), which can match anywhere.
- A search runs under `RegexTimeout` as one match. If it errors, the
  scan stops skipping for that regex, rather than panicking where the
  per-position matches wouldn't have.
- `canStart` is a pure function of the position; remembering the next
  match needs a little state per search, so it becomes a method on a
  value made for each search, used by `Find`, `FindAll`, `Split` and
  `FindErr`'s second search.
- `FuzzMatch`'s grammars are written in Raku syntax, which has no `Re`,
  so nothing fuzzes a grammar starting with one.

**Steps:**

1. Baseline: a `BenchmarkFindRe` beside `BenchmarkFindAll`, on the same
   text, with the grammar above.
2. In `starts.go`: a start kind holding a search regex; `add` gives one
   for a `re` that can't match empty and has no `\G`, for its case
   setting; `merge` carries them; a per-search scanner remembers each
   regex's next match, searching again only once the search passes it.
3. Tests: `starts_test.go` compares results with skipping on and off
   for grammars starting with `Re` (alone, in an alternation with a
   literal, under `:i`, after a lookahead, with lookbehind in the
   pattern, with `\G`, matching empty, and in a called rule); a planted
   bug (a scanner that never searches again) fails them. `FuzzMatch`'s
   grammar gains a rule `<r>` built with `Re`, and seeds that call it.
4. Measure `BenchmarkFindRe` and `BenchmarkFindAll` before and after.
5. The reference: the gap row narrowed to grammars that can match empty
   text, and `Re` patterns with `\G` or that can match empty; the Speed
   section's account of skipping; the CHANGELOG.

**Done when** a grammar starting with `Re` skips positions, with every
result unchanged, as the skipping tests and `FuzzMatch` show with
skipping on and off; `BenchmarkFindRe` shows the gain, and
`BenchmarkFindAll` no cost; and the reference and its gap say what's
still not skipped.

## What happened

- **Outcome:** a grammar starting with `Re` skips positions, and every
  result is unchanged, with skipping on and off, over the new
  `TestReStartsChangeNothing`, the existing `TestStartsChangeNothing`,
  and `FuzzMatch`, now with a regex terminal `<e>` in its grammar.
  `BenchmarkFindRe` gained as estimated. `BenchmarkFindAll` did not
  come out free: see Measurements. The reference and its gap say what's
  still not skipped.
- **Changes from the plan:** `canStart` became a `scan`, made per search
  and returned by value, so a search allocates nothing more for it. A
  pattern such as `(?=\d)`, which matches only empty text, counts as
  matching empty, as `reCanBeEmpty` says, and skips nothing.
- **Found on the way:** the first 17 seconds of fuzzing found that
  lookbehinds narrow their starts with the same `startFinder`, and read
  only its characters, so `<!after <e>>`, with `<e>` an `Re`, tried no
  starts at all and wrongly matched. A lookbehind whose pattern starts
  with a regex now tries every start, as before. `TestLookbehindChangesNothing`
  gained two such grammars, which failed first; the fuzz input is in
  `testdata/fuzz`. `TestReStartsSkip` also caught the scanner treating
  position 0 as a match before its first search.
- **A planted bug,** a scanner that searches once and never again,
  fails 144 checks.
- **Measurements,** Apple M2, ten runs each, with benchstat:
  `BenchmarkFindRe` 442 µs to 39.8 µs (−91%), 17,218 allocations to
  113. `BenchmarkParse` and `BenchmarkFindLines` unchanged.
  `BenchmarkFindAll` 38.4 µs to 39.8 µs (+3–4%, p<0.001, run against a
  worktree at the start commit, interleaved), allocations unchanged. A
  profile puts about 1.5 points of that in the per-position check
  (`canStart` 0.23 s against `scan.can` 0.30 s of 4.6 s); the rest
  looks like layout. Tried, without moving it: working out "skip
  nothing" once per search, a flag for "has regexes" on the scanner,
  and moving the regex path out of `can`. Kept, as they cost nothing.
- **What's left:** nothing for this item. Iain accepted the 3% on
  `FindAll` with a grammar that doesn't start with `Re`, the one point
  of Done when not met, for the elevenfold gain on grammars that do.
- **The 60 seconds of fuzzing** after the fix were clean.
- **The wash-up** found nothing to fix. It ran searches with a shared
  `Re` grammar from eight goroutines under the race detector, clean, and
  found the fallback after a skip search runs past `RegexTimeout`
  untested; `TestReStartsTimeout` covers it now, and fails if the
  fallback skips the rest of the text instead.
