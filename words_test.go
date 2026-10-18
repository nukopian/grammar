package grammar

import (
	"slices"
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

func titleList(opts ...ListOption) *WordList {
	l := NewWordList("titles", opts...)
	l.Add("Software Engineer", "Software Developer", "SWE", "Developer")
	l.Add("Senior Software Engineer", "Senior Developer")
	l.Add("Front-end Developer", "Frontend Developer", "Front End Developer")
	return l
}

// A rule that is a word list has the canonical value as its made value,
// and the longest phrase wins.
func TestWordsCanonical(t *testing.T) {
	g := MustCompile(`grammar T { token TOP { <title> .* }; token title { @titles } }`, titleList())
	for in, want := range map[string]string{
		"SWE, Payments":          "Software Engineer",
		"Senior Developer (Go)":  "Senior Software Engineer",
		"Developer":              "Software Engineer",
		"Software Engineer":      "Software Engineer",
		"Frontend Developer III": "Front-end Developer",
	} {
		if got := g.Parse(in).At("title.made").Str(); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	if g.Parse("software engineer").Defined() {
		t.Error("matched another case without FoldCase or :i")
	}
}

func TestWordsFoldCaseAndLoose(t *testing.T) {
	g := MustCompile(`grammar T { token TOP { <title> }; token title { @titles } }`, titleList(FoldCase(), LooseSpacing()))
	for in, want := range map[string]string{
		"swe":                     "Software Engineer",
		"front  –  end developer": "Front-end Developer",
		"FRONT_END DEVELOPER":     "Front-end Developer",
		"senior\tdeveloper":       "Senior Software Engineer",
	} {
		if got := g.Parse(in).At("title.made").Str(); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	// Spacing between words must still be there.
	if g.Parse("frontenddeveloper").Defined() {
		t.Error("loose spacing matched no spacing at all")
	}
}

// Under :i, an exact list matches any case, as in Raku, and the canonical
// value is still found.
func TestWordsIgnoreCase(t *testing.T) {
	g := MustCompile(`grammar T { token TOP { <title> }; token title { :i @titles } }`, titleList())
	if got := g.Parse("SENIOR DEVELOPER").At("title.made").Str(); got != "Senior Software Engineer" {
		t.Errorf("got %q", got)
	}
}

// Rules in the format Elasticsearch's synonyms API returns.
func TestAddRules(t *testing.T) {
	l := NewWordList("units", FoldCase())
	err := l.AddRules(`
# periods
annum, year, yr, pa, p.a.
week, wk, pw
hour, hr => Hour
pm => product manager, project manager
c\, d => cd
`)
	if err != nil {
		t.Fatal(err)
	}
	for phrase, want := range map[string]string{
		"YR": "annum", "p.a.": "annum", "pw": "week", "hr": "Hour", "hour": "Hour",
		"Hour": "Hour", "pm": "product manager,project manager", "c, d": "cd", "nope": "",
	} {
		if got := strings.Join(l.Canonical(phrase), ","); got != want {
			t.Errorf("%q: %q, want %q", phrase, got, want)
		}
	}
	amb := l.Ambiguous()
	if len(amb) != 1 || !slices.Equal(amb["pm"], []string{"product manager", "project manager"}) {
		t.Errorf("Ambiguous: %v", amb)
	}
	for _, bad := range []string{"a => b => c", "a, b =>"} {
		if err := NewWordList("x").AddRule(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

// An action sees the canonical value as made, and may replace it.
func TestWordsAction(t *testing.T) {
	g := MustCompile(`grammar T { token TOP { <title> }; token title { @titles } }`, titleList())
	m := g.Parse("SWE", Actions(map[string]func(*scalar.SV) any{
		"title": func(m *scalar.SV) any { return "code:" + m.At("made").Str() },
	}))
	if got := m.At("title.made").Str(); got != "code:Software Engineer" {
		t.Errorf("got %q", got)
	}
}

func TestWordsBuilder(t *testing.T) {
	l := titleList()
	g := New("T").Token("TOP", Seq(Call("title"), Star(AnyChar))).Token("title", Words(l))
	if got := g.Parse("SWE II").At("title.made").Str(); got != "Software Engineer" {
		t.Errorf("got %q", got)
	}
	if l.Len() != 9 || l.Name() != "titles" || !slices.Contains(l.Phrases(), "SWE") {
		t.Errorf("Len %d, Name %q, Phrases %v", l.Len(), l.Name(), l.Phrases())
	}
}

// A role's rules can use lists and call the grammar's rules; the grammar's
// own rules win wherever Does is called.
func TestDoesBuilder(t *testing.T) {
	lex := NewRole("Lexicon").
		Token("title", Words(titleList())).
		Token("TOP", Seq(Call("title"), Call("rest")))
	before := New("A").Token("rest", Star(AnyChar)).Token("title", Lit("x")).Does(lex)
	after := New("B").Does(lex).Token("rest", Star(AnyChar)).Token("title", Lit("x"))
	for _, g := range []*Grammar{before, after} {
		if !g.Parse("x!").Defined() || g.Parse("SWE!").Defined() {
			t.Errorf("%s: own title didn't win", g.Name())
		}
	}
	uses := New("C").Does(lex).Token("rest", Star(AnyChar))
	if got := uses.Parse("SWE!").At("title.made").Str(); got != "Software Engineer" {
		t.Errorf("composed title: %q", got)
	}
	if !lex.IsRole() || uses.IsRole() {
		t.Error("IsRole")
	}
}

func TestDoesConflicts(t *testing.T) {
	r1 := NewRole("R1").Token("x", Lit("a"))
	r2 := NewRole("R2").Token("x", Lit("b"))
	for name, f := range map[string]func(){
		"two roles":  func() { New("G").Does(r1, r2) },
		"not a role": func() { New("G").Does(New("H")) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: didn't panic", name)
				}
			}()
			f()
		}()
	}
	// Defining the rule in the grammar resolves the conflict.
	g := New("G").Token("x", Lit("c")).Does(r1, r2).Token("TOP", Call("x"))
	if !g.Parse("c").Defined() {
		t.Error("own x didn't resolve the conflict")
	}
}

// Digits are word characters, so a phrase run into a figure isn't a whole
// word, as WholeWords' doc says; <!before \w> checks only the end.
func TestWholeWordsAfterDigits(t *testing.T) {
	units := NewWordList("units", WholeWords()).Add("pa")
	whole := MustCompile(`grammar W { token TOP { \d+ @units } }`, units)
	end := MustCompile(`grammar E { token TOP { \d+ @units <!before \w> } }`, NewWordList("units").Add("pa"))
	if whole.Parse("45000pa").Defined() {
		t.Error("WholeWords took pa run into 45000")
	}
	if !end.Parse("45000pa").Defined() || end.Parse("45000pay").Defined() {
		t.Error("<!before \\w> doesn't check only the end")
	}
}
