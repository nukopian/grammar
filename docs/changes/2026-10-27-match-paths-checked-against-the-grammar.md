# Match paths checked against the grammar

**Status:** shipped in v0.4.0

**Origin:** roadmap 16

**Started:** 2026-10-27, in cec0be91c537ee2f4def239fea83cc06b24471e2

**Finished:** 2026-10-27

## The plan

**Serves:** debugging

**From:** grammar's session, 2026-10-27 (grammar cec0be91c537ee2f4def239fea83cc06b24471e2)
**Waiting on:** a scalar release that exports its path parser

**What:** a `(*Grammar).CheckPath(path string, opts ...Option) error`
that says whether a path, as `At` takes it, can ever be defined in a
match of the grammar: from `TOP`, or the rule `Rule(name)` names. It
returns nil when some text could define it, and otherwise an error
naming the step that can't be, and what could come there instead:
`grammar: path "per.weak": rule per has no capture "weak" (it has: day, week)`.
For tests, mostly: a check that the paths a program reads are real.

**Why:** a misspelt capture, such as `m.At("per.weak")`, reads undef,
as an optional capture that didn't match does, so the typo fails
silently (from jdc-lab's use of the facts grammars). So does a capture
read from the wrong place, as roadmap 15 found: `title.level.made`
for a `level` inside `$<title>=[ … ]`. The grammar knows every capture
its rules can make, so it can say a path can never be defined.

**What's known:**

- `scope.count` (`grammar.go`) already works out, for each rule body
  and `( … )` group, every key it can capture and whether it can
  capture it more than once. It doesn't record where a key leads:
  `<name>` and `<alias=name>` to rule name's captures (for a proto,
  any candidate's); `$<k>=( … )` to the group's own scope;
  `$<k>=<name>` to the rule's; `$<k>=[ … ]`, `$<k>=@list` and
  `$<k>=<@list>` to text only. Captures inside a lookaround are kept
  by none.
- A key that can capture more than once is an array: a step into it
  must be an index (`[0]`, `.0`, `[-1]`), and a step into a match
  must be a key. `.0` after a dot is either, as in scalar: an index
  into an array, or a match's positional capture 0.
- `~`, `from` and `to` are leaves: no step may follow them. `made` may
  be anything an action returns, so any steps after it are accepted.
- Rules are looked up by name when checking, so `Extend`, `Does` and
  protos need nothing special. A path is finite, so recursive rules
  end the walk.
- scalar's path syntax (dots, brackets, braces, quotes, escapes,
  negative indices) is parsed by its unexported `parseAt` into
  unexported `atStep`s. Copying the parser would let the two drift, so
  scalar exports it first (a task in scalar's `TODO.md`); scalar's
  `CheckPath` checks only that a path is well formed.
- Raku has nothing like this, so it goes in the reference's list of
  what grammar adds to Raku. Matching is unchanged.

**Steps:**

1. scalar exports its path parser and releases it (scalar's task);
   grammar requires that release, checked with `GOWORK=off`.
2. Tests first: a misspelt key, a capture inside `$<k>=[ … ]` read
   from inside it, an index into a single capture, a key into an
   array, a step past `~`, `from` or `to`, steps after `made`, a
   proto's candidates, an alias, a role's rules, `Rule(name)`, and a
   malformed path (scalar's error).
3. Record where each key leads, beside `scope.count`, and the walk.
4. The differential check: for the matches `FuzzMatch` makes, and the
   Raku cases' inputs, every path present in a real match passes
   `CheckPath`; a planted bug to show the check notices; 60 seconds.
5. `ExampleGrammar_CheckPath`, with pay or hours, and
   `docs/EXAMPLES.md` regenerated; the reference, where matches are
   read, and its list of what grammar adds to Raku; a how-to recipe for
   checking paths in tests; the README; the CHANGELOG, and this item
   out of the roadmap.
6. Dogfood: check jdc-lab's paths with it, against this checkout, and
   leave jdc-lab a task to take it up after the release.

**Done when** `CheckPath` rejects each misreading in step 2 with an
error naming the step, accepts every path real matches have, and is
documented and dogfooded as in steps 5 and 6.

## What happened

- **Outcome:** `(*Grammar).CheckPath(path, opts...)`, in `checkpath.go`.
  It rejects each misreading in step 2, naming the step and what could
  come there, and accepts every path real matches have: those of the
  Raku cases' inputs, of a grammar with each kind of capture, and of
  every match `FuzzMatch` makes (about 2.1 million runs in 60 seconds).
  Two planted bugs, dropping aliases and dropping the captures inside
  `$<k>=[ … ]`, each failed the check. Documented in the reference
  (Checking paths, and what grammar adds to Raku), the how-to (When a
  grammar won't match), the README and `ExampleGrammar_CheckPath`.
- **Changes from the plan:** scalar shipped the parser as
  `scalar.AtSteps` in v0.6.1, not `ParsePath`. The walk is a pass of its
  own beside `scope.count`, not part of it: it collects, for each rule
  and group scope, where each key leads, and is kept per call, not on
  the grammar, as a grammar can change after compiling. A capture
  inside `$<k>=[ … ]` read from inside it gets the path that reads it
  ("so read "open.made""), beyond saying it's text only; and a proto's
  capture is named as the proto (`proto unit`), as that's what the
  grammar calls.
- **Found on the way:** `$<k>=( … )` with a quantifier after it doesn't
  match Raku. grammar reads `$<e>=( a (\d) )?` as `$<e>=[ ( a (\d) )? ]`:
  `e` is text only, and the group becomes positional `0` of the rule.
  Raku names the quantified group: with `?`, `e` is a match with
  captures of its own, `e.0`; with `*`, `+` or `**`, a list of them; and
  the rule's next `( … )` is `0`. `CheckPath` follows what grammar's
  matches have, and its check in `FuzzMatch` will hold it to the fix.
  A task in `TODO.md`.
- **Dogfooding:** every path jdc-lab reads from its grammars (36 in
  `internal/facts` and `internal/wording`) passes, against this
  checkout; `hours.CheckPath("per.weak")` and
  `opening.CheckPath("phrase.open.made")`, the two mistakes that
  prompted it, are caught. jdc-lab has a task to take it up after the
  release.
- **What's left:** a `CheckTo[T]`, checking the fields of a struct for
  `To` the same way, if one is wanted.
