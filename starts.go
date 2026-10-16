package grammar

import (
	"slices"
	"unicode"
)

// Find and FindAll try a grammar at each position in the text in turn. Most
// positions can't start a match: pay can only start at a £. So they work
// out, from the grammar, every character a match can start with, and skip
// the positions where none does. The working out is cautious: where it can't
// tell, as with a regex terminal (Re), or a rule that can match nothing, it
// skips nothing.

// skipStarts turns the skipping off, so tests can check that it changes no
// results.
var skipStarts = true

// starts are the characters a match can start with, or more: any, or a
// character in runes, or one some pred accepts.
type starts struct {
	any   bool
	runes map[rune]bool
	preds []func(rune) bool
}

func (s *starts) has(r rune) bool {
	return s.any || s.runes[r] || slices.ContainsFunc(s.preds, func(p func(rune) bool) bool { return p(r) })
}

// addRune adds r, and with ci its other cases.
func (s *starts) addRune(r rune, ci bool) {
	s.runes[r] = true
	if ci {
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			s.runes[f] = true
		}
	}
}

// addPred adds a class's test, and with ci, a test of the other cases too, as
// a class under :i matches them.
func (s *starts) addPred(in func(rune) bool, ci bool) {
	if !ci {
		s.preds = append(s.preds, in)
		return
	}
	s.preds = append(s.preds, func(r rune) bool {
		if in(r) {
			return true
		}
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if in(f) {
				return true
			}
		}
		return false
	})
}

func (s *starts) merge(o *starts) {
	s.any = s.any || o.any
	for r := range o.runes {
		s.runes[r] = true
	}
	s.preds = append(s.preds, o.preds...)
}

// startsEntry caches a grammar's starts for one rule, until any grammar
// changes.
type startsEntry struct {
	gen   uint64
	rule  string
	s     *starts
	empty bool
}

// startsOf returns what a match of rule can start with, and whether it can
// match nothing, in which case it can match anywhere. It's cached.
func (g *Grammar) startsOf(rule string) (*starts, bool) {
	gen := generation.Load()
	if e := g.starts.Load(); e != nil && e.gen == gen && e.rule == rule {
		return e.s, e.empty
	}
	f := &startFinder{g: g, rules: map[string]*ruleStarts{}}
	s := &starts{runes: map[rune]bool{}}
	empty := f.add(&call{name: rule}, false, s)
	g.starts.Store(&startsEntry{gen, rule, s, empty})
	return s, empty
}

type startFinder struct {
	g     *Grammar
	rules map[string]*ruleStarts
}

type ruleStarts struct {
	s     *starts
	empty bool
	done  bool // false while it's being worked out
}

// add adds what n can start with to s, and reports whether n can match
// nothing, in which case what follows n can start the match too. ci is
// whether n is under :i.
func (f *startFinder) add(n Node, ci bool, s *starts) bool {
	switch n := n.(type) {
	case lit:
		if len(n.rs) == 0 {
			return true
		}
		s.addRune(n.rs[0], ci)
		return false
	case class:
		if ci && n.inCI != nil {
			s.preds = append(s.preds, n.inCI)
		} else {
			s.addPred(n.in, ci)
		}
		return false
	case words:
		l := n.l
		if l.joined && len(l.root.spacing) > 0 {
			// A phrase starts with spacing the text may leave out, so it
			// can start with anything.
			s.preds = append(s.preds, func(rune) bool { return true })
			return false
		}
		if l.fold || ci {
			root := l.rootFold
			s.preds = append(s.preds, func(r rune) bool { return root.next[unicode.ToLower(r)] != nil })
		} else {
			for r := range l.root.next {
				s.runes[r] = true
			}
		}
		return false
	case anchor, *look:
		return true // they match no text, so what follows starts the match
	case *goal:
		return f.add(n.seq(), ci, s)
	case seq:
		for _, c := range n {
			if !f.add(c, ci, s) {
				return false
			}
		}
		return true
	case alt, first:
		empty := false
		for _, c := range children(n) {
			empty = f.add(c, ci, s) || empty
		}
		return empty
	case *Quant:
		itemEmpty := f.add(n.item, ci, s)
		sepEmpty := true
		if itemEmpty && n.sep != nil {
			sepEmpty = f.add(n.sep, ci, s)
		}
		return n.min == 0 || itemEmpty && (n.min <= 1 || sepEmpty)
	case *call:
		r := f.rule(n.name)
		s.merge(r.s)
		return r.empty
	case *mode:
		if n.ci != nil {
			ci = *n.ci
		}
		return f.add(n.inner, ci, s)
	case *group, *named, commit:
		return f.add(children(n)[0], ci, s)
	}
	// A regex terminal, or anything else: it could start with anything.
	s.any = true
	return true
}

// rule works out a rule's starts once. A rule reached again while it's
// being worked out could start with anything, as far as this knows.
func (f *startFinder) rule(name string) *ruleStarts {
	if r, ok := f.rules[name]; ok {
		if !r.done {
			return &ruleStarts{s: &starts{any: true}, empty: true}
		}
		return r
	}
	r := &ruleStarts{s: &starts{runes: map[rune]bool{}}}
	f.rules[name] = r
	if rl := f.g.lookup(name); rl != nil {
		r.empty = f.add(rl.body, false, r.s) // :i doesn't reach into a call
	} else {
		r.s.any, r.empty = true, true
	}
	r.done = true
	return r
}

// canStart reports whether a match can start at position at of in, as far
// as s says.
func canStart(s *starts, empty bool, in []rune, at int) bool {
	if !skipStarts || empty || s.any {
		return true
	}
	return at < len(in) && s.has(in[at])
}
