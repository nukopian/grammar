package grammar

import (
	"strconv"
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// checkPathGrammar has each kind of capture: an alias, an alias only, a
// repeat, text captures with and without a word list, a ( ... ) capture
// with captures of its own, positional captures, a proto, a lookaround and
// a pattern list.
func checkPathGrammar(t *testing.T) *Grammar {
	t.Helper()
	levels := NewWordList("levels", FoldCase()).Add("Senior", "snr")
	shifts := MustPatternList("shifts", `(early) | late`)
	g, err := Compile(`
grammar Pay {
    token TOP    { <!before <week>> $<title>=[ $<level>=@levels \s+ \w+ ]? <min=money> [ <.sep> <max=money> ]?
                   <low=.money>? <per>? <tag>* [ $<extra>=( <day> (\d) ) ]? <unit>? <@shifts>? ( x )? }
    token money  { '£' $<amount>=[ \d+ ] $<k>=[:i k]? }
    token sep    { \s* '-' \s* }
    token per    { \s* per \s+ [ <week> | <day> ] }
    token week   { week }
    token day    { day }
    token tag    { \s* '#' (\w+) }
    proto token unit {*}
    token unit:sym<hour>  { hour $<s>=[s]? }
    token unit:sym<shift> { shift <day>? }
}`, levels, shifts)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestCheckPath(t *testing.T) {
	g := checkPathGrammar(t)
	for _, c := range []struct {
		path string
		opts []Option
		want string // "" for no error, or what the error says
	}{
		// The match itself, and what every match has.
		{path: ""},
		{path: "~"},
		{path: "from"},
		{path: "to"},
		// An action may make anything, so whatever follows made is taken.
		{path: "made"},
		{path: "made.any[0].thing"},
		{path: "min.made"},
		// Captures, through rules, aliases and text captures.
		{path: "min.amount"},
		{path: "money[1].amount.~"}, // min and max are money too
		{path: "money.amount", want: `money is a list of matches, as it can capture more than once: index it, as money[0]`},
		{path: "max.k"},
		{path: "low.amount"},
		{path: "per.week"},
		{path: "per.day.from"},
		{path: "title.~"},
		{path: "level.made"},
		{path: "$.min.amount"},
		{path: `{min}["amount"]`},
		// A repeat is a list, indexed from either end.
		{path: "tag[0].0"},
		{path: "tag.0.0.~"},
		{path: "tag[-1]"},
		// A ( ... ) capture has captures of its own.
		{path: "extra.day"},
		{path: "extra.0.~"},
		{path: "0"},
		{path: "[\"0\"].~"},
		// A proto's capture is any of its variants' matches.
		{path: "unit.s"},
		{path: "unit.day.~"},
		// Rule starts the path at another rule.
		{path: "amount", opts: []Option{Rule("money")}},
		{path: "0", opts: []Option{Rule("tag")}},

		{path: "per.weak", want: `grammar Pay: path "per.weak": rule per has no capture "weak" (it has: day, week)`},
		{path: "amount", want: `grammar Pay: path "amount": rule TOP has no capture "amount"`},
		{path: "unit.q", want: `grammar Pay: path "unit.q": proto unit has no capture "q" (it has: day, s)`},
		{path: "sep", want: `rule TOP has no capture "sep"`},
		{path: "week", want: `rule TOP has no capture "week"`},
		{path: "1", want: `rule TOP has no capture "1"`},
		{path: "extra.week", want: `extra, a ( … ) capture, has no capture "week" (it has: 0, day)`},
		{path: "low.k.0", want: `k is text only`},
		// A capture inside $<k>=[ ... ] is beside k, not in it.
		{path: "title.level.made", want: `title is text only: a capture inside $<title>=[ … ] is beside it, so read "level.made"`},
		{path: "min.amount.made", want: `min.amount never has a made value`},
		{path: "extra.made", want: `extra never has a made value`},
		{path: "tag.x", want: `tag is a list of matches, as it can capture more than once: index it, as tag[0]`},
		{path: "per[0]", want: `per is a single match, not a list`},
		{path: "tag[0][1]", want: `tag[0] is a single match, not a list`},
		{path: "~.x", want: `~ is the matched text, with nothing inside`},
		{path: "min.from[0]", want: `min.from is a position, with nothing inside`},
		{path: "day", opts: []Option{Rule("money")}, want: `rule money has no capture "day"`},
		{path: "x", opts: []Option{Rule("nope")}, want: `grammar Pay: no rule nope`},
		{path: "a[", want: scalar.CheckPath("a[").Error()},
	} {
		err := g.CheckPath(c.path, c.opts...)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%q: %v", c.path, err)
		case c.want != "" && err == nil:
			t.Errorf("%q: no error, want %q", c.path, c.want)
		case c.want != "" && !strings.Contains(err.Error(), c.want):
			t.Errorf("%q:\n got %v\nwant %s", c.path, err, c.want)
		}
	}
}

// The paths CheckPath accepts are those a match of the grammar has, for
// each match the tests can make.
func TestCheckPathAcceptsRealPaths(t *testing.T) {
	g := checkPathGrammar(t)
	for _, text := range []string{
		"Snr Engineer£45k - £50 per week #a #b day7hours",
		"£45£3 per dayshiftdayearlyx",
		"£45",
	} {
		m := g.Subparse(text)
		if !m.Defined() {
			t.Fatalf("%q didn't match", text)
		}
		checkRealPaths(t, g, m)
	}
}

// So for every match of the Raku cases: CheckPath takes each path in it.
func TestCheckPathRakuCases(t *testing.T) {
	for _, c := range rakuCases {
		g := MustCompile(c.Src, caseUses(c.Lists, c.Patterns)...)
		for _, in := range c.Inputs {
			m := g.Parse(in)
			if c.Sub {
				m = g.Subparse(in)
			}
			if m.Defined() {
				checkRealPaths(t, g, m)
			}
		}
	}
}

// $<k>= before a ( ... ) or a call names each match, so with a quantifier
// after it, k is a list, or a single match that may be missing, with
// captures of its own; before [ ... ], k is text.
func TestCheckPathNamedQuantified(t *testing.T) {
	g := MustCompile(`grammar Q { token TOP { $<one>=( a (\d) )? $<many>=( b (\d) )* $<ds>=<d>+ $<t>=[ c (\d) ]? (x) }; token d { (\d) } }`)
	for _, path := range []string{"one.0.~", "many[0].0", "ds[1].0", "d[0].0", "t.~", "0", "1"} {
		if err := g.CheckPath(path); err != nil {
			t.Error(err)
		}
	}
	for path, want := range map[string]string{
		"one[0]":  "one is a single match, not a list",
		"many.0x": "many is a list of matches",
		"t.0":     `t is text only: a capture inside $<t>=[ … ] is beside it, so read "0"`,
		"2":       `rule TOP has no capture "2" (it has: 0, 1, d, ds, many, one, t)`,
	} {
		if err := g.CheckPath(path); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %s", path, err, want)
		}
	}
	m := g.Parse("a1b2b3456c7x")
	if !m.Defined() {
		t.Fatal("no match")
	}
	checkRealPaths(t, g, m)
}

func TestCheckPathRoles(t *testing.T) {
	g := MustCompile(`
role Money { token money { '£' $<amount>=[ \d+ ] } }
grammar Base { token TOP { <money> <per> }; token money { '$' \d+ }; token per { per } }
grammar Pay is Base does Money { token per { per \s+ <unit> }; token unit { week } }`)
	if err := g.CheckPath("money.amount"); err != nil {
		t.Error(err)
	}
	if err := g.CheckPath("per.unit"); err != nil {
		t.Error(err)
	}
	err := MustCompile(`role Money { token money { '£' <amount> } }`).CheckPath("amount", Rule("money"))
	if err == nil || !strings.Contains(err.Error(), "is a role") {
		t.Errorf("a role's paths: %v", err)
	}
}

// checkRealPaths checks that CheckPath accepts every path in m, as a
// match of g with opts.
func checkRealPaths(t *testing.T, g *Grammar, m *scalar.SV, opts ...Option) {
	t.Helper()
	for _, path := range realPaths(m, "") {
		if err := g.CheckPath(path, opts...); err != nil {
			t.Fatalf("a match has %q, but CheckPath says: %v", path, err)
		}
	}
}

// realPaths returns the paths in match m, under prefix: each capture's,
// with each element of a list, down to the text, positions and made value
// of every match.
func realPaths(m *scalar.SV, prefix string) []string {
	join := func(k string) string {
		if prefix == "" {
			return k
		}
		return prefix + "." + k
	}
	var out []string
	if prefix != "" {
		out = append(out, prefix)
	}
	for k, v := range m.HV().All() {
		switch {
		case k == "~" || k == "from" || k == "to" || k == "made":
			out = append(out, join(k))
		case v.Kind() == scalar.ArrayRef:
			out = append(out, join(k))
			for i, e := range v.AV().All() {
				out = append(out, realPaths(e, join(k)+"["+strconv.Itoa(i)+"]")...)
			}
		default:
			out = append(out, realPaths(v, join(k))...)
		}
	}
	return out
}
