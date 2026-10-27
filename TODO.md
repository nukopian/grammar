# TODO

## Name the quantified group in $<k>=( … )?, as Raku does

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
