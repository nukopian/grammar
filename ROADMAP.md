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
number: 16.

When an item ships, it moves from here to the CHANGELOG, whose line
for it ends with its number, such as "(roadmap 7)", and links to its
record. An idea that's
dropped is deleted, and the commit that drops it names its number.

## Now

Nothing yet.

## Next

### 1. A trace of a parse

**Serves:** debugging

**What:** a `Trace(w io.Writer)` option that prints each rule as it's
entered, indented by depth, then `* MATCH "text"` or `* FAIL`, in the
style of Raku's `Grammar::Tracer`.

**Why:** when a grammar won't match, `ParseErr` reports only the
furthest failure, not which rules were tried, where, or what each
matched. Watching the rules is the quickest way to debug a grammar, and
to learn how grammars work. The README's
[When not to use it](README.md#when-not-to-use-it) names it as missing.

**Done when** tracing is hooked into rule calls (not every step, which
would bury the rules), `BenchmarkParse` shows no real cost with it off,
`FuzzMatch` checks that tracing changes no result, it has an example and
a place in the reference, and the README no longer says there's no
trace.

## Later

### 3. Code assertions

**Serves:** Raku

`<?{name}>` and `<!{name}>`, calling a Go function passed to `Compile`
with the captures so far, for checks only code can make mid-parse, such
as a pay range's minimum being below its maximum.

## Ideas

An idea is a numbered heading, the date it was added, and a line or two
on what and why. Each wash-up prunes them: each is kept, promoted to
Later or Next with its number, or dropped.

### 4. Failure reports from Find and FindAll

Added 2026-10-25. Say why pay wasn't found anywhere in a posting, not
just why one string didn't parse: the furthest failure across every
start position, with the noise of positions that fail at once kept out.

### 5. A picture of a grammar

Added 2026-10-25. Print a grammar as EBNF, with a note wherever a Raku
feature has no EBNF form, and draw railroad diagrams from it with an
existing tool, for documenting, reviewing and learning a grammar.

### 6. Handing a settled grammar to Participle

Added 2026-10-25. Three steps, each building on the one before: a
phrasebook from Raku to Participle, with every row run in both; a
Raku-like dialect that accepts only what the phrasebook covers; and a
generator that writes Participle structs and a lexer from a dialect
grammar, checked by running both on the same inputs.

### 7. True longest-token matching

Added 2026-10-25. Decide `|` from each branch's declarative prefix with
an automaton, as Rakudo does, rather than by matching every branch in
full. It would close the cases where `Alt`'s approximation differs from
Raku, and speed up large alternations and protos.

### 8. Rule parameters and dynamic variables

Added 2026-10-25. `token amount($cur)` and `$*CURRENCY`, for parsing
that depends on context, such as a currency that changes how amounts
read.

### 9. Fewer closures in the matcher

Added 2026-10-25. Commit to a rule body or a group without making a
closure, taking a pay parse closer to Participle's 6 µs from about 9 µs,
with each new fast path checked on and off as the existing ones are.

### 10. Find skipping positions for regex terminals

Added 2026-10-25. Work out where a grammar starting with `Re` can begin,
from the pattern's first characters or a hint, so `Find` can skip the
rest of a long text as it does for other grammars.

### 11. Stemming in word lists

Added 2026-10-25. Make a word's other forms when it's added, as
`Plurals` does for regular plurals, so "visiting" matches a list that
has "visit". English needs a real stemmer, such as Porter's.

### 15. Say where captures inside a text capture go

Added 2026-10-27, from jdc-lab's use (jdc-lab 67eecbab405c9d4c694e7c5c2710f3cc70bac5c2, work
uncommitted). The reference's row for `$<k>=[ … ]` says "`k`, the text
only", but not that a capture inside it, such as `$<open>=@list` in
`$<phrase>=[ [ \d+ \s+ ]? $<open>=@list ]`, belongs to the enclosing
rule: it's `At("open.made")`, not `At("phrase.open.made")`, which
reads undef. That's Raku's way too, but it took two tries to find; a
line in the table, and an example, would save them.

[gaps]: docs/REFERENCE.md#gaps-and-possible-additions
