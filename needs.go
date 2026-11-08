package grammar

// Most texts a grammar is searched in hold no match, and many can't: a line
// of prose with no digit can't hold a pay figure. So Find and the rest work
// out, from the grammar, a set of characters every match holds at least one
// of, its needs, and try no position past the last of them in the text. A
// text with none is rejected without trying the grammar at all. Skipping
// starts can't do this alone: a rule that allows a space before a figure
// can start at every space.
//
// The working out is cautious, as for starts: where it can't tell, as with
// a regex terminal or a rule that can match nothing, there are no needs, and
// every position is tried as before.

// skipNeeds turns the rejecting off, so tests can check that it changes no
// results.
var skipNeeds = true

// needFinder works out needs, each rule's once: nil for a rule that needs
// nothing, as for one while it's being worked out.
type needFinder struct {
	g     *Grammar
	rules map[string]*starts
}

// needsOf returns a set of characters every match of rule holds at least
// one of, or nil if it can't tell. The set is tabled, and mustn't change.
func (g *Grammar) needsOf(rule string) *starts {
	f := &needFinder{g: g, rules: map[string]*starts{}}
	s := f.need(&call{name: rule}, false)
	if s != nil {
		s.table()
	}
	return s
}

// need returns a set of characters every match of n holds one of, or nil.
// ci is whether n is under :i. A set it returns may be shared with others:
// it's never changed after, so a set made from others is a new one.
func (f *needFinder) need(n Node, ci bool) *starts {
	switch n := n.(type) {
	case lit:
		// Each of its characters is in every match: the rarest will do.
		var best *starts
		for _, r := range n.rs {
			s := &starts{runes: map[rune]bool{}}
			s.addRune(r, ci)
			best = rarer(best, s)
		}
		return best
	case class, words:
		// A match starts with one of the characters it can start with.
		s := &starts{runes: map[rune]bool{}}
		(&startFinder{g: f.g}).add(n, ci, s)
		return s
	case *goal:
		return f.need(n.seq(), ci)
	case seq, conj:
		// Every item matches, so any item's needs are the sequence's: the
		// rarest will do. Every branch of a conjunction matches the same text.
		var best *starts
		for _, c := range children(n) {
			best = rarer(best, f.need(c, ci))
		}
		return best
	case *alt, first, patterns:
		// Any branch may be the one that matches, so the needs are all of
		// theirs, and none if any branch has none.
		cs := children(n)
		if len(cs) == 0 {
			return nil
		}
		u := &starts{runes: map[rune]bool{}}
		for _, c := range cs {
			s := f.need(c, ci)
			if s == nil {
				return nil
			}
			u.merge(s)
		}
		return u
	case *Quant:
		if n.min == 0 {
			return nil
		}
		return f.need(n.item, ci)
	case *call:
		return f.rule(n.name)
	case *mode:
		if n.ci != nil {
			ci = *n.ci
		}
		return f.need(n.inner, ci)
	case *group, *named, commit:
		return f.need(children(n)[0], ci)
	}
	// Anchors, lookarounds and code assertions match no text; a backref
	// matches what a capture did, which may be nothing; a regex terminal
	// could match anything. Anything else, too, needs nothing as far as
	// this knows.
	return nil
}

// rule works out a rule's needs once. A rule reached again while it's being
// worked out needs nothing, as far as this knows.
func (f *needFinder) rule(name string) *starts {
	if s, ok := f.rules[name]; ok {
		return s
	}
	f.rules[name] = nil
	var s *starts
	if rl := f.g.lookup(name); rl != nil {
		s = f.need(rl.body, false) // :i doesn't reach into a call
	}
	f.rules[name] = s
	return s
}

// rarer returns whichever of a and b holds fewer ASCII characters, as a
// guess at which a text is less likely to hold, preferring a; nil, no
// needs, loses to any set.
func rarer(a, b *starts) *starts {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	}
	if asciiIn(b) < asciiIn(a) {
		return b
	}
	return a
}

// asciiIn counts the ASCII characters in s.
func asciiIn(s *starts) int {
	n := 0
	for r := rune(0); r < 128; r++ {
		if s.has(r) {
			n++
		}
	}
	return n
}

// lastNeeded returns the last position in in holding a character in needs,
// or -1 if none does. No match can start after it.
func lastNeeded(needs *starts, in []rune) int {
	for i := len(in) - 1; i >= 0; i-- {
		// The table, looked up here, so that a line of ASCII prose is
		// scanned without a call for each character.
		if r := in[i]; needs.tabled && r >= 0 && r < 128 {
			if needs.ascii[r>>6]&(1<<(r&63)) != 0 {
				return i
			}
		} else if needs.has(r) {
			return i
		}
	}
	return -1
}

// rejects reports whether text, as it is, can't hold a match of rule, by
// its needs, before it's read into runes in NFC: most lines searched are
// ASCII prose, and reading them costs more than the search would. It
// answers only for ASCII text, which NFC leaves as it is; NFC can compose
// a needed character from others, so other text is left to the search. It
// reads from the end, as lastNeeded does, so that where the text holds a
// needed character, the two stop at the same place.
func (g *Grammar) rejects(rule, text string) bool {
	needs := g.searchFor(rule).needs
	if !skipNeeds || needs == nil || !needs.tabled {
		return false
	}
	for i := len(text) - 1; i >= 0; i-- {
		b := text[i]
		if b >= 128 || needs.ascii[b>>6]&(1<<(b&63)) != 0 {
			return false
		}
	}
	return true
}
