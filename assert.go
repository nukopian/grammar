package grammar

import (
	"fmt"
	"slices"
	"strings"

	"github.com/nukopian/scalar"
)

// Assertion is a Go function for a grammar's code assertions to call by
// name: <?{ name($/) }> goes on where fn returns true, and <!{ name($/) }>
// where it returns false. Pass it to Compile with the grammar's source. As
// in Raku, fn is given the match so far: the rule's (or the ( ... )
// group's) captures up to the assertion, each with the made value its
// action gave it, and its text from where the rule started to where the
// assertion is. In a regex, a false answer backtracks, so a reading that
// passes can still be found:
//
//	grammar Range {
//	    regex TOP { <min=num> '-' <max=num> <?{ hourly($/) }> }
//	    regex num { \d+ }
//	}
//
// with hourly checking that max is at most 100 takes "5-12" of "5-1234".
// The same source runs in Raku, given a sub hourly($/).
//
// The match fn is given is read-only, and it's called as the parse reaches
// the assertion, from the goroutine parsing, so a grammar parsing on
// several goroutines at once calls fn on each.
//
// Naming the captures fn reads, as reads, builds only those, with the
// match's text and position, which is quicker where the rule has others:
// Assertion("pay", pay, "cur", "period"). A named capture the match
// hasn't got reads as undef, or an empty array where it can capture more
// than once, and reading a capture that isn't named panics, saying which.
func Assertion(name string, fn func(m *scalar.SV) bool, reads ...string) Use {
	return &assertion{name: name, fn: fn, reads: reads}
}

type assertion struct {
	name  string
	fn    func(*scalar.SV) bool
	reads []string
}

func (a *assertion) useName() string { return a.name }

// Assert is <?{ name($/) }>: it matches no text, and goes on only if fn,
// given the match so far, returns true. The name is how failure reports
// show it, and reads names the captures fn reads. See Assertion.
func Assert(name string, fn func(m *scalar.SV) bool, reads ...string) Node {
	return &code{name: name, fn: fn, reads: reads}
}

// AssertNot is <!{ name($/) }>: it goes on only if fn returns false.
func AssertNot(name string, fn func(m *scalar.SV) bool, reads ...string) Node {
	return &code{name: name, fn: fn, neg: true, reads: reads}
}

// code is a code assertion.
type code struct {
	name  string
	fn    func(*scalar.SV) bool
	neg   bool
	reads []string // nil: every capture
}

func (n *code) match(st *state, pos int, c *capList, e env, k cont) bool {
	st.tick()
	if n.call(st.sofar(pos, c, e, n.reads)) == n.neg {
		st.fail(pos, n, e)
		return false
	}
	return k(pos, c)
}

// call calls fn, saying which assertion it was if fn reads a capture it
// doesn't name, which the restricted match it's given panics at.
func (n *code) call(m *scalar.SV) bool {
	if n.reads != nil && namedReads {
		defer func() {
			if r := recover(); r != nil {
				if s, ok := r.(string); ok && strings.HasPrefix(s, "scalar: Attempt to access disallowed key") {
					r = fmt.Sprintf("grammar: %s($/) reads a capture it doesn't name (it names %s): %s",
						n.name, strings.Join(n.reads, ", "), s)
				}
				panic(r)
			}
		}()
	}
	return n.fn(m)
}

// namedReads has an assertion that names the captures it reads build only
// those. Tests turn it off to check that it changes no match.
var namedReads = true

// sofar is the match so far, as a code assertion at pos sees it, Raku's $/
// there: the scope's captures since it started, from where it started,
// which start's mark says. Inside a lookaround, that's the lookaround's
// own captures, from where it is. It has no made value of its own, as its
// rule hasn't finished.
func (st *state) sofar(pos int, c *capList, e env, reads []string) *scalar.SV {
	at := c
	for at != nil && at.key != startMark {
		at = at.next
	}
	if at == nil {
		panic("grammar: a code assertion outside any rule") // hasCode marks every scope one is in
	}
	from, mine := at.m.from, c // build passes over the mark at the bottom
	if at.next != nil {
		// A lookaround's: only the captures above the mark, newest
		// first, as a list of their own that ends at nil.
		var caps []*capList
		for ; c != at; c = c.next {
			caps = append(caps, c)
		}
		mine = nil
		for i := len(caps) - 1; i >= 0; i-- {
			mine = &capList{caps[i].key, caps[i].m, mine}
		}
	}
	sc := e.sc
	if sc == nil {
		sc = emptyScope
	}
	m := &mnode{from: from, to: pos, caps: mine, sc: sc}
	if reads != nil && namedReads {
		return st.only(m, reads)
	}
	if st.act == nil {
		// With no actions, nothing built here is kept for the result, so
		// it needs no copy to stay read-only.
		b := &builder{in: st.in, built: map[*mnode]*scalar.SV{}}
		return b.build(m).Lock().Autovivify(false)
	}
	b := st.builder()
	sv := b.build(m)
	delete(b.built, m) // it's made afresh at each assertion
	// A copy, locked, so that nothing fn does changes the match the parse
	// returns, which shares the captures' matches.
	return sv.Clone().Lock().Autovivify(false)
}

// only is sofar for an assertion that names the captures it reads: the
// match's text and position, an undef made, as it has none yet, and those
// captures, each named one there, undef or an empty array if it hasn't
// matched. Its own keys are fixed, so reading another panics, while what's
// below them reads as sofar's does.
func (st *state) only(m *mnode, reads []string) *scalar.SV {
	var b *builder
	if st.act == nil {
		b = &builder{in: st.in, built: map[*mnode]*scalar.SV{}}
	} else {
		b = st.builder()
	}
	h := scalar.Hash("~", string(st.in[m.from:m.to]), "from", m.from, "to", m.to, "made", nil)
	for _, k := range reads {
		if m.sc.many[k] {
			h.Set(k, scalar.Array())
		} else {
			h.Set(k, nil)
		}
	}
	var caps []*capList
	for c := m.caps; c != nil; c = c.next {
		if slices.Contains(reads, c.key) {
			caps = append(caps, c)
		}
	}
	for _, c := range slices.Backward(caps) {
		if v := b.build(c.m); m.sc.many[c.key] {
			h.Get(c.key).AV().Push(v)
		} else {
			h.Set(c.key, v)
		}
	}
	sv := scalar.Value(h)
	if st.act != nil {
		// A copy, as in sofar: the captures are the result's too.
		sv = sv.Clone()
		h = sv.HV()
	}
	sv.Lock()
	h.Autovivify(true, false)
	return sv
}
