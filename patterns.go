package grammar

import "fmt"

// PatternList is a list of patterns, each a regex in Raku syntax, for
// <@name> to match, as Raku's <@array> does: where a WordList holds phrases
// to match as text, with @name, a PatternList holds patterns, such as
// \d+ k or per \s* annum, so that a family of patterns can be data rather
// than grammar. Pass it to Compile, as a WordList is passed.
//
// <@name> matches each pattern at the current position and keeps the
// longest match, the first in the list on a tie, and, as in Raku, commits
// to it: if what follows fails, no other pattern is tried, even in a
// regex. A pattern can call the grammar's rules, as <amount> or <alpha>,
// and :i reaches into it, but it's read as a regex, not a rule, so its
// spaces don't match spaces in the text, even in a rule: quote them, as in
// 'per annum'. What a pattern captures isn't kept, and <@name> captures
// nothing itself; wrap it to capture, as in $<period>=<@periods>. A pattern
// can't use a word list.
type PatternList struct {
	name     string
	patterns []string
	nodes    []Node
	scopes   []*scope
}

// NewPatternList compiles patterns into a list called name, which is how
// Raku syntax refers to it: <@name>. It reports the first pattern that
// doesn't compile, where Raku would fail only when matching it.
func NewPatternList(name string, patterns ...string) (*PatternList, error) {
	l := &PatternList{name: name, patterns: patterns}
	for _, pat := range patterns {
		n, err := compilePattern(pat)
		if err != nil {
			return nil, fmt.Errorf("pattern list %s: pattern %q: %w", name, pat, err)
		}
		l.nodes = append(l.nodes, n)
		l.scopes = append(l.scopes, newScope(n))
	}
	return l, nil
}

// MustPatternList is NewPatternList, panicking if a pattern doesn't compile.
func MustPatternList(name string, patterns ...string) *PatternList {
	l, err := NewPatternList(name, patterns...)
	if err != nil {
		panic(err.Error())
	}
	return l
}

// Name returns the list's name.
func (l *PatternList) Name() string { return l.name }

func (l *PatternList) useName() string { return l.name }

// compilePattern reads one pattern as the body of a regex.
func compilePattern(pat string) (n Node, err error) {
	p := &parser{src: []rune(nfc(pat)), grammars: map[string]*Grammar{}, lists: map[string]*WordList{}}
	defer func() {
		if r := recover(); r != nil {
			pe, ok := r.(parseError)
			if !ok {
				panic(r)
			}
			err = pe
		}
	}()
	n = p.regex()
	if !p.eof() {
		p.fail("unexpected %q", string(p.src[p.i]))
	}
	n, _ = distinct(n, map[*group]bool{}, "")
	walk(n, func(m Node) {
		if _, ok := m.(symText); ok {
			p.fail("<sym> is only for a rule named name:sym<…>")
		}
	})
	return n, nil
}

type patterns struct{ l *PatternList }

func (n patterns) match(st *state, pos int, c *capList, e env, k cont) bool {
	st.tick()
	st.deeper()
	defer st.shallower()
	end := -1
	for i, pat := range n.l.nodes {
		// Each pattern is a regex, backtracking within itself; its first
		// match is its match.
		if p, _, ok := once(pat, st, pos, nil, env{ci: e.ci, sc: n.l.scopes[i]}); ok && p > end {
			end = p
		}
	}
	if end < 0 {
		st.fail(pos, n, e)
		return false
	}
	return k(end, c) // keeping none of the pattern's captures
}
