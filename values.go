package grammar

import (
	"errors"
	"slices"

	"github.com/nukopian/scalar"
)

// ErrNoMatch is the error To returns when it's given no match, as Parse,
// Subparse and Find return when the text doesn't match.
var ErrNoMatch = errors.New("grammar: no match")

// Values reduces a match to the values in it, ready for scalar.To. Each
// match, at every level, becomes:
//
//   - its made value, if an action made one (or it's a word list's rule,
//     whose made value is the canonical value);
//   - otherwise, if it has captures, a hash of them, each reduced in turn,
//     with arrays of captures as arrays;
//   - otherwise its text.
//
// So a rule that only holds captures together becomes a hash of what they
// hold, and the hash's keys are the capture names, which scalar.To matches to
// struct fields ignoring case, as encoding/json does. Captures that didn't
// match are missing, so their fields keep their zero values. Raku's nearest
// idiom is .made // ~$/, for one match.
//
// An undef match, as Parse returns when the text doesn't match, gives undef.
func Values(m *scalar.SV) *scalar.SV {
	if m.Kind() != scalar.HashRef {
		return m
	}
	h := m.HV()
	if made := h.Get("made"); made.Defined() {
		return made
	}
	out := scalar.Hash()
	for key, v := range h.All() {
		if slices.Contains(reserved, key) {
			continue
		}
		if v.Kind() == scalar.ArrayRef {
			out.Set(key, v.AV().Map(func(item *scalar.SV) any { return Values(item) }))
		} else {
			out.Set(key, Values(v))
		}
	}
	if out.Length() == 0 {
		return h.Get("~")
	}
	return scalar.Value(out)
}

// To reduces a match with Values and converts it to a T with scalar.To, so
// a grammar fills a struct directly:
//
//	title, err := grammar.To[Title](g.Parse(text))
//
// It returns ErrNoMatch for an undef match, so a failed parse isn't
// mistaken for an empty T; otherwise its errors are scalar.To's.
func To[T any](m *scalar.SV) (T, error) {
	if !m.Defined() {
		var zero T
		return zero, ErrNoMatch
	}
	return scalar.To[T](Values(m))
}

// ToAll is To for FindAll: it reduces each match with Values and converts
// them to a []T, in order, so a word list's canonical values come back as
// Go strings:
//
//	kinds, err := grammar.ToAll[string](g.FindAll(text))
//
// No match gives an empty slice; its errors are scalar.To's, which name
// the match by its index. Raku's nearest idiom is @matches».made.
func ToAll[T any](matches *scalar.AV) ([]T, error) {
	return scalar.To[[]T](matches.Map(func(m *scalar.SV) any { return Values(m) }))
}
