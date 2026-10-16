package grammar

import (
	"strings"
	"testing"

	"github.com/nukopian/job-meta/internal/scalar"
)

var pay = MustCompile(`
grammar Pay {
    token TOP    { <min=money> [ <.sep> <max=money> ]? <period>? }
    token money  { '£' \s? <amount> }
    token amount { <num> [ $<k>=[:i k] ]? }
    token num    { \d ** 1..3 [ ',' \d ** 3 ]* [ '.' \d+ ]? }
    token sep    { \s* [ '-' | '–' | to ] \s* }
    token period { :i \s* [ 'p.a.' | pa | per \s+ [ annum | year | hour | day ] ] }
}`)

type payActions struct{}

func (payActions) Amount(m *scalar.SV) any {
	n := m.At("num.~").S(`,`, "", "gr").Num()
	if m.At("k").Defined() {
		n *= 1000
	}
	return n
}

func (payActions) Period(m *scalar.SV) any {
	switch p := strings.ToLower(m.At("~").Str()); {
	case strings.Contains(p, "hour"):
		return "Hourly"
	case strings.Contains(p, "day"):
		return "Daily"
	}
	return "Annual"
}

func TestActions(t *testing.T) {
	for _, c := range []struct {
		in       string
		min, max float64
		period   string
	}{
		{"£45,000 - £55,000 per annum", 45000, 55000, "Annual"},
		{"£45k to £55K", 45000, 55000, ""},
		{"£18.50 per hour", 18.5, -1, "Hourly"},
	} {
		m := pay.Subparse(c.in, Actions(payActions{}))
		if got := m.At("min.amount.made").Num(); got != c.min {
			t.Errorf("%q: min %v, want %v", c.in, got, c.min)
		}
		if got := m.At("max.amount.made").Or(-1).Num(); got != c.max {
			t.Errorf("%q: max %v, want %v", c.in, got, c.max)
		}
		if got := m.At("period.made").Str(); got != c.period {
			t.Errorf("%q: period %q, want %q", c.in, got, c.period)
		}
	}
}

func TestActionsMap(t *testing.T) {
	m := pay.Parse("£5k", Actions(map[string]func(*scalar.SV) any{
		"TOP": func(m *scalar.SV) any { return "top:" + m.At("~").Str() },
	}))
	if got := m.At("made").Str(); got != "top:£5k" {
		t.Errorf("made %q", got)
	}
}

func TestActionsWrongSignature(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "not func(*scalar.SV) any") {
			t.Errorf("recovered %v", r)
		}
	}()
	pay.Parse("£5k", Actions(badActions{}))
}

type badActions struct{}

func (badActions) Amount(s string) string { return s }

// A grammar built in Go matches as the same grammar compiled from Raku
// syntax does.
func TestBuilderMatchesSyntax(t *testing.T) {
	built := New("Pay").
		Token("TOP", Seq(Alias("min", "money"), Opt(Seq(Silent("sep"), Alias("max", "money"))), Opt(Call("period")))).
		Token("money", Seq(Lit("£"), Opt(Space), Call("amount"))).
		Token("amount", Seq(Call("num"), Opt(Named("k", IgnoreCase(Lit("k")))))).
		Token("num", Seq(Repeat(Digit, 1, 3), Star(Seq(Lit(","), Repeat(Digit, 3, 3))), Opt(Seq(Lit("."), Plus(Digit))))).
		Token("sep", Seq(Star(Space), Alt(Lit("-"), Lit("–"), Lit("to")), Star(Space))).
		Token("period", IgnoreCase(Seq(Star(Space), Alt(Lit("p.a."), Lit("pa"),
			Seq(Lit("per"), Plus(Space), Alt(Lit("annum"), Lit("year"), Lit("hour"), Lit("day")))))))
	for _, in := range []string{"£45,000 - £55,000 per annum", "£45k to £55K", "£18.50 per hour", "nope"} {
		if d := built.Subparse(in).Diff(pay.Subparse(in)); d != "" {
			t.Errorf("%q: %s", in, d)
		}
	}
}

func TestRe(t *testing.T) {
	g := New("R").Token("TOP", Seq(Re(`[a-z]+(?=\d)`), Capture(Re(`\d+`))))
	m := g.Parse("abc123")
	if m.At("0.~").Str() != "123" {
		t.Errorf("got %v", m)
	}
	if g.Parse("abc").Defined() {
		t.Error("abc matched")
	}
	ci := New("R").Token("TOP", IgnoreCase(Re(`abc`)))
	if !ci.Parse("ABC").Defined() {
		t.Error("IgnoreCase didn't reach Re")
	}
}

// An extension's rule replaces its parent's wherever the parent calls it.
func TestExtend(t *testing.T) {
	us := MustCompile(`grammar PayUS is Pay { token money { '$' <amount> } }`, pay)
	m := us.Subparse("$60k - $70k")
	if got := m.At("max.amount.~").Str(); got != "70k" {
		t.Errorf("max %q in %v", got, m)
	}
	if us.Subparse("£60k").Defined() {
		t.Error("PayUS still matches £")
	}
	built := pay.Extend("PayEUR").Token("money", Seq(Lit("€"), Call("amount")))
	if !built.Subparse("€5k").Defined() {
		t.Error("PayEUR didn't match €")
	}
}

func TestFind(t *testing.T) {
	text := "Salary: £45,000 - £55,000 p.a. Bonus up to £5k."
	if got := pay.Find(text).At("~").Str(); got != "£45,000 - £55,000 p.a." {
		t.Errorf("Find: %q", got)
	}
	all := pay.FindAll(text)
	var got []string
	for _, m := range all.All() {
		got = append(got, m.At("~").Str()+"@"+m.At("from").Str())
	}
	if want := "£45,000 - £55,000 p.a.@8,£5k@43"; strings.Join(got, ",") != want {
		t.Errorf("FindAll: %s, want %s", strings.Join(got, ","), want)
	}
	if pay.Find("no money here").Defined() {
		t.Error("Find matched")
	}
}

func TestRuleOption(t *testing.T) {
	if got := pay.Parse("12,500", Rule("num")).At("~").Str(); got != "12,500" {
		t.Errorf("got %q", got)
	}
}

// The match is ordinary scalar data, so To turns it into a struct.
func TestTo(t *testing.T) {
	type amount struct {
		Text string `json:"~"`
		From int    `json:"from"`
	}
	type match struct {
		Money []amount `json:"money"`
	}
	got, err := scalar.To[match](pay.Parse("£1 - £2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Money) != 2 || got.Money[1].Text != "£2" || got.Money[1].From != 5 {
		t.Errorf("got %+v", got)
	}
}

func TestCompileErrors(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`grammar G { token TOP { a - b } }`, `line 1, column 27: '-' must be quoted`},
		{`grammar G { token TOP { <nope> } }`, `rule TOP calls <nope>, which doesn't exist`},
		{`grammar G { method TOP { a } }`, `expected token, regex or rule`},
		{`grammar G { token TOP { a { say 1 } } }`, `code blocks aren't supported`},
		{`grammar G { token TOP { a % ',' } }`, `% needs a quantifier`},
		{`grammar G { token TOP { a }; token TOP { b } }`, `TOP is declared twice`},
		{`grammar G is H { token TOP { a } }`, `no grammar or role H: declare it earlier`},
		{`grammar G { token TOP { @nope } }`, `no word list @nope: pass it to Compile`},
		{`grammar G { token TOP { <@nope> } }`, `use @name, which matches them as text`},
		{`grammar G { token TOP { a } } grammar G2 is G is G { token TOP { b } }`, `can only extend one grammar`},
		{`role R { token x { a } } grammar G is R { token TOP { b } }`, `R is a role: use does`},
		{`grammar R { token x { a } } grammar G does R { token TOP { b } }`, `R is a grammar, not a role`},
		{`role R1 { token x { a } } role R2 { token x { b } } grammar G does R1 does R2 { token TOP { <x> } }`,
			`rule x is in both R1 and R2; define it in G to choose`},
		{`grammar G { token TOP { a } } stray`, `expected grammar or role`},
		{`grammar G {
		    token TOP { <[a..z }
		}`, `line 3`},
	} {
		_, err := Compile(c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.src, err, c.want)
		}
	}
}

func TestReservedCaptureName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "reserved") {
			t.Errorf("recovered %v", r)
		}
	}()
	New("G").Token("TOP", Named("from", Lit("a")))
}

// A long input doesn't run out of stack, and a quantified empty match
// doesn't loop.
func TestLongAndEmpty(t *testing.T) {
	g := MustCompile(`grammar Long { token TOP { [ <.w>+ % ',' ]* % ';' }; token w { \w+ } }`)
	in := strings.Repeat("ab,cd;", 20000) + "x"
	if got := g.Parse(in).At("~").Str(); got != in {
		t.Errorf("long input: matched %d of %d", len(got), len(in))
	}
	empty := MustCompile(`grammar Empty { regex TOP { [ a? ]* b } }`)
	if !empty.Parse("aab").Defined() || !empty.Parse("b").Defined() {
		t.Error("empty-item quantifier")
	}
}

// BenchmarkFindAll hunts for pay in a posting-sized text.
func BenchmarkFindAll(b *testing.B) {
	text := strings.Repeat("We offer a competitive package and flexible working. ", 80) +
		"Salary: £45,000 - £55,000 p.a. plus a bonus of up to £5k."
	for b.Loop() {
		if pay.FindAll(text).Length() != 2 {
			b.Fatal("wrong count")
		}
	}
}

// BenchmarkParse parses one pay string: the matcher's work on a short
// text, with no positions to skip.
func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if !pay.Subparse("£45,000 - £55,000 per annum").Defined() {
			b.Fatal("no match")
		}
	}
}
