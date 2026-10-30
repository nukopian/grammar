# To do

## A word list's value in a rule with more in it than the list

**From:** jdc-lab's session, 2026-10-30 (jdc-lab 996d9e8a5b9430221f35ad3e4cfd7ea0bd8f7258)
**Waiting on:** nothing

**What:** decide how a word list's canonical value should be had when
the list isn't a rule's whole body, and either make it so or document
it where a reader will find it, then release, so that jdc-lab can take
it up.

**Why:** jdc-lab's work pattern grammar was `token TOP { @patterns }`,
its value read as `made`. To stop "Contract" in "Contract type: Temp to
Perm" counting as a pattern, it gained a lookahead, and `made` came back
empty for every match: the value is set only when the list is the whole
rule. The README's table says "`@list` in a rule, its canonical value as
`made`", which reads as if any rule with a list in it had one, and
`CheckPath` passed the old path, so nothing warned. The lab worked
around it with a rule of its own for the list (`token word { @patterns }`,
read as `word.made`); a named capture works too, but no document says
so.

**What's known:** run against this working copy at
78bee6780fc6e97a9bb17fab0956f4699531bfdf, with the list
`permanent, perm` and `temp to perm, temp-to-perm` (FoldCase,
WholeWords), finding in "Perm role":

| Rule | `made` | `w.made` |
|---|---|---|
| `token TOP { @patterns }` | "permanent" | |
| `token TOP { @patterns <!before ':'> }` | "" | |
| `token TOP { @patterns <?before \s> }` | "" | |
| `token TOP { @patterns \s* }` | "" | |
| `token TOP { <!before :i contract> @patterns }` | "" | |
| `token TOP { <w> <!before ':'> } token w { @patterns }` | "" | "permanent" |
| `token TOP { $<w>=@patterns <!before ':'> }` | "" | "permanent" |
| `token TOP { $<w>=[ @patterns ] <!before ':'> }` | "" | "permanent" |

The `WordList` doc is exact ("a rule whose whole body is a word list has
the canonical value … as its made value"); the README's summary line is
looser; `checkpath.go` already knows that a named capture of a list has
a made value. So this is a documentation gap at least, and perhaps a
feature worth having: a rule whose only value-bearing part is one word
list, beside zero-width assertions, taking that list's value as its own.

The reproduction, as a program in a module whose `go.mod` replaces
`github.com/nukopian/grammar` with this working copy:

```go
l := grammar.NewWordList("patterns", grammar.FoldCase(), grammar.WholeWords())
if err := l.AddRules("permanent, perm\ntemp to perm, temp-to-perm\n"); err != nil {
	panic(err)
}
for _, src := range []string{
	`grammar A { token TOP { @patterns } }`,
	`grammar B { token TOP { @patterns <!before ':'> } }`,
	`grammar G { token TOP { $<w>=@patterns <!before ':'> } }`,
} {
	m := grammar.MustCompile(src, l).Find("Perm role")
	fmt.Printf("%s: made %q, w.made %q\n", src, m.At("made").Str(), m.At("w.made").Str())
}
```

**Steps:**

1. Decide between (a) documenting that a list's value is `made` only for
   a rule that's nothing but the list, and that `$<name>=@list` gives it
   as `name.made` anywhere, with an example in the README's table and
   the `WordList` doc; and (b) also making a rule whose only
   value-bearing part is one list, beside zero-width assertions such as
   `<!before …>`, take that list's value as `made`. (b) would have kept
   jdc-lab's first rule working; whether it fits Raku's semantics, where
   a rule's `made` comes from its action, is the question.
2. If (b), consider whether `CheckPath` should then accept `made` for
   such a rule, and still refuse it for one like `@patterns \s*` whose
   other parts consume text.
3. Add a test of each case above, and an example for the named capture.
4. Release, and tell Iain, so jdc-lab can take the release up.

**Done when** a reader of the README or the `WordList` doc learns how to
get a list's value in a rule with more in it, the cases above are tested
to behave as documented, and a release has it. Then jdc-lab can keep its
`word` rule, or move to `$<word>=@patterns` or a bare `@patterns`, as
the release allows: a task for jdc-lab's next session after it.
