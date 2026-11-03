package grammar

import (
	"slices"
	"strings"
	"unicode"

	"github.com/dlclark/regexp2"
	"github.com/nukopian/scalar"
)

// Find and FindAll try a grammar at each position in the text in turn. Most
// positions can't start a match: pay can only start at a £. So they work
// out, from the grammar, every character a match can start with, and skip
// the positions where none does. A regex terminal (Re) that a match can start
// with is asked where it next matches, and the positions before that are
// skipped. The working out is cautious: where it can't tell, as with a rule
// that can match nothing, it skips nothing.

// skipStarts turns the skipping off, so tests can check that it changes no
// results.
var skipStarts = true

// starts are the characters a match can start with, or more: any, or a
// character in runes, or one some pred accepts, or a position where one of
// res matches.
type starts struct {
	any   bool
	runes map[rune]bool
	preds []func(rune) bool
	res   []*regexp2.Regexp // regex terminals' patterns, unanchored

	// ascii is what has says for each ASCII character, once tabled says
	// it's been worked out; see table.
	ascii  [2]uint64
	tabled bool
}

// startsTable turns on starts' table of ASCII characters, so tests can
// check that it changes no results.
var startsTable = true

// has reports whether a match can start with r, as far as the characters
// say. A regex in res can start a match where has says no: ask a scan.
func (s *starts) has(r rune) bool {
	if s.tabled && r >= 0 && r < 128 {
		return s.ascii[r>>6]&(1<<(r&63)) != 0
	}
	return s.any || s.runes[r] || slices.ContainsFunc(s.preds, func(p func(rune) bool) bool { return p(r) })
}

// table works out has for every ASCII character once, so that a search,
// which asks at every position, mostly looks it up instead of asking each
// pred. s mustn't change after.
func (s *starts) table() {
	if !startsTable {
		return
	}
	for r := rune(0); r < 128; r++ {
		if s.has(r) {
			s.ascii[r>>6] |= 1 << (r & 63)
		}
	}
	s.tabled = true
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
	s.res = append(s.res, o.res...)
}

// startsEntry caches a grammar's starts for each rule searched for,
// until any grammar changes. It's never changed once stored: a rule's
// starts are added to a copy.
type startsEntry struct {
	gen   uint64
	rules map[string]ruleStartsOf
}

// ruleStartsOf is what startsOf returns for a rule.
type ruleStartsOf struct {
	s     *starts
	empty bool
}

// startsOf returns what a match of rule can start with, and whether it can
// match nothing, in which case it can match anywhere. It's cached.
func (g *Grammar) startsOf(rule string) (*starts, bool) {
	gen := generation.Load()
	e := g.starts.Load()
	if e != nil && e.gen != gen {
		e = nil
	}
	if e != nil {
		if r, ok := e.rules[rule]; ok {
			return r.s, r.empty
		}
	}
	f := &startFinder{g: g, rules: map[string]*ruleStarts{}}
	s := &starts{runes: map[rune]bool{}}
	empty := f.add(&call{name: rule}, false, s)
	s.table()
	rules := map[string]ruleStartsOf{rule: {s, empty}}
	if e != nil {
		for k, r := range e.rules {
			rules[k] = r
		}
	}
	g.starts.Store(&startsEntry{gen, rules})
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
			acro := l.acro // an acronym starts with its capital, as written
			s.preds = append(s.preds, func(r rune) bool {
				return foldStep(root, unicode.ToLower(r)) != nil || acro && foldStep(root, r) != nil
			})
		} else {
			for r := range l.root.next {
				s.runes[r] = true
			}
		}
		return false
	case anchor, *look, *code:
		return true // they match no text, so what follows starts the match
	case *goal:
		return f.add(n.seq(), ci, s)
	case patterns:
		empty := false
		for _, c := range n.l.nodes {
			empty = f.add(c, ci, s) || empty
		}
		return empty
	case conj:
		// Every branch starts where the first does, so its starts will do.
		return f.add(n[0], ci, s)
	case seq:
		for _, c := range n {
			if !f.add(c, ci, s) {
				return false
			}
		}
		return true
	case *alt, first:
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
	case re:
		// A match can start only where the pattern matches, which regexp2
		// finds with the pattern unanchored. A pattern that can match
		// empty text could start anywhere; one with \G means something
		// else unanchored.
		if n.empty || strings.Contains(n.pattern, `\G`) {
			break
		}
		opt := regexp2.None
		if ci {
			opt = regexp2.IgnoreCase
		}
		rx := regexp2.MustCompile(`(?:`+n.pattern+`)`, opt)
		rx.MatchTimeout = scalar.RegexTimeout
		s.res = append(s.res, rx)
		return false
	}
	// Anything else could start with anything.
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

// scan says where in a text a search can start a match of a rule. For each
// regex in its starts, it remembers where the regex next matches, so a
// search asks again only once it has passed that.
type scan struct {
	s     *starts
	all   bool // every position can start a match: skip none
	re    bool // there are regexes to ask
	in    []rune
	from  []int  // where each regex was last asked from; -1 before it's asked
	next  []int  // where it next matches, at or after from; past the end if nowhere
	stuck []bool // it failed, as by running past RegexTimeout: don't skip for it
}

// scanner starts a scan of in for a match of rule. It's a value, so a
// search that has no regex to ask allocates nothing for it.
func (g *Grammar) scanner(rule string, in []rune) scan {
	s, empty := g.startsOf(rule)
	c := scan{s: s, all: !skipStarts || empty || s.any, re: len(s.res) > 0, in: in}
	if n := len(s.res); n > 0 {
		c.from, c.next, c.stuck = make([]int, n), make([]int, n), make([]bool, n)
		for i := range c.from {
			c.from[i], c.next[i] = -1, -1 // not yet asked
		}
	}
	return c
}

// can reports whether a match can start at position at, as far as the
// starts say. Positions are asked in order, as a search goes.
func (c *scan) can(at int) bool {
	if c.all {
		return true
	}
	if at < len(c.in) && c.s.has(c.in[at]) {
		return true
	}
	return c.re && c.canRe(at)
}

// canRe reports whether a regex in the starts matches at position at.
func (c *scan) canRe(at int) bool {
	for i, rx := range c.s.res {
		if c.stuck[i] {
			return true
		}
		if at < c.from[i] || at > c.next[i] {
			m, err := rx.FindRunesMatchStartingAt(c.in, at)
			switch {
			case err != nil:
				// A search over the rest of the text can run past
				// RegexTimeout where a match at one position wouldn't, so
				// try every position, as the matcher would without it.
				c.stuck[i] = true
				return true
			case m == nil:
				c.from[i], c.next[i] = at, len(c.in)+1
			default:
				c.from[i], c.next[i] = at, m.Index
			}
		}
		if c.next[i] == at {
			return true
		}
	}
	return false
}
