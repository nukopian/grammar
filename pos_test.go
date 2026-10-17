package grammar

import (
	"testing"
)

// Pos starts matching part-way into the text, as Raku's :pos does; these
// are Raku's answers (2026-10-17). Positions stay the whole text's.
func TestPos(t *testing.T) {
	g := MustCompile(`grammar G { token TOP { \d+ } }`)
	if m := g.Subparse("ab12cd", Pos(2)); m.At("~").Str() != "12" || m.At("from").Int() != 2 || m.At("to").Int() != 4 {
		t.Errorf("Subparse at 2 = %q from %v to %v, want 12 from 2 to 4", m.At("~").Str(), m.At("from"), m.At("to"))
	}
	if m := g.Parse("ab12", Pos(2)); m.At("~").Str() != "12" {
		t.Errorf("Parse at 2 = %v, want 12", m.At("~"))
	}
	if m := g.Parse("ab12cd", Pos(2)); m.Defined() {
		t.Errorf("Parse at 2 of ab12cd = %v, want no match: it must reach the end", m.At("~"))
	}
	if m := g.Subparse("ab12cd", Pos(0)); m.Defined() {
		t.Errorf("Subparse at 0 = %v, want no match", m.At("~"))
	}
	if m := g.Subparse("ab12", Pos(9)); m.Defined() {
		t.Error("Subparse past the end matched")
	}
	if m, err := g.SubparseErr("ab12cd", Pos(2)); err != nil || m.At("~").Str() != "12" {
		t.Errorf("SubparseErr at 2 = %v, %v", m.At("~"), err)
	}
	// What's before Pos is still there to look back at.
	l := MustCompile(`grammar L { token TOP { <?after b> \d+ } }`)
	if m := l.Subparse("ab12", Pos(2)); m.At("~").Str() != "12" {
		t.Errorf("a lookbehind at Pos: %v, want 12", m.At("~"))
	}
	w := MustCompile(`grammar W { token TOP { << \w+ } }`)
	if m := w.Subparse("ab12", Pos(2)); m.Defined() {
		t.Error("a word boundary inside a word at Pos matched")
	}
	// Find, FindAll and Split search from Pos.
	if m := g.Find("1 ab 23", Pos(1)); m.At("~").Str() != "23" {
		t.Errorf("Find from 1 = %v, want 23", m.At("~"))
	}
	if all := g.FindAll("1 2 3", Pos(1)); all.Length() != 2 {
		t.Errorf("FindAll from 1 found %d, want 2", all.Length())
	}
	if pieces := g.Split("1 2 3", Pos(1)).Join("|"); pieces != " |2| |3|" {
		t.Errorf("Split from 1 = %q", pieces)
	}
	defer func() {
		if recover() == nil {
			t.Error("Pos(-1) didn't panic")
		}
	}()
	g.Subparse("x", Pos(-1))
}
