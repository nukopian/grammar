# Code assertions

**Status:** in progress

**Origin:** roadmap 3

**Started:** 2026-11-02, in 88a4f68abfe1a11ea84140f3cf77ba8ef08a1d7a

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

Nothing yet.
