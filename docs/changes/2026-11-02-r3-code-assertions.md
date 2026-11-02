# Code assertions

**Status:** unreleased

**Origin:** roadmap 3

**Started:** 2026-11-02, in 88a4f68abfe1a11ea84140f3cf77ba8ef08a1d7a

**Finished:** 2026-11-02

## The plan

**Serves:** Raku

**Size:** M (3)

**What:** `<?{ name($/) }>` and `<!{ name($/) }>`, calling a Go
function passed to `Compile` as a `Use`, `grammar.Assertion(name, fn)`,
with the match so far, which says whether the match goes on. The match
holds the `made` values of the rules already matched, built through the
actions, as Raku's does. Written so, a grammar runs unchanged in Raku
given a `sub name($/)`. The builder has the same as nodes holding the
function, `grammar.Assert(name, fn)` and `grammar.AssertNot(name, fn)`,
the name kept for traces and failure reports.

**Why:** some checks only code can make mid-parse: a pay range's
minimum below its maximum, a plausible year, a listed word that isn't a
stopword. Without them wrong parses get through and are cleaned up
afterwards, and the grammar can't backtrack to a reading that passes,
as Raku's does. jdc-lab is the case in point: `facts.payOf` drops a Pay
match with neither a currency nor a period after `FindAll` has returned
it, so with `Outermost` the dropped match can hide a smaller one, and
nothing can try another reading.

**What's known:**

- Asked of Rakudo 2025.11 on 2026-11-02: inside `<?{ … }>`, `$/` runs
  from the rule's start to where the assertion is (`from` and `to`, and
  `~` the text between), and holds the captures made so far; a captured
  rule's `.made` is already set, its action having run as the rule
  matched. `<!{ … }>` is the negation. In a `regex`, a failing assertion
  backtracks into what came before it (`5-12` with `min < max` takes
  `12` whole); in a `token`, it can't.
- Raku runs each action as its rule matches, on paths the parse later
  abandons too (`<num>` on "123x" ran it on 123, 12 and 1). grammar runs
  actions when it builds the finished match, and documents that among
  the Differences from Raku.
- The matcher holds a rule's captures so far as a `capList` and its
  `scope`, and builds the match only at the end (`builder.build`, from
  `mnode`s). An assertion can build a provisional `mnode` from its
  `env`'s scope and the `capList` it's given, from the scope's start to
  `pos`; `env` doesn't hold the start yet, so it gains one, set where a
  scope starts (`call.run`, `matchProto`, `group.match`).
- `mnode`s never change once made, so one builder per parse, kept on
  the `state`, can cache what it has built: each match's action then
  runs at most once a parse, whether an assertion or the result asked
  first. The provisional match must be read-only to the function, so
  what it sees can't change the result: locked, or a copy.
- Whether `$/` inside a `( … )` group is the group's match or the
  rule's is to be asked of Raku before it's built.
- `<?{` falls through to `p.ident()` today, an unhelpful error. The
  syntax accepted is `name($/)` exactly, spaces allowed; any other code
  is a compile error saying only a call of a named function is
  supported, and a name with no `Assertion` passed is one naming it.
- Every switch over node types that lists `*look` or `anchor` needs the
  new node: `canBeEmpty` and `leftCalls` (zero-width, calls nothing),
  `describe` (failure reports), the trace, `starts.go` (it can't start
  a match, so it's like an anchor), `lookbehind.go`, `checkpath.go`,
  `words.go`'s `wordsIn`, and `children`, `walk` and `distinct`.
- A function is called from whatever goroutine parses, so concurrent
  parses call it concurrently; that's the caller's to make safe, and
  the docs say so. A panic in it propagates.

**Steps:**

1. Ask Raku the open question (groups), and write the `rakuCases`
   first: a case field for the Raku `sub`s and one for the Go functions
   (`json:"-"`), with cases for a pay range's `min <= max` in a `regex`
   (backtracking to a reading that passes) and a `token` (not), `<!{ }>`,
   an assertion reading a subrule's `made` through actions, one inside a
   group, one in a quantifier, and one at a rule's start.
2. `Assertion`, `Assert` and `AssertNot`, the node, and `env`'s start;
   the per-parse builder cache, with `result` using it; the parser for
   `<?{ name($/) }>` and `<!{ name($/) }>`, and its errors.
3. Every node switch above; `TestRakuCasesCompile` and the Raku tests
   pass, and `FuzzCompile` and `FuzzMatch` learn the syntax, with seed
   cases, and run for 60 seconds each.
4. Examples: `Assertion` with a pay range in `example_grammar_test.go`,
   `Assert` and `AssertNot` in `example_node_test.go`;
   `go test -run TestReference -update`.
5. The reference: a section on code assertions, the Differences row on
   actions updated (actions an assertion reads run during the parse), a
   new row for naming a function rather than inline code, and the gap
   row gone; the how-to's recipe if one fits; the CHANGELOG.
6. Dogfooding: rework jdc-lab's Pay to check for a currency or a period
   in the grammar, against this checkout, and compare jdc-lab's output
   before and after; any difference is looked at, and either explained
   by backtracking finding a better reading, or a bug.
7. Benchmarks before and after, to show grammars without assertions
   pay nothing for them.

**Done when** both forms work in rules and the builder; a name with no
function is a compile error that says so; `rakuCases` holds cases run
in Raku with the same check as inline code, matching alike; it has an
example with a pay range; and the reference documents it, with the
named function among the Differences from Raku, and its gap is gone.

## What happened

- **Outcome:** done as planned. Both forms work in rules and the
  builder; a name with no function, and any other code in the braces,
  are compile errors that say what to do; `rakuCases` has eight cases,
  Code1 to Code8, run in Raku with `sub`s holding the same checks, and
  matching alike; `ExampleAssertion` has a pay range; the reference has
  a section, two rows in the Differences, and the gap is gone; the
  how-to has a recipe.
- **Raku's answers** (Rakudo 2025.11, 2026-11-02): inside `( … )`, `$/`
  is the group's match; inside `$<n>=[ … ]`, the rule's, without `n`
  yet; inside a lookahead, the lookahead's, from where it is, with its
  own captures and none of the rule's. All three are cases.
- **Changes from the plan:**
  - The match so far is a locked copy of what's built, as planned, but
    only when the parse has actions. Without them nothing built for an
    assertion is kept for the result, so it's built fresh and locked,
    with no copy: that took `BenchmarkAssert` from 2.3 ms to 1.87 ms.
  - `env` first gained a start and a base, the captures a lookaround
    started with, which took it from two words to four. Every node
    passes it, so grammars without assertions were 5–15% slower
    (`Lookbehind/unbounded` 14.6%), with up to 19% more bytes. The start
    is now an `int32` beside the two flags, and a lookaround with an
    assertion directly inside puts a mark on the capture list instead of
    a base in `env`, which keeps `env` at two words.
  - Keeping the parse's builder on the `state` cost three allocations a
    parse, as the result's builder could no longer stay on the stack;
    the result uses the parse's only if an assertion made one.
  - The trace needed nothing: it traces rule calls, as Raku's
    `Grammar::Tracer` does, and neither shows code assertions.
  - `rakuCases` became a named type, `rakuCase`, with `uses` and
    `parse`, so the seven tests that compile its grammars pass the
    functions and actions without each knowing about them; its Raku
    runners set `$*ACTS` from a case's `Raku` code.
- **A fast path for grammars without assertions,** asked for by Iain
  after the first commit, once he'd accepted the cost. It needed no
  promise from the caller, as a grammar knows whether it has
  assertions; but first, what the 1.2% was. Built against the commit
  before assertions, interleaved: HEAD with the start's stores and the
  lookaround marks taken out was as slow as HEAD (`Parse` +1.8%, against
  +1.65%), as was HEAD without `*code` in `oneWay`; the commit before
  assertions with only `env`'s new `int32` field added, and nothing else,
  was as slow as HEAD (`Parse` +1.8%, `FindLines` +1.4%). Go passes a
  struct in registers a field at a time, so a fourth field is a fourth
  register at every match call, though `env` stayed 16 bytes. So `env`
  went back to three fields, and a rule's or group's body starts its
  capture list with a mark of where it started, as lookarounds did, only
  if a code assertion is in it (`hasCode`, worked out when the rule or
  group is made); building a match passes over the mark. The lookaround's
  check, inlined but made at each start a lookbehind tries, cost
  lookbehinds 1.7%; choosing its closure once, before the loop, left
  0.8% (15 runs: `Lookbehind/bounded` +1.5% against the commit before
  assertions, +0.65% with no mark at all). Final, against that commit,
  eight runs: `FindAll`, `FindRe` and `Parse` unchanged, `FindLines`
  +0.5%, `Lookbehind/bounded` +1.5–2.2%, `Lookbehind/unbounded` within
  0.6%, with the same allocations; `BenchmarkAssert` 1.83 ms, against
  1.87 ms. A planted bug, no mark at a rule's start, panicked on the
  first Raku case with an assertion, as `sofar` is meant to; one at a
  lookahead's failed Code8. `FuzzCompile` and `FuzzMatch` ran 60 seconds
  each again, with nothing found, and the tests passed under `-race`.
- **Planted bugs:** taking the match so far from the text's start, and
  ignoring the lookaround's mark, each failed the Raku cases (Code7 and
  Code8). The second passed at first, as Code8 had no capture before its
  lookahead; it was given one, `<a=alpha>`, that Raku says isn't seen.
- **Measurements** (Apple M2, benchstat, six runs each, interleaved,
  against 704e4832e7e5ebe2ca3ea77372da93e0f81b97bc): grammars without
  assertions are 1.2% slower by the geometric mean (`Parse` 1.5%,
  `FindLines` 1.7%, `Lookbehind/bounded` 2.8%; `FindAll` and `FindRe`
  unchanged), with the same allocations and 0.07% more bytes.
  `BenchmarkAssert`, a pay grammar checking for a currency or a period
  on a text of figures, mostly not pay: 1.87 ms and 42,963 allocations,
  against 1.14 ms and 25,621 with the check made on `FindAll`'s matches
  afterwards. Building the match so far is most of the difference.
- **Fuzzing:** `FuzzCompile` and `FuzzMatch`, which now call an
  assertion in their grammars, ran 60 seconds each, with nothing found.
- **Dogfooding:** jdc-lab's Pay checked for a currency or a period
  after the search, in two places, `facts.payOf` and `headings`, kept
  in step by hand. Moved into the grammar as `<?{ pay($/) }>`, against
  this checkout, chunking all 28,000 ads of the latest run gave output
  identical, byte for byte, to grammar v0.4.4's, and jdc-lab's tests
  passed; so did the same ads with the assertion-free code against this
  checkout. The chunker took 38.4–39.0 s against 36.7–36.9 s, 4–6%
  longer, the cost of building the match so far at each figure.
- **What's left:** building only what the function reads, which would
  need a lazy value in scalar, is in the reference's
  [Engineering gaps](../REFERENCE.md#engineering).
