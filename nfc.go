package grammar

import (
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Text is read in NFC, Unicode's composed form: an accented letter written
// as a letter and a combining accent, as some text arrives, becomes the one
// character it's equivalent to, as most text has it already. So a grammar
// matches either spelling, whichever way it's written itself, and . or
// <:L> takes the accented letter whole, as in Raku, which reads text as
// whole characters too. The text given to Parse, Find and the rest is
// read this way, and so are a grammar's literals, sets and regexes, and a
// word list's phrases.
//
// A match describes the text as read: its ~, from and to, a failure's
// column, and Split's pieces are of the text in NFC. For text already in
// NFC, nearly all text, that's the text itself.

// nfc returns s in NFC, and s itself if it's valid UTF-8 in NFC already,
// which it checks without copying.
//
// Checking takes about 1 ns a byte, so it starts with a shortcut. Every
// character below U+0300 is in NFC and never combines with the one before
// it, and every character from U+0300 on (each combining accent among
// them) is written in UTF-8 with a first byte of 0xCC or more. So the text
// up to the first such byte is in NFC, and only the rest needs checking,
// from the character before it, which an accent after it could combine
// with. English text with £ and accented Latin letters takes the shortcut
// all the way.
func nfc(s string) string {
	// Invalid UTF-8 first: each byte that isn't is read as U+FFFD, as Go
	// reads text into runes. The normaliser would leave such a byte alone,
	// and stop normalising there, so a phrase and the text it's read from
	// could normalise differently.
	if !utf8.ValidString(s) {
		s = string([]rune(s))
	}
	i := 0
	for i < len(s) && s[i] < 0xCC {
		i++
	}
	if i == len(s) {
		return s
	}
	// Back up to the start of the character before s[i].
	from := i
	if from > 0 {
		from--
		for from > 0 && s[from]&0xC0 == 0x80 {
			from--
		}
	}
	if norm.NFC.IsNormalString(s[from:]) {
		return s
	}
	return norm.NFC.String(s)
}
