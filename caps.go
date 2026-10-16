package grammar

import (
	"cmp"
	"iter"
	"slices"
	"strconv"

	"github.com/nukopian/job-meta/internal/scalar"
)

// Caps iterates over m's captures in the order they come in the text, as
// Raku's .caps does: each capture's name and match. A capture that matched
// more than once (an array) gives each match in its own place, so the
// captures of a repeat come out interleaved, as the text has them: for
// [ <a> | <b> ]+ on "a,b,a", a, b, a. A match captured under two names, as
// <min=money> is, comes under both.
//
// Captures come by where they start, then where they end, so an empty
// capture comes before a longer one starting at the same place, as it was
// made first. Captures at the same place, such as an alias and its rule's
// name, come in name order: positional captures by number, then named ones
// alphabetically. Raku leaves those to its hash order, which changes from
// run to run.
//
// Caps on undef, as when the text didn't match, gives nothing.
func Caps(m *scalar.SV) iter.Seq2[string, *scalar.SV] {
	return func(yield func(string, *scalar.SV) bool) {
		for _, c := range capsOf(m) {
			if !yield(c.key, c.m) {
				return
			}
		}
	}
}

// Chunks iterates over m in pieces, as Raku's .chunks does: its captures,
// in Caps' order, with the text between them under "~", as a string. Text
// before the first capture and after the last is a chunk too, and with no
// captures, the whole of m's text is one. Where captures overlap, as an
// alias and its rule's name do, there's no text between them. So the
// chunks' text, joined, is m's text, plus each overlapping capture's again.
//
// Chunks on undef gives nothing.
func Chunks(m *scalar.SV) iter.Seq2[string, *scalar.SV] {
	return func(yield func(string, *scalar.SV) bool) {
		if m.Kind() != scalar.HashRef {
			return
		}
		text := []rune(m.At("~").Str())
		from := int(m.At("from").Int())
		pos := from
		gap := func(to int) bool {
			if to <= pos {
				return true
			}
			return yield("~", scalar.Value(string(text[pos-from:to-from])))
		}
		for _, c := range capsOf(m) {
			if !gap(c.from) || !yield(c.key, c.m) {
				return
			}
			pos = max(pos, c.to)
		}
		gap(from + len(text))
	}
}

// capture is one of a match's captures, for Caps and Chunks.
type capture struct {
	key      string
	m        *scalar.SV
	from, to int
}

// capsOf returns m's captures in Caps' order.
func capsOf(m *scalar.SV) []capture {
	if m.Kind() != scalar.HashRef {
		return nil
	}
	var out []capture
	add := func(key string, c *scalar.SV) {
		if c.Kind() == scalar.HashRef {
			out = append(out, capture{key, c, int(c.At("from").Int()), int(c.At("to").Int())})
		}
	}
	for k, v := range m.HV().All() {
		switch {
		case slices.Contains(reserved, k):
		case v.Kind() == scalar.ArrayRef:
			for _, c := range v.AV().All() {
				add(k, c)
			}
		default:
			add(k, v)
		}
	}
	// Stable, so the matches of one array stay in order, wherever they are.
	slices.SortStableFunc(out, func(a, b capture) int {
		return cmp.Or(cmp.Compare(a.from, b.from), cmp.Compare(a.to, b.to), keyCmp(a.key, b.key))
	})
	return out
}

// keyCmp orders capture names: positional ones by number, before named
// ones, which are in string order.
func keyCmp(a, b string) int {
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	switch {
	case aerr == nil && berr == nil:
		return cmp.Compare(ai, bi)
	case aerr == nil:
		return -1
	case berr == nil:
		return 1
	}
	return cmp.Compare(a, b)
}
