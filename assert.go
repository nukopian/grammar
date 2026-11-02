package grammar

import "github.com/nukopian/scalar"

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
func Assertion(name string, fn func(m *scalar.SV) bool) Use {
	return &assertion{name: name, fn: fn}
}

type assertion struct {
	name string
	fn   func(*scalar.SV) bool
}

func (a *assertion) useName() string { return a.name }

// Assert is <?{ name($/) }>: it matches no text, and goes on only if fn,
// given the match so far, returns true. The name is how failure reports
// show it. See Assertion.
func Assert(name string, fn func(m *scalar.SV) bool) Node { return &code{name: name, fn: fn} }

// AssertNot is <!{ name($/) }>: it goes on only if fn returns false.
func AssertNot(name string, fn func(m *scalar.SV) bool) Node {
	return &code{name: name, fn: fn, neg: true}
}

// code is a code assertion.
type code struct {
	name string
	fn   func(*scalar.SV) bool
	neg  bool
}

func (n *code) match(st *state, pos int, c *capList, e env, k cont) bool {
	st.tick()
	if n.fn(st.sofar(pos, c, e)) == n.neg {
		st.fail(pos, n, e)
		return false
	}
	return k(pos, c)
}

// sofar is the match so far, as a code assertion at pos sees it, Raku's $/
// there: the scope's captures since it started, from where it started,
// which start's mark says. Inside a lookaround, that's the lookaround's
// own captures, from where it is. It has no made value of its own, as its
// rule hasn't finished.
func (st *state) sofar(pos int, c *capList, e env) *scalar.SV {
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
