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
number: 27.

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

### 8. Rule parameters and dynamic variables

**Serves:** Raku

**Size:** M (3)

**What:** `token amount($cur)` and `$*CURRENCY`: arguments in the
syntax and on a rule call, and dynamic variables visible to the rules
a rule calls, as in Raku.

**Why:** parsing that depends on context, such as a currency that
changes how amounts read, where inheritance and roles would need a
grammar for each context. Measured on 2026-11-02: a frame pointer on
every call costs 2.1% on every grammar, where a stack on the state,
checked at each call as failure tracking is, costs 0.2%, within noise.
What's open is the design: how a code assertion sees the variables,
and whether `<$var>` interpolation comes with them.

**Done when** both work as Raku's do, with cases in the Raku tests; the
design questions are answered in the reference; and the reference's
gap is gone.

### 26. Phrase needs

**Serves:** speed

**Size:** M (3)

**What:** needs as strings as well as characters, so that `Find` and
the rest reject a text with none of the phrases every match needs: a
word list needs one of its phrases' first words, a literal itself, in
any case under `:i` or `FoldCase`, and an alternation the union of its
branches'. The text is checked for them before it's read into runes, as
needs of characters are (roadmap 23).

**Why:** a word list's needs are only its phrases' first letters,
which most lines of prose hold, so jdc-lab still checks by hand for a
phrase's first word, or a day's first letters, before its pattern and
schedule grammars; those checks save about 30% of its time (2026-11-08).
Schedule's check may not be matched: it passes a line only if it has a
digit, a day's first three letters, or "M", "F" and "-" all present,
for "M - F", and an alternation needs the union of its branches', so
that branch could weaken Schedule's needs to a single common letter or
"-". Pattern's word list should gain fully.

**Done when** jdc-lab's `MayHold` check for its pattern grammar can go,
and its check for its schedule grammar only if that grammar's needs stay
as sharp as the check (an "M - F" branch's needs kept sharp, or the
check kept, and why), each measured with jdc-lab's
`BenchmarkFindSamples`, with its output unchanged, not assumed; behind
a switch with a differential test and a planted bug; and the
reference's gap is gone.

## Later

### 11. Stemming in word lists

**Serves:** word lists

**Size:** M (3)

Make a word's other forms when it's added, as `Plurals` does for
regular plurals, so "visiting" matches a list that has "visit". English
needs a real stemmer, such as Porter's. Synonym data rarely lists every
form, and jdc-lab's word lists, which already lean on `Plurals`, would
gain from it.

### 20. Each branch's declarative prefix

**Serves:** Raku, speed

**Size:** M (3). **Moves with:** 21 and 22.

Split from 7 (true longest-token matching). Work out the part of each
`|` branch and proto candidate before anything that could backtrack or
run code, as Rakudo does in NQP. Roadmap 25's branch starts, kept on each
alternation, are its first character.

### 21. An automaton from the prefixes

**Serves:** Raku, speed

**Size:** L (5). **Moves with:** 20 and 22.

Split from 7. Build an NFA, then a DFA, from the branches' declarative
prefixes, that says how much of the text each prefix matches.

### 22. Alternation decided by the automaton

**Serves:** Raku, speed

**Size:** M (3). **Moves with:** 20 and 21.

Split from 7. Try `|` branches and protos in the order the automaton
gives, rather than matching every branch in full. It would close the
cases where `Alt`'s approximation differs from Raku, and speed up large
alternations, protos and nested alternations in a `regex`.

## Ideas

An idea is a numbered heading, the date it was added, its size, and a
line or two on what and why. Each wash-up prunes them: each is kept,
promoted to Later or Next with its number, or dropped.

### 5. A picture of a grammar

Added 2026-10-26; parked from Next 2026-11-02. **Size:** S (2). Print a
grammar as EBNF, noting where a Raku feature has no EBNF form, and draw
railroad diagrams from it with an existing tool, for documenting,
reviewing and learning a grammar.

### 18. A Participle dialect

Added 2026-10-31, split from 6 (handing a settled grammar to
Participle, added 2026-10-25). **Size:** M (3). A Raku-like syntax
that accepts only what Participle can express, failing at compile
time, saying why, on anything else.

### 19. A generator for Participle

Added 2026-10-31, split from 6. **Size:** M (3). Write Participle
structs and a lexer from a dialect grammar (18) with `go generate`,
checked by running both on the same inputs. Needs 18.

[gaps]: docs/REFERENCE.md#gaps-and-possible-additions
