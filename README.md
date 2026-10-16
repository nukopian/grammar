# grammar

Pull structured facts out of messy text, such as job titles and pay, with
Raku-style grammars and synonym lists, in Go.

Some parts of a job posting are small languages of their own: pay
(`£45k – £55,000 per annum`), hours (`37.5 hrs/wk`), titles
(`Snr SWE – Payments (WFH)`). A single regex for any of them soon becomes
unreadable, splitting it into pieces means hand-written glue to put the
pieces back together, and even then "SWE", "Software Developer" and
"Software Engineer" come out as three different things.

## Why this exists

The alternative is sending every posting to a language model, which works
well but costs more with every posting. This package does the structured
parts locally, with three ideas that work together:

- **Grammars** say where each kind of thing goes: a seniority, then a title,
  then optionally a team, then a working arrangement in brackets. They're
  written in Raku's grammar syntax, which reads much as you'd describe the
  text in words, or built in Go.
- **Word lists** say what counts as each kind of thing, and what it means:
  every synonym maps to a canonical value, so "SWE", "Software Developer"
  and "software  engineer" all come out as "Software Engineer". They take
  synonym rules in the format Elasticsearch and Solr use (`pm => product
  manager, project manager`), so they can be filled from a synonym store.
- **Typed results:** `grammar.To` fills a Go struct straight from a match,
  each capture filling the field of the same name, and actions turn what
  matched into any other value you need.

```go
titles := grammar.NewWordList("titles", grammar.FoldCase(), grammar.LooseSpacing())
titles.AddRules("Software Engineer, Software Developer, SWE, Developer")
// ... and lists for seniority and arrangements, the same way

g := grammar.MustCompile(`
grammar Title {
    token TOP         { [ <seniority> \s+ ]? <title> [ \s* <.dash> \s* <team> ]? [ \s* '(' <arrangement> ')' ]? }
    token seniority   { @seniority }
    token title       { @titles }
    token arrangement { @arrangements }
    token dash        { '-' | '–' | '|' }
    token team        { \w+ [ ' ' \w+ ]* }
}`, seniority, titles, arrangements)

type Title struct{ Seniority, Title, Team, Arrangement string }
t, err := grammar.To[Title](g.Parse("Snr SWE – Payments (WFH)"))
// {Seniority:Senior Title:Software Engineer Team:Payments Arrangement:Remote}
```

The whole example, with all three lists, is [Titles](#titles) in the
Reference, and runs as a test.

Two more things set it apart:

- **Roles** let grammars share rules: one library of money and period
  tokens for pay, bonus and allowance grammars, or one lexicon for titles
  and requirements. See [Roles](#roles).
- **It behaves as Raku does, checked against Raku itself.** The tests run
  dozens of grammars through both this package and Rakudo, and compare the
  results. You don't need to know Raku to use it, but if you do,
  nothing should surprise you. See
  [Testing against Raku](#testing-against-raku).

**Status: experimental.** It works and is well tested, and a pay grammar
agreed with a language model's extraction on all 16 postings tried so far,
but it hasn't yet met real volumes, and the API may still change. What it
doesn't do yet is listed, with what each would take, in
[Gaps and possible additions](#gaps-and-possible-additions).

## A first grammar

A grammar is a set of named rules that call each other. Here's one for
pay:

```go
var Pay = grammar.MustCompile(`
grammar Pay {
    token TOP    { <min=money> [ <.sep> <max=money> ]? <period>? }
    token money  { '£' \s? <amount> }
    token amount { <num> [ $<k>=[:i k] ]? }
    token num    { \d ** 1..3 [ ',' \d ** 3 ]* [ '.' \d+ ]? }
    token sep    { \s* [ '-' | '–' | to ] \s* }
    token period { :i \s* [ 'p.a.' | pa | per \s+ [ annum | year | hour | day ] ] }
}`)
```

Parsing starts at `TOP`, which reads: a `<money>`, captured as `min`; then
optionally a separator and another `<money>`, captured as `max`; then
optionally a `<period>`. Each rule is described in the same way, and the
[tables below](#writing-a-grammar) say what each piece means.

An action is a Go method named after a rule. It's called with that rule's
match, and what it returns is stored in the match as `made`:

```go
type PayActions struct{}

// Amount is called for each <amount>: "45,000" is 45000, and "45k" is too.
func (PayActions) Amount(m *scalar.SV) any {
	n := m.At("num.~").S(`,`, "", "gr").Num() // num.~ is the text <num> matched
	if m.At("k").Defined() {
		n *= 1000
	}
	return n
}

m := Pay.Find("Salary: £45k – £55,000 per annum, plus bonus.", grammar.Actions(PayActions{}))
m.At("~")                // "£45k – £55,000 per annum": the text matched
m.At("min.amount.made")  // 45000
m.At("max.amount.made")  // 55000
m.At("period.~")         // " per annum"
```

### What a match looks like

A match is a tree that follows the grammar: each rule that matched is a
match of its own, under the name it was captured as. Every match has its
text under `~`, and each also has `from` and `to`, its position, left out
here:

```yaml
"~": £45k – £55,000 per annum   # the text TOP matched
min:                            # <min=money>
  "~": £45k
  amount:                       # <amount>, inside money
    "~": 45k
    num: { "~": "45" }
    k:   { "~": k }             # $<k>=[ k ]: only there when it matched
    made: 45000                 # what the Amount action returned
max:                            # <max=money>
  "~": £55,000
  amount:
    "~": 55,000
    num: { "~": "55,000" }
    made: 55000
money: [ …, … ]                 # the same two matches: money is captured twice, so it's an array
period:
  "~": " per annum"
```

So `m.At("min.amount.made")` walks down the tree to a value, and
`m.At("period.~")` reads "the text of the period capture". A capture that
didn't match is missing, and reads as undef, so `Or` and `Defined` work on
it. The match is ordinary `scalar` data, so `At`, `Or`, `To` and the rest
all work on it. [The match](#the-match) has the details, and the Reference's
[Match tree](#match-tree) example prints a real one.

## Writing a grammar

### Rules

| Raku | Go builder | Meaning |
|---|---|---|
| `token name { … }` | `g.Token(name, node)` | A rule that doesn't backtrack: each part keeps its first match. Use this unless you need `regex`. |
| `regex name { … }` | `g.Regex(name, node)` | A rule that backtracks, like a Perl regex. |
| `rule name { … }` | `g.Token`, with `Silent("ws")` where the spaces go | A token in which spaces in the rule match spaces in the text. See [Spaces in rules](#spaces-in-rules). |
| `grammar B is A { … }` | `a.Extend("B")` | B has A's rules; B's own rules replace A's of the same name, even where A's rules call them. |
| `grammar C is A is B { … }` | `a.Extend("C", b)` | C has A's rules and B's, looked up in C3 order; see [Several parents](#several-parents). |
| `proto token p {*}` | `g.Proto("p")` | A family of rules, its variants, that extensions and roles can add to. See [Protos](#protos). |
| `token p:sym<x> { … }` | `g.Token("p:sym<x>", node)` | A variant of the proto `p`. `<sym>` (`Sym()`) inside it matches `x`. |
| `role R { … }` | `grammar.NewRole("R")` | A library of rules for grammars to share. See [Roles](#roles). |
| `grammar B does R { … }` | `b.Does(r)` | B gets R's rules as its own. |

Parsing starts at the rule `TOP`, or the rule named with
`grammar.Rule("name")`.

One source can declare several roles and grammars; `Compile` returns the
last. The source can refer by name to anything declared earlier in it, and
to the grammars, roles and word lists passed to `Compile`:

```go
g := grammar.MustCompile(src, Pay, units)   // is Pay, does …, @units
```

### Inside a rule

Whitespace and `#` comments mean nothing, so space a rule out as you like.
Letters, digits and `_` are literal; anything else must be quoted or
escaped.

| Raku | Go builder | Matches |
|---|---|---|
| `'£'`, `"a\tb"`, `abc` | `Lit("£")` | that text |
| `.` | `AnyChar` | any character, newlines too |
| `\d` `\w` `\s` `\h` `\v` `\n` `\t` | `Digit`, `Word`, `Space` | the usual classes (Unicode-aware); capitals, such as `\D`, negate |
| `\-`, `\[` | `Lit("-")` | that punctuation character |
| `<[a..z 0..9 _]>` | `Class(func(rune) bool)` | one character from the set |
| `<-[aeiou]>`, `<[a..z] - [aeiou]>`, `<+alpha +[-]>` | `Class(…)` | a negated or combined set |
| `<:Lu>`, `<:Letter>`, `<:Latin>`, `<:Script<Cyrillic>>`, `<:!Lu>` | `Class(func(r rune) bool { return unicode.Is(unicode.Lu, r) })` | a character with a Unicode property, or without it; see below |
| `x y` | `Seq(x, y)` | x then y |
| `x \| y` | `Alt(x, y)` | whichever of x or y matches more text |
| `x \|\| y` | `First(x, y)` | x if it matches, otherwise y |
| `x & y`, `x && y` | `Conj(x, y)` | x and y both, over the same text; see below |
| `[ … ]` | (just nest nodes) | grouping, with no capture |
| `x?` `x*` `x+` | `Opt(x)`, `Star(x)`, `Plus(x)` | quantifiers, greedy |
| `x ** 2..5`, `x ** 3`, `x ** 2..*` | `Repeat(x, 2, 5)` | between 2 and 5 of x; `-1` for no limit |
| `x+ % ','` | `Plus(x).Sep(Lit(","))` | x separated by commas |
| `x+ %% ','` | `Plus(x).SepTrailing(Lit(","))` | the same, with an optional comma at the end |
| `'(' ~ ')' x` | `Goal(Lit("("), Lit(")"), x)` | `'(' x ')'`, with the brackets written together; `ParseErr` names the `'('` a missing `')'` was for |
| `x*?`, `x+?` | `Star(x).Frugal()` | as few as possible |
| `x+:`, `x:` | | never give back what x first matched: a possessive repeat, or any atom committed, as in a token; `[ a \| ab ]: b` can't match "ab" |
| `^` `$` | `Start`, `End` | start and end of the text |
| `^^` `$$` | `LineStart`, `LineEnd` | start and end of a line |
| `<<` `>>` | `WordStart`, `WordEnd` | start and end of a word |
| `<?before x>`, `<!before x>` | `Before(x)`, `NotBefore(x)` | x follows (or doesn't), without matching it |
| `<?after x>`, `<!after x>` | `After(x)`, `NotAfter(x)` | x comes just before (or doesn't) |
| `<?>`, `<!>` | | always matches, never matches, matching no text: `$<none>=[ <?> ]` is an empty capture |
| `:i` | `IgnoreCase(x)` | ignore case, for the rest of the group |
| `:r`, `:!r` | `Ratchet(x, on)` | stop or start backtracking, for the rest of the group |
| `:s`, `:!s` | `Silent("ws")` where the spaces go | make spaces in the rule match spaces in the text, or stop, for the rest of the group, as in a `rule` |
| `@name` | `Words(list)` | the longest phrase in a word list; see [Word lists](#word-lists) |
| `<@name>` | | the longest match of a pattern list's patterns; see [Pattern lists](#pattern-lists) |
| | `Re(pattern)` | a Perl-style regex, as `scalar`'s `M` takes, as one atom |

### Spaces in rules

In a `token` or `regex`, spaces in the rule mean nothing, so a phrase needs
its spaces spelled out: `per \s+ annum`. In a `rule`, spaces mean "spaces
may go here": each becomes a call to `<.ws>`, which matches any whitespace,
or none between a word and punctuation, but not none between two words.
So a rule reads like the text it matches:

```raku
rule rate   { up to <money> per <unit> }     # "up to £500 per day", "up to  £500 per  day"
token rate  { up \s+ to \s+ <money> \s+ per \s+ <unit> }   # the same, as a token
```

Where a space counts, exactly as in Raku:

| In the rule | Means |
|---|---|
| `a b` | `a <.ws> b`: a space after something is `<.ws>` |
| `rule x { a }`, `[ a`, `\| a` | nothing: a space at the start of a rule, a group or an alternative doesn't count |
| `a }`, `a ]` | `a <.ws>`: a space at the end does, so a rule takes the spaces after it |
| `a+ b` | `a+ <.ws> b` |
| `a + b` | `[ a <.ws> ]+ <.ws> b`: a space before a quantifier is repeated with the atom |
| `<item>+ % ',' }` | `<item>+ % [ ',' <.ws> ]`: a space after a separator is part of it, so `"x, y"` matches but `"x , y"` doesn't; write `<item> + % ','` to allow both |
| `'a b'` | the text `a b`, with exactly one space, as quoted text is literal |
| `^ a` | `^ <.ws> a`, so leading spaces in the text are allowed |

`:s` switches this on for the rest of a group in a `token` or `regex`, and
`:!s` switches it off in a `rule`. `<.ws>` is an ordinary rule, so a grammar
can define its own `ws`, to skip comments as well as spaces, say, and every
`rule` in it uses that.

In Go there's no sigspace: put `grammar.Silent("ws")` where the spaces go.

### Calling rules and capturing

| Raku | Go builder | Captured as |
|---|---|---|
| `<name>` | `Call("name")` | `name` |
| `<.name>` | `Silent("name")` | nothing |
| `<alias=name>` | `Alias("alias", "name")` | both `alias` and `name`, as in Raku |
| `<alias=.name>` | `AliasOnly("alias", "name")` | `alias` only |
| `<?name>`, `<!name>` | `Before(Silent("name"))` | nothing; it only looks ahead |
| `( … )` | `Capture(x)` | `0`, `1`, … in order, each a match with captures of its own |
| `$<k>=[ … ]`, `$<k>=\d+` | `Named("k", x)` | `k`, the text only |
| `$<k>=( … )` | `Named("k", Capture(x))` | `k`, with captures of its own |
| `$<k>`, `$0` | `Backref("k")`, `Backref("0")` | the text the capture last matched, again, exactly: `$<q>=<['"]> \w+ $<q>` closes a quote with the same mark |

Raku's built-in rules are there too: `<ws>` (optional whitespace, but not
inside a word), `<alpha>` (a letter or `_`), `<digit>`, `<alnum>`,
`<upper>`, `<lower>`, `<space>` and `<punct>`.

**Unicode properties** pick characters by what Unicode says they are, so a
rule works in any language: `<:Lu>` is any capital letter, `É` and `Ж` as
well as `A`, where `<[A..Z]>` is only the 26. A property is:

- a general category, by its short or long name: `<:L>` or `<:Letter>`,
  `<:Lu>` or `<:Uppercase_Letter>`, `<:Nd>` (a decimal digit, in any
  script), `<:Pd>` (any dash: hyphen, en dash, em dash), `<:Zs>`, `<:Sc>`
  (a currency symbol);
- a script: `<:Latin>`, `<:Cyrillic>`, `<:Han>`, or `<:Script<Latin>>`;
- a binary property: `<:White_Space>`, `<:Dash>`, `<:Alpha>`
  (`Alphabetic`), `<:Upper>`, `<:Lower>`, and the others in Go's
  `unicode.Properties`.

Names ignore case, spaces, hyphens and underscores, as Unicode's do, so
`<:lu>` and `<:uppercase-letter>` are `<:Lu>`. `<:!Lu>` (or `<-:Lu>`) is
anything but. Properties combine with sets and named classes, as in
`<:L + [_]>` or `<[a..z] - :Lu>`. They come from Go's `unicode` tables, so
they're the Unicode version Go has.

Under `:i`, a set ignores case and a property doesn't, as in Raku:
`:i <[A..Z]>` matches "a", but `:i <:Lu>` doesn't, as "a" isn't a capital
whatever the case of the rule. The same goes for the named classes:
`:i <+upper>` takes only capitals. A property that doesn't exist is an
error when the grammar compiles.

**Conjunctions**, `&` and `&&`, match text that every branch matches,
all of it: each branch starts in the same place and must end in the same
place. `@titles & [ <:Lu> \S* ]+ % ' '` is a phrase from a list that's
also written in title case, and `<:Lu>+ & @acronyms` an acronym written
in capitals. In a `regex`, the branches backtrack until they agree, so
`[ \w+ && ab ] c` matches "abc", as `\w+` gives back the c; in a
`token`, each takes its first match, and they must agree on that. The
captures of every branch are kept. `&` binds tighter than `|`, and `&&`
tighter than `||`, as in Raku; Raku may try `&`'s branches in any order,
which can't make a difference here, so both go left to right.

As every branch must cover the same text, a branch that matches nothing,
such as a lookahead, can never agree with one that matches something:
`<w> & <!stop>` never matches. To match a word that isn't a stopword,
look ahead first: `<!stop> <w>`.

## Word lists

A word list is a set of words and phrases, each with a canonical value, so
that synonyms match as one thing. It's meant for data you build in code,
such as job titles or skills from a synonym store, and it keeps that data
out of the grammar: the grammar says where a title goes, the list says what
counts as one.

```go
titles := grammar.NewWordList("titles", grammar.FoldCase(), grammar.LooseSpacing())
titles.Add("Software Engineer", "Software Developer", "SWE")
titles.Add("Front-end Developer", "Frontend Developer")

g := grammar.MustCompile(`grammar Title { token TOP { <title> .* }; token title { @titles } }`, titles)
g.Parse("Front End Developer (React)").At("title.made")   // "Front-end Developer"
```

In Raku syntax a list is `@name`, and in Go `grammar.Words(list)`. It
matches the longest phrase at the current position, and like Raku's, it's
one atom: it never gives back part of what it matched, even in a `regex`.
Like Raku's, it doesn't check word boundaries, so "Java" in a list matches
the start of "JavaScript". Give the list the `WholeWords()` option (below)
to match only whole words, rather than writing `<< @langs >>` everywhere.

**Canonical values.** A rule whose whole body is a list, such as
`token title { @titles }`, gets the canonical value of what it matched as
its `made` value. An action for that rule sees it there, and may replace
it. So does a named capture of a list inside a bigger rule, so a list needs
no rule of its own: in `token TOP { $<level>=@levels? \s* $<role>=[ \N+ ] }`,
`level`'s `made` is "Senior" for "Snr Developer", and `grammar.To` reads it
straight into a struct. A quantified list (`@levels?`) has a canonical value
when what it matched is one of its phrases.

**Loading synonyms.** `Add(canonical, synonyms...)` adds phrases one group
at a time. `AddRule` and `AddRules` take rules in the format Elasticsearch
and Solr use, as Elasticsearch's synonyms API returns them:

```
annum, year, yr, pa        all mean the first, annum
hr, ph => Hour             each phrase on the left means Hour
pm => product manager, project manager
```

In `AddRules`, a rule ending with a comma carries on onto the next line, so
long rules can be wrapped. `WordLists(map[string]string{...}, opts...)`
builds a list for each set of rules, keyed by name, ready to pass to
`Compile`: one call instead of a loop per list.

**Matching options**, given to `NewWordList`:

| Option | Effect |
|---|---|
| `FoldCase()` | ignore case everywhere the list is used. Without it, the list ignores case only under `:i`, as in Raku. |
| `LooseSpacing()` | any run of spaces, hyphens, dashes and underscores matches any other, so "front-end" matches "front end". There must still be some spacing: "frontend" is a different phrase, unless the list has `JoinedSpacing()` too. |
| `JoinedSpacing()` | the text may leave out the spacing in a phrase, so "on site" matches "onsite" and "front-end" matches "frontend". Write the spaced form; see below. |
| `Plurals()` | each phrase matches its plural too, with the same canonical value: "Care Assistant" matches "Care Assistants". Regular English plurals of the last word only; see below. |
| `WholeWords()` | match a phrase only where it isn't part of a longer word: "lead" doesn't match "leadership". It's `<< @list >>` without writing the boundaries, except that a phrase starting or ending with punctuation, such as "c++", can end where the punctuation does. The longest phrase that fits still wins. Digits are word characters, as in Raku, so "pa" in "£45000pa" isn't a whole word; to check only the end, as for a unit after a figure, leave this off and write `@units <!before \w>`. |

**Joined spellings.** Postings write the same term spaced, hyphenated and
run together: "on site", "on-site" and "onsite"; "full stack", "full-stack"
and "fullstack". Without help, a list must spell out every form, and the
titleeval lists did just that. `LooseSpacing()` covers the spaced and
hyphenated forms; `JoinedSpacing()` covers the run-together one, so with
both, one phrase covers them all:

```go
places := grammar.NewWordList("places", grammar.FoldCase(), grammar.LooseSpacing(), grammar.JoinedSpacing()).
	Add("On-site", "on site")
// "on site", "On-Site", "on – site" and "ONSITE" all match, and all mean "On-site"
```

It works one way only, so write the spaced form in the list: from "on
site" it's clear that "onsite" is the same thing, but from "onsite" there's
no telling where a space would go. Each space, hyphen, dash or underscore
in a phrase may be left out on its own, so "full stack web developer" also
matches "fullstack web developer". `Canonical` knows the joined forms too:
`Canonical("onsite")` is `[On-site]`. `Phrases()` still lists only what was
added.

Two words run together can make a different word: "pen is" would match
"penis". A phrase in the list as written always wins over a joined reading
of another, so where a joined form means something else, add it as a
phrase of its own. `Ambiguous()` doesn't report these, as they aren't
phrases; it's worth reading a list's two-word phrases with this in mind
before turning the option on.

Matching a joined spelling searches the list's trie both ways at each
space, remembering where it's been, so it costs at most the trie's size
times the text's length. In practice the difference doesn't show:
`BenchmarkListJoined` and `BenchmarkListExact`, a `Find` over a sentence
with and without the option, are within each other's noise.
Without the option, a list matches as before, and as Raku's does.

**Plurals.** Postings use both "Care Assistant" and "Care Assistants",
"development opportunity" and "development opportunities". Without help, a
list must spell out both, or use a stem such as "opportunit" that matches
neither word whole, which then can't be used with `WholeWords`. With
`Plurals()`, each phrase matches its plural too, and means the same:

```go
roles := grammar.NewWordList("roles", grammar.FoldCase(), grammar.Plurals()).
	Add("Care Assistant", "carer").Add("Secretary")
// "care assistants", "carers" and "secretaries" match, meaning "Care Assistant" and "Secretary"
```

The plural is made from the last word only, by the regular English rules:

| Last word ends in | Plural | Examples |
|---|---|---|
| s, x, z, ch, sh | -es | coach → coaches, box → boxes |
| a consonant then y | y → -ies | secretary → secretaries |
| anything else | -s | engineer → engineers, day → days, PM → PMs |
| something other than a letter | none | "level 2", "c++" |

The ending is added in lower case, which is right for acronyms ("PMs");
with `FoldCase`, case doesn't matter anyway. Rules this simple get some
words wrong, but harmlessly: "sales" gets "saleses", which no text will
ever contain. Irregular plurals ("people", "analyses") are ordinary
synonyms: add them. Stemming ("visiting" for "visit") is a different job,
and isn't done.

The plurals are made, not added. `Phrases()`, `Len()` and `Ambiguous()`
show only what you added, so a list's contents read as you wrote them,
while matching and `Canonical` know the plurals: `Canonical("PMs")` is
what "PM" means. A phrase added as itself always beats a plural made from
another, whichever was added first, so with "new" and "news" both in a
list, "news" means news, not more than one new. The options combine:
with `JoinedSpacing` too, "front end developer" matches "frontend
developers".

**Ambiguity.** A phrase added for two canonical values, such as "PM" for
both product and project manager, matches as the first. `Ambiguous()` lists
such phrases at build time, and `Canonical(phrase)` returns everything a
phrase could mean, so the caller can decide from context, or pass the
posting on to the LLM.

Build a list fully before parsing with it. To refresh one, build a new list
and a grammar that uses it. A list is one trie (a prefix tree) for its exact
phrases and one for case-folded ones, so matching takes about the same time
however long the list is.

Raku's `<@name>` is something else: it matches each entry as a regex.
Here that's a different kind of list, a `PatternList`, so that synonym
data is never read as patterns by mistake; see [Pattern lists](#pattern-lists).

## Pattern lists

Some families of things are patterns rather than phrases: amounts written
`45k`, `45,000` or `45.5k`, or periods with optional spacing. A
`PatternList` holds them as data, each a regex in Raku syntax, and
`<@name>` matches them, as Raku's `<@array>` does:

```go
periods := grammar.MustPatternList("periods", `:i per \s* annum`, `:i 'p.a.'`, `:i pa >>`)
g := grammar.MustCompile(`grammar Pay { token TOP { '£' \d+ \s* <@periods> } }`, periods)
```

`<@name>` matches every pattern at the current position and keeps the
longest match, and, as in Raku, commits to it: if what follows fails, no
other pattern is tried, even in a `regex`. A pattern can call the
grammar's rules (`<amount>`, `<alpha>`), and `:i` reaches into it. Each is
read as a regex, not a rule, so its spaces don't match spaces in the text,
even in a `rule`: write `\s+`, or quote them, as in `'per annum'`.

What a pattern captures isn't kept, and `<@name>` captures nothing
itself, as in Raku; to keep what it matched, name it:
`$<period>=<@periods>`. A pattern can't use a word list.

`NewPatternList` compiles every pattern when the list is made, and
returns an error naming any that doesn't compile, where Raku fails only
when it comes to match one; `MustPatternList` panics instead. A pattern
that calls a rule the grammar doesn't have is caught when the grammar is
compiled, as any call is.

## Roles

A role is a library of rules that several grammars can share: money and
period tokens for both pay and bonus grammars, say, or one lexicon of
list-backed tokens for titles and requirements.

```go
units := grammar.NewWordList("units", grammar.FoldCase()).Add("Year", "annum", "pa").Add("Week", "wk")

g := grammar.MustCompile(`
role Pay {
    token money  { '£' <amount> }
    token period { \s* [ per | a | '/' ] \s* <unit> }
    token unit   { @units }
}
grammar Salary does Pay {
    token TOP    { <money> <period>? }
    token amount { \d+ [ ',' \d ** 3 ]* }
}`, units)
```

A role's rules become the grammar's own when it's composed:

- **They can call the grammar's rules.** `money` calls `<amount>`, which
  `Salary` provides. A role may call rules it doesn't have; the grammar
  that composes it must have them.
- **The grammar's own rules win** over the role's, and an extension's win
  over both.
- **Two roles with a rule of the same name are an error,** unless the
  grammar defines that rule itself to choose, as Raku requires.

In Go, `grammar.NewRole("Pay")` makes a role, with `Token` and `Regex` as
for a grammar, and `g.Does(role)` composes it, before or after `g`'s own
rules.

### Protos

A proto is a family of rules that grows. Each variant is one way of
writing the same kind of thing, a pay period, say, and an extension or a
role adds its own without changing the grammar it extends:

```raku
grammar Pay {
    token TOP    { '£' $<amount>=[ \d+ ] \s* <period> }
    proto token period {*}
    token period:sym<annum> { :i [ per \s+ annum | 'p.a.' | pa ] }
    token period:sym<hour>  { :i per \s+ hour }
}
grammar PayUK is Pay {
    token period:sym<week> { :i [ per \s+ week | pw ] }
    token period:sym<hour> { :i [ per \s+ hour | ph ] }   # replaces Pay's
}
```

Without protos, `period` would be one alternation, and every new way of
writing a period would mean editing it, in the grammar that owns it. With
them, `Pay` stays as it is, and each extension says only what it adds.

A call to `<period>` matches every variant the grammar being parsed with
can see, its own, its parents' and its roles', and keeps the one that
matches most text; of equal ones, the first declared. As in Raku, the call
then commits to it: if what follows fails, no other variant is tried, even
in a `regex`. An extension's variant with the same sym replaces its
parent's. The match is the variant's, captured as `period`, and the action
that runs is the variant's: a method named after both, `PeriodSymHour` for
`period:sym<hour>`, or the key `"period:sym<hour>"` in a map of actions,
which also works for syms that aren't words, such as `op:sym<+>`. The
proto's own action, `Period`, doesn't run.

Inside a variant, `<sym>` matches the text between the brackets exactly,
even under `:i`, as in Raku, and captures it as `sym`: in
`token unit:sym<year> { <sym> s? }`, `unit.sym.~` is "year". A sym can be
any text, written `:sym<…>` or `:sym«…»`.

In Go, `g.Proto("period")` declares the proto, variants are added with
`Token` or `Regex` by their full names, and `grammar.Sym()` is `<sym>`.

### Several parents

A grammar can extend more than one, as `grammar PayUK is Money is Period
{ … }` does, or `money.Extend("PayUK", period)` in Go. A rule is looked up
in the grammar itself, with its roles, then in its parents, in the order
Raku looks up a class's methods, called C3: parents in the order written,
and each one before the grammars it extends. In a diamond, where `L` and
`R` both extend `Base`, a grammar that `is L is R` looks in `L`, `R`, then
`Base`, so `R`'s version of a rule beats `Base`'s, even though `L` comes
first and inherits `Base`'s. As in Raku, a rule in a parent that calls
another calls the extension's version, if it has one.

Parents written in an order that contradicts their own, such as `is Base
is L` where `L is Base`, are an error, as are a parent named twice and a
role named with `is`.

Roles or parents? Where two roles have a rule of the same name, the
grammar must choose, which is an error until it does; where two parents
do, the earlier parent wins, silently. So roles suit libraries of rules
brought together, and parents suit extending grammars that work on their
own. A parent must work on its own here: each grammar a source declares is
checked as it's compiled, so a parent can't call a rule that only its
extensions define. Raku allows that, and fails only if the rule is
called; here, a set of rules that needs others to complete it is a role.

## The match

A match is a hash:

| Key | Holds |
|---|---|
| `~` | the matched text |
| `from`, `to` | where the match starts and ends, in characters |
| `made` | the value the rule's action returned, if it has one |
| a capture's name | its match |
| `0`, `1`, … | positional captures |

A capture that can match more than once is always an array, empty if it
didn't match. That includes anything under `*`, `+` or `**`, and any name
used twice in one rule. Every other capture is a single match, or missing
if it didn't match. Like Raku, this is decided by the grammar, not by what
matched, so the shape of the result never depends on the input. In `Pay`,
`money` is an array (it's captured twice, once as `min`, once as `max`), and
`period` is a single match that may be missing.

### Reading the text: `~`

To get the text a capture matched, add `.~` to its path:

```go
m.At("unit")     // the <unit> match: a hash
m.At("unit.~")   // its text: "week"
```

Leaving off the `.~` is an easy mistake. `m.At("unit").Str()` stringifies the
hash itself, as `HASH(0x…)` in Perl, and not the text. A capture that's an
array (see above) needs an index first: `m.At("money.0.~")`, not
`m.At("money.~")`, and `m.At("money").Str()` is `ARRAY(0x…)`. When the capture is
missing, `m.At("unit.~")` is undef, as any missing path is, so `Or` and
`Defined` work on it directly.

The name comes from Raku, where `~` is the string operator: `~$<unit>` is
the unit capture's text, and `~$/` the whole match's. It's also a key no
capture can clash with, as a rule can't be called `~`.

Because `~`, `from`, `to` and `made` are the match's own keys, captures
can't use those names.

A match captured under two names, as `<min=money>` captures under both
`min` and `money`, is one match under both, not two copies, as in Raku. Its
action runs once.

### Accented letters: text is read in NFC

An accented letter can be written two ways: as one character (`é`), or as
a letter followed by a combining accent (`e` and U+0301). They look the
same, and Unicode says they're equivalent, but they're different runes,
so a grammar written one way wouldn't match text written the other, and
`.` could match half of the letter. Most text arrives composed, but text
copied from some editors, PDFs and Mac file names doesn't.

So text is read in NFC, Unicode's composed form: the text given to
`Parse`, `Find` and the rest, and a grammar's literals, sets and regexes,
and a word list's phrases. Either spelling matches either, and `.` or
`<:L>` takes the accented letter whole. Raku goes further, reading text
as whole graphemes (see the differences table), but agrees on these.

A match describes the text as it was read: its `~`, `from` and `to`, a
failure's column, and `Split`'s pieces are of the text in NFC. For text in
NFC already, nearly all text, that's the text itself; for text that
isn't, positions count composed characters, as Raku's do, so normalise the
original with `golang.org/x/text/unicode/norm` before slicing it by them.

A byte that isn't valid UTF-8 is read as U+FFFD first, as Go reads text
into runes, everywhere text is read, so a phrase and the text it's found
in always agree.

Checking that text is in NFC takes about 1.3 ns a byte (`BenchmarkNFC`), but
a shortcut skips the check for text below U+0300 (English with `£` and
accented Latin letters), so a posting in plain English costs nothing
measurable.

### Captures in order: `Caps` and `Chunks`

A match is a hash, so it keeps the order of each name's matches (an
array), but not the order of different names. Where that matters, such as
the qualifiers in "Senior Engineer (Remote, Contract)", where each kind is
its own capture, `grammar.Caps(m)` gives a match's captures in the order
they come in the text, as Raku's `.caps` does:

```go
// token TOP { <title> [ ' (' [ <arrangement> | <contract> ]+ % ', ' ')' ]? }
for name, c := range grammar.Caps(m) {
	fmt.Println(name, c.At("~")) // title, then arrangement and contract as the text has them
}
```

A capture that matched more than once gives each match in its own place,
so the captures of a repeat come out interleaved. A match under two names
comes under both. `grammar.Chunks(m)` is the same, with the text between
captures under `~`, as Raku's `.chunks` does, so the chunks rebuild the
text: useful for marking up the parts of a title, or for checking what a
grammar left uncaptured.

Captures come by where they start, then where they end, so an empty
capture comes before a longer one at the same place, as it was made first.
Captures at the very same place, such as an alias and its rule's name,
come in name order. Raku leaves those to its hash, so its order changes
from one run to the next; `TestCapsMatchRaku` checks every Raku case with
its ties put in name order.

## Parsing

| Method | Does |
|---|---|
| `g.Parse(text)` | match all of text, as Raku's `.parse` |
| `g.Subparse(text)` | match from the start of text, not necessarily to the end |
| `g.Find(text)` | the first match anywhere in text |
| `g.FindAll(text)` | every match in text, left to right, not overlapping |
| `g.Split(text)` | the text between matches, with each match's text in place, as Raku's `split` with `:v` does |
| `g.ParseErr(text)`, `g.SubparseErr(text)` | as `Parse` and `Subparse`, with a `*ParseError` saying why when the text doesn't match |

Each returns undef (or an empty array) when nothing matches. Each takes
options: `grammar.Rule("name")` to start somewhere other than `TOP`, and
`grammar.Actions(a)`.

### Into Go structs: `To` and `Values`

`grammar.To[T](m)` fills a struct from a match:

```go
type Title struct{ Seniority, Title, Team, Arrangement string }
t, err := grammar.To[Title](g.Parse(text))
```

It reduces the match with `grammar.Values`, which turns each match, at
every level, into:

1. its `made` value, if an action made one (a word list's rule makes its
   canonical value);
2. otherwise, if it has captures, a hash of them, each reduced the same way;
3. otherwise its text.

Then `scalar.To` fills the struct, matching capture names to field names
ignoring case, as `encoding/json` does. Use a json tag for a name that
can't match a field, such as a positional capture (`json:"0"`) or a rule
called `pay-range`. A capture that didn't match leaves its field at its
zero value. When the text didn't match at all, `To` returns
`grammar.ErrNoMatch`, so a failed parse isn't mistaken for an empty struct.

So the usual pattern is: let the grammar's capture names be your field
names, give an action only to rules whose value isn't their text (a number,
say), and call `To`. `Values` on its own gives the reduced match as
`scalar` data, for reading with `At`.

### Actions

`grammar.Actions(a)` calls a's method for each rule that matched, named
after the rule: `amount` calls `Amount`, and `pay-range` or `pay_range`
calls `PayRange`. The method takes the match and returns its made value:

```go
func (PayActions) Amount(m *scalar.SV) any
```

Inside an action, `m.At("~")` is the text the rule matched, and
`m.At("num.~")` the text of its `<num>`; `m.At("num.made")` is what `<num>`'s
own action returned.

A `map[string]func(*scalar.SV) any`, keyed by rule name, works too. Actions
run innermost first, so a rule's action can read the made values of the
rules it called. Rules without an action are left alone.

### Speed

`Find` and `FindAll` work out, from the grammar, every character a match
could start with, and skip positions where none does. A pay grammar can
only start at a `£`, so a search for pay in a 4 KB posting skips almost
everything, and takes about 46 µs (`BenchmarkFindAll`); trying the grammar
at every position took 1.6 ms. A grammar of common words, which can start
almost anywhere, gains less: about twice as fast. Where the start can't be
worked out, as for a grammar starting with `Re`, nothing is skipped, so
the results are always the same either way: `TestStartsChangeNothing`
checks that on every Raku case.

## Errors

### When text doesn't match

`Parse` returns undef when text doesn't match. `ParseErr` returns the same,
plus an error saying where the grammar got furthest and what it expected
there:

```go
m, err := Pay.ParseErr("£45k - 55,000")
// grammar Pay: line 1, column 8: expected <money>, found "55,000"
```

That point is usually where the mistake is, as everything before it
matched something. A bare word in a rule is matched as one literal, so a
misspelt word is reported whole: `expected 'annum', 'year', 'hour' or
'day'`. What's expected is written as the grammar writes it: a
rule that couldn't start there by name (`<money>`), and otherwise the
pieces that could have come next (`'k'`, `\d`, `<[a..z]>`, `@units`, `end
of text`). A few failures are left out as no news: those inside
lookarounds and `<ws>`, and a class repeat such as `\s*` stopping, as it's
free to.

A closing bracket written with goal matching, `'(' ~ ')' <qualifiers>`,
is reported with the bracket it was for, and where that was: a title
such as "Senior Engineer (Remote, FTC" fails with `expected ', ' or ')'
to close '(' from line 1, column 17`, which points at the cause, not just
the end of the text. Goal matching is otherwise just `'(' <qualifiers>
')'`: as in Raku, a missing closer fails like anything else, so another
branch can still match. In a `rule`, each part keeps the space written
after it, so `rule TOP { '(' ~ ')' <w> }` matches "( ab )".

The error is a `*ParseError`, whose fields (`Line`, `Column`, `Offset`,
`Expected`, `Found`) give the same facts for a program to use: logging
which rule postings fail at, for instance, to see what a grammar is
missing. Working the report out costs a little, so `Parse` doesn't; use
`ParseErr` when you want to know why.

### Runaway matches

A `regex` rule backtracks, so a badly written one can take a very long time
on a text it nearly matches. `regex TOP { [ a+ ]+ $ }` tries every way of
sharing the a's out between the repeats before giving up, so each extra
`a` doubles the time: 30 of them, then a `b`, would take minutes. To stop
one odd posting from hanging a whole run, every call counts the matcher's
steps (a repeat taking an item, an alternation, a rule call, a lookbehind
trying a start), and gives up past `grammar.StepLimit`: 10 million by
default, about a second. `Find`, `FindAll` and `Split` count across every
position they try, so the limit is for the whole call.

Past the limit, `Parse`, `Subparse`, `Find`, `FindAll` and `Split` panic
with a `*StepLimitError`, as a `scalar` regex panics past
`scalar.RegexTimeout`; `ParseErr` and `SubparseErr` return it as their
error. A program reading texts it can't trust can recover it and go on to
the next text (see the `StepLimitError` example). Steps are counted rather
than timed, so a text that runs out runs out every time, on any machine.

A word list with `JoinedSpacing` counts too: its search for a joined
spelling counts a step for each place it tries, as a phrase with a long
run of spacing (a few thousand tabs, which the fuzzer found) can take
seconds otherwise.

The job-posting grammars in this repository take at most a few hundred
thousand steps a call; counting costs nothing measurable
(`BenchmarkFindAll`). Set `StepLimit` at startup, before matching; zero
or less means no limit.

A match can also go too deep. Matching recurses as it goes, a level or so
for each item matched, so a long enough match runs out of stack, such as
`a*` over a million a's, or `'' ** 2111111`, which the fuzzer found. Go
ends the program when a goroutine's stack passes its limit (1 GB on
64-bit systems), and nothing can recover from that. So every call also
counts how deep it is, and past `grammar.DepthLimit` it panics with a
`*DepthLimitError`, which can be recovered, and which `ParseErr` and
`SubparseErr` return, as for `StepLimit`.

The default, 250,000 levels, comes from measuring how much stack a level
takes, by running grammars of different shapes until they ran out:

| Shape | Stack a level |
|---|---|
| `token TOP { a* }`, and `'' ** n` | 572 bytes |
| `token TOP { a <TOP>? }` (a rule calling itself) | 685 bytes |
| `regex TOP { [ <x> [ <y> \| <z> ] <.ws> ]* }` | 633 bytes |
| `regex TOP { [ ( ( ( ( 'a' ) ) ) ) ]* }` (nested captures) | 585 bytes |
| `regex TOP { [ 'a' 'b' 'c' 'd' 'e' 'f' 'g' 'h' ]* }` | 1,047 bytes |

Every node that recurses into another counts a level, so no shape can
take much more than the largest of these, and 250,000 levels stay within
a quarter of the stack. That still lets `.*` match about 125,000
characters; the longest paragraph `grammarsections` sees is about 2,000.
A program that raises Go's limit with `debug.SetMaxStack` can raise
`DepthLimit` in proportion. Counting costs nothing measurable either.

### Mistakes in grammars

`Compile` returns an error for bad syntax, with the line and column, for a
call to a rule that doesn't exist, for a name it can't find (a grammar or
role after `is` or `does`, or a word list after `@`), and for a
left-recursive rule: one that can call itself again before matching any
text, such as `token list { <list>? ',' <item> }`, which would recurse for
ever. Write such rules with a quantifier instead: `token list { <item>+ %
',' }`. A call after a `Re` that can match no text counts too, as with
`Re("(?=x)")` or ``Re(`\b`)``, though neither matches an empty string on
its own: `Check` asks whether the regex could match empty text anywhere,
by leaving its assertions out. `MustCompile` panics where `Compile` returns
an error; use it
for grammars in package-level variables, so a bad one fails as the program
starts. Grammars built in Go are checked when they're first used, and
panic, as a bad regex does in `scalar`.

## Testing against Raku

`TestMatchesRaku` compiles each grammar in `rakuCases` here and in Raku,
parses each input with both, and compares the two match trees: the same
text, positions, captures and shape. It needs `raku` on the PATH, and is
skipped without it.

When the Go and Raku results differ, Raku is right, unless the difference is
one of those listed under
[Differences from Raku](#differences-from-raku).

To find out what Raku does with something, ask it directly, then add the
case to `rakuCases`:

```sh
raku -e 'grammar G { token TOP { [ a .*? ] b } }; say G.parse("aaab")'
```

### Fuzz tests

Three fuzz tests look for inputs that break promises the package makes,
whatever the grammar or text:

| Test | Checks |
|---|---|
| `FuzzCompile` | `Compile` never panics, however mangled the source |
| `FuzzMatch` | for grammars built around a fuzzed rule, with a proto and a pattern list to call: no panics; `Parse` and `ParseErr` agree; a parse matches all of the text; `Find` finds what `FindAll` finds first; skipping start positions changes nothing |
| `FuzzWords` | synonym rules never panic, and every phrase in a list matches itself; with `JoinedSpacing`, run together too, and with `Plurals`, its plural |

`go test` runs them on their seeds and on the inputs that once failed, kept
in `testdata/fuzz`. To search for new failures:

```sh
go test ./internal/scalar/grammar -run '^$' -fuzz '^FuzzMatch$' -fuzztime 2m
```

`FuzzMatch` keeps texts short and leaves out `regex` rules and `:!r`,
whose backtracking can take exponential time by design (see [Runaway
matches](#runaway-matches)). A failure it finds is written to `testdata/fuzz`;
commit it, so it's checked from then on.

## Gaps and possible additions

What this package doesn't do yet, where it behaves differently from Raku,
and what it would take to change. Each entry says when you'd need it, so
that it's clear which to do first when the need comes up.

**Effort** is a rough guide for someone new to the code:

| Effort | Means |
|---|---|
| 1 | an hour or two |
| 2 | about a day |
| 3 | a few days |
| 4 | a week or two |
| 5 | weeks, or a redesign of part of the matcher |

**Fixable** is *yes*, *partly* (it can be done, but not quite as Raku does
it), or *by design* (it's deliberate; change it only for a good reason).

### Where things are

Before changing anything, it helps to know how the package is put together:

- **[node.go](node.go)** has the matcher: one type per kind of node, each
  with a `match` method. Matching is continuation-passing: a node calls
  `k` for each way it can match, and returning false means "try another
  way", which is all backtracking is. `step` is where a token's ratchet
  happens: it commits to a node's first match, except where `extendable`
  says a frugal quantifier inside may still need to take more.
- **[grammar.go](grammar.go)** has grammars, roles, rule lookup (by name,
  when called, so overriding works), the capture scopes worked out from
  each rule (`newScope`: which captures are arrays), turning matches into
  scalar data (`build`), and actions.
- **[syntax.go](syntax.go)** is the Raku-syntax parser: hand-written
  recursive descent that produces the same nodes as the Go builder.
- **[words.go](words.go)** has word lists: a trie for exact phrases and one
  for case-folded phrases.
- **[raku_test.go](raku_test.go)** compares match trees with Raku's. Any
  change to how matching behaves should start with a case there, and Raku
  saying what's right.

### Missing Raku features

| Feature | When you'd need it | What it would take | Fixable | Effort |
|---|---|---|---|---|
| **Code assertions**: `<?{ … }>`, `<!{ … }>` | Checks only code can make, in the middle of a parse: min ≤ max, a plausible year, a word that's in a list but isn't a stopword. Without them, wrong parses get through and need cleaning up afterwards. | Raku code can't run here, so: a builder node holding `func(m *scalar.SV) bool`, and a syntax such as `<?{name}>` naming a function passed to `Compile`. The function needs the captures so far, so build a provisional match of the current scope from the `capList`. | partly | 2–3 |
| **Code blocks**: `{ make … }` | Making values in the grammar itself. | Actions do this. Not worth adding unless grammars are written by people who won't write Go. | by design | — |
| **True longest-token matching** for `\|` | When `Alt`'s approximation (below) gives a different answer from Raku, which the Raku tests will show; and to speed up large alternations and protos. | Work out each branch's declarative prefix (the part before anything that could backtrack or run code), build an automaton (NFA, then DFA) from the prefixes, and try branches in order of how much of the text their prefix matches. This is what Rakudo does in NQP. | yes | 5 |
| **Rule parameters and dynamic variables**: `token amount($cur)`, `$*CURRENCY` | Parsing that depends on context, such as a currency that changes how amounts read. | Arguments in the syntax and on `call`, and a stack of dynamic variables in `state`, visible to called rules. Inheritance and roles usually do the job more simply. | yes | 3 |
| **Interpolation**: `$var`, `<$regex>`, `<{ … }>` | Patterns chosen at run time. Backreferences, `$<name>` and `$0`, are supported. | Word lists cover literal phrases, pattern lists (`<@name>`) patterns from data, and `Re` regex strings. Variables would come with rule parameters (below). | partly | 2 |
| **Failure reports from `Find` and `FindAll`** | Finding out why pay wasn't found in a posting, rather than why one string didn't parse. | Track failures across every start position and report the furthest. The noise is the hard part: most positions fail at once, for no interesting reason. | partly | 2 |
| **Backtracking control beyond `:`**: `::`, `:::`, `<commit>`, `<cut>` | Taming slow, heavily backtracking `regex` rules. `:` after an atom (`[ a \| ab ]:`, `<w>:`) is supported, as Rakudo supports it. | Rakudo doesn't implement these either (2025.11: `::` and `:::` are "not yet implemented", and `<commit>` and `<cut>` are taken as calls to rules of those names), so there's nothing to check them against, and a grammar using them wouldn't run in Raku; `Compile` rejects `::` and `:::`. They need the matcher to unwind to a marked point, which continuation-passing makes awkward. `:`, tokens and `StepLimit` usually do the job. | partly | 3 |

### Differences from Raku

| Difference | When it matters | What it would take | Fixable | Effort |
|---|---|---|---|---|
| **A grammar that calls a rule it doesn't have is an error,** even if an extension would supply it. Raku allows it, and fails only if the rule is called. | Only for grammars meant to be completed by extensions: write those as roles, which may call rules they don't have. | — | by design | — |
| **`<p:sym<x>>` is an error.** Raku reads it as a call to the proto `<p>`, ignoring the `:sym<x>`, which is never what it looks like. | Only when calling one variant directly: give its rules a name of their own instead. | — | by design | — |
| **A pattern list's bad pattern is an error when the list is made.** Raku fails only when it comes to match it. | Never for patterns that compile. | — | by design | — |
| **An unknown Unicode property is an error.** Raku's `<:Bogus>` matches nothing, silently. | Never for a property that exists. A misspelt one fails when the grammar compiles, rather than making a rule that can never match. | — | by design | — |
| **`Caps` puts captures at the same place in name order.** Raku's `.caps` leaves them in hash order, which changes from run to run. | Never for captures at different places. An alias and its rule's name, or two empty captures at one spot, come in a fixed order here and a varying one in Raku. | — | by design | — |
| **`\|`, and a call to a proto, pick the branch whose first match is longest,** and of equal ones, the first. Raku picks by the longest declarative prefix, roughly the part of each branch before anything that could backtrack. | Rarely: they agree on ordinary alternations of words and phrases. They can differ when branches share a start and something after it decides. | True longest-token matching (above). | yes | 5 |
| **Actions run after the whole match succeeds,** innermost first. Raku runs each as its rule matches, including for matches it later backtracks out of. | Only for actions with side effects, such as counting or logging. | Run actions in `call`'s `finish`, keeping their values on the `mnode`. Running after is easier to reason about, so keep it unless something needs the other. | by design | 2 |
| **Text is read as UTF-8, a rune at a time.** A byte that isn't valid UTF-8 becomes U+FFFD (`�`), as Go's `[]rune(text)` does, so a match's `~` holds `�` where the text held the bad byte. Raku can't hold such text at all: it fails to decode it. | Scraped text with bad bytes in it, when the exact bytes matter. | Clean or decode the text first, or match byte strings as `scalar` does (each byte a character from 0 to 255). | by design | 2 |
| **Positions count runes, not graphemes,** in NFC. Text is read in NFC, so an accented letter that has a composed form is one character, as in Raku. One that doesn't, such as q with a combining tilde (q̃), is two here, and one grapheme in Raku, so `.` can match half of it. | Rare in job postings: languages whose letters have no composed form, and emoji sequences. | Segment the input with `rivo/uniseg` and match on graphemes. | yes | 3 |
| **`<alias=name>` captures under both names,** as Raku does. | When you expect only the alias. | Nothing to fix: use `<alias=.name>`. | by design | — |
| **`Re` takes Perl-style regex syntax.** | Only that it isn't Raku syntax. It's for patterns you already have. | — | by design | — |
| **Word lists add to Raku's `@list`:** canonical values, `FoldCase`, `LooseSpacing`, `JoinedSpacing`, `Plurals` and `WholeWords`. | Never for Raku-compatible lists: without the options, a list matches as Raku's does. | — | by design | — |

### Engineering

| Gap | When you'd need it | What it would take | Fixable | Effort |
|---|---|---|---|---|
| **`Find` can't skip positions for some grammars.** It skips positions where no match can start, but only when it can work that out: not for a grammar that starts with a regex terminal (`Re`), or that can match empty text, as those could start anywhere. Such grammars search at about the old speed. | Searching long texts with grammars like those. | For `Re`, ask regexp2 for the pattern's possible first characters (it works them out internally, but doesn't expose them), or let `Re` take them as a hint. A grammar that can match empty text matches everywhere, so there's nothing to skip. | partly | 2 |
| **The matcher's general machinery runs even for trivial steps.** Matching takes more than half of a parse, and most of its allocations are small closures: one each time a token commits to an atom's first match (`once`), and more in the quantifier loop. Matching "£45,000 - £55,000 per annum" makes about 530 allocations (`BenchmarkParse`). | Parse speed at high volumes. A whole parse of a pay string takes about 14 µs (`BenchmarkParse`), against Participle's 6 µs; profiled, most of a parse's own time is allocation. | Fast paths for atoms that can match only one way (a literal, a class, an anchor): match them directly when committing, with no closure, and run a greedy repeat of one (`\d+`, `\s*`) as a plain loop. Estimated, when a parse took 22 µs, to save about a third. Trade-offs: the fast paths duplicate what the general engine does, so later changes to atoms (case folding, failure reports) must be made twice; they must step aside for frugal, possessive or separated repeats and items that can match nothing; and they must record failures exactly as the slow path does, or `ParseErr` reports would change. Add a switch to turn them off, as `skipStarts` does for `Find`, so every test can compare both paths. A lexer was considered instead and rejected: it would make matching cheaper, but cost precise errors, word lists that span spaces, `Find` anywhere in text, and Raku compatibility, and the match tree and conversions would still cost about 9 µs. | yes | 1–2 |
| **Each match's text is a copy.** A match's `~` is rebuilt from the runes of the text, rather than being a slice of the original string. | Never, as measured. | Tried and measured, then reverted: sharing the original string (when it's valid UTF-8, with a cursor from rune positions to byte offsets) saved about 1% of allocations, made a parse no faster, and made `FindAll` on a 4 KB posting 20% slower, through checking the whole text was UTF-8 and walking the cursor to each match. Copying a match's few characters was always cheap, and storing text in a scalar allocates either way. | no | — |
| **No memoisation.** A rule may be tried at the same position more than once. | Rarely, as measured (2026-10-16): of 37,400 token calls across titleeval, payeval and grammarrules, 18 were repeats at the same position, and `BenchmarkParse` and `BenchmarkFindAll` have none. Only `regex` rules that backtrack heavily through tokens repeat much, and `StepLimit` bounds those. | Cache each token's result by rule and position in `state` (a token has at most one result there), skipped under `ParseErr`, which must record every failure. Not worth it for these grammars: it would add a map lookup to every call to save almost none. Worth reconsidering, perhaps as an option, for a grammar whose profile shows repeats. | yes | 2 |
| **`<?after …>` tries every earlier position.** It matches its pattern ending at the current position by trying each start before it, so it costs time in proportion to how far into the text it is, and heavy use in `FindAll` grows with the square of the text's length. | Long texts with many lookbehinds. | Where the pattern's length is fixed or bounded (a literal, a class, a bounded repeat), try only those start positions. | yes | 2 |
| **Nested alternations in a `regex` double their work at each level.** `\|` matches each branch to measure it, then matches the winner again for real. In a `token` the second match is skipped, as the first is final. | Deeply nested alternations inside `regex` rules, which are rare. | Part of true longest-token matching (above), which decides from prefixes without matching branches in full. | yes | 5 |
| **`extendable` is worked out at every step.** | Never, as measured (2026-10-16): profiling `BenchmarkParse`, it's under 1% of the samples, and about 2% of the parse's own time, so a cache could save no more. | A cache would need a map lookup per step, as `seq` is a slice with nowhere to keep one, which costs about what working it out does. Not worth doing; the allocations in the row above are where a parse's time goes. | no | — |
| **Word lists: no stemming or fuzzy matching.** (`JoinedSpacing` covers "frontend" against "front end", and `Plurals` regular plurals.) | Synonym data that doesn't list every form of a word: "visiting" for "visit". | Stemming could follow `Plurals`, making forms in `Add`, but English stems are irregular enough to need a real stemmer, such as Porter's (effort 2–3). Fuzzy matching belongs elsewhere: compare embeddings for near misses, and send the rest to the LLM (effort 3+). | yes | 2–3 |
| **Word lists can't change while in use.** | Refreshing synonyms in a long-running service. | Build a new list and grammar, and swap them in with an `atomic.Pointer`. Needs no change here. | yes | 1 |
| **The Raku-syntax parser is hand-written,** not built with the package's own builder as first planned. | Only if the syntax grows much larger. | Rewrite it as a grammar of Raku regex syntax, with actions building nodes. | yes | 3 |

## Acknowledgements

The ideas here aren't new. `grammar` follows Raku's grammars, and is
tested against Rakudo to keep its behaviour faithful to theirs; the match
values it builds come from `scalar`, which borrows Perl's way with data.
Thanks to the Raku and Perl communities for decades of thought about
parsing text well.

Much of the code was written with Claude, Anthropic's AI model, working
from my direction and review.

## Reference

One example for every function and method, with the output it prints. The
examples are a guide to grammars as well as to the API: read them in order.
<!-- reference:start -->

Generated from the examples in `example_*_test.go`, which `go test` runs, so every
example here works and prints what it says. Edit the examples, not this section, then run
`go test ./internal/scalar/grammar -run TestReference -update`.

- [A worked example](#reference-a-worked-example)
- [Writing grammars](#reference-writing-grammars)
- [Parsing and matches](#reference-parsing-and-matches)
- [Building rules in Go](#reference-building-rules-in-go)
- [Word lists](#reference-word-lists)

### Reference: A worked example

#### Overview

The whole idea in one example: find pay in a sentence, and read the figures the actions made from it.

```go
text := "Salary: £45k – £55,000 per annum, plus bonus."

m := Pay.Find(text, grammar.Actions(PayActions{}))
fmt.Println(m.At("~")) // the text of the whole match
fmt.Println(m.At("min.amount.made"), m.At("max.amount.made"))
fmt.Println(m.At("period.~"))
fmt.Println(m.At("from"), m.At("to")) // where in text it was
```

```
£45k – £55,000 per annum
45000 55000
 per annum
8 32
```

#### Match tree

A whole match, printed as YAML to show its shape. Every match is a hash with its text under "~" and its position under "from" and "to". Each \<rule> it called is a match of its own, under the rule's name, so the tree follows the grammar. \<.sep> has a dot, so it isn't captured.

\<low=n> captures under both low and n, as in Raku, and as n is captured twice, n is an array. low and n's first element are the same match, not copies, as in Raku; YAML shows that with an anchor, &2, and a reference back to it, \*2.

```go
g := grammar.MustCompile(`
grammar Range {
    token TOP { <low=n> <.sep> <high=n> }
    token n   { \d+ }
    token sep { \s* '-' \s* }
}`)
out, _ := g.Parse("18 - 21").ToYAML()
fmt.Print(string(out))
```

```
from: 0
high: &1
  from: 5
  to: 7
  "~": "21"
low: &2
  from: 0
  to: 2
  "~": "18"
"n":
  - *2
  - *1
to: 7
"~": 18 - 21
```

#### Titles

The workflow this package is for: synonym lists, such as you'd pull from a synonym store, a grammar that says where each kind of word goes, and grammar.To to hand back typed Go. Every spelling of a title, a seniority or a working arrangement comes out as its canonical value.

```go
seniority := grammar.NewWordList("seniority", grammar.FoldCase())
titles := grammar.NewWordList("titles", grammar.FoldCase(), grammar.LooseSpacing())
arrangements := grammar.NewWordList("arrangements", grammar.FoldCase(), grammar.LooseSpacing())
for l, rules := range map[*grammar.WordList]string{
	seniority:    "Senior, Snr, Sr\nLead\nJunior, Jnr, Jr",
	titles:       "Software Engineer, Software Developer, SWE, Developer\nProduct Manager, PM",
	arrangements: "Remote, WFH, home based\nHybrid\nOnsite, on-site, office based",
} {
	if err := l.AddRules(rules); err != nil {
		panic(err)
	}
}

g := grammar.MustCompile(`
grammar Title {
    token TOP         { [ <seniority> \s+ ]? <title> [ \s* <.dash> \s* <team> ]? [ \s* '(' <arrangement> ')' ]? }
    token seniority   { @seniority }
    token title       { @titles }
    token arrangement { @arrangements }
    token dash        { '-' | '–' | '|' }
    token team        { \w+ [ ' ' \w+ ]* }
}`, seniority, titles, arrangements)

// Capture names fill the fields of the same names, ignoring case: each
// word list's rule gives its canonical value, and team its text.
type Title struct {
	Seniority, Title, Team, Arrangement string
}
for _, text := range []string{
	"Snr SWE – Payments (WFH)",
	"product manager | Growth (office based)",
	"Lead Software Developer",
} {
	t, err := grammar.To[Title](g.Parse(text))
	fmt.Printf("%+v %v\n", t, err)
}
```

```
{Seniority:Senior Title:Software Engineer Team:Payments Arrangement:Remote} <nil>
{Seniority: Title:Product Manager Team:Growth Arrangement:Onsite} <nil>
{Seniority:Lead Title:Software Engineer Team: Arrangement:} <nil>
```

### Reference: Writing grammars

#### `Compile`

Compile compiles grammars and roles written in Raku syntax, and returns the last one declared:

A grammar is a set of named rules. Parsing starts at TOP, and a rule calls another by writing its name in angle brackets. Here TOP calls \<word> and \<num>, and the match has one of each.

Inside a rule, spaces and # comments mean nothing; letters and digits match themselves, and anything else must be quoted, as '-' is.

```go
g, err := grammar.Compile(`
grammar Code {
    token TOP  { <word> '-' <num> }   # such as REF-1234
    token word { <[A..Z]>+ }          # one or more capital letters
    token num  { \d+ }
}`)
if err != nil {
	fmt.Println(err)
	return
}
m := g.Parse("REF-1234")
fmt.Println(m.At("word.~"), m.At("num.~"))

// Errors say where.
_, err = grammar.Compile(`grammar Bad { token TOP { a - b } }`)
fmt.Println(err)
```

```
REF 1234
grammar: line 1, column 29: '-' must be quoted, as '-', or escaped, as \-
```

#### `Compile.several`

One source can declare several grammars and roles; Compile returns the last. Later declarations can use earlier ones, here with is.

```go
g, _ := grammar.Compile(`
grammar Base  { token TOP { <greeting> ', ' \w+ }; token greeting { hello } }
grammar Polite is Base { token greeting { good \s+ morning } }
`)
fmt.Println(g.Name(), g.Parse("good morning, Ada").At("greeting.~"))
```

```
Polite good morning
```

#### `MustCompile`

MustCompile is Compile, panicking if src doesn't compile.

MustCompile is for grammars written into the program. As a package-level variable, a grammar with a mistake in it stops the program as it starts, rather than when it's first used.

```go
// var Hours = grammar.MustCompile(`...`) at package level, usually.
hours := grammar.MustCompile(`grammar Hours { token TOP { \d+ [ '.' \d+ ]? \s* hours } }`)
fmt.Println(hours.Find("Working 37.5 hours a week").At("~"))
```

```
37.5 hours
```

#### `New`

New returns an empty grammar called name.

New starts a grammar in Go, for building rules from data or code rather than writing them in Raku syntax. Token and Regex add rules to it.

```go
hours := grammar.New("Hours").
	Token("TOP", grammar.Seq(grammar.Call("num"), grammar.Opt(grammar.Space),
		grammar.IgnoreCase(grammar.Alt(grammar.Lit("hours"), grammar.Lit("hrs"), grammar.Lit("h"))))).
	Token("num", grammar.Seq(grammar.Plus(grammar.Digit), grammar.Opt(grammar.Seq(grammar.Lit("."), grammar.Plus(grammar.Digit)))))

fmt.Println(hours.Find("Working 37.5 HRS per week").At("num.~"))
```

```
37.5
```

#### `Grammar.Token`

Token adds a rule that doesn't backtrack, as Raku's token does.

A token doesn't backtrack: each part keeps the first match it finds and never gives any of it back. Here \\d\* takes all the digits, 5 included, so the 5 after it has nothing left to match, and the token fails, even on "5" alone. Written as \\d\* \<?after 5>, or \[ \\d\* ] \<?after 5>, it would work; or use Regex.

That sounds like a limitation, but it's what you want almost always: it makes a grammar's behaviour easy to predict, and fast. Use token unless you need Regex.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Star(grammar.Digit), grammar.Lit("5")))
// The same in Raku syntax: token TOP { \d* 5 }

fmt.Println(g.Parse("12345").Defined())
fmt.Println(g.Parse("5").Defined())
```

```
false
false
```

#### `Grammar.Regex`

Regex adds a rule that backtracks, as Raku's regex does.

A regex backtracks, as a Perl regex does: when what follows fails, \\d\* gives back a digit and tries again, so \\d\* 5 matches "12345".

```go
g := grammar.New("G").Regex("TOP", grammar.Seq(grammar.Star(grammar.Digit), grammar.Lit("5")))
// The same in Raku syntax: regex TOP { \d* 5 }

fmt.Println(g.Parse("12345").Defined())
```

```
true
```

#### `Grammar.Extend`

Extend returns an empty grammar called name that inherits g's rules, as grammar name is g { ...

An extension inherits its parent's rules and can replace any of them. A replaced rule is used everywhere, even where the parent's own rules call it: here Pay's TOP and money call \<amount> and \<period> as before, but PayUS's money is the one they get.

```go
us := Pay.Extend("PayUS").Token("money", grammar.Seq(grammar.Lit("$"), grammar.Call("amount")))
fmt.Println(us.Subparse("$60k - $70k per year").At("max.~"))

// The same, in Raku syntax.
us2 := grammar.MustCompile(`grammar PayUS is Pay { token money { '$' <amount> } }`, Pay)
fmt.Println(us2.Subparse("$60k - $70k per year").At("max.~"))
```

```
$70k
$70k
```

#### `Grammar.Extend_several`

Extend with more grammars inherits from them all, as is A is B does. Rules are looked up in the new grammar, then its parents in the order given: here Money's and Period's rules work together, and PayUK overrides Period's \<unit>, which Period's own \<period> then calls.

```go
g := grammar.MustCompile(`
grammar Money  { token money  { '£' <num> }; token num { \d+ } }
grammar Period { token period { ' per ' <unit> }; token unit { year } }
grammar PayUK is Money is Period {
    token TOP  { <money> <period> }
    token unit { hour | day }
}`)
fmt.Println(g.Parse("£18 per hour").At("period.unit.~"), g.Parse("£18 per year").Defined())

// The same, in Go.
money := grammar.New("Money").Token("money", grammar.Seq(grammar.Lit("£"), grammar.Call("num"))).
	Token("num", grammar.Plus(grammar.Digit))
period := grammar.New("Period").Token("period", grammar.Seq(grammar.Lit(" per "), grammar.Call("unit"))).
	Token("unit", grammar.Lit("year"))
payUK := money.Extend("PayUK", period).
	Token("TOP", grammar.Seq(grammar.Call("money"), grammar.Call("period"))).
	Token("unit", grammar.Alt(grammar.Lit("hour"), grammar.Lit("day")))
fmt.Println(payUK.Parse("£500 per day").At("money.num.~"))
```

```
hour false
500
```

#### `Grammar.Proto`

Proto declares name a proto, as proto token name {*} does: a family of rules, the variants, each named name:sym<…> and added with Token or Regex, which extensions and roles can add to without changing the grammar that declares it.

A proto is a family of rules that grows: each variant, named period:sym\<…>, is one way to write a period, and an extension adds its own without changing the grammar it extends. A call to \<period> tries them all, the extension's and its parents', and keeps the longest. Here PayUK adds weekly and daily rates to Pay's, and replaces Pay's hourly one, which now also takes "ph".

```go
g := grammar.MustCompile(`
grammar Pay {
    token TOP    { '£' $<amount>=[ \d+ ] \s* <period> }
    proto token period {*}
    token period:sym<annum> { :i [ per \s+ annum | 'p.a.' | pa ] }
    token period:sym<hour>  { :i per \s+ hour }
}
grammar PayUK is Pay {
    token period:sym<week> { :i [ per \s+ week | pw ] }
    token period:sym<day>  { :i per \s+ day }
    token period:sym<hour> { :i [ per \s+ hour | ph ] }
}`)
acts := map[string]func(*scalar.SV) any{
	"period:sym<annum>": func(*scalar.SV) any { return "Year" },
	"period:sym<week>":  func(*scalar.SV) any { return "Week" },
	"period:sym<day>":   func(*scalar.SV) any { return "Day" },
	"period:sym<hour>":  func(*scalar.SV) any { return "Hour" },
}
for _, text := range []string{"£45000 per annum", "£450 pw", "£18 ph", "£150 per day"} {
	fmt.Println(g.Parse(text, grammar.Actions(acts)).At("period.made"))
}
```

```
Year
Week
Hour
Day
```

#### `NewRole`

NewRole returns an empty role called name: a library of rules to compose into grammars with Does, as Raku's role is.

A role is a library of rules to share between grammars. Grammars take a role's rules as their own with does. A role's rules can call rules it doesn't have, here \<amount>, for each grammar that uses it to provide.

```go
money := grammar.NewRole("Money").
	Token("money", grammar.Seq(grammar.Lit("£"), grammar.Call("amount")))

whole := grammar.New("Whole").Does(money).
	Token("TOP", grammar.Call("money")).
	Token("amount", grammar.Plus(grammar.Digit))
pence := grammar.New("Pence").Does(money).
	Token("TOP", grammar.Call("money")).
	Token("amount", grammar.Seq(grammar.Plus(grammar.Digit), grammar.Lit("."), grammar.Repeat(grammar.Digit, 2, 2)))

fmt.Println(whole.Parse("£12").Defined(), whole.Parse("£12.50").Defined())
fmt.Println(pence.Parse("£12").Defined(), pence.Parse("£12.50").Defined())
```

```
true false
false true
```

#### `Grammar.Does`

Does composes roles into g, as grammar g does role { ...

Does composes roles into a grammar. The grammar's own rules win over the role's, wherever Does is called. Two roles with a rule of the same name are an error, unless the grammar defines that rule itself to choose.

In Raku syntax: role Money { ... } and grammar G does Money { ... }.

```go
g := grammar.MustCompile(`
role Units { token unit { hours | hrs } }
role Extra { token unit { h } }
grammar Hours does Units does Extra {
    token TOP  { \d+ \s* <unit> }
    token unit { hours | hrs | h }    # ours: needed, as both roles have one
}`)
fmt.Println(g.Parse("8 h").At("unit.~"))

_, err := grammar.Compile(`
role Units { token unit { hours } }
role Extra { token unit { h } }
grammar Hours does Units does Extra { token TOP { \d+ <unit> } }`)
fmt.Println(err)
```

```
h
grammar: line 4, column 65: rule unit is in both Units and Extra; define it in Hours to choose
```

#### `Grammar.Check`

Check reports the first call to a rule that doesn't exist, and the first left-recursive rule: one that can call itself again, directly or through other rules, before matching any text, and so would recurse for ever.

Check reports a call to a rule that doesn't exist. Compile checks, but a grammar built in Go is only checked when it's first used, and then a missing rule panics, so call Check after building one from data.

```go
g := grammar.New("G").Token("TOP", grammar.Call("missing"))
fmt.Println(g.Check())
```

```
grammar G: rule TOP calls <missing>, which doesn't exist
```

#### `Grammar.Name`

Name returns the grammar's name.

```go
fmt.Println(Pay.Name())
```

```
Pay
```

#### `Grammar.IsRole`

IsRole reports whether g is a role.

```go
fmt.Println(Pay.IsRole(), grammar.NewRole("Money").IsRole())
```

```
false true
```

#### `Compile.rule`

In a rule, the spaces in the rule match spaces in the text: each becomes a call to \<.ws>, which matches any amount of whitespace, so the rule reads like the text it matches. Here "up to £500 per day" matches with any spacing, but "upto" doesn't, as \<.ws> won't match nothing between two words. See the README's "Spaces in rules" for exactly where a space counts.

```go
g := grammar.MustCompile(`
grammar Rate {
    rule  TOP   { up to <money> per <unit> }
    token money { '£' \d+ }
    token unit  { day | hour | week }
}`)
for _, text := range []string{"up to £500 per day", "up  to £500   per day", "upto £500 per day"} {
	fmt.Println(g.Parse(text).At("money.~").Or("no match"))
}
```

```
£500
£500
no match
```

#### Unicode properties

Unicode properties pick characters by what Unicode says they are, so a rule works in any language: \<:Lu> is any capital, where \<\[A..Z]> is only the 26. Here, capitalised words in names from three scripts.

```go
g := grammar.MustCompile(`grammar Names { token TOP { <:Lu> <:Ll>+ } }`)
ascii := grammar.MustCompile(`grammar ASCII { token TOP { <[A..Z]> <[a..z]>+ } }`)

text := "Zoë Ölander, Жанна Петрова, 李 Wei"
fmt.Println(g.FindAll(text).Map(func(m *scalar.SV) any { return m.At("~") }).Join(" | "))
fmt.Println(ascii.FindAll(text).Map(func(m *scalar.SV) any { return m.At("~") }).Join(" | "))
```

```
Zoë | Ölander | Жанна | Петрова | Wei
Zo | Wei
```

### Reference: Parsing and matches

#### `Grammar.Parse`

Parse matches the whole of text, as Raku's .parse does, and returns the match, or undef if text doesn't match.

Parse matches the whole text, or nothing: a match that would leave text over is no match. When it fails, the result is undef, so Defined, Or and the like work on it, and reading through it gives undef too.

```go
fmt.Println(Pay.Parse("£18.50 per hour").At("min.amount.num.~"))
fmt.Println(Pay.Parse("£18.50 per hour, negotiable").Defined()) // not all of it
fmt.Println(Pay.Parse("negotiable").At("min.~").Or("none"))
```

```
18.50
false
none
```

#### `Grammar.Subparse`

Subparse matches from the start of text, without needing to reach the end, as Raku's .subparse does.

Subparse matches from the start of the text, without needing to reach its end.

```go
m := Pay.Subparse("£18.50 per hour, negotiable")
fmt.Println(m.At("~"))
fmt.Println(Pay.Subparse("Pay: £18.50").Defined()) // it must start at the start
```

```
£18.50 per hour
false
```

#### `Grammar.Find`

Find returns the first match anywhere in text, or undef.

Find hunts for the first place in the text where the grammar matches, which is how to pull a fact out of free text.

```go
m := Pay.Find("The role pays £30k to £35k depending on experience.")
fmt.Println(m.At("~"), "at", m.At("from"))
```

```
£30k to £35k at 14
```

#### `Grammar.FindAll`

FindAll returns every match in text, left to right and not overlapping.

FindAll finds every match, left to right, without overlapping.

```go
for _, m := range Pay.FindAll("£30k basic, £5k car allowance, £2k bonus").All() {
	fmt.Println(m.At("~"))
}
```

```
£30k
£5k
£2k
```

#### `Rule`

Rule starts the match at the named rule instead of TOP, as :rule does.

Rule starts the match at another rule instead of TOP, which is handy for testing one rule on its own, or for reusing part of a grammar.

```go
fmt.Println(Pay.Parse("12,500", grammar.Rule("num")).At("~"))
fmt.Println(Pay.Parse("12,500").Defined()) // TOP wants a £
```

```
12,500
false
```

#### `Actions`

Actions has a's methods turn matches into values, as :actions does.

Actions turns matches into values. For each rule that matched, the method named after it is called with its match, and what it returns is stored in the match under "made". Actions run innermost first, so here Amount has run for each \<amount> before TOP's action reads their made values.

```go
m := Pay.Parse("£45k - £55,000", grammar.Actions(PayActions{}))
fmt.Println(m.At("min.amount.made"), m.At("max.amount.made"))
```

```
45000 55000
```

#### `Actions.map`

A map of functions, keyed by rule name, works as well as a type with methods, and suits actions built at run time.

```go
m := Pay.Parse("£45k - £55k per annum", grammar.Actions(map[string]func(*scalar.SV) any{
	"amount": PayActions{}.Amount,
	"period": func(m *scalar.SV) any { return strings.TrimSpace(m.At("~").Str()) },
}))
fmt.Println(m.At("max.amount.made"), m.At("period.made"))
```

```
55000 per annum
```

#### `Actions.to`

A match is ordinary scalar data, so once TOP's action has built the value you want, scalar.To makes it typed Go.

```go
type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}
m := Pay.Parse("£45k - £55k", grammar.Actions(map[string]func(*scalar.SV) any{
	"amount": PayActions{}.Amount,
	"TOP": func(m *scalar.SV) any {
		min := m.At("min.amount.made")
		return scalar.Hash("min", min, "max", m.At("max.amount.made").Or(min))
	},
}))
r, err := scalar.To[Range](m.At("made"))
fmt.Printf("%+v %v\n", r, err)
```

```
{Min:45000 Max:55000} <nil>
```

#### Capture shapes

How captures are shaped. A capture that can match more than once, because it's quantified (\*, +, \*\*) or its name appears twice, is always an array, even when it matched once or not at all. Any other capture is a single match, or missing when it didn't match. The grammar decides this, not the text, so code reading a match never has to check which it got.

( ... ) captures by position, as "0", "1" and so on; $\<name>=\[ ... ] captures the text it matched under a name.

```go
g := grammar.MustCompile(`
grammar Shapes {
    token TOP  { <tag>* <.ws> $<code>=[ \d+ ] ( '!' )? }
    token tag  { '#' \w+ <.ws> }
}`)
for _, text := range []string{"#a #b 42!", "42"} {
	m := g.Parse(text)
	fmt.Println(m.At("tag").AV().Length(), "tags;",
		"code", m.At("code.~"), ";",
		"bang", m.At("0.~").Or("none"))
}
```

```
2 tags; code 42 ; bang !
0 tags; code 42 ; bang none
```

#### `Values`

Values reduces a match to the values in it, ready for `scalar.To`.

Values reduces a match to the values in it: each match becomes its made value if an action made one, a hash of its captures if it has any, and its text otherwise. That's usually the shape you want to read, or to pass to scalar.To.

```go
g := grammar.MustCompile(`
grammar Person {
    token TOP   { <name> ',' <.ws> <born> }
    token name  { <first> ' ' <last> }
    token first { \w+ }
    token last  { \w+ }
    token born  { \d ** 4 }
}`)
out, _ := grammar.Values(g.Parse("Ada Lovelace, 1815")).ToYAML()
fmt.Print(string(out))
```

```
born: "1815"
name:
  first: Ada
  last: Lovelace
```

#### `To`

To reduces a match with Values and converts it to a T with `scalar.To`, so a grammar fills a struct directly:

To fills a struct from a match, by way of Values: capture names match field names, ignoring case, as in encoding/json, and a json tag can name one that doesn't, such as a positional capture or a rule called pay-range. A capture that didn't match leaves its field at its zero value. If the text didn't match at all, To returns ErrNoMatch.

```go
type Range struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Period string
}
acts := grammar.Actions(map[string]func(*scalar.SV) any{
	"money":  func(m *scalar.SV) any { return m.At("amount.made") }, // a money is its amount
	"amount": PayActions{}.Amount,
	"period": func(m *scalar.SV) any { return strings.TrimSpace(m.At("~").Str()) },
})

r, err := grammar.To[Range](Pay.Parse("£45k - £55,000 per annum", acts))
fmt.Printf("%+v %v\n", r, err)

r, err = grammar.To[Range](Pay.Parse("£18.50", acts))
fmt.Printf("%+v %v\n", r, err)

_, err = grammar.To[Range](Pay.Parse("negotiable", acts))
fmt.Println(err, errors.Is(err, grammar.ErrNoMatch))
```

```
{Min:45000 Max:55000 Period:per annum} <nil>
{Min:18.5 Max:0 Period:} <nil>
grammar: no match true
```

#### `Grammar.ParseErr`

ParseErr is Parse, but when the text doesn't match it also returns a *ParseError, saying where the parse got furthest and what it expected there.

ParseErr says why a parse failed: where in the text the grammar got furthest, and what it expected there. A rule that couldn't start there is named, \<money> here; one that got part of the way is reported by what it needed next. That's usually exactly where the mistake is, as everything before it matched.

```go
for _, text := range []string{
	"£45k - £55,000 per annum",
	"£45k - 55,000",       // the second figure has no £
	"£45k - £55,00",       // a thousands group cut short
	"£45k - £55k pa, DOE", // more after the pay
} {
	m, err := Pay.ParseErr(text)
	if err != nil {
		fmt.Println(err)
		continue
	}
	fmt.Println("ok:", m.At("~"))
}
```

```
ok: £45k - £55,000 per annum
grammar Pay: line 1, column 8: expected <money>, found "55,000"
grammar Pay: line 1, column 14: expected \d, found end of text
grammar Pay: line 1, column 15: expected end of text, found ", DOE"
```

#### `Grammar.SubparseErr`

SubparseErr is Subparse, with a *ParseError when the text doesn't match, as ParseErr has.

SubparseErr is ParseErr for Subparse: the text must match from its start, but needn't match to its end.

```go
_, err := Pay.SubparseErr("Salary: £45k")
fmt.Println(err)
```

```
grammar Pay: line 1, column 1: expected <money>, found "Salary: £45k"
```

#### `ParseError.Error`

A ParseError's fields give the same facts as its message, for a program to use: to log which rule postings fail at, say, or to underline the place in an editor.

```go
_, err := Pay.ParseErr("£45k -\n55k")
var pe *grammar.ParseError
if errors.As(err, &pe) {
	fmt.Println(pe.Line, pe.Column, pe.Expected, pe.Found)
	fmt.Println(pe)
}
```

```
2 1 [<money>] 55k
grammar Pay: line 2, column 1: expected <money>, found "55k"
```

#### `DepthLimitError.Error`

Matching recurses as it goes, a level or so for each item matched, and past DepthLimit it stops with a \*DepthLimitError, which can be recovered, rather than running out of stack, which can't: Go would end the program. ParseErr returns it. Here the limit is low, to show it.

```go
g := grammar.MustCompile(`grammar Long { token TOP { <[a..z]>* } }`)
defer func(old int) { grammar.DepthLimit = old }(grammar.DepthLimit)
grammar.DepthLimit = 1_000

_, err := g.ParseErr(strings.Repeat("a", 600))
fmt.Println(err)
```

```
grammar Long: match too deep: gave up after 1000 levels (see DepthLimit)
```

#### `Backref`

Backref is $<name>, or $0 for the first positional capture: it matches the text the capture last matched, as Raku's does: a separator used again, as in \d+ $<sep>=<[,.]> \d ** 3 - \d+ $<sep> \d ** 3, or a closing quote.

Backref is $\<name>: the text a capture matched, again. Here a pay range must use the same thousands separator in both figures, so a range that mixes them, likely two figures run together, isn't read as one.

```go
g := grammar.MustCompile(`
grammar SameSep {
    token TOP { \d+ $<sep>=<[,.]> \d ** 3 \s* '-' \s* \d+ $<sep> \d ** 3 }
}`)
for _, text := range []string{"45,000 - 55,000", "45,000 - 55.000"} {
	fmt.Printf("%-16q %v\n", text, g.Parse(text).Defined())
}
```

```
"45,000 - 55,000" true
"45,000 - 55.000" false
```

#### `Caps`

Caps iterates over m's captures in the order they come in the text, as Raku's .caps does: each capture's name and match.

Caps gives a match's captures in the order they come in the text, as Raku's .caps does. A match is a hash, which keeps no order between names; here, the qualifiers of a title are captured by kind, and Caps puts them back in the order the title has them.

```go
g := grammar.MustCompile(`
grammar Title {
    token TOP         { $<title>=[ <-[(]>+? ] ' (' [ <arrangement> | <contract> ]+ % ', ' ')' }
    token arrangement { :i remote | hybrid | onsite }
    token contract    { :i contract | permanent | temp }
}`)
m := g.Parse("Senior Engineer (Contract, Remote, Temp)")
for name, c := range grammar.Caps(m) {
	fmt.Printf("%-11s %s\n", name, c.At("~"))
}
```

```
title       Senior Engineer
contract    Contract
arrangement Remote
contract    Temp
```

#### `Chunks`

Chunks iterates over m in pieces, as Raku's .chunks does: its captures, in Caps' order, with the text between them under "~", as a string.

Chunks is Caps with the text between the captures, under "~", so the chunks put the text back together: here, to mark up the parts of a pay string.

```go
m := Pay.Parse("£45k - £55k per annum")
var b strings.Builder
for name, c := range grammar.Chunks(m) {
	switch name {
	case "~":
		b.WriteString(c.Str())
	case "money":
		// money is min and max again, under a second name
	default:
		fmt.Fprintf(&b, "[%s: %s]", name, c.At("~"))
	}
}
fmt.Println(b.String())
```

```
[min: £45k] - [max: £55k][period:  per annum]
```

#### `StepLimit`

StepLimit stops a match that backtracks badly. Nested repeats in a regex rule, as here, try every way of sharing the a's out before giving up, so each extra a doubles the time: 30 of them would take minutes. The default limit stops it in about a second; this one is lower.

```go
g := grammar.MustCompile(`grammar Blowup { regex TOP { [ a+ ]+ $ } }`)
defer func(old int) { grammar.StepLimit = old }(grammar.StepLimit)
grammar.StepLimit = 100_000

_, err := g.ParseErr(strings.Repeat("a", 30) + "b")
fmt.Println(err)
```

```
grammar Blowup: match too long: gave up after 100000 steps (see StepLimit)
```

#### `StepLimitError.Error`

Parse, Find and the rest panic with a \*StepLimitError, as scalar's regexes panic past scalar.RegexTimeout. A program that reads texts it can't trust can recover it, and go on to the next text.

```go
g := grammar.MustCompile(`grammar Blowup { regex TOP { [ a+ ]+ $ } }`)
defer func(old int) { grammar.StepLimit = old }(grammar.StepLimit)
grammar.StepLimit = 100_000

find := func(text string) (m *scalar.SV, err error) {
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*grammar.StepLimitError)
			if !ok {
				panic(r) // some other problem
			}
			m, err = &scalar.SV{}, e
		}
	}()
	return g.Find(text), nil
}
for _, text := range []string{"aaa", strings.Repeat("a", 30) + "b"} {
	m, err := find(text)
	fmt.Printf("%q %v\n", m.At("~"), err)
}
```

```
"aaa" <nil>
"" grammar Blowup: match too long: gave up after 100000 steps (see StepLimit)
```

#### `Grammar.Split`

Split returns the text between the grammar's matches, with each match's text in place between them, as Raku's split with :v does: "a, b" split at commas is "a", ", ", "b".

Split gives the text between matches with the matches in place, as Raku's split with :v does: here, a paragraph split at headings run into it.

```go
g := grammar.MustCompile(`
grammar Inline {
    token TOP { <?after '.' \s+> [ 'What you will do' | 'How to apply' ] }
}`)
for _, piece := range g.Split("We're hiring. What you will do Lead a team. How to apply Email us.").All() {
	fmt.Printf("%q\n", piece)
}
```

```
"We're hiring. "
"What you will do"
" Lead a team. "
"How to apply"
" Email us."
```

### Reference: Building rules in Go

#### `Lit`

Lit matches s exactly, or ignoring case inside IgnoreCase.

Lit matches text exactly. In Raku syntax, letters and digits are literal as they stand, and anything else is quoted: '£', or "a\\tb" for escapes.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit)))
// Raku: token TOP { '£' \d+ }

fmt.Println(g.Subparse("£450 per week").At("~"))
fmt.Println(g.Subparse("$450").Defined())
```

```
£450
false
```

#### `Class`

Class matches one character for which in returns true.

Class matches one character that a function accepts. Digit, Word, Space and AnyChar are the classes Raku writes \\d, \\w, \\s and . (any character, newlines too).

```go
upper := grammar.Class(unicode.IsUpper)
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Plus(upper), grammar.Lit("-"), grammar.Plus(grammar.Digit)))
// Raku, for ASCII capitals: token TOP { <[A..Z]>+ '-' \d+ }

fmt.Println(g.Parse("REF-12").Defined(), g.Parse("ref-12").Defined())

w := grammar.New("W").Token("TOP", grammar.Seq(grammar.Plus(grammar.Word), grammar.Space, grammar.AnyChar))
// Raku: token TOP { \w+ \s . }
fmt.Println(w.Parse("ab_1 !").Defined())
```

```
true false
true
```

#### `Re`

Re matches pattern, a Perl-style regex (as scalar's M takes), at the current position.

Re matches a Perl-style regex, the kind scalar's M takes, as one piece of a rule. It's for patterns you already have; whatever it matches first is final, as if it were a token. Raku syntax has no equivalent.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Re(`[A-Z]{2,4}`), grammar.Lit("-"), grammar.Capture(grammar.Re(`\d+`))))
fmt.Println(g.Parse("REF-1234").At("0.~"))
```

```
1234
```

#### `Seq`

Seq matches each node in turn.

Seq matches its parts one after another. In Raku syntax, that's just writing them in order.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Lit("per"), grammar.Plus(grammar.Space), grammar.Lit("hour")))
// Raku: token TOP { per \s+ hour }

fmt.Println(g.Parse("per   hour").Defined())
```

```
true
```

#### `Alt`

Alt is Raku's |: the branch that matches the most text wins, and the earlier branch wins a tie.

Alt is Raku's |: of the alternatives that match, the one that matches the most text wins, whatever order they're written in. So hr, hour and hours can be listed in any order.

```go
g := grammar.New("G").Token("TOP", grammar.Alt(grammar.Lit("hr"), grammar.Lit("hour"), grammar.Lit("hours")))
// Raku: token TOP { [ hr | hour | hours ] }

fmt.Println(g.Subparse("hours a week").At("~"))
```

```
hours
```

#### `First`

First is Raku's ||: the first branch that matches wins.

First is Raku's ||: the first alternative that matches wins, even if a later one would match more. In a token, that choice is final, so here Parse fails on "hours": First chose "hour", leaving the "s" over.

```go
g := grammar.New("G").Token("TOP", grammar.First(grammar.Lit("hour"), grammar.Lit("hours")))
// Raku: token TOP { [ hour || hours ] }

fmt.Println(g.Subparse("hours").At("~"))
fmt.Println(g.Parse("hours").Defined())
```

```
hour
false
```

#### `Goal`

Goal is Raku's goal matching, open ~ close inner: open, then inner, then close, as Seq(open, inner, close) is, written with the brackets together, as in '(' ~ ')' <qualifiers>.

Goal is Raku's goal matching, '(' ~ ')' x: x between brackets, written with the brackets together. It matches as Seq(open, x, close) does; what it adds is that ParseErr names the opening bracket a missing closer was for, and where it was.

```go
quals := grammar.Seq(grammar.Plus(grammar.Class(unicode.IsLetter)),
	grammar.Star(grammar.Seq(grammar.Lit(", "), grammar.Plus(grammar.Class(unicode.IsLetter)))))
g := grammar.New("Quals").Token("TOP", grammar.Goal(grammar.Lit("("), grammar.Lit(")"), quals))
// Raku: token TOP { '(' ~ ')' [ <:L>+ ]+ % ', ' }

fmt.Println(g.Parse("(Remote, FTC)").At("~"))
_, err := g.ParseErr("(Remote, FTC")
fmt.Println(err)
```

```
(Remote, FTC)
grammar Quals: line 1, column 13: expected ', ' or ')' to close '(' from line 1, column 1, found end of text
```

#### `Conj`

Conj is Raku's & and &&: every node matches, over the same text, as in [ <:Lu> \S* ]+ % ' ' & @titles, a list phrase in title case.

Conj is & and &&: every node matches, over the same text. Here, an acronym from a list, but only where it's written in capitals.

```go
acronyms := grammar.NewWordList("acronyms", grammar.FoldCase(), grammar.WholeWords()).Add("SQL").Add("AWS")
g := grammar.New("Acronym").Token("TOP",
	grammar.Conj(grammar.Plus(grammar.Class(unicode.IsUpper)), grammar.Words(acronyms)))
// Raku: token TOP { <:Lu>+ & @acronyms }

fmt.Println(g.FindAll("SQL, sql, AWS and SQLITE").Map(func(m *scalar.SV) any { return m.At("~") }).Join(" "))
```

```
SQL AWS
```

#### `Sym`

Sym is <sym>: in a rule named name:sym<…>, it matches the text between the angle brackets, and captures it as sym, as Raku's does.

Proto and Sym are proto token and \<sym> built in Go: variants are rules named name:sym\<…>, and Sym matches that name, captured as sym.

```go
g := grammar.New("Units").
	Token("TOP", grammar.Plus(grammar.Call("unit")).Sep(grammar.Lit(", "))).
	Proto("unit").
	Token("unit:sym<year>", grammar.Seq(grammar.Sym(), grammar.Opt(grammar.Lit("s")))).
	Token("unit:sym<week>", grammar.Seq(grammar.Sym(), grammar.Opt(grammar.Lit("s"))))
// Raku: token TOP { <unit>+ % ', ' }; proto token unit {*}
//       token unit:sym<year> { <sym> s? }; token unit:sym<week> { <sym> s? }

m := g.Parse("years, week")
fmt.Println(m.At("unit.0.sym.~"), m.At("unit.1.sym.~"))
```

```
year week
```

#### `Opt`

Opt is n?.

Opt is ?: zero or one.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit), grammar.Opt(grammar.Lit("k"))))
// Raku: token TOP { '£' \d+ k? }

fmt.Println(g.Parse("£45").Defined(), g.Parse("£45k").Defined())
```

```
true true
```

#### `Star`

Star is n*.

Star is \*: zero or more, as many as possible.

```go
thousands := grammar.Seq(grammar.Lit(","), grammar.Repeat(grammar.Digit, 3, 3))
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Repeat(grammar.Digit, 1, 3), grammar.Star(thousands)))
// Raku: token TOP { \d ** 1..3 [ ',' \d ** 3 ]* }

fmt.Println(g.Parse("45").Defined(), g.Parse("45,000,000").Defined(), g.Parse("45,00").Defined())
```

```
true true false
```

#### `Plus`

Plus is n+.

Plus is +: one or more, as many as possible.

```go
g := grammar.New("G").Token("TOP", grammar.Plus(grammar.Digit))
// Raku: token TOP { \d+ }

fmt.Println(g.Subparse("2024 onwards").At("~"))
fmt.Println(g.Parse("").Defined())
```

```
2024
false
```

#### `Repeat`

Repeat is n ** min..max; a negative max is no limit, as in ** 2..*.

Repeat is \*\*: between min and max times, with -1 for no limit. In Raku, \\d \*\* 4 is exactly four, \\d \*\* 1..3 one to three, \\d \*\* 2..\* two or more.

```go
g := grammar.New("G").Token("TOP", grammar.Repeat(grammar.Digit, 1, 3))
// Raku: token TOP { \d ** 1..3 }

fmt.Println(g.Subparse("12345").At("~"))
```

```
123
```

#### `Quant.Sep`

Sep returns q with sep between items, as % does.

Sep puts a separator between repeats, as Raku's % does: \<tag>+ % ',' is one or more tags with commas between. There mustn't be a comma at the end; SepTrailing allows one.

```go
g := grammar.New("G").
	Token("TOP", grammar.Plus(grammar.Call("tag")).Sep(grammar.Seq(grammar.Lit(","), grammar.Star(grammar.Space)))).
	Token("tag", grammar.Plus(grammar.Word))
// Raku: token TOP { <tag>+ % [ ',' \s* ] }; token tag { \w+ }

fmt.Println(g.Parse("go, sql, aws").At("tag").AV().Length())
fmt.Println(g.Parse("go, sql,").Defined())
```

```
3
false
```

#### `Quant.SepTrailing`

SepTrailing returns q with sep between items and optionally after the last, as %% does.

SepTrailing is Raku's %%: separators between repeats, and optionally one at the end.

```go
g := grammar.New("G").
	Token("TOP", grammar.Plus(grammar.Call("tag")).SepTrailing(grammar.Lit(","))).
	Token("tag", grammar.Plus(grammar.Word))
// Raku: token TOP { <tag>+ %% ',' }; token tag { \w+ }

fmt.Println(g.Parse("go,sql,").At("tag").AV().Length())
```

```
2
```

#### `Quant.Frugal`

Frugal returns q matching as few items as it can, as *? and +? do.

Frugal makes a quantifier match as little as it can, as \*? and +? do in Raku and Perl, taking more only when what follows fails. Here the frugal .\*? stops at the first " - "; a greedy .\* would run to the last.

In a token, where nothing else gives back what it matched, a frugal quantifier still takes more when it needs to, so .\*? is how a token skips ahead. That works through \[ ... ] and $\<name>=\[ ... ], but a capture ( ... ) commits to its first match, as in Raku: (.\*?) in a token matches nothing, and stays that way. Name the text with $\<name>=\[ .\*? ] instead, or use a regex.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(
	grammar.Named("role", grammar.Star(grammar.AnyChar).Frugal()),
	grammar.Lit(" - "),
	grammar.Named("rest", grammar.Star(grammar.AnyChar)),
))
// Raku: token TOP { $<role>=[.*?] ' - ' $<rest>=[.*] }

m := g.Parse("Engineer - Payments - Remote")
fmt.Printf("%q %q\n", m.At("role.~"), m.At("rest.~"))

capture := grammar.New("G").Token("TOP", grammar.Seq(
	grammar.Capture(grammar.Star(grammar.AnyChar).Frugal()), grammar.Lit(" - "), grammar.Capture(grammar.Star(grammar.AnyChar))))
// Raku: token TOP { (.*?) ' - ' (.*) }
fmt.Println(capture.Parse("Engineer - Payments - Remote").Defined())
```

```
"Engineer" "Payments - Remote"
false
```

#### `Call`

Call is <name>: it matches the rule name and captures it under that name.

Call matches another rule and captures its match under the rule's name: \<num> in Raku syntax.

```go
g := grammar.New("G").
	Token("TOP", grammar.Seq(grammar.Call("num"), grammar.Lit("%"))).
	Token("num", grammar.Plus(grammar.Digit))
// Raku: token TOP { <num> '%' }; token num { \d+ }

fmt.Println(g.Parse("10%").At("num.~"))
```

```
10
```

#### `Silent`

Silent is <.name>: it matches the rule name without capturing it.

Silent matches another rule without capturing it: \<.name> in Raku syntax. Use it for rules that only hold the text together, such as separators and whitespace.

```go
g := grammar.New("G").
	Token("TOP", grammar.Seq(grammar.Call("num"), grammar.Silent("ws"), grammar.Lit("days"))).
	Token("num", grammar.Plus(grammar.Digit))
// Raku: token TOP { <num> <.ws> days }

m := g.Parse("25 days")
fmt.Println(m.At("num.~"), m.At("ws").Defined())
```

```
25 false
```

#### `Alias`

Alias is <alias=name>.

Alias captures a rule's match under another name too: \<min=money> in Raku syntax. As in Raku, the match is under both names, so here it's under min and money. Two calls of money make money an array.

```go
g := grammar.New("G").
	Token("TOP", grammar.Seq(grammar.Alias("min", "money"), grammar.Lit("-"), grammar.Alias("max", "money"))).
	Token("money", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit)))
// Raku: token TOP { <min=money> '-' <max=money> }

m := g.Parse("£30-£35")
fmt.Println(m.At("min.~"), m.At("max.~"), m.At("money").AV().Length())
```

```
£30 £35 2
```

#### `AliasOnly`

AliasOnly is <alias=.name>, captured only as alias.

AliasOnly captures a rule's match under another name only: \<min=.money> in Raku syntax.

```go
g := grammar.New("G").
	Token("TOP", grammar.Seq(grammar.AliasOnly("min", "money"), grammar.Lit("+"))).
	Token("money", grammar.Seq(grammar.Lit("£"), grammar.Plus(grammar.Digit)))
// Raku: token TOP { <min=.money> '+' }

m := g.Parse("£700+")
fmt.Println(m.At("min.~"), m.At("money").Defined())
```

```
£700 false
```

#### `Capture`

Capture is ( n ): a positional capture, numbered from 0 in its scope.

Capture is ( ... ): a capture by position, numbered from 0 in the order the groups are written, and read with At("0"), At("1") and so on.

```go
digits := func(n int) grammar.Node { return grammar.Repeat(grammar.Digit, n, n) }
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Capture(digits(4)), grammar.Lit("-"), grammar.Capture(digits(2))))
// Raku: token TOP { (\d ** 4) '-' (\d ** 2) }

m := g.Parse("2026-10")
fmt.Println(m.At("0.~"), m.At("1.~"))
```

```
2026 10
```

#### `Named`

Named is $<name>=n.

Named captures the text of part of a rule under a name: $\<year>=\[ ... ] in Raku syntax. Named(name, Capture(...)), $\<name>=( ... ), captures with a scope of its own, so positional captures inside it are its own.

```go
digits := func(n int) grammar.Node { return grammar.Repeat(grammar.Digit, n, n) }
g := grammar.New("G").Token("TOP", grammar.Seq(
	grammar.Named("year", digits(4)),
	grammar.Lit("-"),
	grammar.Named("rest", grammar.Capture(grammar.Seq(grammar.Capture(digits(2)), grammar.Lit("-"), grammar.Capture(digits(2))))),
))
// Raku: token TOP { $<year>=[ \d ** 4 ] '-' $<rest>=( (\d ** 2) '-' (\d ** 2) ) }

m := g.Parse("2026-10-15")
fmt.Println(m.At("year.~"), m.At("rest.~"), m.At("rest.0.~"), m.At("rest.1.~"))
```

```
2026 10-15 10 15
```

#### `Before`

Before is <?before n>, NotBefore <!before n>, After <?after n> and NotAfter <!after n>.

Before checks what comes next without matching it: \<?before ...> in Raku syntax. Here the digits must be followed by k, but the k isn't part of the match.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Plus(grammar.Digit), grammar.Before(grammar.Lit("k"))))
// Raku: token TOP { \d+ <?before k> }

fmt.Println(g.Subparse("45k").At("~"))
```

```
45
```

#### `NotBefore`

NotBefore checks that something doesn't come next: \<!before ...> in Raku syntax. Here it skips a percentage to find a plain number.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Plus(grammar.Digit), grammar.NotBefore(grammar.Lit("%"))))
// Raku: token TOP { \d+ <!before '%'> }

fmt.Println(g.Find("10% bonus and 25 days' holiday").At("~"))
```

```
25
```

#### `After`

After checks what came just before: \<?after ...> in Raku syntax.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.After(grammar.Lit("£")), grammar.Plus(grammar.Digit)))
// Raku: token TOP { <?after '£'> \d+ }

fmt.Println(g.Find("Ref 123, pays £45").At("~"))
```

```
45
```

#### `NotAfter`

NotAfter checks that something didn't come just before: \<!after ...> in Raku syntax. Here it finds a number that isn't money; WordStart stops it matching the 5 of £45.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.NotAfter(grammar.Lit("£")), grammar.WordStart, grammar.Plus(grammar.Digit)))
// Raku: token TOP { <!after '£'> << \d+ }

fmt.Println(g.Find("£45 for 3 days").At("~"))
```

```
3
```

#### `IgnoreCase`

IgnoreCase is :i over n.

IgnoreCase is :i: letters match in either case, for the part of the rule it covers. In Raku syntax, :i applies to the rest of the group it's in: \[:i per annum] or, at the start of a rule, all of it.

```go
g := grammar.New("G").Token("TOP", grammar.IgnoreCase(grammar.Lit("per annum")))
// Raku: token TOP { :i 'per annum' }

fmt.Println(g.Parse("PER ANNUM").Defined(), g.Parse("Per Annum").Defined())
```

```
true true
```

#### `Ratchet`

Ratchet is :r over n, or :!r when on is false.

Ratchet turns backtracking off (:r) or on (:!r) for part of a rule. A token is :r throughout, so Ratchet(..., false), :!r in Raku syntax, lets one part of a token backtrack, as a regex would.

```go
g := grammar.New("G").Token("TOP", grammar.Ratchet(grammar.Seq(grammar.Star(grammar.Digit), grammar.Lit("5")), false))
// Raku: token TOP { :!r \d* 5 }

fmt.Println(g.Parse("12345").Defined())
```

```
true
```

#### `Start`

Start and End match at the start and end of the text, as ^ and $ do in Raku. LineStart and LineEnd, ^^ and $$, match at the start and end of any line.

```go
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.Start, grammar.Plus(grammar.Digit)))
// Raku: token TOP { ^ \d+ }

fmt.Println(g.Find("12 days").Defined(), g.Find("Day 12").Defined())
```

```
true false
```

#### `LineStart`

LineStart is ^^: the start of any line. Here it finds the bulleted lines of a description, and only those.

```go
line := grammar.Plus(grammar.Class(func(r rune) bool { return r != '\n' }))
g := grammar.New("G").Token("TOP", grammar.Seq(grammar.LineStart, grammar.Lit("- "), grammar.Capture(line)))
// Raku: token TOP { ^^ '- ' (\N+) }

text := "Benefits:\n- 25 days' holiday\n- Pension - 5%\nApply now"
for _, m := range g.FindAll(text).All() {
	fmt.Println(m.At("0.~"))
}
```

```
25 days' holiday
Pension - 5%
```

#### `WordStart`

WordStart and WordEnd are \<\< and >>: where a word starts and ends. Without them, "pa" is found inside "company".

```go
bare := grammar.New("G").Token("TOP", grammar.Lit("pa"))
word := grammar.New("G").Token("TOP", grammar.Seq(grammar.WordStart, grammar.Lit("pa"), grammar.WordEnd))
// Raku: token TOP { << pa >> }

text := "a company that pays 30k pa"
fmt.Println(bare.Find(text).At("from"), word.Find(text).At("from"))
```

```
5 24
```

### Reference: Word lists

#### `NewWordList`

NewWordList returns an empty list called name, which is how Raku syntax refers to it: @name.

A word list holds words and phrases, each with a canonical value, so that synonyms match as one thing. Pass it to Compile and match it with @name; a rule that's nothing but the list gets the canonical value of what it matched as its made value.

```go
titles := grammar.NewWordList("titles")
titles.Add("Software Engineer", "Software Developer", "SWE")
titles.Add("Data Engineer", "Data Platform Engineer")

g := grammar.MustCompile(`
grammar Title {
    token TOP   { <title> .* }      # a title, then anything
    token title { @titles }
}`, titles)

for _, text := range []string{"SWE (Payments)", "Data Platform Engineer, Remote"} {
	fmt.Println(g.Parse(text).At("title.made"))
}
```

```
Software Engineer
Data Engineer
```

#### `Words`

Words matches the longest phrase in l, as @name does.

Words is @name for grammars built in Go.

```go
titles := grammar.NewWordList("titles").Add("Software Engineer", "SWE")
g := grammar.New("Title").
	Token("TOP", grammar.Seq(grammar.Call("title"), grammar.Star(grammar.AnyChar))).
	Token("title", grammar.Words(titles))

fmt.Println(g.Parse("SWE II").At("title.made"))
```

```
Software Engineer
```

#### `Words.longestAndBoundaries`

A list matches the longest phrase it can, whatever order they were added in, and like Raku's, never gives back part of it. It doesn't look at word boundaries: "Java" matches the start of "JavaScript", unless the rule says the word must end there, with >>.

```go
langs := grammar.NewWordList("langs").Add("Java").Add("JavaScript")
longest := grammar.MustCompile(`grammar L { token TOP { @langs } }`, langs)
fmt.Println(longest.Subparse("JavaScript developer").At("~"))

java := grammar.NewWordList("java").Add("Java")
loose := grammar.MustCompile(`grammar L2 { token TOP { @java } }`, java)
whole := grammar.MustCompile(`grammar L3 { token TOP { @java >> } }`, java)
fmt.Println(loose.Subparse("JavaScript").Defined(), whole.Subparse("JavaScript").Defined())
```

```
JavaScript
true false
```

#### `WordList.Add`

Add adds canonical and each of synonyms to the list, all meaning canonical.

Add adds a canonical value and its synonyms. The canonical value is a phrase in the list too. Add returns the list, so calls can be chained.

```go
units := grammar.NewWordList("units").
	Add("Year", "annum", "pa").
	Add("Week", "wk")

fmt.Println(units.Canonical("annum"), units.Canonical("Year"), units.Canonical("wk"))
```

```
[Year] [Year] [Week]
```

#### `WordList.AddRule`

AddRule adds one synonym rule in the format Elasticsearch and Solr use:

AddRule takes one synonym rule in the format Elasticsearch and Solr use. A list of phrases means its first; phrases => phrases means the right side; a backslash escapes a comma or =>.

```go
l := grammar.NewWordList("units")
for _, rule := range []string{
	"Year, annum, yr",       // annum and yr mean Year
	"hr, ph => Hour",        // hr and ph mean Hour, as does Hour
	`£\, per annum => Year`, // the phrase "£, per annum"
} {
	if err := l.AddRule(rule); err != nil {
		fmt.Println(err)
	}
}
fmt.Println(l.Canonical("yr"), l.Canonical("ph"), l.Canonical("£, per annum"))

fmt.Println(l.AddRule("a => b => c"))
```

```
[Year] [Hour] [Year]
grammar: synonym rule "a => b => c" has more than one =>
```

#### `WordList.AddRules`

AddRules adds each line of rules as a rule, as AddRule does, and reports the first bad one.

AddRules takes many rules, one per line, as Elasticsearch's synonyms API returns them. Blank lines and lines starting with # are skipped.

```go
units := grammar.NewWordList("units", grammar.FoldCase())
err := units.AddRules(`
# periods
Year, annum, yr, pa, p.a.
Week, wk, pw
hour, hr, ph => Hour
`)
if err != nil {
	fmt.Println(err)
	return
}
g := grammar.MustCompile(`
grammar Rate {
    token TOP  { '£' $<amount>=[ \d+ ] \s* [ per | a | '/' ]? \s* <unit> }
    token unit { @units }
}`, units)

for _, text := range []string{"£450 per WK", "£18/hr", "£40000 p.a."} {
	m := g.Parse(text)
	fmt.Println(m.At("amount.~"), m.At("unit.made"))
}
```

```
450 Week
18 Hour
40000 Year
```

#### `WordList.Canonical`

Canonical returns what phrase means: its canonical values, in the order they were added, or nil if it isn't in the list.

Canonical says what a phrase means: everything it means, in the order added, or nothing if it isn't in the list. It reads the phrase as the list matches text, so with FoldCase and LooseSpacing it ignores case and spacing too.

```go
titles := grammar.NewWordList("titles", grammar.FoldCase(), grammar.LooseSpacing()).
	Add("Front-end Developer", "Frontend Developer")

fmt.Println(titles.Canonical("FRONT END developer"))
fmt.Println(titles.Canonical("Back-end Developer") == nil)
```

```
[Front-end Developer]
true
```

#### `WordList.Ambiguous`

Ambiguous returns the phrases that mean more than one thing, with what each means.

Ambiguous lists the phrases that mean more than one thing. A match of one has the first meaning as its made value; Canonical gives the rest, for the caller to choose from context, or to pass on to something that can.

```go
titles := grammar.NewWordList("titles", grammar.FoldCase())
titles.Add("Product Manager", "PM")
titles.Add("Project Manager", "PM")

fmt.Println(titles.Ambiguous())

g := grammar.MustCompile(`grammar T { token TOP { <title> }; token title { @titles } }`, titles)
fmt.Println(g.Parse("PM").At("title.made"))
```

```
map[pm:[Product Manager Project Manager]]
Product Manager
```

#### `WordList.Phrases`

Phrases returns the list's phrases, normalised, in sorted order, without the plurals that Plurals made.

Phrases lists every phrase, as the list stores it: case folded with FoldCase, spacing collapsed with LooseSpacing.

```go
l := grammar.NewWordList("units", grammar.FoldCase()).Add("Year", "Annum", "p.a.")
fmt.Println(l.Phrases())
```

```
[annum p.a. year]
```

#### `WordList.Len`

Len returns how many distinct phrases the list holds, not counting plurals that Plurals made.

Len counts distinct phrases: a synonym added twice counts once.

```go
l := grammar.NewWordList("units").Add("Year", "annum").Add("Year", "annum", "yr")
fmt.Println(l.Len())
```

```
3
```

#### `WordList.Name`

Name returns the list's name.

Name is how Raku syntax refers to the list: @units.

```go
fmt.Println(grammar.NewWordList("units").Name())
```

```
units
```

#### `FoldCase`

FoldCase matches a list's phrases ignoring case, wherever it's used.

FoldCase makes a list ignore case wherever it's used. Without it, a list ignores case only where the rule says :i, as in Raku.

```go
exact := grammar.NewWordList("exact").Add("Senior")
folded := grammar.NewWordList("folded", grammar.FoldCase()).Add("Senior")
g := grammar.MustCompile(`
grammar Seniority {
    token TOP    { @exact }
    token folded { @folded }
    token ci     { :i @exact }
}`, exact, folded)

fmt.Println(g.Parse("SENIOR").Defined())
fmt.Println(g.Parse("SENIOR", grammar.Rule("folded")).Defined())
fmt.Println(g.Parse("SENIOR", grammar.Rule("ci")).Defined())
```

```
false
true
true
```

#### `LooseSpacing`

LooseSpacing matches any run of spaces, hyphens, dashes and underscores in the text with any such run in a phrase, so "front-end" in the list matches "front end" and "front – end" in the text.

LooseSpacing makes any run of spaces, hyphens, dashes and underscores match any other, as job titles are written every which way. There must still be some spacing: "frontend" is a different phrase from "front-end", unless the list has JoinedSpacing too.

```go
l := grammar.NewWordList("titles", grammar.LooseSpacing()).Add("front-end developer")
g := grammar.MustCompile(`grammar T { token TOP { @titles } }`, l)

for _, text := range []string{"front end developer", "front – end developer", "front_end  developer", "frontend developer"} {
	fmt.Println(g.Parse(text).Defined())
}
```

```
true
true
true
false
```

#### `JoinedSpacing`

JoinedSpacing lets the text leave out the spacing in a phrase, so "on site" in the list matches "onsite" too, and "front-end" matches "frontend".

JoinedSpacing lets the text leave out a phrase's spacing, as postings write "on site", "on-site" and "onsite" for the same thing. Write the spaced form: there's no telling where a space would go in "onsite". With LooseSpacing too, one phrase covers every spelling, and the canonical value is the same for each.

```go
places := grammar.NewWordList("places", grammar.FoldCase(), grammar.LooseSpacing(),
	grammar.JoinedSpacing(), grammar.WholeWords()).
	Add("On-site", "on site", "office based")
g := grammar.MustCompile(`grammar P { token TOP { @places } }`, places)

for _, text := range []string{"Onsite", "on-site", "On Site", "office-based", "Officebased", "onsiteness"} {
	fmt.Printf("%-14q %v\n", text, g.Parse(text).At("made").Or("-"))
}
fmt.Println(places.Canonical("officebased"))
```

```
"Onsite"       On-site
"on-site"      On-site
"On Site"      On-site
"office-based" On-site
"Officebased"  On-site
"onsiteness"   -
[On-site]
```

#### `Plurals`

Plurals lets each phrase match its plural too, as postings write "Care Assistants" for a list's "Care Assistant": the canonical value is the same.

Plurals lets each phrase match its plural, with the same canonical value. Only the last word changes, by the regular English rules; the plurals don't show in Phrases, which lists what was added.

```go
roles := grammar.NewWordList("roles", grammar.FoldCase(), grammar.WholeWords(), grammar.Plurals()).
	Add("Care Assistant", "carer").Add("Secretary").Add("Coach")
g := grammar.MustCompile(`grammar R { token TOP { @roles } }`, roles)

for _, text := range []string{"Care Assistants", "carers", "secretaries", "coaches", "Coachs"} {
	fmt.Printf("%-18q %v\n", text, g.Parse(text).At("made").Or("-"))
}
fmt.Println(roles.Phrases())
```

```
"Care Assistants"  Care Assistant
"carers"           Care Assistant
"secretaries"      Secretary
"coaches"          Coach
"Coachs"           -
[care assistant carer coach secretary]
```

#### `WholeWords`

WholeWords matches a phrase only where it isn't part of a longer word: "java" doesn't match the start of "JavaScript", nor "lead" the start of "leadership".

WholeWords matches a phrase only where it isn't part of a longer word, which \<\< @list >> would do, without having to write the boundaries.

```go
levels := grammar.NewWordList("levels", grammar.FoldCase(), grammar.WholeWords()).
	Add("Lead", "team lead", "team leader").Add("Senior", "snr")
g := grammar.MustCompile(`grammar L { token TOP { @levels } }`, levels)

for _, title := range []string{"Team Leader (Care)", "Snr Engineer", "Leadership Trainer", "SNR-level role"} {
	fmt.Printf("%-20q %v\n", title, g.Find(title).At("made").Or("-"))
}
```

```
"Team Leader (Care)" Lead
"Snr Engineer"       Senior
"Leadership Trainer" -
"SNR-level role"     Senior
```

#### `WordLists`

WordLists makes a word list of each set of synonym rules in lists, named by its key and read as AddRules reads them, with opts for every list.

WordLists makes a list for each set of synonym rules, ready for Compile: one call instead of a loop per list. A rule ending with a comma carries on onto the next line.

```go
uses, err := grammar.WordLists(map[string]string{
	"units": `Year, annum, yr, pa,
	          p.a., per annum
	          Week, wk`,
	"levels": "Senior, snr, sr\nJunior, jnr, graduate",
}, grammar.FoldCase())
if err != nil {
	fmt.Println(err)
	return
}
g := grammar.MustCompile(`
grammar Rate {
    token TOP   { <level> \s+ <unit> }
    token level { @levels }
    token unit  { @units }
}`, uses...)
m := g.Parse("Snr per annum")
fmt.Println(m.At("level.made"), m.At("unit.made"))
```

```
Senior Year
```

#### `Named.wordList`

A named capture of a word list, $\<name>=@list, has the canonical value as its made value, so a list inside a bigger rule needs no rule of its own, and grammar.To can read it straight into a struct.

```go
levels := grammar.NewWordList("levels", grammar.FoldCase()).Add("Senior", "snr", "sr")
g := grammar.MustCompile(`grammar T { token TOP { $<level>=@levels? \s* $<role>=[ \N+ ] } }`, levels)

for _, title := range []string{"Snr Software Engineer", "Software Engineer"} {
	t, _ := grammar.To[struct{ Level, Role string }](g.Parse(title))
	fmt.Printf("%+v\n", t)
}
```

```
{Level:Senior Role:Software Engineer}
{Level: Role:Software Engineer}
```

#### `NewPatternList`

NewPatternList compiles patterns into a list called name, which is how Raku syntax refers to it: <@name>.

A pattern list holds patterns as data, each a regex in Raku syntax, for \<@name> to match, as Raku's \<@array> does: the longest match wins. Here, the ways a period is written, matched with their spacing and case.

```go
periods, err := grammar.NewPatternList("periods",
	`:i per \s* annum`, `:i 'p.a.'`, `:i pa >>`, `:i per \s* hour`)
if err != nil {
	fmt.Println(err)
	return
}
g := grammar.MustCompile(`grammar Pay { token TOP { '£' $<amount>=[ \d+ ] \s* $<period>=<@periods> } }`, periods)
for _, text := range []string{"£45000 per annum", "£45000 PA", "£18 per hour", "£45000 p.a."} {
	fmt.Println(g.Parse(text).At("period.~"))
}

_, err = grammar.NewPatternList("bad", `\d+ k`, `per [ annum`)
fmt.Println(err)
```

```
per annum
PA
per hour
p.a.
pattern list bad: pattern "per [ annum": grammar: line 1, column 12: expected ]
```

#### `MustPatternList`

MustPatternList is NewPatternList, panicking if a pattern doesn't compile.

MustPatternList is NewPatternList, panicking if a pattern doesn't compile, for lists written into the program.

```go
amounts := grammar.MustPatternList("amounts", `\d+ k`, `\d ** 1..3 [ ',' \d ** 3 ]+`, `\d+`)
g := grammar.MustCompile(`grammar A { token TOP { '£' <@amounts> } }`, amounts)
fmt.Println(g.FindAll("£45k, £45,000 or £45").Map(func(m *scalar.SV) any { return m.At("~") }).Join(" | "))
```

```
£45k | £45,000 | £45
```

#### `PatternList.Name`

Name returns the list's name.

Name is how Raku syntax refers to the list: \<@periods>.

```go
fmt.Println(grammar.MustPatternList("periods", `pa`).Name())
```

```
periods
```

<!-- reference:end -->
