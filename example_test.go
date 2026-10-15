package grammar_test

import (
	"fmt"

	"github.com/nukopian/job-meta/internal/scalar"
	"github.com/nukopian/job-meta/internal/scalar/grammar"
)

// Pay is the grammar many of the examples share: a pay figure or range,
// such as "£45,000 - £55,000 per annum".
//
// Read a rule as a description of the text it matches. TOP, where parsing
// starts, is: a <money> (captured as min), then optionally a separator and
// another <money> (captured as max), then optionally a <period>.
var Pay = grammar.MustCompile(`
grammar Pay {
    token TOP    { <min=money> [ <.sep> <max=money> ]? <period>? }
    token money  { '£' \s? <amount> }
    token amount { <num> [ $<k>=[:i k] ]? }
    token num    { \d ** 1..3 [ ',' \d ** 3 ]* [ '.' \d+ ]? }
    token sep    { \s* [ '-' | '–' | to ] \s* }
    token period { :i \s* [ 'p.a.' | pa | per \s+ [ annum | year | hour | day ] ] }
}`)

// PayActions turns matches into values, as a Raku actions class does: each
// method is named after a rule, and is called with that rule's match.
type PayActions struct{}

// Amount is called for each <amount>: "45,000" is 45000, and "45k" is too.
// m is the <amount> match; "num.~" is the text of the <num> inside it.
func (PayActions) Amount(m *scalar.SV) any {
	n := m.At("num.~").S(`,`, "", "gr").Num()
	if m.At("k").Defined() {
		n *= 1000
	}
	return n
}

// The whole idea in one example: find pay in a sentence, and read the
// figures the actions made from it.
func Example() {
	text := "Salary: £45k – £55,000 per annum, plus bonus."

	m := Pay.Find(text, grammar.Actions(PayActions{}))
	fmt.Println(m.At("~")) // the text of the whole match
	fmt.Println(m.At("min.amount.made"), m.At("max.amount.made"))
	fmt.Println(m.At("period.~"))
	fmt.Println(m.At("from"), m.At("to")) // where in text it was
	// Output:
	// £45k – £55,000 per annum
	// 45000 55000
	//  per annum
	// 8 32
}

// A whole match, printed as YAML to show its shape. Every match is a hash
// with its text under "~" and its position under "from" and "to". Each
// <rule> it called is a match of its own, under the rule's name, so the
// tree follows the grammar. <.sep> has a dot, so it isn't captured.
//
// <low=n> captures under both low and n, as in Raku, and as n is captured
// twice, n is an array. low and n's first element are the same match, not
// copies, as in Raku; YAML shows that with an anchor, &2, and a reference
// back to it, *2.
func Example_matchTree() {
	g := grammar.MustCompile(`
grammar Range {
    token TOP { <low=n> <.sep> <high=n> }
    token n   { \d+ }
    token sep { \s* '-' \s* }
}`)
	out, _ := g.Parse("18 - 21").ToYAML()
	fmt.Print(string(out))
	// Output:
	// from: 0
	// high: &1
	//   from: 5
	//   to: 7
	//   "~": "21"
	// low: &2
	//   from: 0
	//   to: 2
	//   "~": "18"
	// "n":
	//   - *2
	//   - *1
	// to: 7
	// "~": 18 - 21
}

// The workflow this package is for: synonym lists, such as you'd pull from a
// synonym store, a grammar that says where each kind of word goes, and an
// action that hands back typed Go. Every spelling of a title, a seniority
// or a working arrangement comes out as its canonical value.
func Example_titles() {
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

	type Title struct {
		Seniority   string `json:"seniority"`
		Title       string `json:"title"`
		Team        string `json:"team"`
		Arrangement string `json:"arrangement"`
	}
	toTitle := grammar.Actions(map[string]func(*scalar.SV) any{
		"TOP": func(m *scalar.SV) any {
			return scalar.Hash(
				"seniority", m.At("seniority.made").Or(""),
				"title", m.At("title.made"),
				"team", m.At("team.~").Or(""),
				"arrangement", m.At("arrangement.made").Or(""),
			)
		},
	})

	for _, text := range []string{
		"Snr SWE – Payments (WFH)",
		"product manager | Growth (office based)",
		"Lead Software Developer",
	} {
		t, err := scalar.To[Title](g.Parse(text, toTitle).At("made"))
		fmt.Printf("%+v %v\n", t, err)
	}
	// Output:
	// {Seniority:Senior Title:Software Engineer Team:Payments Arrangement:Remote} <nil>
	// {Seniority: Title:Product Manager Team:Growth Arrangement:Onsite} <nil>
	// {Seniority:Lead Title:Software Engineer Team: Arrangement:} <nil>
}
