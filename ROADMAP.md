# Roadmap

What grammar means to do, and why. The [CHANGELOG](CHANGELOG.md) says
what it has done, and the [README](README.md) what it is now. The
reference's [Gaps and possible additions][gaps] has the detail behind
each item: when you'd need it, what it would take, and the effort.

Each item in Now, Next and Later names what it serves:

- **debugging:** seeing why a grammar matches what it does, or doesn't;
- **Raku:** doing more of what Raku's grammars do, as Raku does it;
- **word lists:** matching synonym data as it's written;
- **speed:** parsing and searching at high volumes;
- **handing on:** taking a grammar that has settled to other tools.

Items come in four groups, the most useful first in each:

- **Now:** being worked on, each with a plan detailed enough to pick up
  cold, in its [change record](docs/changes/README.md); never more than
  three, so that it stays a queue;
- **Next:** decided, but not started;
- **Later:** meant to be done, but not yet planned;
- **Ideas:** might be done, or never; kept so they aren't lost, in a
  line or two, newest last.

Each item has a number, given when it's added, in whichever group. It
keeps it as it moves between groups, and a number is never used again,
so that 7 means the same item in a commit message months later. Next
number: 24.

Each item has a size too, a guess at how much work it is, as a T-shirt
size and its story points: XS 1, S 2, M 3 and L 5. Sizes compare items
with each other; they aren't a measure of time. The reference's Effort
for the gap behind an item guides it. An item bigger than L is broken
into items that aren't, each saying what it was split from. Items that
can't be done one without another say so ("Moves with:"), and move
between groups together.

When an item ships, it moves from here to the CHANGELOG, whose line
for it ends with its number, such as "(roadmap 7)", and links to its
record. An idea that's
dropped is deleted, and the commit that drops it names its number.

## Now

Nothing yet.

## Next

### 5. A picture of a grammar

**Serves:** debugging

**Size:** S (2)

**What:** print a grammar as EBNF, with a note wherever a Raku feature
has no EBNF form (backreferences, lookbehind, word lists, a token's
ratchet), and draw railroad diagrams from it with an existing tool.

**Why:** documenting, reviewing and learning a grammar: a diagram of
the pay grammar beside its rules shows at a glance what the rules take
a while to read.

**Done when** every node prints, with a test that each does; it has an
example; the how-to has a recipe that draws a diagram with a named
tool, checked to work; and the reference's gap is gone.

## Later

### 11. Stemming in word lists

**Serves:** word lists

**Size:** M (3)

Make a word's other forms when it's added, as `Plurals` does for
regular plurals, so "visiting" matches a list that has "visit". English
needs a real stemmer, such as Porter's. Synonym data rarely lists every
form, and jdc-lab's word lists, which already lean on `Plurals`, would
gain from it.

## Ideas

An idea is a numbered heading, the date it was added, its size, and a
line or two on what and why. Each wash-up prunes them: each is kept,
promoted to Later or Next with its number, or dropped.

### 8. Rule parameters and dynamic variables

Added 2026-10-25. **Size:** M (3). `token amount($cur)` and
`$*CURRENCY`, for parsing that depends on context, such as a currency
that changes how amounts read.

### 17. A phrasebook from Raku to Participle

Added 2026-10-31, split from 6 (handing a settled grammar to
Participle, added 2026-10-25). **Size:** S (2). Each Raku grammar
construct beside the same thing in Participle, and what Participle
can't do, with every row run in both on the same inputs, as the Raku
phrasebook's rows are. It's useful alone, and the specification for 18.

### 18. A Participle dialect

Added 2026-10-31, split from 6. **Size:** M (3). A Raku-like syntax
that accepts only what the phrasebook (17) covers, failing at compile
time, saying why, on anything else. Needs 17.

### 19. A generator for Participle

Added 2026-10-31, split from 6. **Size:** M (3). Write Participle
structs and a lexer from a dialect grammar (18) with `go generate`,
checked by running both on the same inputs. Needs 18.

### 20. Each branch's declarative prefix

Added 2026-10-31, split from 7 (true longest-token matching, added
2026-10-25). **Size:** M (3). **Moves with:** 21 and 22. Work out the
part of each `|` branch and proto candidate before anything that could
backtrack or run code, as Rakudo does in NQP.

### 21. An automaton from the prefixes

Added 2026-10-31, split from 7. **Size:** L (5). **Moves with:** 20 and
22. Build an NFA, then a DFA, from the branches' declarative prefixes,
that says how much of the text each prefix matches.

### 22. Alternation decided by the automaton

Added 2026-10-31, split from 7. **Size:** M (3). **Moves with:** 20 and
21. Try `|` branches and protos in the order the automaton gives,
rather than matching every branch in full. It would close the cases
where `Alt`'s approximation differs from Raku, and speed up large
alternations, protos and nested alternations in a `regex`.

### 23. Rejecting a text that can't hold a match

Added 2026-11-01, from jdc-lab. **Size:** S (2). Work out characters
every match must contain, such as a digit for a pay figure, and have
`Find` and the rest return at once for a text with none, without
trying a position. jdc-lab keeps such a check beside each fact grammar
by hand (`MayHold`), as a prose line costs a search 17 µs where a digit
check takes 0.17 µs; this would make it automatic.

[gaps]: docs/REFERENCE.md#gaps-and-possible-additions
