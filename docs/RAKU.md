# grammar: from Raku to Go

A phrasebook for Raku programmers coming to Go, and for Go programmers
curious about grammars. Each task below is shown three ways: in Raku, with
[grammar](../README.md), and in plain Go with `regexp`. The grammar is the
same text in Raku and in grammar, which is the point: what you'd write in
Raku runs in Go. The plain Go column shows the same job without a grammar.

All three versions of every task run as a test, `TestMappingMatchesRaku`
in `raku_mapping_test.go`, on the same posting, and must give the same
answer, Raku's included.

For how each feature behaves, see the [reference](REFERENCE.md), and
[what you might assume](REFERENCE.md#what-you-might-assume) if you know
regexes; for tasks in more depth, the [how-to guide](HOWTO.md).

- [The posting](#the-posting)
- [The first match](#the-first-match)
- [A named capture](#a-named-capture)
- [Every match](#every-match)
- [Values from actions](#values-from-actions)
- [A list of captures](#a-list-of-captures)
- [Synonyms and their canonical values](#synonyms-and-their-canonical-values)
- [The longest alternative, or the first](#the-longest-alternative-or-the-first)
- [Split, keeping the separators](#split-keeping-the-separators)
- [Which to use](#which-to-use)

## The posting

One line of a job posting, and a grammar for the pay in it:

```text
Senior SWE (Hybrid). Pay: £45k – £55,000 per annum. Weekend: £18.50 per hour. Skills: Go, Raku, SQL.
```

```raku
grammar Pay {
    token TOP    { <min=money> [ <.sep> <max=money> ]? <period>? }
    token money  { '£' \s? <amount> }
    token amount { <num> [ $<k>=[:i k] ]? }
    token num    { \d ** 1..3 [ ',' \d ** 3 ]* [ '.' \d+ ]? }
    token sep    { \s* [ '-' | '–' | to ] \s* }
    token period { :i \s* [ 'p.a.' | pa | per \s+ [ annum | year | hour | day ] ] }
}
```

In Go, the same text compiles: `pay := grammar.MustCompile(src)`. The
plain Go column uses one regex for the pay instead:

```go
var plainPayRe = regexp.MustCompile(`£\s?(\d{1,3}(?:,\d{3})*(?:\.\d+)?)([kK])?(?:\s*(?:-|–|to)\s*£\s?(\d{1,3}(?:,\d{3})*(?:\.\d+)?)([kK])?)?(?:\s*(?i:p\.a\.|pa|per\s+(?:annum|year|hour|day)))?`)
```

## The first match

The first pay in the posting. Answer: `£45k – £55,000 per annum`.

```raku
~$posting.match(/ <pay=Pay::TOP> /)<pay>
```

```go
// grammar
pay.Find(posting).At("~").Str()
```

```go
// plain Go
plainPayRe.FindString(posting)
```

## A named capture

The top of the range. Answer: `£55,000`.

```raku
~$posting.match(/ <pay=Pay::TOP> /)<pay><max>
```

```go
// grammar: a capture is read by its name, as a path
pay.Find(posting).At("max.~").Str()
```

```go
// plain Go: groups are numbered, and the regex has no £ in the group
"£" + plainPayRe.FindStringSubmatch(posting)[3]
```

## Every match

Every pay in the posting. Answer: `£45k – £55,000 per annum;£18.50 per hour`.

```raku
$posting.match(/ <pay=Pay::TOP> /, :g).map({ ~.<pay> }).join(";")
```

```go
// grammar
pay.FindAll(posting).Map(func(m *scalar.SV) any { return m.At("~") }).Join(";")
```

```go
// plain Go
strings.Join(plainPayRe.FindAllString(posting, -1), ";")
```

## Values from actions

The range as numbers: "45k" is 45000. Answer: `45000 55000`.

```raku
class PayActions {
    method amount($/) { make $<num>.Str.subst(",", "", :g) * ($<k> ?? 1000 !! 1) }
}
my $pay = Pay.subparse($posting, :pos($posting.index("£")), :actions(PayActions));
"$pay<min><amount>.made() $pay<max><amount>.made()"
```

```go
// grammar: an action is a function, or a method, named after its rule
acts := grammar.Actions(map[string]func(*scalar.SV) any{
	"amount": func(m *scalar.SV) any {
		n := m.At("num.~").S(`,`, "", "gr").Num()
		if m.At("k").Defined() {
			n *= 1000
		}
		return n
	},
})
m := pay.Find(posting, acts)
m.At("min.amount.made").Str() + " " + m.At("max.amount.made").Str()
```

```go
// plain Go
amount := func(num, k string) string {
	n, _ := strconv.ParseFloat(strings.ReplaceAll(num, ",", ""), 64)
	if k != "" {
		n *= 1000
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}
sm := plainPayRe.FindStringSubmatch(posting)
amount(sm[1], sm[2]) + " " + amount(sm[3], sm[4])
```

## A list of captures

The skills, a list separated by commas. A capture that can repeat is a
list, in Raku and in grammar. Answer: `Go,Raku,SQL`.

```raku
grammar Skills { token TOP { 'Skills:' \s* <skill>+ % [ ',' \s* ] }; token skill { \w+ } }
Skills.subparse($posting, :pos($posting.index("Skills:")))<skill>».Str.join(",")
```

```go
// grammar
skills := grammar.MustCompile(`grammar Skills { token TOP { 'Skills:' \s* <skill>+ % [ ',' \s* ] }; token skill { \w+ } }`)
skills.Find(posting).At("skill").AV().Map(func(s *scalar.SV) any { return s.At("~") }).Join(",")
```

```go
// plain Go: a regex captures the list as one string, to split again
var skills []string
for _, s := range strings.Split(plainSkillsRe.FindStringSubmatch(posting)[1], ",") {
	skills = append(skills, strings.TrimSpace(s))
}
strings.Join(skills, ",")
```

with `plainSkillsRe = regexp.MustCompile(`Skills:\s*(\w+(?:,\s*\w+)*)`)`.

## Synonyms and their canonical values

Each period, as a canonical value: "per annum" and "pa" are both a
Year. Answer: `Year,Hour`.

Matching whole words matters here: without it, "pa" matches the start of
"**Pa**y:" too, in all three, which the test caught. Raku's `<< … >>`,
grammar's `WholeWords` and Go's `\b` each say so.

```raku
my %canonical = 'per annum' => 'Year', 'pa' => 'Year', 'per hour' => 'Hour';
my @phrases = %canonical.keys;
$posting.match(/:i << @phrases >>/, :g).map({ %canonical{.lc} }).join(",")
```

```go
// grammar: a word list knows each phrase's canonical value, its made
periods := grammar.NewWordList("periods", grammar.FoldCase(), grammar.WholeWords()).
	Add("Year", "per annum", "pa").Add("Hour", "per hour")
canonical := grammar.MustCompile(`grammar Period { token TOP { @periods } }`, periods)
canonical.FindAll(posting).Map(func(m *scalar.SV) any { return m.At("made") }).Join(",")
```

```go
// plain Go
canonical := map[string]string{"per annum": "Year", "pa": "Year", "per hour": "Hour"}
var periods []string
for _, p := range regexp.MustCompile(`(?i)\b(?:per annum|pa|per hour)\b`).FindAllString(posting, -1) {
	periods = append(periods, canonical[strings.ToLower(p)])
}
strings.Join(periods, ",")
```

In Raku the canonical values are a hash beside the list; in grammar
they're part of it, and a list can come from synonym rules, as
Elasticsearch and Solr write them: `Year, per annum, pa`.

## The longest alternative, or the first

`|` and `||` on "ab": the longest alternative, then the first. Answer:
`ab a`.

```raku
grammar Longest { token TOP { a | ab } }
grammar First   { token TOP { a || ab } }
Longest.subparse("ab") ~ " " ~ First.subparse("ab")
```

```go
// grammar
longest := grammar.MustCompile(`grammar Longest { token TOP { a | ab } }`)
first := grammar.MustCompile(`grammar First { token TOP { a || ab } }`)
longest.Subparse("ab").At("~").Str() + " " + first.Subparse("ab").At("~").Str()
```

```go
// plain Go: a Go regex takes the first alternative, unless told Longest
longest := regexp.MustCompile(`^(?:a|ab)`)
longest.Longest()
longest.FindString("ab") + " " + regexp.MustCompile(`^(?:a|ab)`).FindString("ab")
```

This is the difference that surprises regex users most: in a grammar, `|`
is longest-match, and `||` is what a regex's `|` does.

## Split, keeping the separators

Split at `;` or `,`, keeping what was split at. Answer: `a|; |b|, |c`.

```raku
"a; b, c".split(/ <[;,]> \s* /, :v).join("|")
```

```go
// grammar
sep := grammar.MustCompile(`grammar Sep { token TOP { <[;,]> \s* } }`)
sep.Split("a; b, c").Join("|")
```

```go
// plain Go: Go's Split drops the separators, so keep them by hand
text, sepRe := "a; b, c", regexp.MustCompile(`[;,]\s*`)
var pieces []string
last := 0
for _, loc := range sepRe.FindAllStringIndex(text, -1) {
	pieces = append(pieces, text[last:loc[0]], text[loc[0]:loc[1]])
	last = loc[1]
}
strings.Join(append(pieces, text[last:]), "|")
```

## Which to use

For one pattern, the plain Go column is shorter, and a regex is the
right tool. The grammar column pays off as the structure grows: rules
with names, captures read by name rather than number, lists that are
lists, synonyms with canonical values, and the same text running in Raku
and in Go. See [When not to use it](../README.md#when-not-to-use-it) for
the rest.
