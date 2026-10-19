package grammar_test

// The phrasebook in docs/RAKU.md: each task done in Raku, with grammar and
// in plain Go, on the same posting. TestMappingMatchesRaku checks that all
// three give the same answer, so the page can't drift from what the code
// does. Keep the two in step when changing either.

import (
	"bufio"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/nukopian/grammar"
	"github.com/nukopian/scalar"
)

const mappingPosting = "Senior SWE (Hybrid). Pay: £45k – £55,000 per annum. Weekend: £18.50 per hour. Skills: Go, Raku, SQL."

// The pay grammar, as Raku and grammar both read it.
const mappingPay = `
grammar Pay {
    token TOP    { <min=money> [ <.sep> <max=money> ]? <period>? }
    token money  { '£' \s? <amount> }
    token amount { <num> [ $<k>=[:i k] ]? }
    token num    { \d ** 1..3 [ ',' \d ** 3 ]* [ '.' \d+ ]? }
    token sep    { \s* [ '-' | '–' | to ] \s* }
    token period { :i \s* [ 'p.a.' | pa | per \s+ [ annum | year | hour | day ] ] }
}`

// What every version must give, task by task.
var mappingWant = map[string]string{
	"first":     "£45k – £55,000 per annum",
	"named":     "£55,000",
	"all":       "£45k – £55,000 per annum;£18.50 per hour",
	"made":      "45000 55000",
	"list":      "Go,Raku,SQL",
	"canonical": "Year,Hour",
	"longest":   "ab a",
	"split":     "a|; |b|, |c",
}

// mappingRaku is each task in Raku, printing "task<TAB>answer".
const mappingRaku = mappingPay + `
my $posting = q:to/END/.chomp;
Senior SWE (Hybrid). Pay: £45k – £55,000 per annum. Weekend: £18.50 per hour. Skills: Go, Raku, SQL.
END
my %out;

%out<first> = ~$posting.match(/ <pay=Pay::TOP> /)<pay>;

%out<named> = ~$posting.match(/ <pay=Pay::TOP> /)<pay><max>;

%out<all> = $posting.match(/ <pay=Pay::TOP> /, :g).map({ ~.<pay> }).join(";");

class PayActions {
    method amount($/) { make $<num>.Str.subst(",", "", :g) * ($<k> ?? 1000 !! 1) }
}
my $pay = Pay.subparse($posting, :pos($posting.index("£")), :actions(PayActions));
%out<made> = "$pay<min><amount>.made() $pay<max><amount>.made()";

grammar Skills { token TOP { 'Skills:' \s* <skill>+ % [ ',' \s* ] }; token skill { \w+ } }
%out<list> = Skills.subparse($posting, :pos($posting.index("Skills:")))<skill>».Str.join(",");

my %canonical = 'per annum' => 'Year', 'pa' => 'Year', 'per hour' => 'Hour';
my @phrases = %canonical.keys;
%out<canonical> = $posting.match(/:i << @phrases >>/, :g).map({ %canonical{.lc} }).join(",");

grammar Longest { token TOP { a | ab } }
grammar First   { token TOP { a || ab } }
%out<longest> = Longest.subparse("ab") ~ " " ~ First.subparse("ab");

%out<split> = "a; b, c".split(/ <[;,]> \s* /, :v).join("|");

say "$_\t%out{$_}" for %out.keys.sort;
`

// mappingGrammar is each task with grammar.
func mappingGrammar() map[string]string {
	out := map[string]string{}
	pay := grammar.MustCompile(mappingPay)

	out["first"] = pay.Find(mappingPosting).At("~").Str()

	out["named"] = pay.Find(mappingPosting).At("max.~").Str()

	out["all"] = pay.FindAll(mappingPosting).
		Map(func(m *scalar.SV) any { return m.At("~") }).Join(";")

	acts := grammar.Actions(map[string]func(*scalar.SV) any{
		"amount": func(m *scalar.SV) any {
			n := m.At("num.~").S(`,`, "", "gr").Num()
			if m.At("k").Defined() {
				n *= 1000
			}
			return n
		},
	})
	m := pay.Find(mappingPosting, acts)
	out["made"] = m.At("min.amount.made").Str() + " " + m.At("max.amount.made").Str()

	skills := grammar.MustCompile(`grammar Skills { token TOP { 'Skills:' \s* <skill>+ % [ ',' \s* ] }; token skill { \w+ } }`)
	out["list"] = skills.Find(mappingPosting).At("skill").AV().
		Map(func(s *scalar.SV) any { return s.At("~") }).Join(",")

	periods := grammar.NewWordList("periods", grammar.FoldCase(), grammar.WholeWords()).
		Add("Year", "per annum", "pa").Add("Hour", "per hour")
	canonical := grammar.MustCompile(`grammar Period { token TOP { @periods } }`, periods)
	out["canonical"] = canonical.FindAll(mappingPosting).
		Map(func(m *scalar.SV) any { return m.At("made") }).Join(",")

	longest := grammar.MustCompile(`grammar Longest { token TOP { a | ab } }`)
	first := grammar.MustCompile(`grammar First { token TOP { a || ab } }`)
	out["longest"] = longest.Subparse("ab").At("~").Str() + " " + first.Subparse("ab").At("~").Str()

	sep := grammar.MustCompile(`grammar Sep { token TOP { <[;,]> \s* } }`)
	out["split"] = sep.Split("a; b, c").Join("|")

	return out
}

var (
	plainPayRe    = regexp.MustCompile(`£\s?(\d{1,3}(?:,\d{3})*(?:\.\d+)?)([kK])?(?:\s*(?:-|–|to)\s*£\s?(\d{1,3}(?:,\d{3})*(?:\.\d+)?)([kK])?)?(?:\s*(?i:p\.a\.|pa|per\s+(?:annum|year|hour|day)))?`)
	plainSkillsRe = regexp.MustCompile(`Skills:\s*(\w+(?:,\s*\w+)*)`)
	plainPeriodRe = regexp.MustCompile(`(?i)\b(?:per annum|pa|per hour)\b`)
)

// mappingPlain is each task in plain Go, with regexp.
func mappingPlain() map[string]string {
	out := map[string]string{}

	out["first"] = plainPayRe.FindString(mappingPosting)

	out["named"] = "£" + plainPayRe.FindStringSubmatch(mappingPosting)[3]

	out["all"] = strings.Join(plainPayRe.FindAllString(mappingPosting, -1), ";")

	amount := func(num, k string) string {
		n, _ := strconv.ParseFloat(strings.ReplaceAll(num, ",", ""), 64)
		if k != "" {
			n *= 1000
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	sm := plainPayRe.FindStringSubmatch(mappingPosting)
	out["made"] = amount(sm[1], sm[2]) + " " + amount(sm[3], sm[4])

	var skills []string
	for _, s := range strings.Split(plainSkillsRe.FindStringSubmatch(mappingPosting)[1], ",") {
		skills = append(skills, strings.TrimSpace(s))
	}
	out["list"] = strings.Join(skills, ",")

	canonical := map[string]string{"per annum": "Year", "pa": "Year", "per hour": "Hour"}
	var periods []string
	for _, p := range plainPeriodRe.FindAllString(mappingPosting, -1) {
		periods = append(periods, canonical[strings.ToLower(p)])
	}
	out["canonical"] = strings.Join(periods, ",")

	longest := regexp.MustCompile(`^(?:a|ab)`)
	longest.Longest()
	out["longest"] = longest.FindString("ab") + " " + regexp.MustCompile(`^(?:a|ab)`).FindString("ab")

	text, sepRe := "a; b, c", regexp.MustCompile(`[;,]\s*`)
	var pieces []string
	last := 0
	for _, loc := range sepRe.FindAllStringIndex(text, -1) {
		pieces = append(pieces, text[last:loc[0]], text[loc[0]:loc[1]])
		last = loc[1]
	}
	out["split"] = strings.Join(append(pieces, text[last:]), "|")

	return out
}

func TestMappingMatchesRaku(t *testing.T) {
	versions := map[string]map[string]string{
		"grammar":  mappingGrammar(),
		"plain Go": mappingPlain(),
	}

	// Raku joins in where it's installed; grammar and plain Go are
	// compared regardless. CI fails if Raku wasn't compared.
	if _, err := exec.LookPath("raku"); err != nil {
		t.Log("Raku not compared: raku isn't installed")
	} else {
		cmd := exec.Command("raku", "-e", mappingRaku)
		stdout, err := cmd.Output()
		if err != nil {
			t.Fatalf("raku: %v", err)
		}
		raku := map[string]string{}
		sc := bufio.NewScanner(strings.NewReader(string(stdout)))
		for sc.Scan() {
			task, answer, _ := strings.Cut(sc.Text(), "\t")
			raku[task] = answer
		}
		versions["Raku"] = raku
	}

	for task, want := range mappingWant {
		for name, answers := range versions {
			if got := answers[task]; got != want {
				t.Errorf("%s, %s: got %q, want %q", task, name, got, want)
			}
		}
	}
	for name, answers := range versions {
		if len(answers) != len(mappingWant) {
			t.Errorf("%s answers %d tasks, want %d", name, len(answers), len(mappingWant))
		}
	}
}
