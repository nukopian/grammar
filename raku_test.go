package grammar

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/nukopian/job-meta/internal/scalar"
)

// rakuCases are grammars in Raku syntax, each with inputs to parse. Each
// grammar has its own name, as Raku declares them all in one program.
var rakuCases = []struct {
	Src    string              `json:"src"`
	Sub    bool                `json:"sub"` // subparse rather than parse
	Inputs []string            `json:"inputs"`
	Lists  map[string][]string `json:"lists,omitempty"` // word lists, as Raku arrays
}{
	// The pay grammar from the README.
	{Src: `grammar Pay {
	    token TOP    { <min=money> [ <.sep> <max=money> ]? <period>? }
	    token money  { '£' \s? <amount> }
	    token amount { <num> $<k>=[:i k]? }
	    token num    { \d ** 1..3 [ ',' \d ** 3 ]* [ '.' \d+ ]? }
	    token sep    { \s* [ '-' | '–' | to ] \s* }
	    token period { :i \s* [ 'p.a.' | pa | per \s+ [ annum | year | hour | day ] ] }
	}`, Sub: true, Inputs: []string{
		"£45,000 - £55,000 per annum", "£45k to £55K", "£18.50 per hour", "£1,23", "45,000", "£60,000 P.A. plus bonus",
	}},
	// Aliases, quantified captures, a named capture over a group, and a
	// positional capture inside it.
	{Src: `grammar Caps {
	    token TOP { <a=x>* $<k>=[ (b) c ]? <y>? }
	    token x { x }
	    token y { y }
	}`, Inputs: []string{"", "xxbc", "xbcy", "bc", "xxy"}},
	// | takes the longest branch; || the first.
	{Src: `grammar Longest { token TOP { [ a | ab | abc ] } }`, Inputs: []string{"abc", "ab", "a", "abcd"}},
	{Src: `grammar FirstOf { token TOP { [ a || ab ] c } }`, Inputs: []string{"abc", "ac"}},
	// A token doesn't backtrack; a regex does, even into a regex it calls.
	{Src: `grammar Tok { token TOP { \d* 5 } }`, Inputs: []string{"12345", "5"}},
	{Src: `grammar Rx { regex TOP { \d* 5 } }`, Inputs: []string{"12345", "5", "1234"}},
	{Src: `grammar RxTok { regex TOP { <d> 5 }; token d { \d* } }`, Inputs: []string{"12345"}},
	{Src: `grammar RxRx { regex TOP { <d> 5 }; regex d { \d* } }`, Inputs: []string{"12345"}},
	{Src: `grammar Frugal { regex TOP { (a+?) (a*) } }`, Inputs: []string{"aaa"}},
	{Src: `grammar Possessive { regex TOP { a+: a } }`, Inputs: []string{"aaa"}},
	// Separators.
	{Src: `grammar List1 { token TOP { <w>+ % ',' }; token w { \w+ } }`, Inputs: []string{"a,bb,c", "a,b,", "a"}},
	{Src: `grammar List2 { token TOP { <w>+ %% ',' }; token w { \w+ } }`, Inputs: []string{"a,bb,c", "a,b,", ""}},
	{Src: `grammar Count { token TOP { (\d) ** 2..3 % '-' } }`, Inputs: []string{"1-2", "1-2-3", "1-2-3-4", "1"}},
	{Src: `grammar SepAtom { token TOP { [ \w+ % ',' ]* % ';' } }`, Sub: true, Inputs: []string{"ab,cd;x", "a,b;c"}},
	// Character classes, anchors and lookaround.
	{Src: `grammar NamedCls { token TOP { <+alpha +[-]>+ <-alpha -[\s]> <+digit +[x]>? } }`, Inputs: []string{"ab-c1x", "a_b.", "ab ", "a-.7"}},
	{Src: `grammar Cls { token TOP { <[a..z]>+ <-[aeiou\d]> <[a..z] - [aeiou]>* \d } }`, Inputs: []string{"abcxyz9", "abc9", "ab-9"}},
	{Src: `grammar Look { token TOP { (\w+) <?before \d> \d <!before x> } }`, Inputs: []string{"ab1", "ab1x", "ab"}},
	{Src: `grammar Behind { token TOP { .*? <?after 'b'> (c) } }`, Sub: true, Inputs: []string{"abc", "acc"}},
	{Src: `grammar Words { token TOP { .* << (\w+) >> .* } }`, Inputs: []string{"  hello  "}},
	{Src: `grammar Lines { token TOP { [ ^^ (\w+) $$ \n? ]+ } }`, Inputs: []string{"ab\ncd\n", "ab cd"}},
	// Nested positional captures.
	{Src: `grammar Nest { token TOP { ( (a) (b)? )+ } }`, Inputs: []string{"abaab", "a"}},
	{Src: `grammar Branches { token TOP { [ (a) (b) | (c) ] (d) } }`, Inputs: []string{"abd", "cd"}},
	// Built-in rules.
	{Src: `grammar Builtin { token TOP { <alpha>+ <.ws> <digit>+ <ws> } }`, Inputs: []string{"ab 12", "ab12", "a_b 1 "}},
	// Adverbs are lexical.
	{Src: `grammar Ci1 { token TOP { :i abc } }`, Inputs: []string{"ABC", "aBc"}},
	{Src: `grammar Ci2 { token TOP { [:i a] b } }`, Inputs: []string{"Ab", "AB"}},
	{Src: `grammar Ci3 { token TOP { [:i a | b ] c } }`, Inputs: []string{"Bc", "AC"}},
	{Src: `grammar Ci4 { token TOP { :i <x> }; token x { a } }`, Inputs: []string{"A", "a"}},
	{Src: `grammar Ci5 { token TOP { [ a :i b | c ] } }`, Inputs: []string{"C", "aB", "Ab"}},
	{Src: `grammar Ci6 { token TOP { [ a :i b || c ] d } }`, Inputs: []string{"Cd", "CD"}},
	{Src: `grammar Ci7 { token TOP { [ a | b :i c | d ] } }`, Inputs: []string{"D", "bC", "Bc"}},
	{Src: `grammar Ci8 { token TOP { :i [ a | b ] c || d } }`, Inputs: []string{"AC", "D"}},
	// Named captures over subrules, over quantified atoms, and with scopes.
	{Src: `grammar Named1 { token TOP { $<k>=<d> }; token d { (\d) } }`, Inputs: []string{"5"}},
	{Src: `grammar Named2 { token TOP { $<k>=\d+ $<j>=( (\w) ) } }`, Inputs: []string{"12a"}},
	{Src: `grammar Named3 { token TOP { <x=.d> <d> }; token d { \d } }`, Inputs: []string{"12"}},
	// Backtracking through alternations, and switching ratchet off.
	{Src: `grammar RxAlt1 { regex TOP { [ a | ab ] c } }`, Inputs: []string{"abc", "ac"}},
	{Src: `grammar RxAlt2 { regex TOP { [ ab | a ] bc } }`, Inputs: []string{"abc", "abbc"}},
	{Src: `grammar NoRatchet { token TOP { [:!r \d* 5 ] } }`, Inputs: []string{"12345"}},
	// Zero-width calls, and \N.
	{Src: `grammar ZeroWidth { token TOP { <?digit> \w+ <!alpha> } }`, Sub: true, Inputs: []string{"1a2", "a12"}},
	{Src: `grammar NotNewline { token TOP { (\N+) \n (<-[\n]>+) } }`, Inputs: []string{"£1 x\nyz"}},
	// A frugal quantifier extends under ratchet through brackets, named
	// brackets and :i, but not through a capture, a call, an alternation or
	// another quantifier.
	{Src: `grammar Fr1 { token TOP { (.*?) ' - ' (.*) } }`, Inputs: []string{"Engineer - Payments - Remote"}},
	{Src: `grammar Fr2 { token TOP { [.*?] ' - ' (.*) } }`, Inputs: []string{"Engineer - Payments - Remote"}},
	{Src: `grammar Fr3 { token TOP { $<a>=[.*?] ' - ' (.*) } }`, Inputs: []string{"Engineer - Payments - Remote"}},
	{Src: `grammar Fr4 { token TOP { [ a .*? ] b } }`, Inputs: []string{"aaab"}},
	{Src: `grammar Fr5 { token TOP { <x> b }; token x { a .*? } }`, Inputs: []string{"aaab"}},
	{Src: `grammar Fr6 { token TOP { [ a .*? | z ] b } }`, Inputs: []string{"aaab"}},
	{Src: `grammar Fr7 { token TOP { [ a .*? || z ] b } }`, Inputs: []string{"aaab"}},
	{Src: `grammar Fr8 { token TOP { [ a .*? ]? b } }`, Inputs: []string{"aaab"}},
	{Src: `grammar Fr9 { token TOP { [:i a .*? ] b } }`, Inputs: []string{"Aaab"}},
	{Src: `grammar Fr10 { token TOP { [ [ a .*? ] ] b } }`, Inputs: []string{"aaab"}},
	// Word lists: literal text, the longest phrase, as one atom; :i applies,
	// and an empty list matches nothing.
	{Src: `grammar List3 { token TOP { @units } }`, Lists: units, Inputs: []string{"years", "yr", "YEAR", "year", "a.b", "axb"}},
	{Src: `grammar List9 { token TOP { $<k>=@units <.ws> $<j>=[ @units ]+ % ',' } }`, Lists: units, Inputs: []string{"yr wk,a.b", "yr"}},
	{Src: `grammar List4 { token TOP { :i @units } }`, Lists: units, Inputs: []string{"YEARS", "years"}},
	{Src: `grammar List5 { regex TOP { @units s } }`, Lists: units, Inputs: []string{"years", "wks"}},
	{Src: `grammar List6 { token TOP { <unit>+ % ',' }; token unit { @units } }`, Lists: units, Inputs: []string{"yr,wk,years", "yr,,wk"}},
	{Src: `grammar List7 { token TOP { a @empty b } }`, Lists: map[string][]string{"empty": {}}, Inputs: []string{"ab"}},
	{Src: `grammar List8 { token TOP { <title> .* }; token title { @titles } }`,
		Lists:  map[string][]string{"titles": {"software", "software engineer", "senior software engineer"}},
		Inputs: []string{"senior software engineers", "software engineer", "softwar"}},
	// Roles: composed rules call the grammar's, the grammar's own win, and
	// an extension overrides a composed rule.
	{Src: `role Rl1 { token TOP { <x> <y> }; token y { y } }; grammar Rg1 does Rl1 { token x { x } }`, Inputs: []string{"xy"}},
	{Src: `role Rl2 { token x { r } }; grammar Rg2 does Rl2 { token TOP { <x> }; token x { g } }`, Inputs: []string{"g", "r"}},
	{Src: `role Rl3 { token money { '£' \d+ } }; grammar Rg3 does Rl3 { token TOP { <money> } }; grammar Rg4 is Rg3 { token money { '$' \d+ } }`,
		Inputs: []string{"$5", "£5"}},
	{Src: `role Rl4 { token a { a } }; role Rl5 does Rl4 { token b { b } }; grammar Rg5 does Rl5 { token TOP { <a> <b> } }`, Inputs: []string{"ab"}},
	// Sigspace: in a rule, or after :s, whitespace after a term is <.ws>;
	// at the start of a rule, group or alternative it means nothing; between
	// an atom and its quantifier it's repeated with the atom; after a
	// separator it's part of the separator.
	{Src: `grammar Sig1 { token w { \w }; rule TOP { a b } }`, Inputs: []string{"ab", "a b", "a  b", " a b", "a b "}},
	{Src: `grammar Sig2 { token w { \w }; rule TOP {a b} }`, Inputs: []string{"a b", " a b", "a b "}},
	{Src: `grammar Sig3 { token w { \w }; rule TOP { a+ b } }`, Inputs: []string{"aa b", "a a b", "aab"}},
	{Src: `grammar Sig4 { token w { \w }; rule TOP { a + b } }`, Inputs: []string{"aa b", "a a b", "aab"}},
	{Src: `grammar Sig5 { token w { \w }; rule TOP { a +b } }`, Inputs: []string{"aab", "aa b", "a ab"}},
	{Src: `grammar Sig6 { token w { \w }; rule TOP { <w>+ % "," } }`, Inputs: []string{"x,y", "x , y", "x, y", "x ,y"}},
	{Src: `grammar Sig7 { token w { \w }; rule TOP { <w>+ % [ "," ] } }`, Inputs: []string{"x,y", "x , y", "x, y"}},
	{Src: `grammar Sig8 { token w { \w }; rule TOP { <w> + % "," } }`, Inputs: []string{"x , y", "x,y", "x, y"}},
	{Src: `grammar Sig9 { token w { \w }; rule TOP { <w>+ %"," } }`, Inputs: []string{"x , y", "x,y", "x, y"}},
	{Src: `grammar Sig10 { token w { \w }; rule TOP { <w>+ %% "," } }`, Inputs: []string{"x, y,", "x, y, ", "x,y"}},
	{Src: `grammar Sig11 { token w { \w }; rule TOP { [ a | b ] c } }`, Inputs: []string{"a c", "ac", "b c", " a c"}},
	{Src: `grammar Sig12 { token w { \w }; rule TOP { [a|b] c } }`, Inputs: []string{"a c", "ac"}},
	{Src: `grammar Sig13 { token w { \w }; rule TOP { [ a ] } }`, Inputs: []string{"a", "a ", " a"}},
	{Src: `grammar Sig14 { token w { \w }; rule TOP { a | b } }`, Inputs: []string{"a ", "b ", " b", "a", "b"}},
	{Src: `grammar Sig15 { token w { \w }; rule TOP { a| b } }`, Inputs: []string{"a", "a ", "b ", "b"}},
	{Src: `grammar Sig16 { token w { \w }; rule TOP { a || b } }`, Inputs: []string{"a ", "b ", "b"}},
	{Src: `grammar Sig17 { token w { \w }; rule TOP { "x y" } }`, Inputs: []string{"x y", "x  y"}},
	{Src: `grammar Sig18 { token w { \w }; rule TOP { :i a b } }`, Inputs: []string{"A B", "AB"}},
	{Src: `grammar Sig19 { token w { \w }; token TOP { :s a b } }`, Inputs: []string{"a b", "ab", "a b "}},
	{Src: `grammar Sig20 { token w { \w }; token TOP { a :s b c } }`, Inputs: []string{"a b c", "ab c", "a bc", "ab c "}},
	{Src: `grammar Sig21 { token w { \w }; rule TOP { a :!s b c } }`, Inputs: []string{"a bc", "a b c", "ab c"}},
	{Src: `grammar Sig22 { token w { \w }; token TOP { [ :s a b ] c } }`, Inputs: []string{"a bc", "a b c", "a b  c"}},
	{Src: `grammar Sig23 { token w { \w }; rule TOP { ( a ) b } }`, Inputs: []string{"a b", "ab"}},
	{Src: `grammar Sig24 { token w { \w }; rule TOP { ( a b ) c } }`, Inputs: []string{"a b c", "a bc"}},
	{Src: `grammar Sig25 { token w { \w }; rule TOP { $<x>=[ a ] b } }`, Inputs: []string{"a b", "ab"}},
	{Src: `grammar Sig26 { token w { \w }; rule TOP { $<x>=a b } }`, Inputs: []string{"a b", "ab"}},
	{Src: `grammar Sig27 { token w { \w }; rule TOP { $<x>=a + b } }`, Inputs: []string{"a a b"}},
	{Src: `grammar Sig28 { token w { \w }; rule TOP { a # c
 b } }`, Inputs: []string{"a b", "ab"}},
	{Src: `grammar Sig29 { token w { \w }; rule TOP { a? b } }`, Inputs: []string{"b", " b", "a b", "ab"}},
	{Src: `grammar Sig30 { token w { \w }; rule TOP { 1 2 } }`, Inputs: []string{"12", "1 2"}},
	{Src: `grammar Sig31 { token w { \w }; rule TOP { a ** 2 b } }`, Inputs: []string{"aa b", "a a b"}},
	{Src: `grammar Sig32 { token w { \w }; rule TOP { <w> } }`, Inputs: []string{"x", "x ", " x"}},
	{Src: `grammar Sig33 { token w { \w }; rule TOP { <.w> <w> } }`, Inputs: []string{"x y", "xy"}},
	{Src: `grammar Sig34 { token w { \w }; rule TOP { ^ a $ } }`, Inputs: []string{"a", " a", "a "}},
	{Src: `grammar Sig35 { token w { \w }; rule TOP { a <?before b> b } }`, Inputs: []string{"ab", "a b"}},
	{Src: `grammar Sig36 { token w { \w }; rule TOP { [ a b ]+ } }`, Inputs: []string{"a b a b", "a ba b", "a b a b "}},
	{Src: `grammar Sig37 { token w { \w }; rule TOP { <w>* } }`, Inputs: []string{"x y", "xy", ""}},
	{Src: `grammar Sig38 { token w { \w }; rule TOP { <x> }; rule x { a } }`, Inputs: []string{"a", "a ", " a"}},
	{Src: `grammar Sig39 { token w { \w }; rule TOP { <x> b }; token x { a } }`, Inputs: []string{"a b", "ab"}},
	{Src: `grammar Sig40 { token w { \w }; rule TOP { "," b } }`, Inputs: []string{", b", ",b"}},
	{Src: `grammar Sig41 { token w { \w }; rule TOP { <x> <w> }; token x { a }; token ws { '_'* } }`, Inputs: []string{"a_y", "a__y", "a y"}},
	{Src: `grammar Sig42 { token w { \w }; rule TOP { [ <w> ]+ % "," } }`, Inputs: []string{"x, y", "x ,y"}},
	// Quoted literals and escapes.
	{Src: `grammar Quote { token TOP { "a\tb" '\'' \- \[ } }`, Inputs: []string{"a\tb'-["}},
}

var units = map[string][]string{"units": {"year", "yr", "years", "wk", "a.b"}}

// The grammars match as Raku's do: the same text, the same captures, the
// same shape. It needs raku, and is skipped without it.
func TestMatchesRaku(t *testing.T) {
	if _, err := exec.LookPath("raku"); err != nil {
		t.Skip("raku isn't installed")
	}
	in, _ := json.Marshal(rakuCases)
	const script = `
use MONKEY-SEE-NO-EVAL;
sub conv($m) {
    given $m {
        when Match {
            my %h = '~' => ~$m, from => $m.from, to => $m.to;
            for $m.hash.kv -> $k, $v { %h{$k} = conv($v) if $v.defined }
            for $m.list.kv -> $i, $v { %h{~$i} = conv($v) if $v.defined }
            %h
        }
        when Positional { [ $m.map(&conv) ] }
        default { Any }
    }
}
my @out;
for @(Rakudo::Internals::JSON.from-json($*IN.slurp)) -> $case {
    my $prelude = ($case<lists> // {}).kv.map(-> $k, $v {
        "my \@$k = " ~ ($v.elems ?? $v.map({ "'" ~ .subst("\\", "\\\\", :g).subst("'", "\\'", :g) ~ "'" }).join(", ") !! "()") ~ ";\n"
    }).join;
    my $g = EVAL $prelude ~ $case<src>;
    @out.push: [ $case<inputs>.map: -> $in {
        my $m = $case<sub> ?? $g.subparse($in) !! $g.parse($in);
        $m ?? conv($m) !! Any
    } ];
}
print Rakudo::Internals::JSON.to-json(@out, :!pretty);
`
	cmd := exec.Command("raku", "-e", script)
	cmd.Stdin = strings.NewReader(string(in))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("raku: %v: %s", err, stderr.String())
	}
	want, err := scalar.FromJSON(out)
	if err != nil {
		t.Fatalf("raku's output: %v", err)
	}
	for i, c := range rakuCases {
		g, err := Compile(c.Src, caseLists(c.Lists)...)
		if err != nil {
			t.Errorf("case %d: %v", i, err)
			continue
		}
		for j, input := range c.Inputs {
			var got *scalar.SV
			if c.Sub {
				got = g.Subparse(input)
			} else {
				got = g.Parse(input)
			}
			w := want.AV().Get(i).AV().Get(j)
			stripMade(got) // Raku's <@list> has no canonical values
			if d := got.Diff(w); d != "" {
				gj, _ := got.MarshalJSON()
				wj, _ := w.MarshalJSON()
				t.Errorf("%s on %q: %s\n got: %s\nwant: %s", g.Name(), input, d, gj, wj)
			}
		}
	}
}

// caseLists turns a case's lists into exact word lists, as Raku's arrays
// are.
func caseLists(lists map[string][]string) []Use {
	var uses []Use
	for name, phrases := range lists {
		l := NewWordList(name)
		for _, p := range phrases {
			l.Add(p)
		}
		uses = append(uses, l)
	}
	return uses
}

// stripMade deletes made values throughout a match.
func stripMade(m *scalar.SV) {
	switch m.Kind() {
	case scalar.HashRef:
		m.HV().Delete("made")
		for _, v := range m.HV().All() {
			stripMade(v)
		}
	case scalar.ArrayRef:
		for _, v := range m.AV().All() {
			stripMade(v)
		}
	}
}

// The Raku cases compile here even where raku isn't installed.
func TestRakuCasesCompile(t *testing.T) {
	for i, c := range rakuCases {
		if _, err := Compile(c.Src, caseLists(c.Lists)...); err != nil {
			t.Errorf("case %d: %v\n%s", i, err, c.Src)
		}
	}
}
