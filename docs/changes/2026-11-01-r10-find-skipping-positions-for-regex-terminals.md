# Find skipping positions for regex terminals

**Status:** in progress

**Origin:** roadmap 10

**Started:** 2026-11-01, in 8678e28ae8f274bc4f9562a2674483fdbb3b1eee

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
