# Name the quantified group in $<k>=( … )?, as Raku does

**Status:** shipped in v0.4.0

**Origin:** a TODO task from grammar's session (grammar 05622cf99c8552c7a47c295587ec897f4ba764b4), found building roadmap 16

**Started:** 2026-10-27, in fe34207b5d6bfbd3b10ff6b2eea0416fb8a5cfec

**Finished:** 2026-10-27

## The plan

**From:** grammar's session, 2026-10-27 (grammar 05622cf99c8552c7a47c295587ec897f4ba764b4, work uncommitted)
**Waiting on:** nothing

**What:** make `$<k>=( … )` followed by a quantifier capture as Raku
does: the quantifier applies to the group, and `k` names what it
captures. With `?`, `k` is a match with captures of its own, or missing;
with `*`, `+` or `**`, a list of such matches. Today grammar reads
`$<e>=( a (\d) )?` as `$<e>=[ ( a (\d) )? ]`: `e` is text only, and the
group becomes a positional capture of the rule, shifting the numbers of
the `( … )`s after it.

**Why:** Raku is the baseline, and this isn't a listed difference. Found
while building `CheckPath` (roadmap 16):

```
raku -e 'grammar G { token TOP { <min=m> <max=m> $<e>=( a (\d) )? (x) }; token m { m } }; my $m = G.parse("mma1x"); say $m<e>; say $m[0]'
｢a1｣
 0 => ｢1｣
｢x｣
```

grammar gives `e.0.~` "", `0.~` "a1" and `1.~` "x" for the same.

**What's known:** `$<k>=[ … ]?` puts the quantifier inside the capture,
as the reference says and Raku agrees, so only the `( … )` form differs.
`Named` (`node.go`) turns `Named(name, Capture(x))` into a named group,
but the syntax hands it `Named(name, Quant(Capture(x)))`, which becomes
`named`, text only; look at where `syntax.go` parses `$<k>=` and the
atom's quantifier. Raku with `*`, `+` and `** 2` gives a list of matches,
each with its own captures, and positional numbering in the rule skips
the named group. Check `$<k>=<name>?` too. It changes what a match holds
for anyone writing `$<k>=( … )?`, so it's a fix with a CHANGELOG line
saying so.

**Steps:**

1. Raku cases in `rakuCases` for `$<e>=( a (\d) )` with `?`, `*`, `+`
   and `** 2`, matching and not, with a `( … )` after it; they fail.
2. Fix the parse, so the quantifier wraps the named group.
3. `TestCheckPath` cases for the shape (`e.0`, `e[0].0`), and the
   differential checks still pass; `FuzzMatch` 60 seconds.
4. The reference's capture table says it; the CHANGELOG.

**Done when** the Raku cases pass and `CheckPath` agrees with the new
shape.

## What happened

- **Outcome:** the Raku cases NamedQ1 to NamedQ7 pass: `$<k>=` before
  a `( … )`, a call (`<name>` or `<.name>`), a word list or a pattern
  list names each match, and a quantifier after it repeats the named
  capture, so `k` is a list under `*`, `+` and `**`, and a match or
  missing under `?`, with the rule's next `( … )` numbered from `0`.
  `[ … ]` and plain atoms keep the quantifier inside the capture. The
  fix is in `quantified` (`syntax.go`): `namesEach` says which atoms,
  and a bracket is told apart, as `[ @list ]` is the list itself once
  parsed. `TestCheckPathNamedQuantified` checks `CheckPath` follows,
  and the differential checks pass; `FuzzCompile` and `FuzzMatch` ran
  60 seconds each.
- **Changes from the plan:** Raku showed it wasn't only `( … )`: a
  call, a word list and a pattern list behave the same, and
  `$<k>=<name>?` was wrong in the same way. The existing case List9
  (`$<j>=[ @units ]+ % ','`, text only) caught a first version that
  took `[ @units ]` for `@units`.
- **Found on the way:** NamedQ7 showed `Chunks` left out the text
  between captures inside another, where Raku's `.chunks` gives it, as
  it measures each gap from the end of the capture before, not the
  furthest end. Fixed in the same change (`caps.go`), one line.
- **What's left:** nothing.
