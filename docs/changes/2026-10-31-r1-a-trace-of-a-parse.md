# A trace of a parse

**Status:** in progress

**Origin:** roadmap 1

**Started:** 2026-10-31, in 964ed623ae2ad64da19fd9dc427aa5c3e8892084

**Finished:**

## The plan

**Serves:** debugging

**Size:** S (2)

**What:** a `Trace(w io.Writer)` option that prints each rule as it's
entered, indented by depth, then `* MATCH "text"` or `* FAIL`, in the
style of Raku's `Grammar::Tracer`.

**Why:** when a grammar won't match, `ParseErr` reports only the
furthest failure, not which rules were tried, where, or what each
matched. Watching the rules is the quickest way to debug a grammar, and
to learn how grammars work. The README's
[When not to use it](../../README.md#when-not-to-use-it) names it as
missing.

**What's known:** `Grammar::Tracer` (from `Grammar::Debugger`) isn't
installed here, so its source
([Tracer.rakumod](https://github.com/jnthn/grammar-debugger/blob/master/lib/Grammar/Tracer.rakumod))
was copied, without its colours, into a scratch module and run against
small grammars on Rakudo 2025.11. It wraps every method of the grammar
that's a `Regex`, so:

- each rule prints its name as it's entered, with `|  ` for each level
  of depth, then `* MATCH "…"` or `* FAIL` at the caller's depth;
- the text is the match's `.Str`, cut to 60 − 3 × depth characters and
  quoted as Raku's `.raku` quotes a string (`\"`, `\\`, `\$`, `\@`,
  `\%`, `\&`, `\{`, `\t`, `\n`, `\r`, `\0`, `\b`, other control
  characters as `\x[1B]`, everything else as it is); at depth 20 and
  past, the text is left off;
- a call prints once: when matching backtracks into a rule that has
  matched, nothing more is printed for it, and the calls after it are
  printed again as they're retried;
- built-in rules, such as `<ws>` and `<alpha>`, aren't traced, unless
  the grammar defines its own; word lists aren't rules, so aren't
  traced either; calls inside lookarounds are;
- a proto prints its name, then the variant it called inside it;
- wrapping makes every rule non-declarative, so under the tracer Raku's
  longest-token matching falls back to trying branches in order:
  `<h> | <hours>` on "hours" matches "hour" traced, "hours" untraced. So
  a trace can't be compared with Raku's line for line: what Raku traces
  isn't what Raku matches.

`call.match` in `node.go` is where a rule is entered; it keeps the
failure tracker's stack right around continuations (`t.enter`,
`t.leave`), and the trace's depth needs the same. `Alt` matches each
branch once to measure it, and in a `regex` matches the winner again;
a proto matches every variant.

**Steps:**

1. Record `BenchmarkParse`, `BenchmarkFindAll` and
   `BenchmarkLookbehind` against a worktree of the starting commit,
   alternating, as roadmap 2 did.
2. Tests first: the traces above, made in Go for the same grammars;
   quoting; the depth cut-off; backtracking into a rule; Find over
   several positions; a proto; built-in rules left out and a grammar's
   own `ws` traced; and tracing changing no result, in `FuzzMatch`.
3. A `Trace(w io.Writer)` option, a tracer on `state` set by `runState`,
   and a traced path in `call.match` and `matchProto`, apart from the
   untraced one so it costs a nil check.
4. Measure. If tracing off costs more than noise, find out why before
   going on.
5. An example, the reference (where it sits, how it differs from Raku's
   tracer and why), the README, `docs/EXAMPLES.md`, the CHANGELOG, and
   the gap on limit errors, which said to record the rule only while
   tracing.

**Done when** tracing is hooked into rule calls (not every step, which
would bury the rules), `BenchmarkParse` shows no real cost with it off,
`FuzzMatch` checks that tracing changes no result, it has an example and
a place in the reference, and the README no longer says there's no
trace.

## What happened
