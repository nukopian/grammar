package grammar

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// rakuCases are grammars in Raku syntax, each with inputs to parse. Each
// grammar has its own name, as Raku declares them all in one program.
var rakuCases = []struct {
	Src    string              `json:"src"`
	Sub    bool                `json:"sub"` // subparse rather than parse
	Inputs []string            `json:"inputs"`
	Lists  map[string][]string `json:"lists,omitempty"` // word lists, as Raku arrays
	// Pattern lists, for <@name>: Raku arrays of regexes.
	Patterns map[string][]string `json:"patterns,omitempty"`
}{
	// The pay grammar from the how-to guide's quick start.
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
	// >> inside a lookaround is a word boundary, not the lookaround's end
	// (found dogfooding job-meta's internal/hours, 2026-10-17).
	{Src: `grammar LookBoundary {
	    token TOP { <!before [ \s | ',' | << [:i as \s+ a ] >> ]* x> \w+ <?after \w >> > }
	}`, Sub: true, Inputs: []string{"as a max", "x y", "abc"}},
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
	// A named capture inside $<k>=[ ... ] belongs to TOP, not to k.
	{Src: `grammar Named4 { token TOP { $<k>=[ [ \d+ \s+ ]? $<u>=@units ] } }`, Lists: units, Inputs: []string{"3 yr", "wk"}},
	// $<k>= on a ( ... ), a rule, a word list or a pattern list names each
	// match, and a quantifier after it repeats the named capture; on
	// anything else, the quantifier is inside the capture, text only.
	{Src: `grammar NamedQ1 { token TOP { $<e>=( a (\d) )? (x) } }`, Inputs: []string{"a1x", "x"}},
	{Src: `grammar NamedQ2 { token TOP { $<e>=( a (\d) )* (x) } }`, Inputs: []string{"a1a2x", "x"}},
	{Src: `grammar NamedQ3 { token TOP { $<e>=( a (\d) )+ % ',' (x) } }`, Inputs: []string{"a1,a2x", "x"}},
	{Src: `grammar NamedQ4 { token TOP { $<e>=( a (\d) ) ** 2 (x) } }`, Inputs: []string{"a1a2x", "a1x"}},
	{Src: `grammar NamedQ5 { token TOP { $<e>=<d>? $<f>=<.d>* (x) }; token d { (\d) } }`, Inputs: []string{"12x", "1x", "x"}},
	{Src: `grammar NamedQ6 { token TOP { $<e>=@units* % ',' $<f>=<@pu>? (x) } }`, Lists: units,
		Patterns: map[string][]string{"pu": {"yr", "wk"}}, Inputs: []string{"yr,wkyrx", "x"}},
	{Src: `grammar NamedQ7 { token TOP { $<e>=[ a (\d) ]* $<f>=<[ab]>+ $<g>='c'? $<h>=\d+ (x) } }`, Inputs: []string{"a1a2abc12x", "b3x"}},
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
	// A run of bare letters is one literal, except where Raku binds to a
	// single letter: before a quantifier, after $<x>=, and as a separator.
	{Src: `grammar Bare1 { token TOP { ab* c } }`, Inputs: []string{"abbbc", "ac", "ababc"}},
	{Src: `grammar Bare2 { token TOP { abc ** 2 } }`, Inputs: []string{"abcc", "abcabc"}},
	{Src: `grammar Bare3 { token TOP { abc ? d } }`, Inputs: []string{"abd", "abcd"}},
	{Src: `grammar Bare4 { token TOP { $<x>=ab c } }`, Inputs: []string{"abc"}},
	{Src: `grammar Bare5 { token TOP { x+ % ab } }`, Inputs: []string{"xaxb", "xabx"}},
	{Src: `grammar Bare6 { rule TOP { ab + c } }`, Inputs: []string{"ab b c", "abb c"}},
	{Src: `grammar Bare7 { token TOP { :i hello <.ws> world } }`, Inputs: []string{"Hello WORLD"}},
	{Src: `grammar Bare8 { token TOP { a1_b2+ } }`, Inputs: []string{"a1_b22", "a1_b2"}},
	// Quoted literals and escapes.
	{Src: `grammar Quote { token TOP { "a\tb" '\'' \- \[ } }`, Inputs: []string{"a\tb'-["}},
	// Capture order, for Caps and Chunks: an alias after its rule's name,
	// whichever sorts first; an empty capture before a longer one at the
	// same place; positional and named captures mixed; captures of a
	// repeat interleaved; and text between captures.
	{Src: `grammar Order1 { token TOP { <A=a> <z=a> }; token a { a } }`, Inputs: []string{"aa"}},
	{Src: `grammar Order2 { token TOP { $<e>=[ <?> ] <w> $<f>=[ <?> ] }; token w { \w+ } }`, Inputs: []string{"ab"}},
	{Src: `grammar Order8 { token TOP { $<z>=[ <?> ] <a> }; token a { \w+ } }`, Inputs: []string{"ab"}},
	{Src: `grammar Order3 { token TOP { (a) $<n>=[b] (c) } }`, Inputs: []string{"abc"}},
	{Src: `grammar Order4 { token TOP { ( <a> ( <a> ) ) 'z' }; token a { a } }`, Inputs: []string{"aaz"}},
	{Src: `grammar Order5 { token TOP { [ <a> | <b> ]+ % ',' }; token a { a }; token b { b } }`, Inputs: []string{"a,b,a"}},
	{Src: `grammar Order6 { token TOP { 'x' <.a> 'y' }; token a { a } }`, Inputs: []string{"xay"}},
	{Src: `grammar Always { token TOP { a <?> b [ <!> c | d ] } }`, Inputs: []string{"abd", "abc"}},
	// Goal matching: '(' ~ ')' X is '(' X ')'. A missing closer fails, and
	// can backtrack into another branch; X is one atom.
	{Src: `grammar Goal1 { token TOP { 'x' '(' ~ ')' <w> 'y'? }; token w { \w+ } }`,
		Inputs: []string{"x(ab)y", "x(ab)", "x(ab", "x(a-b)", "y(ab)"}},
	{Src: `grammar Goal2 { token TOP { [ '(' ~ ')' <w> || '(' \w+ ] }; token w { \w+ } }`, Inputs: []string{"(ab)", "(ab"}},
	{Src: `grammar Goal3 { token TOP { '[' ~ ']' \w+ ',' } }`, Inputs: []string{"[ab],", "[ab,]"}},
	{Src: `grammar Goal4 { rule TOP { '(' ~ ')' <w> }; token w { \w+ } }`, Inputs: []string{"( ab )", "(ab)", "( ab ) "}},
	{Src: `grammar Goal5 { token TOP { <w>* % ',' }; token w { '(' ~ ')' \w+ } }`, Inputs: []string{"(a),(b)", "(a),(b"}},
	{Src: `grammar Goal6 { token TOP { ( '(' ) ~ ( ')' ) ( \w+ ) } }`, Inputs: []string{"(ab)"}},
	{Src: `grammar Goal7 { token TOP { '(' ~ ')' [ <w>+ % ',' ] }; token w { \w+ } }`, Inputs: []string{"(a,b)", "(a,b"}},
	{Src: `grammar Goal8 { regex TOP { '(' ~ ')' .* } }`, Inputs: []string{"(a)b)", "(ab"}},
	// Unicode properties: general categories by short or long name, scripts,
	// binary properties, negated and combined with sets. Under :i a set
	// folds case, and a property or a named class doesn't.
	{Src: `grammar Uni1 { token TOP { <:Lu> <:Letter>+ <:Nd>* } }`, Inputs: []string{"Ab1", "Ж中٣", "ab1", "A"}},
	{Src: `grammar Uni2 { token TOP { [ <:Script<Latin>> | <:Cyrillic> ]+ } }`, Inputs: []string{"aЖé", "a中", "Ⅷ"}},
	{Src: `grammar Uni3 { token TOP { <:!Lu>+ <-:L>* } }`, Inputs: []string{"ab12", "Ab", "a-_"}},
	{Src: `grammar Uni4 { token TOP { <:L + [_]>+ <[a..z] - :Lu>? } }`, Inputs: []string{"a_Ж", "_x", "aB"}},
	{Src: `grammar Uni5 { token TOP { <:Alpha>+ <:White_Space> <:Dash> <:Uppercase_Letter> <:lu> } }`, Inputs: []string{"aⅧ –AB", "a -Ab"}},
	{Src: `grammar Uni6 { token TOP { :i <:Lu> <[A..Z]> <-[A..Z]> <:Lu + [x]> <[a..z] - :Ll> } }`,
		Inputs: []string{"Aa1XA", "aa1XA", "AA1XA", "Aa1xA", "Aa1Xa"}},
	{Src: `grammar Uni7 { token TOP { :i <+upper> <-upper> } }`, Inputs: []string{"Aa", "aa", "AA"}},
	// Conjunctions: every branch matches the same text. In a regex they
	// backtrack until they agree; in a token each takes its first match.
	// & binds tighter than |, and && than ||.
	{Src: `grammar Conj1 { token TOP { [ \w+ & <[a..c]>+ ] } }`, Inputs: []string{"abc", "abcd"}},
	{Src: `grammar Conj2 { regex TOP { [ \w+ && ab ] c } }`, Inputs: []string{"abc", "abd"}},
	{Src: `grammar Conj3 { token TOP { [ \w+ && ab ] c } }`, Inputs: []string{"abc"}},
	{Src: `grammar Conj4 { token TOP { <x=w> & <y=w> }; token w { \w+ } }`, Inputs: []string{"ab"}},
	{Src: `grammar Conj5 { token TOP { a & a | b } }`, Inputs: []string{"a", "b"}},
	{Src: `grammar Conj6 { token TOP { a && b || c } }`, Inputs: []string{"c", "a"}},
	{Src: `grammar Conj7 { token TOP { [ ab & a ] } }`, Sub: true, Inputs: []string{"ab"}},
	{Src: `grammar Conj8 { token TOP { (\w) (\w) & (..) } }`, Inputs: []string{"ab"}},
	{Src: `grammar Conj9 { token TOP { & a & . && \w } }`, Inputs: []string{"a", "b"}},
	{Src: `grammar Conj10 { regex TOP { [ <:Lu> \S* ]+ % ' ' & [ \w+ ]+ % ' ' } }`, Inputs: []string{"Senior Engineer", "Senior engineer", "Senior C++"}},
	// Multiple inheritance: rules are looked up in C3 order, as Raku's
	// classes are, so in a diamond the right-hand parent's override beats
	// the shared base, though the left-hand parent comes first.
	{Src: `grammar MiA1 { token x { a }; token y { a } }
	grammar MiB1 { token x { b }; token z { b } }
	grammar MiC1 is MiA1 is MiB1 { token TOP { <x> <y> <z> } }`, Inputs: []string{"aab", "bab"}},
	{Src: `grammar MiA2 { token x { a }; token y { a } }
	grammar MiB2 { token x { b }; token z { b } }
	grammar MiD2 is MiB2 is MiA2 { token TOP { <x> <y> <z> } }`, Inputs: []string{"bab", "aab"}},
	{Src: `grammar MiBase3 { token w { base }; token v { base }; token TOP { <w> <v> } }
	grammar MiL3 is MiBase3 { token v { l } }
	grammar MiR3 is MiBase3 { token w { r } }
	grammar MiBottom3 is MiL3 is MiR3 { }`, Inputs: []string{"rl", "basel", "basebase", "rbase"}},
	{Src: `grammar MiP4 { token TOP { <a> <b> }; token a { x }; token b { p } }
	role MiRole4 { token b { r } }
	grammar MiQ4 { token b { q } }
	grammar MiS4 is MiP4 is MiQ4 does MiRole4 { token a { y } }`, Inputs: []string{"yr", "yq", "xr", "yp"}},
	// Text is read in NFC, so a letter and a combining accent are one
	// character, as in Raku: positions count it once, . takes it whole, and
	// composed and decomposed spellings match each other, on either side.
	{Src: `grammar Nfc1 { token TOP { 'Café' \s+ <w> }; token w { \w+ } }`, Inputs: []string{"Cafe\u0301 ok", "Caf\u00e9 ok"}},
	{Src: `grammar Nfc2 { token TOP { . . $<rest>=[ \w+ ] } }`, Inputs: []string{"e\u0301xyz", "\u00e9xyz"}},
	{Src: "grammar Nfc3 { token TOP { 'Cafe\u0301' } }", Inputs: []string{"Caf\u00e9", "Cafe\u0301"}},
	{Src: `grammar Nfc4 { token TOP { <[a..z é]>+ } }`, Inputs: []string{"re\u0301sume\u0301", "r\u00e9sum\u00e9"}},
	// : after an atom: never backtrack into it, as a token wouldn't. Raku
	// reads ': b', with a space, as : then b, and :b as an adverb.
	{Src: `grammar Colon1 { regex TOP { [ a | ab ]: b } }`, Inputs: []string{"abb", "ab"}},
	{Src: `grammar Colon2 { regex TOP { \w+: b } }`, Inputs: []string{"aab"}},
	{Src: `grammar Colon3 { regex TOP { <w>: c }; regex w { a+ } }`, Inputs: []string{"aac"}},
	{Src: `grammar Colon4 { regex TOP { <w>: b }; regex w { a+ b? } }`, Inputs: []string{"aab"}},
	{Src: `grammar Colon5 { regex TOP { <w> b }; regex w { a+ b? } }`, Inputs: []string{"aab"}},
	{Src: `grammar Colon6 { regex TOP { [ a | ab ] : b } }`, Inputs: []string{"abb", "ab"}},
	{Src: `grammar Colon7 { regex TOP { (a+): $<rest>=[ a* ]: a } }`, Inputs: []string{"aaa"}},
	{Src: `grammar Colon8 { regex TOP { a :i B } }`, Inputs: []string{"ab", "aB"}},
	// proto and :sym<…>: a call to the proto tries every variant the
	// grammar can see, extensions' and roles' included, and keeps the
	// longest, committing to it, even in a regex. Its match is the
	// variant's, and <sym> matches the variant's name.
	{Src: `grammar Proto1 {
	    token TOP { <period> }
	    proto token period {*}
	    token period:sym<annum> { :i 'per annum' }
	    token period:sym<hour>  { :i 'per ' <sym> }
	    token period:sym<per>   { :i per }
	}`, Inputs: []string{"per annum", "Per Hour", "per", "per week"}},
	{Src: `grammar Proto2 {
	    token TOP { <period> }
	    proto token period {*}
	    token period:sym<annum> { :i 'per annum' }
	    token period:sym<hour>  { :i 'per hour' }
	}
	grammar Proto2UK is Proto2 { token period:sym<pw> { :i pw }; token period:sym<hour> { :i ph } }`,
		Inputs: []string{"pw", "ph", "per hour", "per annum"}},
	{Src: `grammar Proto3 { token TOP { <period> }; proto token period {*}; token period:sym<annum> { 'per annum' } }
	role Proto3Weekly { token period:sym<week> { 'per week' } }
	grammar Proto3W is Proto3 does Proto3Weekly { }`, Inputs: []string{"per week", "per annum"}},
	{Src: `grammar Proto4 { token TOP { <x> }; proto token x {*}; token x:sym<b> { ab }; token x:sym<a> { a. } }`, Inputs: []string{"ab", "ac"}},
	{Src: `grammar Proto4b { token TOP { <x> }; proto token x {*}; token x:sym<a> { a $<second>=[ . ] }; token x:sym<b> { $<first>=[ a ] b } }`, Inputs: []string{"ab", "ac"}},
	{Src: `grammar Proto6 { regex TOP { <x> b }; proto regex x {*}; regex x:sym<long> { ab }; regex x:sym<short> { a } }`, Inputs: []string{"ab", "abb"}},
	{Src: `grammar Proto7 { token TOP { <op>+ }; proto token op {*}; token op:sym<+> { <sym> }; token op:sym«->» { <sym> } }`, Inputs: []string{"+->", "-+", "+-"}},
	{Src: `grammar Proto8 { token TOP { <unit>+ % ', ' }; proto token unit {*}; token unit:sym<year> { <sym> s? }; token unit:sym<week> { <sym> } }`, Inputs: []string{"years, week, year", "weeks"}},
	// <@list>: each entry a regex, the longest winning and the call
	// committing to it, even in a regex. Entries see the grammar's rules,
	// and :i, but not a rule's spaces, and their captures aren't kept.
	{Src: `grammar Pat1 { token TOP { <@r> } }`, Patterns: map[string][]string{"r": {"a", "ab", "a+"}}, Inputs: []string{"ab", "aaa", "b"}},
	{Src: `grammar Pat2 { regex TOP { <@o> b } }`, Patterns: map[string][]string{"o": {"a", "ab"}}, Inputs: []string{"ab", "abb"}},
	{Src: `grammar Pat3 { token TOP { :i <@ci> } }`, Patterns: map[string][]string{"ci": {"perannum", "'per hour'"}}, Inputs: []string{"PERANNUM", "Per Hour"}},
	{Src: `grammar Pat4 { token TOP { <@calls> }; token w { \w } }`, Patterns: map[string][]string{"calls": {"<w>x", "<alpha>+"}}, Inputs: []string{"qx", "abc"}},
	{Src: `grammar Pat5 { token TOP { <@caps> \s* (\w) } }`, Patterns: map[string][]string{"caps": {`$<n>=[\d+]k`, `(\d+)m`}}, Inputs: []string{"45k x", "7m y"}},
	{Src: `grammar Pat6 { token TOP { x <@empty>? y } }`, Patterns: map[string][]string{"empty": {}}, Inputs: []string{"xy"}},
	{Src: `grammar Pat7 { rule TOP { <@sp> } }`, Patterns: map[string][]string{"sp": {"per annum"}}, Inputs: []string{"perannum", "per annum"}},
	{Src: `grammar Pat8 { token TOP { <@r>+ % ',' } }`, Patterns: map[string][]string{"r": {`\d+`, `<[a..c]>+`, `'x y'`}}, Inputs: []string{"12,ab,x y", "12,d"}},
	// Backreferences: $<name> and $0 match the text the capture last
	// matched, exactly, even under :i, and fail if it hasn't matched.
	{Src: `grammar Back1 { token TOP { $<q>=<['"]> \w+ $<q> } }`, Inputs: []string{"'ab'", `"ab"`, `'ab"`}},
	{Src: `grammar Back2 { token TOP { (\d) \- $0 } }`, Inputs: []string{"3-3", "3-4"}},
	{Src: `grammar Back3 { token TOP { [ $<x>=[a] ]? $<x> c } }`, Inputs: []string{"c", "aac"}},
	{Src: `grammar Back4 { token TOP { [ $<x>=[\w] ]+ % \- \s $<x> } }`, Inputs: []string{"a-b a b", "a-b ab", "a-b b"}},
	{Src: `grammar Back5 { token TOP { :i $<q>=[\w+] \s $<q> } }`, Inputs: []string{"ab AB", "ab ab"}},
	{Src: `grammar Back6 { token TOP { \d+ $<s>=<[,.]> \d ** 3 \s* \- \s* \d+ $<s> \d ** 3 } }`, Inputs: []string{"45,000 - 55,000", "45,000 - 55.000"}},
	{Src: `grammar Back7 { token TOP { <a> $<a> }; token a { x } }`, Inputs: []string{"xx", "xy"}},
	{Src: `grammar Back8 { token TOP { [ $<q>=[a] b $<q> ]+ } }`, Inputs: []string{"abaaba", "abaabb"}},
	{Src: `grammar Back9 { token TOP { $<q>=[a] $<r>=$<q> } }`, Inputs: []string{"aa"}},
	{Src: `grammar Back10 { regex TOP { $<x>=[a+] $<x> } }`, Inputs: []string{"aaaa", "aaa"}},
	{Src: `grammar Back11 { token TOP { $<q>=[\w] $<q>+ } }`, Inputs: []string{"aaa", "ab"}},
	// Spaces before a capture's = are allowed; in a rule, the space after a
	// backreference is significant, as after any atom.
	{Src: `grammar Back12 { token TOP { $<x> = [ a ] b $<y>= c } }`, Inputs: []string{"abc"}},
	{Src: `grammar Back13 { rule TOP { $<q>=[a] $<q> b } }`, Inputs: []string{"a a b", "a ab", "aa b"}},
	{Src: `grammar Order7 { token TOP { <a> ' '? <b>+ (x) <c=a> $<n>=[ \d+ ] <.a>? '-' <d=.b> }; token a { a }; token b { b } }`,
		Inputs: []string{"a bbxa12a-b", "abxa1-b"}},
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
    my $prelude = (|($case<lists> // {}).pairs, |($case<patterns> // {}).pairs).map(-> $p {
        my ($k, $v) = $p.kv;
        "my \@$k = " ~ ($v.elems ?? $v.map({ "'" ~ .subst("\\", "\\\\", :g).subst("'", "\\'", :g) ~ "'" }).join(", ") !! "()") ~ ";\n"
    }).join;
    my $g = EVAL $prelude ~ $case<src>;
    @out.push: [ $case<inputs>.map: -> $in {
        my $m = $case<sub> ?? $g.subparse($in) !! $g.parse($in);
        $m<unread><y>; # reading creates nothing
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
		g, err := Compile(c.Src, caseUses(c.Lists, c.Patterns)...)
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
			got.At("unread.y") // reading creates nothing, as in Raku
			w := want.Get(i, j)
			stripMade(got) // Raku's <@list> has no canonical values
			if d := got.Diff(w); d != "" {
				gj, _ := got.MarshalJSON()
				wj, _ := w.MarshalJSON()
				t.Errorf("%s on %q: %s\n got: %s\nwant: %s", g.Name(), input, d, gj, wj)
			}
		}
	}
}

// caseUses turns a case's lists into exact word lists, as Raku's arrays
// are, and its patterns into pattern lists.
func caseUses(lists, patterns map[string][]string) []Use {
	uses := caseLists(lists)
	for name, ps := range patterns {
		uses = append(uses, MustPatternList(name, ps...))
	}
	return uses
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
		m.Delete("made")
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
		if _, err := Compile(c.Src, caseUses(c.Lists, c.Patterns)...); err != nil {
			t.Errorf("case %d: %v\n%s", i, err, c.Src)
		}
	}
}
