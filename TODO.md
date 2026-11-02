# To do

## Say scalar is under Artistic 2.0 too

**From:** scalar's session, 2026-11-02 (scalar 93116ab4c7361352011af4cc5cb28673eb191ea2)
**Waiting on:** a scalar release with the Artistic 2.0 licence (after v0.6.2)

**What:** scalar has been relicensed from the same terms as Perl itself
to the Artistic License 2.0, grammar's licence, so pkg.go.dev shows its
documentation. Versions up to v0.6.2 keep the old terms. grammar's
LICENSE and README still describe scalar's terms as Perl's.

**Why:** both now say something untrue about scalar, and grammar's
LICENSE no longer needs to explain that the two differ.

**Steps,** in one `docs:` commit, once scalar's release is out and
grammar's `go.mod` requires it:

1. LICENSE: replace the paragraph "It uses the scalar package, in the
   directory above, which is under the same terms as Perl itself; see
   its LICENSE." with "It uses the scalar package, which is under the
   same licence." The "directory above" is stale too: scalar is a
   separate repository.
2. README.md, Licence section: replace "which is under the same terms
   as Perl itself; see [its licence](…)" with "which is under the same
   licence; see [its licence](https://github.com/nukopian/scalar#licence)".
3. Rebuild README.pdf and read the Licence page.

**Check:** `grep -rn 'same terms as Perl' LICENSE README.md` finds
nothing, and the link resolves.

## Link The process at its new home

**From:** scalar's session, 2026-11-02 (scalar 366cf829840aa91ff9207f2a7df341d311484d6d)
**Waiting on:** nothing

**What:** Iain's essay, The process, has moved from scalar's
`docs/PROCESS.md` to claude-practice's `thoughts/PROCESS.md`. scalar
keeps a pointer, so the old links still land, but they take a detour.

**Steps,** in one `docs:` commit:

1. README.md: change both links to
   `https://github.com/nukopian/scalar/blob/main/docs/PROCESS.md`
   (in the Documentation list and the Acknowledgements) to
   https://github.com/nukopian/claude-practice/blob/main/thoughts/PROCESS.md.
2. Rebuild README.pdf and read the two pages.

The CHANGELOG's link stays: it records what was true at that release.

**Check:** `grep -rn 'scalar/blob/main/docs/PROCESS' README.md` finds
nothing, and the new link resolves.
