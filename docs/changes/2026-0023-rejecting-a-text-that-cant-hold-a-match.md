# 2026-0023. Rejecting a text that can't hold a match

**Status:** shipped in v0.5.5

**Decisions:** D1 settled: ship as it is, with phrase needs as 2026-0026. None open.

**Origin:** roadmap 2026-0023

**Started:** 2026-11-08, in 5b455edd50dafb698bb450416cc14d3cfdad06d6

**Finished:** 2026-11-08

## The plan

**Serves:** speed

**Size:** S (2)

**What:** work out characters every match must contain, such as a digit
for a pay figure, and have `Find` and the rest stop trying positions
once none of them is left in the text, returning at once for a text
with none. A sequence needs one of its items' sets, an alternation the
union of its branches'.

**Why:** searching many short texts, most holding no match, as jdc-lab
does line by line. At v0.5.1 a 126-character prose line cost jdc-lab's
pay grammar 17 µs, where checking for a digit took 0.17 µs, so jdc-lab
keeps such a check beside each fact grammar by hand (`MayHold`); this
would make it automatic. Roadmap 25 put a table of ASCII characters on
the starts, which the set can use. Measured on 2026-11-02: a scan
before each search with the `starts` type costs about 5 ns a character,
7% of a `FindAll` that matches, and gains nothing where skipping starts
is already sharp; one scan backwards to the last needed character,
with `Find` trying no position past it, avoids that cost.

**Done when** a search of a text with none of the needed characters
tries no position, behind a switch with a differential test and a
planted bug; jdc-lab's `MayHold` checks can go, with its output
unchanged; and the reference's gap is gone.

## What happened

- **Outcome:** against Done when:
  - A search of a text with none of the needed characters tries no
    position: `TestNeedsTryNothing` traces such a search, and the trace
    is empty. It's behind `skipNeeds`, with `TestNeedsChangeNothing`
    comparing every search, and `FindErr`'s and `FindAllErr`'s reports,
    on and off, over every Raku case, and `FuzzMatch` doing the same.
  - jdc-lab's `MayHold` checks for a digit, before its pay, hours,
    postcode and figure grammars, can go: with needs they save 2% of its
    time. Its checks before its schedule and pattern grammars can't:
    they still save about 30%. See Decision 1.
  - The reference's gap is narrowed to what's left, "A word list's
    needs are weak", rather than gone.
- **Needs** are worked out with the starts, and kept with them until a
  grammar changes (`searchFor`, `needs.go`). A literal needs its rarest
  character; a class or word list what it can start with; a sequence or
  conjunction its rarest item's needs; an alternation, first-match
  alternation or pattern list all its branches' together, and nothing if
  any branch needs nothing; a repeat its item's, unless it may match
  nothing. Anchors, lookarounds, code assertions, backreferences and
  regex terminals need nothing. "Rarest" is a guess: the set with the
  fewest ASCII characters.
- **Changes from the plan:**
  - The loops in `Find`, `FindAll` and `Split` stop at the last needed
    position, rather than asking `can` at each one after it, and the
    backward scan looks ASCII characters up in the table inline. With
    the scan alone, a rejected line cost 0.70 µs; with these, 0.34 µs.
  - `Find` and `FindAll` check an ASCII text for its needs as a string,
    before reading it into runes in NFC (`rejects`). In jdc-lab, a
    rejected search's fixed cost, reading the text and making the
    result, was most of what was left: without `MayHold` for its digit
    grammars it took 509 ms, against 463 ms with; with this, 477 ms
    against 466 ms. Only ASCII text is checked this way, as NFC can
    compose a needed character from others (`TestNeedsNFC`). It reads
    from the end, as the backward scan does: reading from the start made
    `BenchmarkFindAll`, whose only needed characters are near the end
    of 4 KB, 6.5% slower.
  - `FindErr` and `FindAllErr` try the positions the needs rule out,
    so that a rejected text is still reported from the attempt that got
    furthest.
- **A bug found on the way,** fixed in its own commit first
  (324e17217a90f1bb058671869b788a1160b9b91b): a phrase added to a word
  list after a search wasn't found by the next, as the starts worked out
  from the list were kept. Needs would have been kept the same way.
  Adding a phrase now counts as a change to the grammars.
- **Measurements,** on an Apple M2, `benchstat`, 8 runs each against a
  baseline taken first, and 12 interleaved before and after for the
  searches:
  - A 100-character line of prose with no digit, searched for pay with
    an optional currency sign: 2.6 µs before, 0.34 µs after.
  - `BenchmarkFindProse`, new, searching each line of a posting for such
    a pay grammar: 456 µs to 284 µs (38% less), 13% fewer bytes.
  - `BenchmarkFindRules`: 335 µs to 310 µs (7.5% less), its lines with
    no digit rejected for the clock rules.
  - `BenchmarkFindAll`, `BenchmarkFindRe`, `BenchmarkFindLines`,
    `BenchmarkParse` and the rest: no change beyond noise, interleaved.
  - jdc-lab's `BenchmarkFindSamples`, every line of 2,000 ads, 64,244
    lines: 497 ms with v0.5.3 and `MayHold`; 466 ms with needs and
    `MayHold`; 477 ms with needs and `MayHold` only for the schedule and
    pattern grammars; 673 ms with needs and no `MayHold`; 1,005 ms with
    v0.5.3 and no `MayHold`. Its output, every line's facts as JSON, was
    the same byte for byte with v0.5.3, with needs, and with needs and no
    `MayHold`.
- **Checks:** three planted bugs, each caught by `TestNeedsChangeNothing`:
  treating a repeat that may match nothing as needing its item;
  `FindErr`'s report limited by the needs; and the string check skipping
  non-ASCII bytes rather than leaving the text to the search. `FuzzMatch`
  ran for 60 seconds, and `FuzzWords` for 60 after the word-list fix.
  The Raku differential tests ran and passed.
- **Tried and reverted:** comparing `FindErr`'s reports over every Raku
  grammar, and stripping characters from every text derived from the
  Raku inputs, made `TestNeedsChangeNothing` take 40 seconds; the
  reports are compared over the package's test grammars, and characters
  stripped from the inputs themselves, which caught every planted bug in
  9 seconds.
- **The wash-up** found that `TestNeedsTryNothing`'s check of
  `FindErr`'s report couldn't fail: it looked for "1" in the report,
  which every report holds ("line 1"), and passed with the report
  limited by the needs. It now checks that "x £ y" is reported from the
  £, at column 5, and fails with that bug planted. It also found that a
  search right after a phrase is added to a word list costs about
  30 µs, against 2 µs, as every grammar's starts, needs and `Check` are
  worked out again; the reference now says so, beside how a list is
  loaded.
- **What's left:** a word list's needs are weak, the first letters of
  its phrases; see the reference's gap "A word list's needs are weak",
  and roadmap 26.

## Decisions

- [x] **D1.** Phrase needs now, or later? Done when said jdc-lab's
  `MayHold` checks could go. The four for a digit can; the two for the
  schedule and pattern grammars, checking for a day's first letters or
  a phrase's first word, can't, as needs of single characters are too
  weak for word lists, and they still save about 30% of jdc-lab's time.
  Needs as strings as well as characters would close that (the gap has
  what it would take, size S to M). The choice is between shipping 23
  as it is, with phrase needs as a new roadmap item, or extending 23
  before the release.

  **Decided by Iain, 2026-11-08: ship 23 as it is, and make phrase
  needs a new roadmap item,** 2026-0026. jdc-lab loses nothing by waiting:
  its schedule and pattern checks work, are tested against the
  grammars, and still save about 30%, and nothing in the lab is
  blocked on them. 23's plan is met apart from this caveat, and adding
  strings would grow it from S to S–M after the work is done: a new
  finding goes in a new item. Phrase needs might not remove Schedule's
  check anyway. It's stricter than any single string: it wants "M",
  "F" and "-" all present, or a day's first three letters, and an
  alternation needs the union of its branches', so the "M - F" branch
  could weaken Schedule's needs to one common letter. Pattern's word
  list would likely gain fully. So 26's Done when names which checks
  it expects to remove, measured against jdc-lab, not assumed, as
  jdc-lab's session also asked.
