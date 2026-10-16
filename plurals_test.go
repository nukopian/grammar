package grammar

import (
	"slices"
	"testing"
)

func TestPluralForms(t *testing.T) {
	for in, want := range map[string]string{
		"engineer":        "engineers",
		"sales assistant": "sales assistants", // the last word only
		"coach":           "coaches",
		"box":             "boxes",
		"boss":            "bosses",
		"buzz":            "buzzes",
		"dish":            "dishes",
		"secretary":       "secretaries",
		"day":             "days", // a vowel before the y
		"key":             "keys",
		"PM":              "PMs", // the ending is lower case
		"front-end dev":   "front-end devs",
		"level 2":         "", // no letter at the end, no plural
		"c++":             "",
		"100%":            "",
		"y":               "ys",
	} {
		if got := pluralOf(in); got != want {
			t.Errorf("pluralOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlurals(t *testing.T) {
	for _, c := range []struct {
		opts       []ListOption
		phrases    []string
		text       string
		want, made string
	}{
		{[]ListOption{Plurals()}, []string{"Software Engineer"}, "Software Engineers", "Software Engineers", "Software Engineer"},
		{[]ListOption{Plurals()}, []string{"Software Engineer"}, "Software Engineer", "Software Engineer", "Software Engineer"},
		{[]ListOption{Plurals()}, []string{"secretary"}, "secretaries", "secretaries", "secretary"},
		{[]ListOption{Plurals()}, []string{"development opportunity"}, "development opportunities",
			"development opportunities", "development opportunity"},
		// Only the last word.
		{[]ListOption{Plurals()}, []string{"sales assistant"}, "saless assistant", "", ""},
		// With FoldCase, any case.
		{[]ListOption{Plurals(), FoldCase()}, []string{"Engineer"}, "ENGINEERS", "ENGINEERS", "Engineer"},
		// Without FoldCase, the ending is lower case.
		{[]ListOption{Plurals()}, []string{"PM"}, "PMs", "PMs", "PM"},
		{[]ListOption{Plurals()}, []string{"PM"}, "PMS", "PM", "PM"},
		// WholeWords: a plural is a whole word now; without Plurals it was
		// part of one.
		{[]ListOption{Plurals(), WholeWords()}, []string{"engineer"}, "engineers", "engineers", "engineer"},
		{[]ListOption{WholeWords()}, []string{"engineer"}, "engineers", "", ""},
		// With JoinedSpacing, a plural can be joined too.
		{[]ListOption{Plurals(), JoinedSpacing()}, []string{"front end developer"}, "frontend developers",
			"frontend developers", "front end developer"},
		// Without the option, nothing changes.
		{nil, []string{"engineer"}, "engineers", "engineer", "engineer"},
	} {
		l := NewWordList("l", c.opts...)
		for _, p := range c.phrases {
			l.Add(p)
		}
		got, made := listMatch(l, c.text)
		if got != c.want || made != c.made {
			t.Errorf("%d opts, %q on %q = %q (made %q), want %q (made %q)", len(c.opts), c.phrases, c.text, got, made, c.want, c.made)
		}
	}
}

// Plurals are made, not added: Phrases, Len and Ambiguous show only what
// was added, while Canonical knows the plurals.
func TestPluralsHidden(t *testing.T) {
	l := NewWordList("l", Plurals()).Add("Product Manager", "PM").Add("Project Manager", "PM")
	if got := l.Phrases(); !slices.Equal(got, []string{"PM", "Product Manager", "Project Manager"}) {
		t.Errorf("Phrases() = %q", got)
	}
	if l.Len() != 3 {
		t.Errorf("Len() = %d", l.Len())
	}
	if got := l.Ambiguous(); len(got) != 1 || got["PM"] == nil {
		t.Errorf("Ambiguous() = %q, want only PM", got)
	}
	if got := l.Canonical("PMs"); !slices.Equal(got, []string{"Product Manager", "Project Manager"}) {
		t.Errorf("Canonical(PMs) = %q", got)
	}
	if got := l.Canonical("Product Managers"); !slices.Equal(got, []string{"Product Manager"}) {
		t.Errorf("Canonical(Product Managers) = %q", got)
	}
}

// A phrase added as itself beats a plural made from another, whichever was
// added first: "news" isn't more than one "new".
func TestPluralsRealPhraseWins(t *testing.T) {
	for _, order := range [][2]string{{"new", "news"}, {"news", "new"}} {
		l := NewWordList("l", Plurals()).Add(order[0]).Add(order[1])
		if got := l.Canonical("news"); !slices.Equal(got, []string{"news"}) {
			t.Errorf("added %q then %q: Canonical(news) = %q", order[0], order[1], got)
		}
		if got := l.Phrases(); !slices.Equal(got, []string{"new", "news"}) {
			t.Errorf("added %q then %q: Phrases() = %q", order[0], order[1], got)
		}
		if _, made := listMatch(l, "news"); made != "news" {
			t.Errorf("added %q then %q: made = %q", order[0], order[1], made)
		}
	}
}
