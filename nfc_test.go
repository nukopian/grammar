package grammar

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

const (
	cafeNFD = "Café" // e and a combining acute accent
	cafeNFC = "Café"  // é, composed
)

// Text, literals, word lists and regexes are all read in NFC, so a
// composed and a decomposed spelling match each other, either way round.
func TestNFC(t *testing.T) {
	for _, c := range []struct {
		name string
		g    *Grammar
		text string
	}{
		{"composed literal, decomposed text", MustCompile(`grammar A { token TOP { 'Café' } }`), cafeNFD},
		{"decomposed literal, composed text", MustCompile("grammar B { token TOP { '" + cafeNFD + "' } }"), cafeNFC},
		{"Lit, decomposed text", New("C").Token("TOP", Lit(cafeNFC)), cafeNFD},
		{"decomposed Lit, composed text", New("D").Token("TOP", Lit(cafeNFD)), cafeNFC},
		{"set, decomposed letter", MustCompile("grammar H { token TOP { Caf <[e\u0301]> } }"), cafeNFC},
		{"Re, decomposed pattern", New("E").Token("TOP", Re(cafeNFD)), cafeNFC},
		{"word list, decomposed phrase", MustCompile(`grammar F { token TOP { @l } }`, NewWordList("l").Add(cafeNFD)), cafeNFC},
		{"word list, decomposed text", MustCompile(`grammar G { token TOP { @l } }`, NewWordList("l").Add(cafeNFC)), cafeNFD},
	} {
		m := c.g.Parse(c.text)
		if !m.Defined() {
			t.Errorf("%s: no match", c.name)
			continue
		}
		// What matched is reported composed, four characters long.
		if m.At("~").Str() != cafeNFC || m.At("to").Int() != 4 {
			t.Errorf("%s: matched %q to %v", c.name, m.At("~"), m.At("to"))
		}
	}
	l := NewWordList("l").Add("Coffee", cafeNFD)
	if got := l.Canonical(cafeNFC); !slices.Equal(got, []string{"Coffee"}) {
		t.Errorf("Canonical = %q", got)
	}
}

// Split gives the text composed, and ParseErr counts columns in composed
// characters.
func TestNFCPositions(t *testing.T) {
	g := MustCompile(`grammar S { token TOP { ',' } }`)
	if got := g.Split(cafeNFD + "," + cafeNFD).StrSlice(); !slices.Equal(got, []string{cafeNFC, ",", cafeNFC}) {
		t.Errorf("Split = %q", got)
	}
	g2 := MustCompile(`grammar P { token TOP { \w+ ' ' \d } }`)
	_, err := g2.ParseErr(cafeNFD + " x")
	if want := `grammar P: line 1, column 6: expected \d, found "x"`; err == nil || err.Error() != want {
		t.Errorf("%v, want %s", err, want)
	}
}

// BenchmarkNFC is the check on text that's NFC already, as nearly all text
// is: a posting-sized text, with a £ and curly quotes, as postings have.
func BenchmarkNFC(b *testing.B) {
	text := strings.Repeat("We offer a competitive package and \u201cflexible\u201d working. ", 80) + "Salary: £45,000."
	for b.Loop() {
		if nfc(text) != text {
			b.Fatal("changed")
		}
	}
}

// The shortcut in nfc agrees with Unicode's own check, on text that starts
// in plain Latin and then has an accent to combine, or not.
func TestNFCShortcut(t *testing.T) {
	for _, s := range []string{
		"", "plain", "£45,000", "caf\u00e9", "cafe\u0301", "e\u0301", "\u0301", "\u00a3\u0301",
		"\u201cquoted\u201d", "x\u201ce\u0301", "Ж", "Же\u0306", "\u2000", "a\u0340", "q\u0303",
		// Invalid UTF-8, which the normaliser would leave alone, and
		// stop normalising at.
		"\xf3\u0374", "\xf3", "a\xffe\u0301", "\xc3", "\x80abc", "\xe0\xbb \x85",
	} {
		want := norm.NFC.String(string([]rune(s))) // invalid bytes read as U+FFFD, as text is
		if got := nfc(s); got != want {
			t.Errorf("nfc(%+q) = %+q, want %+q", s, got, want)
		}
	}
}

// A phrase with invalid UTF-8 means what it did when it matches: text and
// phrases read invalid bytes the same way, as U+FFFD, before NFC. FuzzWords
// found one where they didn't, as an invalid byte stopped the normaliser
// from turning U+0374 into U+02B9.
func TestNFCInvalidUTF8(t *testing.T) {
	phrase := "\xf3\u0374"
	l := NewWordList("w").Add("Sign", phrase)
	m := MustCompile(`grammar W { token TOP { @w } }`, l).Parse(phrase)
	if got := m.At("made").Str(); got != "Sign" {
		t.Errorf("made = %q, want Sign", got)
	}
}
