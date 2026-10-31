# A trace of a parse

**Status:** shipped in v0.4.2

**Origin:** roadmap 1

**Started:** 2026-10-31, in 964ed623ae2ad64da19fd9dc427aa5c3e8892084

**Finished:** 2026-10-31

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

- **Outcome:** met. `Trace(w)` prints in `Grammar::Tracer`'s form,
  hooked into rule calls only (`call.match`, and each variant in
  `matchProto`), with every way of matching. `BenchmarkParse` shows no
  cost with it off. `FuzzMatch` checks that it changes no result from
  `Parse`, `Subparse` and `FindAll`, and that each rule it opens is
  closed, once, at its own depth. It has two examples, `ExampleTrace`
  and `ExampleTrace_failure`, a section in the reference (Watching a
  parse), a row in its Differences from Raku, and a place in the how-to
  and the README, whose "When not to use it" no longer names it.
- **Changes from the plan:**
  - The traces in `TestTrace` are those the copy of `Grammar::Tracer`
    printed for the same grammars and texts, except for `|` and protos,
    where grammar traces every branch it matches to find the longest.
    They aren't compared with Raku in CI, as `Grammar::Tracer` isn't
    part of Rakudo, and what it traces isn't what Raku matches (above).
    That's a new difference from Raku, of a module rather than the
    language, documented in the reference rather than raised first: it
    follows from `Alt`'s and protos' existing difference, and tracing
    the branches as Raku's tracer would means guessing which branches
    Raku would have tried.
  - Step 5 went further than updating the gap: the tracer keeps a stack
    of rules, as the failure tracker does, rather than a bare depth, so
    `Parse`, `Find` and the rest name the rule and place in a limit
    error while traced, as roadmap 2 suggested. The gap now covers only
    untraced calls. A rule stopped by a limit is left open at the end of
    the trace, as the panic unwinds without printing.
  - `TestTraceText` checks the quoting against what Rakudo's `.raku`
    gives for each kind of character, including a combining mark at the
    start, which is written as a code, as it would otherwise join the
    quote. The cut counts runes, where Raku's counts graphemes, which
    differ only as the reference's Differences from Raku already says.
- **Measurements:** `BenchmarkParse`, `BenchmarkFindAll` and
  `BenchmarkLookbehind`, `-count 2` five times, alternating between a
  worktree of 964ed623ae2ad64da19fd9dc427aa5c3e8892084 and the change,
  both with `GOWORK=off`; the median ns/op of ten runs, on the
  maintainer's Mac. Allocations are unchanged.

  | Benchmark | Before | After | Before (B) | Variant B |
  |---|---|---|---|---|
  | `Parse` | 9,859 | 9,828 (−0.3%) | 9,891 | 9,904 (+0.1%) |
  | `FindAll` | 40,677 | 40,600 (−0.2%) | 40,568 | 40,794 (+0.6%) |
  | `Lookbehind/bounded` | 838,718 | 850,196 (+1.4%) | 841,311 | 849,898 (+1.0%) |
  | `Lookbehind/unbounded` | 39,953,998 | 40,204,860 (+0.6%) | 40,680,128 | 40,159,480 (−1.3%) |

  `Lookbehind/bounded` calls `TOP` once at each of about 4,300 positions
  and little else, so it shows a rule call's cost most; its runs before
  the change spread from 834,225 to 878,093 across the two sessions, so
  the 1% is near the noise, and no benchmark that parses moved.
- **Tried and reverted:** variant B, to see whether the 1% on
  `Lookbehind/bounded` was the extra call from `call.match` to `run`:
  the untraced path inline, the traced one wrapping the continuation and
  ending in a deferred call, which needed the tracer stopped at a limit
  so the defer wouldn't unwind it. It was no faster (the table's last
  column), so the simpler version was kept: the cost, if any, is the
  check itself, which tracing can't do without.
- **Checks:** planted bugs, each caught: the depth kept up while the
  caller goes on (`TestTrace`), a MATCH printed each time the body
  matches rather than once (`TestTrace`), and the rule left on the
  stack while the caller goes on (`TestLimitErrorsSayWhere`, "handed
  on", and every trace test). The Raku differential tests ran against
  Rakudo 2025.11 and pass, the tests pass with `GOWORK=off`, and
  `FuzzMatch` ran for 60 seconds twice, before and after the stack, and
  found nothing; staticcheck is clean.
- **Found by the wash-up for the release,** by a review of the diff:
  backtracking into a `regex` rule that had matched, so that it tried
  again, traced the rules it called one level too deep, inside a rule
  already closed, leaving the trace unbalanced. Raku's tracer, through
  the same copy, traces them beside the rule, at its depth, which is
  back to the caller's. The tracer now keeps that depth apart from its
  stack of rules, which a limit error still reads. `FuzzMatch` hadn't
  found it, as its grammar's rules were all tokens, which never
  backtrack into a rule: it now has a `regex` that backtracks into
  another, and a seed for it, which failed before the fix. `TestTrace`
  has Raku's trace as the case "retry".
- **What's left:** limit errors from untraced calls, in the reference's
  gap
  [Untraced, only `ParseErr`'s limit errors say where](../REFERENCE.md#gaps-and-possible-additions).
  Colour, as `Grammar::Tracer` prints, is left out, as a writer may not
  be a terminal.
