# Rejecting a text that can't hold a match

**Status:** in progress

**Origin:** roadmap 23

**Started:** 2026-11-08, in 5b455edd50dafb698bb450416cc14d3cfdad06d6

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

Work has started.
