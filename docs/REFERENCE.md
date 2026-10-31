# grammar: reference

What each part of [grammar](../README.md) does, feature by feature: how it
behaves, where it follows Raku and where it doesn't, how it's tested, and
what it doesn't do yet. For tasks, see the [how-to guide](HOWTO.md); for
one example of every function and method, see the [examples](EXAMPLES.md);
coming from Raku, see the [phrasebook](RAKU.md).

- [What you might assume](#what-you-might-assume)
- [Writing a grammar](#writing-a-grammar)
- [Word lists](#word-lists)
- [Pattern lists](#pattern-lists)
- [Roles](#roles)
- [The match](#the-match)
- [Parsing](#parsing)
- [Errors](#errors)
- [Differences from Raku](#differences-from-raku)
- [Testing against Raku](#testing-against-raku)
- [Gaps and possible additions](#gaps-and-possible-additions)

## What you might assume

Most people come to grammars knowing regular expressions, and a grammar
looks like one. It behaves like Raku's, which differs from a regex in ways
that catch nearly everyone once. Each row below behaves the same in
`grammar` and in Rakudo:

| You might expect | What happens | For what you expected |
|---|---|---|
| `token TOP { \d+ 5 }` matches "125", as a regex would | A `token` never backtracks: `\d+` takes "125", gives none of it back, and there's nothing left for `5`, so it doesn't match. That's what keeps tokens fast and predictable | Write a `regex`, which backtracks: `regex TOP { \d+ 5 }` matches "125". Or say exactly what you mean, as `\d ** 2 5` does |
| `a \| ab` tries `a` first, as in a regex | `\|` takes the alternative that matches the most text, so `a \| ab` matches "ab" | `\|\|` tries the alternatives in order and takes the first that matches. In a token it keeps it, so `token { a \|\| ab }` matches only the "a" of "ab" |
| A space in a pattern matches a space | In a `token` or a `regex`, spaces in the pattern mean nothing, so `token { a b }` matches "ab", not "a b". Rakudo warns about it | Use a `rule`, in which each space in the pattern matches any whitespace in the text (see [Spaces in rules](#spaces-in-rules)), or match it: `\s`, `' '` or `'a b'` |
| `Parse` finds the pattern in the text | `Parse` must match the whole text, as Raku's `.parse` does, and `Subparse` from its start: a `'£' \d+` grammar doesn't `Parse` "£45 pa" | `Find` and `FindAll` search the text, as a regex search does: `Find` finds "£45" in "Salary £45 pa" |
| A capture is one match | A capture that can match more than once, `<x>*`, `<x>+` or `<x>` written twice, is always an array, even of one match, or none. `<x>?` is one match, or missing | Read an array capture with `At("x").AV()`, or `To`, which fills a slice field from it |
| `'e'` matches the e of "é" typed as e plus an accent | Text is read in NFC, so e followed by a combining accent becomes the one character é, which `'e'` doesn't match. `from` and `to` count characters of the text as read, not bytes | Write the accented letter: a grammar's literals are read in NFC too. See [Accented letters](#accented-letters-text-is-read-in-nfc) |
| `.` stops at the end of a line | `.` matches any character, a newline included | `\N` matches any character but a newline |
| A literal ignores case, as the word lists do | `'Ab'` matches only "Ab". Word lists ignore case only with `FoldCase` | `:i` ignores case for the rest of the rule or group: `token { :i 'Ab' }` matches "ab" |

## Writing a grammar

A grammar is written in Raku's syntax and compiled with `Compile`, or
built in Go with the builder functions; both make the same grammar. The
tables below give each piece both ways, with what it means. If you know
regexes, read [What you might assume](#what-you-might-assume) first.

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
| `$<k>=[ … ]`, `$<k>=\d+` | `Named("k", x)` | `k`, the text only. A capture inside it belongs to the enclosing rule, as in Raku: in `$<title>=[ $<level>=@levels \s+ \w+ ]`, read `level.made`, beside `title`; `title.level.made` is undef. To keep it inside, use `$<k>=( … )` |
| `$<k>=[ … ]?` | | `k`, always: an empty match when the group didn't match, as in Raku, for the quantifier is inside the capture. For `k` to be there only when it matched, put the capture inside: `[ $<k>=[ … ] ]?` |
| `$<k>=( … )` | `Named("k", Capture(x))` | `k`, with captures of its own |
| `$<k>=( … )*`, `$<k>=<name>?`, `$<k>=@list+`, `$<k>=<@list>**2` | `Star(Named("k", Capture(x)))` and the like | `k`, each match of the group, rule or list, as in Raku: the quantifier repeats the named capture, so `k` is a list under `*`, `+` or `**`, and under `?` a match, or missing. Only `[ … ]` and plain atoms, as `\d+`, have the quantifier inside the capture |
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

The list may have lookarounds, anchors and the whitespace a `rule` matches
beside it, in the rule or the named capture, as nothing else there matches
text of its own: `token pattern { @patterns <!before ':'> }` won't take
"Contract:", and gets "temp to perm" for "Temp to Perm". The value is
looked up in what the list matched, not in the whole match, so
`rule pattern { @patterns }` gets it too, though its match takes in the
whitespace after. A rule with anything else that matches text, as in
`@patterns \s*`, or with two lists, has no value from them: give the list
a rule of its own, or a named capture.

**Loading synonyms.** `Add(canonical, synonyms...)` adds phrases one group
at a time, the canonical value among them. `AddAs(value, phrases...)` adds
phrases meaning value without adding value itself, for a value that's a
code rather than words: `AddAs("2134", "Software engineer")` matches
"Software engineer" but not "2134". `AddRule` and `AddRules` take rules in the format Elasticsearch
and Solr use, as Elasticsearch's synonyms API returns them:

```
annum, year, yr, pa        all mean the first, annum
hr, ph => Hour             each phrase on the left means Hour, as does Hour
pm => product manager, project manager
```

Each value on the right of `=>` is a phrase too, meaning itself, as in
Elasticsearch, where a text left as written still matches. With the
`MapOnly()` option, it isn't: a rule with `=>` adds its phrases as
`AddAs` does. That's for a value that names something else in the text:
with `legal notice, the legal bit => legal`, a heading "Legal Notice"
means legal, but "Legal Secretary" doesn't. Under `MapOnly`, a rule with
nothing before `=>` is an error, as it would add nothing.

In `AddRules`, a rule ending with a comma carries on onto the next line, so
long rules can be wrapped. `WordLists(map[string]string{...}, opts...)`
builds a list for each set of rules, keyed by name, ready to pass to
`Compile`: one call instead of a loop per list.

**Reading a list back.** `Phrases()` gives its phrases, sorted, and
`Len()` counts them. `Canonical(phrase)` gives what one phrase means.
`Canonicals()` gives the canonical values, each once, in the order they
were first added, however they were added, so the order of a rules file
can be used as data: which value wins when a text has phrases of two,
say, without reading the file a second time. `Ambiguous()` is below.
None of this is in Raku, whose `@list` is only its phrases.

**Matching options**, given to `NewWordList`:

| Option | Effect |
|---|---|
| `FoldCase()` | ignore case everywhere the list is used. Without it, the list ignores case only under `:i`, as in Raku. |
| `LooseSpacing()` | any run of spaces, hyphens, dashes and underscores matches any other, so "front-end" matches "front end". There must still be some spacing: "frontend" is a different phrase, unless the list has `JoinedSpacing()` too. |
| `JoinedSpacing()` | the text may leave out the spacing in a phrase, so "on site" matches "onsite" and "front-end" matches "frontend". Write the spaced form; see below. |
| `Plurals()` | each phrase matches its plural too, with the same canonical value: "Care Assistant" matches "Care Assistants". Regular English plurals of the last word only; see below. |
| `ExactAcronyms()` | keep a list's acronyms in capitals where it otherwise ignores case: "IT" doesn't match "it", though "computing" matches "Computing". An acronym is a word of two or more capitals, perhaps with a plural s ("NHS", "PMs"); the rest of a phrase still ignores case ("NHS trust" matches "NHS Trust", not "nhs trust"). |
| `MapOnly()` | a synonym rule written `phrases => values` doesn't make the values phrases too, as `AddAs` doesn't; see Loading synonyms, above. Rules without `=>`, and `Add` and `AddAs`, are unchanged. |
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

Reading a match creates nothing, as in Raku. A missing capture reads as
undef, so `m.At("period.~").Or("none")` gives "none" for pay with no
period, and `period` stays missing for `AtOK`, `Exists` and the JSON a
match writes. Elsewhere scalar autovivifies on a read, as Perl does, but
each match has its `Autovivify(false)`, after Perl's `no autovivification`.
A write still works, but there's no need to write into a match: `Values`
and `To` build new values from it.

### Checking paths: `CheckPath`

A misspelt capture reads as undef too, so `m.At("min.amont")` fails as
silently as a capture that didn't match. `CheckPath` says whether a path
can ever be defined in a match of the grammar, working from the grammar
alone, and if not, which step can't be there and what could:

```go
Pay.CheckPath("min.amount.num.~") // nil
Pay.CheckPath("min.amont")        // grammar Pay: path "min.amont": rule money has no capture "amont" (it has: amount)
Pay.CheckPath("money.amount")     // … money is a list of matches, as it can capture more than once: index it, as money[0]
```

It follows the match's shape, as above: a capture that can match more
than once is a list, to index, and every other one a single match; `~`,
`from` and `to` have nothing inside; a capture inside `$<k>=[ … ]` is
beside `k`, so it suggests the path that reads it; and a proto's capture
may be any variant's match. Any rule's match may have `made`, as an
action may make one, and what an action makes may be anything, so any
steps after `made` are taken. A path starts at `TOP`, or at the rule
`Rule(name)` names. A path it takes can still be undef for a given text,
as when an optional capture didn't match: it says only that some text
could define it. A malformed path is `scalar.CheckPath`'s error.

It's for tests, mostly: put the paths a program reads in a list, and
check each, so a typo, or a capture moved in the grammar, fails a test
rather than reading undef in production. Raku has nothing like it.

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

Case is folded a character at a time, and a few capitals with an accent
have no composed form where their lower case does: Y with a ring above is
two characters, `Y` and U+030A, and in lower case it's one, `ẙ`. So a word
list that ignores case, with `FoldCase` or under `:i`, compares letters
decomposed, and a phrase matches however it's spelt: `Y̊`, `ẙ` and `y` with
a ring are one phrase, as they are to Raku's `fc`. Its `Phrases` and
`Canonical` hold it in NFC. Text below U+00C0 has nothing to decompose and
takes the same step as before. A literal under `:i` still folds one
character at a time (see the differences table).

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
grammar left uncaptured. As in Raku, the text before a capture is what
lies between it and the end of the capture before, so inside a longer
capture, as with the captures within `$<k>=[ … ]`, the text between the
ones within it comes again.

Captures come by where they start, then where they end, so an empty
capture comes before a longer one at the same place, as it was made first.
Captures at the very same place, such as an alias and its rule's name,
come in name order. Raku leaves those to its hash, so its order changes
from one run to the next; `TestCapsMatchRaku` checks every Raku case with
its ties put in name order.

## Parsing

A compiled grammar runs over text with these: `Parse` and `Subparse`
match from the start of the text, `Find`, `FindAll` and `Split` search
it, and `ParseErr` and `SubparseErr` say why a parse failed.

| Method | Does |
|---|---|
| `g.Parse(text)` | match all of text, as Raku's `.parse` |
| `g.Subparse(text)` | match from the start of text, not necessarily to the end |
| `g.Find(text)` | the first match anywhere in text |
| `g.FindAll(text)` | every match in text, left to right, not overlapping unless given `Overlap()` or `Outermost()` |
| `g.Split(text)` | the text between matches, with each match's text in place, as Raku's `split` with `:v` does |
| `g.ParseErr(text)`, `g.SubparseErr(text)` | as `Parse` and `Subparse`, with a `*ParseError` saying why when the text doesn't match |

Each returns undef (or an empty array) when nothing matches. The undef is
read-only, so reading through it leaves it undef (see
[When text doesn't match](#when-text-doesnt-match)).
Each takes
options: `grammar.Rule("name")` to start somewhere other than `TOP`,
`grammar.Pos(n)` to start n characters in, `grammar.Actions(a)`, and
for `FindAll`, `grammar.Overlap()` and `grammar.Outermost()`.

**Starting part-way in.** `Pos(n)` is Raku's `:pos`: `Parse` and
`Subparse` match from there (`Parse` still to the end), and `Find`,
`FindAll` and `Split` search from there. Unlike matching a slice of the
text, what's before is still there: a lookbehind sees it, and `<<` won't
match in the middle of a word. The match's positions are the whole
text's. [job-meta](https://github.com/nukopian/job-meta)'s occupation
coder uses it to try a word list at each word of a job title.

**Overlapping matches.** `FindAll` goes on from the end of each match,
so a match that starts inside another is never found: in "In this role
you will need", a list with "in this role you will" and "you will need"
gives only the first. `Overlap()` is Raku's `:ov` (`:overlap`): a match
from each position where one starts, each the match `Find` would find
there, so at most one per position. For a word list, that's every phrase
in the text, except a shorter phrase starting where a longer one does,
as a list is one atom. That's for classifying text by the phrases in
it, where each one counts:

```go
g.FindAll("In this role you will need", grammar.Overlap())
// "In this role you will", from 0; "you will need", from 13
```

A phrase wholly inside a longer one is found too, as it starts at a
position of its own: with "role" in the list as well, it's found from 8,
inside "In this role you will". That may not be what you want: tried on
jdc-lab's section headings, "profile" inside "Company Profile" outvoted
"company profile". `Outermost()` keeps only the matches that aren't
inside another: "you will need", which reaches past the end of "in this
role you will", stays, but "role" and "profile" go. It implies
`Overlap()`, so it's the one option to give. An empty match at another's
end is inside it. Raku has no such adverb; it's written in Raku as
`:ov`, then a `grep` dropping each match whose `.from` and `.to` lie
within a longer one's.

The other methods ignore it: Raku's `.parse`, `.subparse` and `.split`
take no `:ov`, and `Find`'s first match is the same either way. Raku's
`:ex` (`:exhaustive`), every way of matching at each position, isn't
here; see [the gaps](#missing-raku-features).

### Into Go structs: `To`, `ToAll` and `Values`

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

`grammar.ToAll[T](matches)` is `To` for `FindAll`: it reduces each match
the same way and gives a `[]T`, in the order found. A word list's
canonical values come back as Go strings, with no scalar in sight:

```go
kinds, err := grammar.ToAll[string](headings.FindAll(text))
```

Finding nothing gives an empty slice, not `ErrNoMatch`, as no match is
an answer for a search. An error names the match by its index, such as
`[1]`. Raku's nearest is `@matches».made`.

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

A lookbehind, `<?after …>`, matches its pattern ending where it is, so it
tries starts before that, latest first. It narrows them the same way:
only as far back as its pattern can reach, when that's bounded (`'£'`,
`\d ** 1..3`, a rule that matches a fixed length, a word list without
`LooseSpacing`, as far as its longest phrase), and only where its
pattern can start (`<[.!?:]> \s+` only at punctuation). Before, it tried
every start back to the beginning of the text, so `FindAll` took time in
proportion to the square of the text's length. On a 4 KB text
(`BenchmarkLookbehind`), `<?after '£'> \d+` went from 137 ms to under
1 ms, and the sections grammar's `<?after <[.!?:]> \s+> What` from
425 ms to 40 ms. Unbounded patterns still try every start they could
begin at, so they stay slower: `<!after @hedges \s*>`, unbounded, took
job-meta's hours package past `StepLimit` on a 37 KB text; with
`\s ** 0..3` it takes 66 ms. `TestLookbehindChangesNothing` and `FuzzMatch` check that
the narrowing changes no results.

The matcher has two fast paths, which skip machinery that can't change
the result. Inside a token, committing to an atom that can match only one
way, a literal, a class, an anchor, a regex or a backreference, needs no
closure to catch its first match, so the atom is matched directly; and a
greedy repeat of a class or a literal, such as `\d+` or `\s*`, takes as
many as it can and never gives one back, so it runs as a plain loop
rather than through closures and recursion. Together they took a pay
parse from 12.5 µs to 9.3 µs, and a third of its allocations
(`BenchmarkParse`), and `FindAll` on a posting 8%. The repeat loop takes
the same steps as before, so `StepLimit` and `DepthLimit` are reached at
the same point. Both sit behind a switch, `fastPaths`:
`TestFastPathsChangeNothing` compares every result, failure report and
step count with them on and off, over every Raku case,
`TestFastPathsKeepLimits` does the same under tight limits, and
`FuzzMatch` checks them too.

## Errors

### When text doesn't match

`Parse` returns undef when text doesn't match, as do `Subparse` and
`Find`. Reading through it gives undef, so
`Pay.Parse("negotiable").At("min.~").Or("none")` gives "none", and the
result stays undef, as it does in Raku:

```go
m := Pay.Find("Salary negotiable")
m.At("min.~").String() // ""
m.Defined()            // false: no match
```

That's because the undef is locked with scalar's `Lock`. Any other undef
in scalar would autovivify into a hash when read through, as in Perl, and
`Defined()` would then report a match. Writing into it panics
(`scalar: Not a HASH reference`); there's nothing to write to.

`ParseErr` returns the same, plus an error saying where the grammar got
furthest and what it expected there:

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

### Watching a parse: `Trace`

`ParseErr` says where a parse got furthest. To see how it got there,
`Trace(w)` writes each rule to `w` as it's entered, indented by depth,
then `* MATCH` and the text it matched, or `* FAIL`, in the style of
Raku's `Grammar::Tracer`:

```go
Pay.Parse("£45k per annum", grammar.Trace(os.Stdout))
```

```
TOP
|  money
|  |  amount
|  |  |  num
|  |  |  * MATCH "45"
|  |  * MATCH "45k"
|  * MATCH "£45k"
|  sep
|  * FAIL
|  period
|  * MATCH " per annum"
* MATCH "£45k per annum"
```

A rule's result is printed at its caller's depth, so the rules it
called sit between its name and its result. Every way of matching takes it:
`Parse`, `Subparse`, `ParseErr`, `SubparseErr`, and `Find`, `FindAll`
and `Split`, which trace each position they try, leaving out those
where no match can start. As in `Grammar::Tracer`:

- **Each call is traced once.** When matching backtracks into a rule
  that has matched, to try another way, nothing more is printed for it.
  The rules it calls as it tries again are traced beside it, at its own
  depth, and the rules after it are traced again as they're retried.
- **The last line is TOP's,** which can be `* MATCH` though `Parse`
  fails: TOP matched, but not to the end of the text, as in
  `ExampleTrace_failure`.
- **Built-in rules aren't traced,** such as `<ws>`, which every `rule`
  calls between its parts, and `<alpha>`, unless the grammar defines its
  own. Word lists, pattern lists and `Re` aren't rules, so they aren't
  traced either. Rules called in lookarounds are.
- **The text is cut** to 60 characters, less three for each level of
  depth, and quoted as Raku quotes a string, so a tab is `\t` and a
  control character `\x[1B]`. Past 20 levels there's no room for it.

It shows what `grammar` does, which isn't always what Raku's tracer
shows (see [Differences from Raku](#differences-from-raku)): `|` and a
proto try every branch to find the longest, so each is traced, and in
a `regex` the winner is matched, and traced, again.

Tracing changes no result, which `FuzzMatch` checks, and each rule it
opens is closed, once. A match that runs past `StepLimit` or
`DepthLimit` while traced stops with the rule it was in still open, at
the end of the trace, and the error names it, as `ParseErr`'s does (see
[Runaway matches](#runaway-matches)). Without it, it costs a check per rule call, too
little to measure (see the [record](changes/2026-10-31-r1-a-trace-of-a-parse.md)).
With it, a parse is much slower, so it's for debugging, not for
production. What `w` returns is ignored, so a failing writer doesn't
stop the parse.

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

From `ParseErr` and `SubparseErr`, both errors also say where: the rule
that was matching when the limit was reached, and the line and column
where that rule's match started, as in "gave up after 100000 steps, in
amount from line 1, column 9". For a proto, it's the variant, such as
`amount:sym<range>`. That's the rule to look at first, though the steps
may have been spent anywhere since the match began. The other calls
say where too while they're traced with `Trace`, from the trace's own
stack of rules, which leaves out built-in rules such as `<ws>`. Untraced,
they leave `Rule` empty and the place 0, as knowing it costs every match
a little; when one stops at a limit, run the same text through
`ParseErr`, or with `Trace`, to see where.

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

## Differences from Raku

Where `grammar` behaves differently from Rakudo, on purpose or not yet,
with when each matters. *Fixable* and *Effort* are as in the
[gaps](#gaps-and-possible-additions).

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
| **A literal under `:i` folds case a character at a time,** so `:i 'ẙ'` doesn't match `Y̊`, a capital with no composed form. Word lists compare such letters decomposed, so they match (see [Accented letters](#accented-letters-text-is-read-in-nfc)). Rakudo 2025.11 agrees that the two are equal under `fc`, but matches only one way round with a literal, and neither way with an array. | Almost never: a handful of letters, such as Y and W with a ring above, J with a caron, and some Greek capitals with accents, none of them in English or most European languages. | Fold literals as word lists do, a decomposed character at a time. | yes | 1 |
| **`Subparse` returns undef when nothing matches,** as `Parse` does. Raku's `.subparse` returns a failed `Match`, which is defined but false: reading it gives `Nil`, and its text is `""`. | Only for Raku code that tests a subparse result with `.defined` rather than by its truth, which grammar's undef agrees with. In Go, one check, `Defined()`, works for every way of finding nothing. | Return a locked `""`, which is defined, false and reads through to undef, but then `Subparse` callers would have to check `Bool()` where every other method's callers check `Defined()`, and `To`, `Values` and `SubparseErr` would need special cases. | by design | — |
| **`<alias=name>` captures under both names,** as Raku does. | When you expect only the alias. | Nothing to fix: use `<alias=.name>`. | by design | — |
| **`Trace` shows what `grammar` tries, not what Raku's `Grammar::Tracer` would.** Both print the rules called in the same form, but for `\|` and protos, `grammar` matches every branch to find the longest, so each is traced, where Raku's tracer shows the branches it tried in order. `Grammar::Tracer` also changes what Raku matches: wrapping a rule stops it being declarative, so `<h> \| <hours>` on "hours" matches "hour" traced and "hours" not. | Only when comparing a trace with Raku's, line for line: the rules a branch calls are the same. | — | by design | — |
| **`Re` takes Perl-style regex syntax.** | Only that it isn't Raku syntax. It's for patterns you already have. | — | by design | — |
| **`Outermost()` adds to Raku's `:ov`:** `Overlap()`'s matches, less each one wholly inside another. | Never without it: `FindAll` and `Overlap()` match as Raku's `:g` and `:ov` do. | — | by design | — |
| **`CheckPath` adds to Raku:** it says whether a path can ever be defined in a match, from the grammar alone. | Never: matching is the same without it. | — | by design | — |
| **Word lists add to Raku's `@list`:** canonical values (a rule's `made`, when the rule is a list, alone or beside lookarounds and anchors, where Raku's `made` is `Nil`), `FoldCase`, `LooseSpacing`, `JoinedSpacing`, `Plurals`, `WholeWords`, `ExactAcronyms` and `MapOnly`. | Never for Raku-compatible lists: without the options, a list matches as Raku's does. | — | by design | — |

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
| `FuzzMatch` | for grammars built around a fuzzed rule, with a proto and a pattern list to call: no panics; `Parse` and `ParseErr` agree; a parse matches all of the text; `Find` finds what `FindAll` finds first; skipping start positions changes nothing; the matcher's fast paths change nothing; `CheckPath` takes every path a match has |
| `FuzzWords` | synonym rules never panic, and every phrase in a list matches itself; with `JoinedSpacing`, run together too, and with `Plurals`, its plural |

`go test` runs them on their seeds and on the inputs that once failed, kept
in `testdata/fuzz`. To search for new failures:

```sh
go test -run '^$' -fuzz '^FuzzMatch$' -fuzztime 2m
```

`FuzzMatch` keeps texts short and leaves out `regex` rules and `:!r`,
whose backtracking can take exponential time by design (see [Runaway
matches](#runaway-matches)). A failure it finds is written to `testdata/fuzz`;
commit it, so it's checked from then on.

## Gaps and possible additions

What this package doesn't do yet, and what it would take. Where it
behaves differently from Raku has its own section,
[Differences from Raku](#differences-from-raku). Each entry says when you'd need it, so
that it's clear which to do first when the need comes up. Which are
planned, and in what order, is in the [roadmap](../ROADMAP.md).

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

- **[node.go](../node.go)** has the matcher: one type per kind of node, each
  with a `match` method. Matching is continuation-passing: a node calls
  `k` for each way it can match, and returning false means "try another
  way", which is all backtracking is. `step` is where a token's ratchet
  happens: it commits to a node's first match, except where `extendable`
  says a frugal quantifier inside may still need to take more.
- **[grammar.go](../grammar.go)** has grammars, roles, rule lookup (by name,
  when called, so overriding works), the capture scopes worked out from
  each rule (`newScope`: which captures are arrays), turning matches into
  scalar data (`build`), and actions.
- **[syntax.go](../syntax.go)** is the Raku-syntax parser: hand-written
  recursive descent that produces the same nodes as the Go builder.
- **[words.go](../words.go)** has word lists: a trie for exact phrases and one
  for case-folded phrases.
- **[raku_test.go](../raku_test.go)** compares match trees with Raku's. Any
  change to how matching behaves should start with a case there, and Raku
  saying what's right.

### Missing Raku features

| Feature | When you'd need it | What it would take | Fixable | Effort |
|---|---|---|---|---|
| **Code assertions**: `<?{ … }>`, `<!{ … }>` | Checks only code can make, in the middle of a parse: min ≤ max, a plausible year, a word that's in a list but isn't a stopword. Without them, wrong parses get through and need cleaning up afterwards. | Raku code can't run here, so: a builder node holding `func(m *scalar.SV) bool`, and a syntax such as `<?{name}>` naming a function passed to `Compile`. The function needs the captures so far, so build a provisional match of the current scope from the `capList`. | partly | 2–3 |
| **Code blocks**: `{ make … }` | Making values in the grammar itself. | Actions do this. Not worth adding unless grammars are written by people who won't write Go. | by design | — |
| **True longest-token matching** for `\|` | When `Alt`'s approximation (below) gives a different answer from Raku, which the Raku tests will show; and to speed up large alternations and protos. | Work out each branch's declarative prefix (the part before anything that could backtrack or run code), build an automaton (NFA, then DFA) from the prefixes, and try branches in order of how much of the text their prefix matches. This is what Rakudo does in NQP. | yes | 5 |
| **Exhaustive matching**: `:ex` | Every way a pattern can match at each position, not only the first, as `Overlap` gives: "aaaa" with `regex { a+ }` gives "aaaa", "aaa", "aa" and "a" from the start. For a word list, or a token, which never gives anything back, Raku's `:ex` is the same as `:ov`, so word lists don't need it. | A `FindAll` option whose continuation records each match and returns false, so the matcher tries every other way; the state's failure tracking and step counting must cope with matches that are then thrown back. The Raku tests would need `:ex` cases. | yes | 2 |
| **Rule parameters and dynamic variables**: `token amount($cur)`, `$*CURRENCY` | Parsing that depends on context, such as a currency that changes how amounts read. | Arguments in the syntax and on `call`, and a stack of dynamic variables in `state`, visible to called rules. Inheritance and roles usually do the job more simply. | yes | 3 |
| **Interpolation**: `$var`, `<$regex>`, `<{ … }>` | Patterns chosen at run time. Backreferences, `$<name>` and `$0`, are supported. | Word lists cover literal phrases, pattern lists (`<@name>`) patterns from data, and `Re` regex strings. Variables would come with rule parameters (below). | partly | 2 |
| **Failure reports from `Find` and `FindAll`** | Finding out why pay wasn't found in a posting, rather than why one string didn't parse. | Track failures across every start position and report the furthest. The noise is the hard part: most positions fail at once, for no interesting reason. | partly | 2 |
| **Backtracking control beyond `:`**: `::`, `:::`, `<commit>`, `<cut>` | Taming slow, heavily backtracking `regex` rules. `:` after an atom (`[ a \| ab ]:`, `<w>:`) is supported, as Rakudo supports it. | Rakudo doesn't implement these either (2025.11: `::` and `:::` are "not yet implemented", and `<commit>` and `<cut>` are taken as calls to rules of those names), so there's nothing to check them against, and a grammar using them wouldn't run in Raku; `Compile` rejects `::` and `:::`. They need the matcher to unwind to a marked point, which continuation-passing makes awkward. `:`, tokens and `StepLimit` usually do the job. | partly | 3 |

### Engineering

| Gap | When you'd need it | What it would take | Fixable | Effort |
|---|---|---|---|---|
| **`Find` can't skip positions for some grammars.** It skips positions where no match can start, but only when it can work that out: not for a grammar that starts with a regex terminal (`Re`), or that can match empty text, as those could start anywhere. Such grammars search at about the old speed. | Searching long texts with grammars like those. | For `Re`, ask regexp2 for the pattern's possible first characters (it works them out internally, but doesn't expose them), or let `Re` take them as a hint. A grammar that can match empty text matches everywhere, so there's nothing to skip. | partly | 2 |
| **The matcher still makes closures for most steps.** Two fast paths, behind `fastPaths` (2026-10-19), took the commonest away (see [Speed](#speed)), and a pay parse went from 12.5 µs and 532 allocations to 9.3 µs and 359 (`BenchmarkParse`). What's left: `once` still makes about a third of a parse's allocations, for rule calls under ratchet and for steps over groups, calls and word lists, and every step of a sequence makes a continuation (`seq.from`). | Parse speed at high volumes: a pay string parses in about 9 µs, against Participle's 6 µs. | Commit to a rule body or a group without a closure: `once`'s closure only records where the first match ended, so a slot on `state` could do it, where nothing can run between the match and reading the slot; that needs checking node by node. The fast paths' trade-offs hold for any more: they duplicate what the general engine does, so a later change to an atom (case folding, failure reports) must be made in both; they must step aside for frugal, possessive or separated repeats and items that can match nothing; and they must leave failure reports alone, as the repeat loop does by running only when failures aren't being tracked. `TestFastPathsChangeNothing` and `TestFastPathsKeepLimits` compare both paths, and must cover a new one, with a planted bug to show they'd notice. A lexer was considered instead and rejected: it would make matching cheaper, but cost precise errors, word lists that span spaces, `Find` anywhere in text, and Raku compatibility. | yes | 2 |
| **Building a match's scalar data takes about 40% of a parse.** Profiled after the fast paths (2026-10-19), building the match took 43% of `BenchmarkParse`'s time, scalar's own code 32%, most of it making each match's hash. | Parse speed at high volumes, once matching is faster still. | Kept: scalar's `HV.Set` stores a new key without first making a placeholder (scalar v0.5.1), which took a parse from 9.30 to 9.13 µs, 8 allocations fewer. Tried and reverted: making each match's hash in one `scalar.Hash` call with every key, so the map is sized once, made a parse slower, 9.13 to 9.58 µs and 21 allocations more, as building the list costs more than the growing it saves. Not pursued: building scalar values only when they're read. It would need a new kind of scalar value in scalar's core, honoured by every way of reading one; it could save up to the 43% where results are hardly read, as in the benchmark, but little in real use, where `To` reads every field. | partly | 3 |
| **Each match's text is a copy.** A match's `~` is rebuilt from the runes of the text, rather than being a slice of the original string. | Never, as measured. | Tried and measured, then reverted: sharing the original string (when it's valid UTF-8, with a cursor from rune positions to byte offsets) saved about 1% of allocations, made a parse no faster, and made `FindAll` on a 4 KB posting 20% slower, through checking the whole text was UTF-8 and walking the cursor to each match. Copying a match's few characters was always cheap, and storing text in a scalar allocates either way. | no | — |
| **No memoisation.** A rule may be tried at the same position more than once. | Rarely, as measured (2026-10-16): of 37,400 token calls across titleeval, payeval and grammarrules, 18 were repeats at the same position, and `BenchmarkParse` and `BenchmarkFindAll` have none. Only `regex` rules that backtrack heavily through tokens repeat much, and `StepLimit` bounds those. | Cache each token's result by rule and position in `state` (a token has at most one result there), skipped under `ParseErr`, which must record every failure. Not worth it for these grammars: it would add a map lookup to every call to save almost none. Worth reconsidering, perhaps as an option, for a grammar whose profile shows repeats. | yes | 2 |
| **Nested alternations in a `regex` double their work at each level.** `\|` matches each branch to measure it, then matches the winner again for real. In a `token` the second match is skipped, as the first is final. | Deeply nested alternations inside `regex` rules, which are rare. | Part of true longest-token matching (above), which decides from prefixes without matching branches in full. | yes | 5 |
| **`extendable` is worked out at every step.** | Never, as measured (2026-10-16): profiling `BenchmarkParse`, it's under 1% of the samples, and about 2% of the parse's own time, so a cache could save no more. | A cache would need a map lookup per step, as `seq` is a slice with nowhere to keep one, which costs about what working it out does. Not worth doing; the allocations in the row above are where a parse's time goes. | no | — |
| **Word lists: no stemming or fuzzy matching.** (`JoinedSpacing` covers "frontend" against "front end", and `Plurals` regular plurals.) | Synonym data that doesn't list every form of a word: "visiting" for "visit". | Stemming could follow `Plurals`, making forms in `Add`, but English stems are irregular enough to need a real stemmer, such as Porter's (effort 2–3). Fuzzy matching belongs elsewhere: compare embeddings for near misses, and send the rest to the LLM (effort 3+). | yes | 2–3 |
| **Word lists can't change while in use.** | Refreshing synonyms in a long-running service. | Build a new list and grammar, and swap them in with an `atomic.Pointer`. Needs no change here. | yes | 1 |
| **The Raku-syntax parser is hand-written,** not built with the package's own builder as first planned. | Only if the syntax grows much larger. | Rewrite it as a grammar of Raku regex syntax, with actions building nodes. | yes | 3 |
| **Untraced, only `ParseErr`'s limit errors say where.** From `Parse`, `Find` and the rest, a `StepLimitError` or `DepthLimitError` names the grammar and the limit, not the rule that was running or the place in the text, unless they're traced with `Trace`; `ParseErr` and `SubparseErr` name both, from the failure tracker's stack of rules. | Finding the rule that backtracks badly in a long run of `Find` or `FindAll`, without running the text again through `ParseErr` or with `Trace`. | Keep the innermost rule in `state` for every match: set it in `call.match`, and put the caller's back around the continuation and on return, as the tracker does. Tried and reverted (2026-10-26): it cost `BenchmarkParse` 2.3%, `BenchmarkFindAll` 1.2% and `BenchmarkLookbehind` 4.5–6.2%, with a deferred restore or explicit ones alike, for an error that's rarely seen. Done instead while tracing ([roadmap 1's record](changes/2026-10-31-r1-a-trace-of-a-parse.md)), which keeps its own stack. [Roadmap 2's record](changes/2026-10-26-r2-limit-errors-that-say-where.md) has the measurements. | yes | 1 |

### Handing a grammar on

`grammar` is for getting rules working quickly on messy text. Once a
grammar has settled, it could hand on to other tools, as `scalar` hands
loose data on to typed Go with `To`.

The first row works for any grammar. The other three are steps towards
[Participle](https://github.com/alecthomas/participle), each building on
the one before. Participle can't do everything a Raku grammar can, so
rather than translate any grammar and mark what was lost, they define the
part that carries over first, as a phrasebook, then a Raku-like dialect
that accepts only that part, then a generator for it. The subset is
then decided before you write a rule, not discovered afterwards. Raku
already divides a grammar the way Participle needs: a `token`, which
never skips whitespace in the text, becomes part of Participle's lexer,
and a `rule`, where whitespace in the pattern matches whitespace in the
text, becomes one of its grammar rules, which skip whitespace between
tokens.

| Gap | When you'd need it | What it would take | Fixable | Effort |
|---|---|---|---|---|
| **No picture of a grammar.** There's no way to print a grammar as EBNF, the standard notation, or to draw it as a railroad diagram. | Documenting, reviewing or learning a grammar: a diagram of the pay grammar beside its rules shows at a glance what the rules take a while to read. | Walk the rules and print EBNF, with a note wherever a Raku feature has no EBNF form: backreferences, lookbehind, word lists, a token's ratchet. Draw the diagrams from the EBNF with an existing tool rather than writing one; check which before relying on it (Participle's repository may have one). | yes | 1–2 |
| **No phrasebook from Raku to Participle.** | Raku programmers curious about Participle, and anyone choosing between the two. It's also the specification for the dialect (next row). | A page like `scalar`'s Perl phrasebook: each construct in Raku grammar syntax, the same thing as Participle structs and tags, and what Participle can't do. That covers backreferences, lookbehind and frugal quantifiers, as Participle works on tokens rather than characters, and word lists' canonical values, as it captures only the text. Every row runs as a test, the Raku form through `grammar` and the Participle form through Participle, on the same inputs, so the page can't drift from either. | yes | 2 |
| **No Participle dialect.** There's no way to write a Participle grammar in a Raku-like syntax. | Describing a Participle grammar the Raku way: readable rules, with Participle's typed output at the end. | A restricted mode of the Raku-syntax parser that accepts exactly what the phrasebook covers. Anything else fails when the grammar compiles, saying why, such as "backreferences need the text, and Participle works on tokens". A `token` becomes a lexer rule, so in the dialect it must be a regular pattern, with no calls to rules and no captures; a `rule` becomes a grammar rule, with whitespace skipped between tokens. The same grammar still runs in `grammar` itself, which the generator (next row) relies on. | yes | 2–3 |
| **No generator for Participle.** | Graduating a dialect grammar: when a typed syntax tree, structure checked when the code compiles, and Participle's error messages and ecosystem are worth more than Raku compatibility. | Generate Go source with `go generate`, as Participle's grammars are struct tags and must exist when the code compiles: structs with their tags, and a lexer from the tokens. Word lists become lexer tokens or literal alternatives, and their canonical values a generated lookup map. It's a starting point you own and edit, not one kept in step with the grammar. As the dialect has already refused what can't translate, the output is complete, with nothing to mark as missing. Prove it as the Raku tests prove `grammar`: run the grammar and the generated parser on the same inputs, and compare. | yes | 3 |

