package grammar

import (
	"slices"
	"strings"
	"testing"
)

// 1. A named capture of a word list has its canonical value as made, as a
// rule that's only a list does.
func TestNamedListMade(t *testing.T) {
	levels := NewWordList("levels", FoldCase()).Add("Senior", "snr", "sr").Add("Junior", "jnr")
	g := MustCompile(`grammar N { token TOP { $<level>=@levels \s+ $<role>=[ \w+ ] } }`, levels)
	m := g.Parse("Snr Developer")
	if m.At("level.made").Str() != "Senior" || m.At("level.~").Str() != "Snr" || m.At("role.made").Defined() {
		t.Errorf("syntax: level %v, made %v, role made %v", m.At("level.~"), m.At("level.made"), m.At("role.made"))
	}
	// Optional, as a seniority in a title is: a canonical value when present.
	opt := MustCompile(`grammar O { token TOP { $<level>=@levels? \s* $<role>=[ \N+ ] } }`, levels)
	if got := opt.Parse("Snr Developer").At("level.made").Str(); got != "Senior" {
		t.Errorf("optional: %q", got)
	}
	if m := opt.Parse("Developer"); m.At("level.made").Defined() || m.At("role.~").Str() != "Developer" {
		t.Errorf("optional, absent: %v", m.At("level.made"))
	}
	b := New("B").Token("TOP", Named("level", Words(levels)))
	if got := b.Parse("jnr").At("level.made").Str(); got != "Junior" {
		t.Errorf("builder: %q", got)
	}
	// Values and To see it, so the canonical value fills a struct.
	r, err := To[struct{ Level, Role string }](g.Parse("sr Tester"))
	if err != nil || r.Level != "Senior" || r.Role != "Tester" {
		t.Errorf("To: %+v %v", r, err)
	}
}

// 2. WholeWords matches a phrase only where it isn't part of a longer word,
// and still takes the longest phrase that fits.
func TestWholeWords(t *testing.T) {
	l := NewWordList("w", FoldCase(), WholeWords()).Add("java").Add("lead").Add("c++").Add("senior").Add("senior dev")
	g := MustCompile(`grammar W { token TOP { @w } }`, l)
	for text, want := range map[string]string{
		"Java developer": "Java", "JavaScript": "", "team lead.": "lead", "leadership": "",
		"c++/go": "c++", "senior developer": "senior", "senior dev role": "senior dev", "misleading": "",
		"x-lead": "lead", "lead2": "",
	} {
		if got := g.Find(text).At("~").Str(); got != want {
			t.Errorf("Find(%q) = %q, want %q", text, got, want)
		}
	}
	// Without it, a phrase matches inside words, as Raku's @list does.
	loose := MustCompile(`grammar L { token TOP { @w } }`, NewWordList("w").Add("lead"))
	if loose.Find("leadership").At("~").Str() != "lead" {
		t.Error("without WholeWords")
	}
	// Skipping start positions still finds the same matches.
	with, without := findBoth(g, "JavaScript and java, the lead in leadership")
	if d := with[1].Diff(without[1]); d != "" {
		t.Errorf("start skipping: %s", d)
	}
}

// 3. Split gives the text between matches, with the matches in place, as
// Raku's split with :v does.
func TestSplit(t *testing.T) {
	g := MustCompile(`grammar S { token TOP { <[,;]> \s* } }`)
	for text, want := range map[string][]string{
		"a, b;c":  {"a", ", ", "b", ";", "c"},
		"a":       {"a"},
		",a,":     {"", ",", "a", ",", ""},
		"":        {""},
		"£1, é;ü": {"£1", ", ", "é", ";", "ü"},
	} {
		if got := g.Split(text).StrSlice(); !slices.Equal(got, want) {
			t.Errorf("Split(%q) = %q, want %q", text, got, want)
		}
	}
	// A rule other than TOP, and a match that's empty.
	e := MustCompile(`grammar E { token TOP { x }; token gap { <?before b> } }`)
	if got := e.Split("ab", Rule("gap")).StrSlice(); !slices.Equal(got, []string{"a", "", "b"}) {
		t.Errorf("empty match: %q", got)
	}
}

// 4. WordLists builds a list for each set of synonym rules.
func TestWordLists(t *testing.T) {
	uses, err := WordLists(map[string]string{
		"units":  "Year, annum, pa\nWeek, wk",
		"levels": "Senior, snr",
	}, FoldCase())
	if err != nil {
		t.Fatal(err)
	}
	g := MustCompile(`grammar U { token TOP { <unit> }; token unit { @units }; token level { @levels } }`, uses...)
	if g.Parse("PA").At("unit.made").Str() != "Year" || g.Parse("snr", Rule("level")).At("made").Str() != "Senior" {
		t.Error("lists not usable")
	}
	if _, err := WordLists(map[string]string{"bad": "a => b => c"}); err == nil || !strings.Contains(err.Error(), "bad") {
		t.Errorf("a bad rule: %v", err)
	}
}

// 5. A rule ending with a comma carries on onto the next line, so long
// rules can be wrapped; otherwise each line is a rule.
func TestAddRulesContinuation(t *testing.T) {
	l := NewWordList("u")
	err := l.AddRules(`Year, annum, yr,
	    per annum, p.a.
Week, wk
Hour,
hr, ph`)
	if err != nil {
		t.Fatal(err)
	}
	for phrase, want := range map[string]string{"p.a.": "Year", "per annum": "Year", "wk": "Week", "ph": "Hour", "hr": "Hour"} {
		if got := l.Canonical(phrase); len(got) != 1 || got[0] != want {
			t.Errorf("Canonical(%q) = %q, want %q", phrase, got, want)
		}
	}
	if l.Canonical("per annum")[0] == "per annum" {
		t.Error("the wrapped line became a rule of its own")
	}
}

// 6. MapOnly: a rule written phrases => values adds the phrases meaning
// the values, as AddAs does, without making the values phrases too.
func TestMapOnly(t *testing.T) {
	const rules = `
benefits, perks, what we offer
legal notice, the legal bit => legal
pm => product manager, project manager
`
	l := NewWordList("sections", FoldCase(), Plurals(), MapOnly())
	if err := l.AddRules(rules); err != nil {
		t.Fatal(err)
	}
	for phrase, want := range map[string]string{
		"legal": "", "Legal": "", "legals": "",
		"legal notice": "legal", "Legal Notices": "legal", "the legal bit": "legal",
		"benefits": "benefits", "What We Offer": "benefits", "Perks": "benefits",
		"pm": "product manager,project manager", "product manager": "", "project manager": "",
	} {
		if got := strings.Join(l.Canonical(phrase), ","); got != want {
			t.Errorf("Canonical(%q) = %q, want %q", phrase, got, want)
		}
	}
	if got := l.Canonicals(); !slices.Equal(got, []string{"benefits", "legal", "product manager", "project manager"}) {
		t.Errorf("Canonicals: %q", got)
	}
	if amb := l.Ambiguous(); len(amb) != 1 || len(amb["pm"]) != 2 {
		t.Errorf("Ambiguous: %v", amb)
	}
	g := MustCompile(`grammar H { token TOP { @sections } }`, l)
	if g.Parse("Legal").Defined() || g.Parse("Legal Notice").At("made").Str() != "legal" {
		t.Error("matching doesn't follow Canonical")
	}

	// Without it, the values are phrases too, as before.
	plain := NewWordList("sections", FoldCase())
	if err := plain.AddRules(rules); err != nil {
		t.Fatal(err)
	}
	if got := plain.Canonical("legal"); !slices.Equal(got, []string{"legal"}) {
		t.Errorf("without MapOnly, Canonical(legal) = %q", got)
	}

	// A rule with nothing before => would add nothing.
	for _, bad := range []string{"=> legal", " , => legal"} {
		if err := NewWordList("x", MapOnly()).AddRule(bad); err == nil || !strings.Contains(err.Error(), "nothing before =>") {
			t.Errorf("%q: %v", bad, err)
		}
		if err := NewWordList("x").AddRule(bad); err != nil {
			t.Errorf("%q without MapOnly: %v", bad, err)
		}
	}

	// WordLists passes it to each list.
	uses, err := WordLists(map[string]string{"sections": rules}, MapOnly())
	if err != nil {
		t.Fatal(err)
	}
	if got := uses[0].(*WordList).Canonical("legal"); got != nil {
		t.Errorf("WordLists: Canonical(legal) = %q", got)
	}
}
