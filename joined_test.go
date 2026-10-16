package grammar

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// matchesOf reports what a rule of nothing but list l matches at the start
// of each text: the text matched, and its canonical value.
func listMatch(l *WordList, text string) (string, string) {
	g := MustCompile(`grammar J { token TOP { @`+l.Name()+` } }`, l)
	m := g.Subparse(text)
	return m.At("~").Str(), m.At("made").Str()
}

func TestJoinedSpacing(t *testing.T) {
	for _, c := range []struct {
		opts       []ListOption
		phrases    []string
		text       string
		want, made string
	}{
		// Spacing in a phrase may be left out of the text.
		{[]ListOption{JoinedSpacing()}, []string{"on site"}, "onsite", "onsite", "on site"},
		{[]ListOption{JoinedSpacing()}, []string{"on site"}, "on site", "on site", "on site"},
		{[]ListOption{JoinedSpacing()}, []string{"front-end"}, "frontend", "frontend", "front-end"},
		{[]ListOption{JoinedSpacing()}, []string{"front-end"}, "front-end", "front-end", "front-end"},
		// Without LooseSpacing, other spacing still doesn't match.
		{[]ListOption{JoinedSpacing()}, []string{"front-end"}, "front end", "", ""},
		// With it, any spacing or none.
		{[]ListOption{JoinedSpacing(), LooseSpacing()}, []string{"front-end"}, "front end", "front end", "front-end"},
		{[]ListOption{JoinedSpacing(), LooseSpacing()}, []string{"front-end"}, "front – end", "front – end", "front-end"},
		{[]ListOption{JoinedSpacing(), LooseSpacing()}, []string{"on site"}, "onsite", "onsite", "on site"},
		// Each space separately: any of them may go.
		{[]ListOption{JoinedSpacing()}, []string{"full stack web developer"}, "fullstack web developer",
			"fullstack web developer", "full stack web developer"},
		{[]ListOption{JoinedSpacing()}, []string{"full stack web developer"}, "full stackweb developer",
			"full stackweb developer", "full stack web developer"},
		{[]ListOption{JoinedSpacing()}, []string{"full stack web developer"}, "fullstackwebdeveloper",
			"fullstackwebdeveloper", "full stack web developer"},
		// Without LooseSpacing, a run of spacing can go too.
		{[]ListOption{JoinedSpacing()}, []string{"in  house"}, "inhouse", "inhouse", "in  house"},
		// The longest phrase still wins.
		{[]ListOption{JoinedSpacing()}, []string{"front end", "front end developer"}, "frontend developer",
			"frontend developer", "front end developer"},
		// With case folded.
		{[]ListOption{JoinedSpacing(), FoldCase()}, []string{"On Site"}, "ONSITE", "ONSITE", "On Site"},
		// WholeWords still needs the joined word to end there.
		{[]ListOption{JoinedSpacing(), WholeWords()}, []string{"on site"}, "onsiteness", "", ""},
		{[]ListOption{JoinedSpacing(), WholeWords()}, []string{"on site"}, "onsite, hybrid", "onsite", "on site"},
		// Without the option, nothing changes.
		{nil, []string{"on site"}, "onsite", "", ""},
		{[]ListOption{LooseSpacing()}, []string{"on site"}, "onsite", "", ""},
	} {
		l := NewWordList("l", c.opts...)
		for _, p := range c.phrases {
			l.Add(p)
		}
		got, made := listMatch(l, c.text)
		if got != c.want || made != c.made {
			t.Errorf("%v %q on %q = %q (made %q), want %q (made %q)", len(c.opts), c.phrases, c.text, got, made, c.want, c.made)
		}
	}
}

// An exact phrase beats a joined reading of another: with both "pen is"
// and "penis" in a list, "penis" means penis.
func TestJoinedSpacingExactWins(t *testing.T) {
	l := NewWordList("l", JoinedSpacing()).Add("pen is").Add("penis")
	if got := l.Canonical("penis"); !slices.Equal(got, []string{"penis"}) {
		t.Errorf("Canonical(penis) = %q", got)
	}
	if _, made := listMatch(l, "penis"); made != "penis" {
		t.Errorf("made = %q, want penis", made)
	}
}

// Canonical reads phrases as the list matches text, so it knows joined
// spellings, and Phrases still lists only what was added.
func TestJoinedSpacingCanonical(t *testing.T) {
	l := NewWordList("l", JoinedSpacing(), FoldCase(), LooseSpacing()).Add("Onsite", "on site", "office based")
	for phrase, want := range map[string][]string{
		"onsite": {"Onsite"}, "On-Site": {"Onsite"}, "officebased": {"Onsite"}, "office": nil, "onsites": nil,
	} {
		if got := l.Canonical(phrase); !slices.Equal(got, want) {
			t.Errorf("Canonical(%q) = %q, want %q", phrase, got, want)
		}
	}
	if got := l.Phrases(); !slices.Equal(got, []string{"office based", "on site", "onsite"}) {
		t.Errorf("Phrases() = %q", got)
	}
}

// Find skips positions where no phrase can start. Without LooseSpacing, a
// phrase can start with spacing, which joining lets the match leave out, so
// the first letter after it must count as a start too.
func TestJoinedSpacingStarts(t *testing.T) {
	l := NewWordList("l", JoinedSpacing()).Add("-net")
	g := MustCompile(`grammar J { token TOP { @l } }`, l)
	if got := g.Find("dotnet").At("~").Str(); got != "net" {
		t.Errorf("Find = %q, want net", got)
	}
}

// Each space may be matched or skipped, so a long run of them has 2ⁿ ways
// through. Found by FuzzWords: this took minutes before the search
// remembered where it had been.
func TestJoinedSpacingLongRuns(t *testing.T) {
	spaces := strings.Repeat(" ", 60)
	l := NewWordList("l", JoinedSpacing()).Add("a" + spaces + "b")
	done := make(chan string, 1)
	go func() {
		got, _ := listMatch(l, "a"+spaces+"c")
		done <- got
	}()
	select {
	case got := <-done:
		if got != "" {
			t.Errorf("matched %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("still searching after 2s")
	}
}

// benchList times Find over a sentence with a list of work-arrangement
// phrases, with and without JoinedSpacing; README.md quotes the comparison.
func benchList(b *testing.B, joined bool, text string) {
	opts := []ListOption{FoldCase(), LooseSpacing(), WholeWords()}
	if joined {
		opts = append(opts, JoinedSpacing())
	}
	l := NewWordList("l", opts...).Add("On-site", "on site", "office based", "site based", "in the office").
		Add("Remote", "work from home", "home based", "fully remote")
	g := MustCompile(`grammar J { token TOP { @l } }`, l)
	b.ReportAllocs()
	for b.Loop() {
		g.Find(text)
	}
}

const benchText = "Hybrid role, three days a week in the office in Leeds; the rest working from home."

func BenchmarkListExact(b *testing.B)  { benchList(b, false, benchText) }
func BenchmarkListJoined(b *testing.B) { benchList(b, true, benchText) }
func BenchmarkListJoinedHit(b *testing.B) {
	benchList(b, true, "Hybrid role, three days a week onsite in Leeds; the rest working from home.")
}
