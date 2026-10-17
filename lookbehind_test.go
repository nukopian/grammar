package grammar

import (
	"testing"

	"github.com/nukopian/job-meta/internal/scalar"
)

// withoutBounds runs f with lookbehinds trying every start, as before.
func withoutBounds(f func()) {
	defer func() { boundLookbehind = true }()
	boundLookbehind = false
	f()
}

// matchAll is what a grammar makes of text, to compare: its parse,
// subparse and every match FindAll finds.
func matchAll(g *Grammar, text string) *scalar.SV {
	return scalar.Value([]any{g.Parse(text), g.Subparse(text), scalar.Value(g.FindAll(text))})
}

// Narrowing a lookbehind's starts changes no match, over every Raku case
// and lookbehinds of every shape of width.
func TestLookbehindChangesNothing(t *testing.T) {
	type tc struct {
		g     *Grammar
		texts []string
	}
	var cases []tc
	for _, c := range rakuCases {
		g, err := Compile(c.Src, caseUses(c.Lists, c.Patterns)...)
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, tc{g, c.Inputs})
	}
	texts := []string{"£45k and £55,000 a year. What next: Lead", "ab. ab:  What a  b", "xxyx", "", "Ab aB", "1 2 3",
		"up to 40 hours, Max 48, upTo 3", "backup to 4", "up   to\tx"}
	for _, src := range []string{
		`grammar L1 { token TOP { <?after '£'> \d+ } }`,
		`grammar L2 { regex TOP { <?after <[.!?:]> \s+> What } }`,
		`grammar L3 { token TOP { <?after \d ** 2..3 > \s } }`,
		`grammar L4 { token TOP { <?after [ ab | b ]> \s* <:Lu> } }`,
		`grammar L5 { token TOP { <!after x> y } }`,
		`grammar L6 { token TOP { <?after <w> '.'> \s }; token w { \w+ } }`,
		`grammar L7 { token TOP { <?after :i ab> . } }`,
		`grammar L8 { token TOP { <?after a? > b } }`,
		`grammar L9 { token TOP { <?after [ x+ ]+ % y > \s? } }`,
		`grammar L10 { token TOP { <?after <@p> > . } }`,
		`grammar L11 { token TOP { <?after [ \w+ & ab ] > . } }`,
		`grammar L12 { token TOP { <?after '(' ~ ')' \w > . } }`,
		`grammar L13 { token TOP { <?after <r> > . }; token r { a <r>? } }`,
		`grammar L14 { token TOP { <!after @h \s+> \d } }`,
		`grammar L15 { token TOP { <?after @h \s ** 1..3> \w } }`,
		`grammar L16 { token TOP { <?after @loose> \s } }`,
	} {
		cases = append(cases, tc{MustCompile(src, MustPatternList("p", `a`, `ab`, `\d+`), hedgeList(), looseList()), texts})
	}
	for _, c := range cases {
		for _, text := range c.texts {
			with := matchAll(c.g, text)
			var without *scalar.SV
			withoutBounds(func() { without = matchAll(c.g, text) })
			if d := with.Diff(without); d != "" {
				t.Errorf("%s on %q: %s", c.g.Name(), text, d)
			}
		}
	}
}

// hedgeList and looseList are word lists for lookbehinds to use: one with
// a bounded width, one without.
func hedgeList() *WordList {
	return NewWordList("h", FoldCase(), WholeWords(), Plurals()).Add("up to").Add("max").Add("ab")
}

func looseList() *WordList {
	return NewWordList("loose", FoldCase(), LooseSpacing()).Add("up to").Add("ab")
}

// The width of what a pattern can match, as lookbehinds use it.
func TestWidth(t *testing.T) {
	g := MustCompile(`grammar W { token TOP { x }; token w { ab }; token r { a <r>? } }`, hedgeList(), looseList())
	for src, want := range map[string][2]int{
		`'£'`:              {1, 1},
		`\d ** 2..3`:       {2, 3},
		`\d+`:              {1, -1},
		`[ ab | b ]`:       {1, 2},
		`<w> '.'`:          {3, 3},
		`<r>`:              {1, -1}, // recursive: no telling how long
		`x+ % y`:           {1, -1},
		`x ** 2 % y`:       {3, 3},
		`x ** 1..3 % ','`:  {1, 5},
		`x ** 1..2 %% ','`: {1, 4},
		`[ \w+ & ab ]`:     {2, 2},
		`'(' ~ ')' \w`:     {3, 3},
		`a?`:               {0, 1},
		`@h`:               {0, 6}, // its longest phrase is the plural "up tos"
		`@h \s`:            {1, 7},
		`@loose`:           {0, -1}, // any run of spacing matches a space
	} {
		body := MustCompile(`grammar T is W { token t { `+src+` } }`, g, hedgeList(), looseList()).lookup("t").body
		lo, hi := g.width(body, map[string]bool{})
		if [2]int{lo, hi} != want {
			t.Errorf("%s: width %d..%d, want %d..%d", src, lo, hi, want[0], want[1])
		}
	}
}
