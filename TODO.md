# To do

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
