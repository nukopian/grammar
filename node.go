package grammar

import (
	"slices"
	"strings"
	"unicode"

	"github.com/dlclark/regexp2"

	"github.com/nukopian/job-meta/internal/scalar"
)

// The matcher works in continuation-passing style: each node matches at pos
// and, for every way it can match, calls k with the position after it and
// the captures so far. When k returns false, the node tries its next way, so
// backtracking, including back into a regex called as a subrule, is just a
// node returning false to its caller. A ratcheted match (a token, or :r)
// takes only the first way of each atom; see step.

// Node is one piece of a grammar: a literal, a character class, a sequence,
// a call to another rule, and so on. Build nodes with this package's
// functions, or write the grammar in Raku syntax and Compile it.
type Node interface {
	match(st *state, pos int, c *capList, e env, k cont) bool
}

type cont func(pos int, c *capList) bool

// env is the lexical context a node matches in: the adverbs in force, and
// the scope its captures belong to.
type env struct {
	ratchet bool
	ci      bool
	sc      *scope
}

// state is one match attempt over one input.
type state struct {
	in []rune
	g  *Grammar

	// tr tracks failures, for ParseErr and SubparseErr. It's nil
	// otherwise, so ordinary parsing pays nothing for it.
	tr *tracker
}

// tracker records failures for a failure report. far is the furthest
// position at which anything failed, and exp what was expected there. calls
// are the rules being matched, outermost first, so that a failure where a
// rule started can be reported as that rule. quiet is above 0 where failures
// aren't worth reporting: inside lookarounds and <ws>. opt are where the
// optional repeats being tried began, innermost last: one more \s in \s* not
// matching is no news.
type tracker struct {
	far   int
	exp   []string
	calls []frame
	quiet int
	opt   []int
}

type frame struct {
	rule string
	pos  int
}

func (t *tracker) enter(f frame) {
	t.calls = append(t.calls, f)
	if f.rule == "ws" {
		t.quiet++
	}
}

func (t *tracker) leave(f frame) {
	t.calls = t.calls[:len(t.calls)-1]
	if f.rule == "ws" {
		t.quiet--
	}
}

// fail records that n didn't match at pos, for a failure report. The report
// names the outermost rule that started at pos, other than the one parsing
// started with, and otherwise describes n.
func (st *state) fail(pos int, n Node, e env) {
	t := st.tr
	if t == nil || t.quiet > 0 || pos < t.far {
		return
	}
	if _, isClass := n.(class); isClass && len(t.opt) > 0 && t.opt[len(t.opt)-1] == pos {
		return // a class repeat such as \s* stopping, as it was free to
	}
	if pos > t.far {
		t.far, t.exp = pos, t.exp[:0]
	}
	desc := ""
	if len(t.calls) > 1 {
		for _, f := range t.calls[1:] {
			if f.pos == pos {
				desc = "<" + f.rule + ">"
				break
			}
		}
	}
	if desc == "" {
		desc = describe(n, e.ci)
	}
	if !slices.Contains(t.exp, desc) {
		t.exp = append(t.exp, desc)
	}
}

// describe says what n matches, for a failure report, as the grammar would
// write it where it can: 'text', \d, <[a..z]>, @list, <rule>.
func describe(n Node, ci bool) string {
	switch n := n.(type) {
	case lit:
		s := "'" + string(n.rs) + "'"
		if ci {
			s += " (ignoring case)"
		}
		return s
	case class:
		return n.desc
	case words:
		return "@" + n.l.name
	case re:
		return "/" + n.pattern + "/"
	case anchor:
		return anchorNames[n]
	case *call:
		return "<" + n.name + ">"
	case *look:
		if n.neg {
			return "not " + describe(n.inner, ci)
		}
		return describe(n.inner, ci)
	case seq:
		if len(n) > 0 {
			return describe(n[0], ci)
		}
	case alt, first:
		var descs []string
		for _, c := range children(n) {
			descs = append(descs, describe(c, ci))
		}
		return strings.Join(descs, " or ")
	case *Quant, *group, *named, commit:
		return describe(children(n)[0], ci)
	case *mode:
		if n.ci != nil {
			ci = *n.ci
		}
		return describe(n.inner, ci)
	}
	return "more"
}

// capList is the captures made so far in one scope, newest first. It's
// persistent, so backtracking past a capture forgets it for free.
type capList struct {
	key  string
	m    *mnode
	next *capList
}

// mnode is a finished match of a rule or a capture group.
type mnode struct {
	rule     string // the rule that made it; "" for a group
	from, to int
	caps     *capList
	sc       *scope
	words    *WordList // the list the rule is, if it is one
}

// once matches n and keeps only its first way of matching.
func once(n Node, st *state, pos int, c *capList, e env) (int, *capList, bool) {
	var end int
	var caps *capList
	ok := n.match(st, pos, c, e, func(p int, cc *capList) bool {
		end, caps = p, cc
		return true
	})
	return end, caps, ok
}

// step matches n as one atom: under ratchet, it commits to n's first way of
// matching, which is what makes quantifiers possessive and alternations
// final in a token. A frugal quantifier still takes one more item when what
// follows it fails, as in Raku, so that token { .*? x } finds the x; see
// extendable.
func step(n Node, st *state, pos int, c *capList, e env, k cont) bool {
	if !e.ratchet || extendable(n) {
		return n.match(st, pos, c, e, k)
	}
	p, cc, ok := once(n, st, pos, c, e)
	return ok && k(p, cc)
}

// extendable reports whether n holds a frugal quantifier that can still
// take more under ratchet. As in Raku, one can through [ ... ], $<name>=[
// ... ] and :i, but not through a capture ( ... ), a call, an alternation or
// another quantifier, all of which commit.
func extendable(n Node) bool {
	switch n := n.(type) {
	case *Quant:
		return n.frugal
	case seq:
		return slices.ContainsFunc(n, extendable)
	case *named:
		return extendable(n.inner)
	case *mode:
		return extendable(n.inner)
	}
	return false
}

// Literals and character classes.

type lit struct{ rs []rune }

// Lit matches s exactly, or ignoring case inside IgnoreCase.
func Lit(s string) Node { return lit{[]rune(s)} }

func (n lit) match(st *state, pos int, c *capList, e env, k cont) bool {
	if pos+len(n.rs) > len(st.in) {
		st.fail(pos, n, e)
		return false
	}
	for i, r := range n.rs {
		if !runeEq(st.in[pos+i], r, e.ci) {
			st.fail(pos, n, e)
			return false
		}
	}
	return k(pos+len(n.rs), c)
}

func runeEq(a, b rune, ci bool) bool {
	if a == b {
		return true
	}
	if !ci {
		return false
	}
	for f := unicode.SimpleFold(a); f != a; f = unicode.SimpleFold(f) {
		if f == b {
			return true
		}
	}
	return false
}

type class struct {
	in   func(rune) bool
	desc string // for failure reports, as the grammar writes it: \d, <[a..z]>
}

// Class matches one character for which in returns true.
func Class(in func(rune) bool) Node { return class{in, "a character of a class"} }

func (n class) match(st *state, pos int, c *capList, e env, k cont) bool {
	if pos >= len(st.in) {
		st.fail(pos, n, e)
		return false
	}
	r := st.in[pos]
	ok := n.in(r)
	if !ok && e.ci {
		for f := unicode.SimpleFold(r); f != r && !ok; f = unicode.SimpleFold(f) {
			ok = n.in(f)
		}
	}
	if !ok {
		st.fail(pos, n, e)
		return false
	}
	return k(pos+1, c)
}

func isDigit(r rune) bool { return unicode.IsDigit(r) }
func isAlpha(r rune) bool { return unicode.IsLetter(r) || r == '_' } // Raku's <alpha> includes _
func isWord(r rune) bool  { return isAlpha(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) }
func isSpace(r rune) bool { return unicode.IsSpace(r) }
func isHSpace(r rune) bool {
	return r == '\t' || unicode.Is(unicode.Zs, r)
}
func isVSpace(r rune) bool                  { return r >= '\n' && r <= '\r' || r == 0x85 || r == 0x2028 || r == 0x2029 }
func not(f func(rune) bool) func(rune) bool { return func(r rune) bool { return !f(r) } }

// The character classes of Raku's backslash sequences.
var (
	AnyChar Node = class{func(rune) bool { return true }, "any character"} // .
	Digit   Node = class{isDigit, `\d`}                                    // \d
	Word    Node = class{isWord, `\w`}                                     // \w
	Space   Node = class{isSpace, `\s`}                                    // \s
)

// Regexes from regexp2, as terminals.

type re struct {
	rx, rxI *regexp2.Regexp
	pattern string
}

// Re matches pattern, a Perl-style regex (as scalar's M takes), at the
// current position. It's one atom: whatever it matches first is final, as
// if it were a token. As with scalar's regexes, an invalid pattern panics,
// and so does a match that runs longer than scalar.RegexTimeout.
func Re(pattern string) Node {
	src := `\G(?:` + pattern + `)`
	n := re{regexp2.MustCompile(src, regexp2.None), regexp2.MustCompile(src, regexp2.IgnoreCase), pattern}
	n.rx.MatchTimeout, n.rxI.MatchTimeout = scalar.RegexTimeout, scalar.RegexTimeout
	return n
}

func (n re) match(st *state, pos int, c *capList, e env, k cont) bool {
	rx := n.rx
	if e.ci {
		rx = n.rxI
	}
	m, err := rx.FindRunesMatchStartingAt(st.in, pos)
	if err != nil {
		panic("grammar: " + err.Error())
	}
	if m == nil || m.Index != pos {
		st.fail(pos, n, e)
		return false
	}
	return k(pos+m.Length, c)
}

// Anchors.

type anchor int

const (
	aStart anchor = iota
	aEnd
	aLineStart
	aLineEnd
	aWordStart
	aWordEnd
	aNotWithinWord // <!ww>, as <ws> uses
)

var anchorNames = map[anchor]string{
	aStart: "start of text", aEnd: "end of text", aLineStart: "start of line", aLineEnd: "end of line",
	aWordStart: "start of word", aWordEnd: "end of word", aNotWithinWord: "a word boundary",
}

// The anchors: ^, $, ^^, $$, << and >>.
var (
	Start     Node = aStart
	End       Node = aEnd
	LineStart Node = aLineStart
	LineEnd   Node = aLineEnd
	WordStart Node = aWordStart
	WordEnd   Node = aWordEnd
)

func (n anchor) match(st *state, pos int, c *capList, e env, k cont) bool {
	in := st.in
	wordBefore := pos > 0 && isWord(in[pos-1])
	wordAfter := pos < len(in) && isWord(in[pos])
	var ok bool
	switch n {
	case aStart:
		ok = pos == 0
	case aEnd:
		ok = pos == len(in)
	case aLineStart:
		ok = pos == 0 || in[pos-1] == '\n' && pos < len(in)
	case aLineEnd:
		ok = pos == len(in) || in[pos] == '\n'
	case aWordStart:
		ok = !wordBefore && wordAfter
	case aWordEnd:
		ok = wordBefore && !wordAfter
	case aNotWithinWord:
		ok = !(wordBefore && wordAfter)
	}
	if !ok {
		st.fail(pos, n, e)
		return false
	}
	return k(pos, c)
}

// Sequences and alternations.

type seq []Node

// Seq matches each node in turn.
func Seq(nodes ...Node) Node {
	if len(nodes) == 1 {
		return nodes[0]
	}
	return seq(nodes)
}

func (n seq) match(st *state, pos int, c *capList, e env, k cont) bool {
	return n.from(0, st, pos, c, e, k)
}

func (n seq) from(i int, st *state, pos int, c *capList, e env, k cont) bool {
	if i == len(n) {
		return k(pos, c)
	}
	return step(n[i], st, pos, c, e, func(p int, cc *capList) bool {
		return n.from(i+1, st, p, cc, e, k)
	})
}

type alt []Node

// Alt is Raku's |: the branch that matches the most text wins, and the
// earlier branch wins a tie. Raku decides by each branch's declarative
// prefix; Alt approximates that by the length of each branch's first match.
func Alt(nodes ...Node) Node { return alt(nodes) }

func (n alt) match(st *state, pos int, c *capList, e env, k cont) bool {
	type cand struct {
		i, end int
		caps   *capList
	}
	var cands []cand
	for i, b := range n {
		if p, cc, ok := once(b, st, pos, c, e); ok {
			cands = append(cands, cand{i, p, cc})
		}
	}
	if e.ratchet {
		// A token commits to the winner's first match, which once has just
		// found, so there's no need to match it again. Matching it again
		// would double the work at each level of nested alternations.
		if len(cands) == 0 {
			return false
		}
		best := cands[0]
		for _, cd := range cands[1:] {
			if cd.end > best.end {
				best = cd
			}
		}
		return k(best.end, best.caps)
	}
	slices.SortStableFunc(cands, func(a, b cand) int { return b.end - a.end })
	for _, cd := range cands {
		if n[cd.i].match(st, pos, c, e, k) {
			return true
		}
	}
	return false
}

type first []Node

// First is Raku's ||: the first branch that matches wins.
func First(nodes ...Node) Node { return first(nodes) }

func (n first) match(st *state, pos int, c *capList, e env, k cont) bool {
	for _, b := range n {
		if b.match(st, pos, c, e, k) {
			return true
		}
	}
	return false
}

// Quantifiers.

// Quant is a quantified node: ?, *, +, or ** min..max, optionally with a
// separator (% or %%) and frugal (?) rather than greedy.
type Quant struct {
	item     Node
	min, max int // max < 0: no limit
	sep      Node
	trailing bool // %%: a separator may follow the last item
	frugal   bool
}

// Opt is n?.
func Opt(n Node) *Quant { return Repeat(n, 0, 1) }

// Star is n*.
func Star(n Node) *Quant { return Repeat(n, 0, -1) }

// Plus is n+.
func Plus(n Node) *Quant { return Repeat(n, 1, -1) }

// Repeat is n ** min..max; a negative max is no limit, as in ** 2..*.
func Repeat(n Node, min, max int) *Quant { return &Quant{item: n, min: min, max: max} }

// Sep returns q with sep between items, as % does.
func (q *Quant) Sep(sep Node) *Quant {
	r := *q
	r.sep, r.trailing = sep, false
	return &r
}

// SepTrailing returns q with sep between items and optionally after the
// last, as %% does.
func (q *Quant) SepTrailing(sep Node) *Quant {
	r := *q
	r.sep, r.trailing = sep, true
	return &r
}

// Frugal returns q matching as few items as it can, as *? and +? do.
func (q *Quant) Frugal() *Quant {
	r := *q
	r.frugal = true
	return &r
}

func (q *Quant) match(st *state, pos int, c *capList, e env, k cont) bool {
	return q.iter(st, pos, c, e, 0, k)
}

func (q *Quant) iter(st *state, pos int, c *capList, e env, count int, k cont) bool {
	more := func() bool {
		if q.max >= 0 && count >= q.max {
			return false
		}
		item := func(p0 int, c0 *capList) bool {
			// An item beyond the minimum is optional: note where it
			// starts, for failure reports, while it's being tried.
			t := st.tr
			optional := t != nil && count >= q.min
			if optional {
				t.opt = append(t.opt, p0)
			}
			ok := step(q.item, st, p0, c0, e, func(p int, cc *capList) bool {
				if p == pos && count >= q.min {
					return false // an empty item would repeat forever
				}
				if optional {
					t.opt = t.opt[:len(t.opt)-1]
				}
				res := q.iter(st, p, cc, e, count+1, k)
				if optional {
					t.opt = append(t.opt, p0)
				}
				return res
			})
			if optional {
				t.opt = t.opt[:len(t.opt)-1]
			}
			return ok
		}
		if count > 0 && q.sep != nil {
			return step(q.sep, st, pos, c, e, item)
		}
		return item(pos, c)
	}
	done := func() bool {
		if count < q.min {
			return false
		}
		if q.trailing && count > 0 && step(q.sep, st, pos, c, e, k) {
			return true
		}
		return k(pos, c)
	}
	if q.frugal {
		return done() || more()
	}
	return more() || done()
}

// Calls and captures.

type call struct {
	name    string
	capture bool   // false for <.name>
	alias   string // <alias=name>
}

// Call is <name>: it matches the rule name and captures it under that name.
func Call(name string) Node { return &call{name: name, capture: true} }

// Silent is <.name>: it matches the rule name without capturing it.
func Silent(name string) Node { return &call{name: name} }

// Alias is <alias=name>. As in Raku, the match is captured under both names.
func Alias(alias, name string) Node { return &call{name: name, capture: true, alias: alias} }

// AliasOnly is <alias=.name>, captured only as alias.
func AliasOnly(alias, name string) Node { return &call{name: name, alias: alias} }

func (n *call) match(st *state, pos int, c *capList, e env, k cont) bool {
	r := st.g.lookup(n.name)
	if r == nil {
		panic("grammar: no rule " + n.name + " in " + st.g.name)
	}
	finish := func(p int, inner *capList) bool {
		m := &mnode{rule: r.name, from: pos, to: p, caps: inner, sc: r.sc, words: r.words}
		cc := c
		if n.capture {
			cc = &capList{n.name, m, cc}
		}
		if n.alias != "" {
			cc = &capList{n.alias, m, cc}
		}
		return k(p, cc)
	}
	ce := env{ratchet: r.ratchet, sc: r.sc}
	t := st.tr
	if t == nil {
		if r.ratchet {
			p, inner, ok := once(r.body, st, pos, nil, ce)
			return ok && finish(p, inner)
		}
		return r.body.match(st, pos, nil, ce, finish)
	}
	// Tracking failures: keep the rule on the stack while its body
	// matches, and off it while the caller goes on.
	f := frame{r.name, pos}
	t.enter(f)
	if r.ratchet {
		p, inner, ok := once(r.body, st, pos, nil, ce)
		t.leave(f)
		return ok && finish(p, inner)
	}
	ok := r.body.match(st, pos, nil, ce, func(p int, inner *capList) bool {
		t.leave(f)
		res := finish(p, inner)
		t.enter(f)
		return res
	})
	t.leave(f)
	return ok
}

// group is ( ... ), or $<name>=( ... ): a capture with a scope of its own.
type group struct {
	inner Node
	sc    *scope
	name  string // "" for a positional capture
}

// Capture is ( n ): a positional capture, numbered from 0 in its scope.
func Capture(n Node) Node {
	n, _ = distinct(n, map[*group]bool{})
	return &group{inner: n, sc: newScope(n)}
}

func (n *group) match(st *state, pos int, c *capList, e env, k cont) bool {
	key := n.name
	if key == "" {
		key = e.sc.posKey[n]
	}
	ie := env{ratchet: e.ratchet, ci: e.ci, sc: n.sc}
	return n.inner.match(st, pos, nil, ie, func(p int, inner *capList) bool {
		m := &mnode{from: pos, to: p, caps: inner, sc: n.sc}
		return k(p, &capList{key, m, c})
	})
}

// named is $<name>=atom, for an atom that isn't a ( ... ) group: it
// captures the text, and captures inside the atom stay in the outer scope.
type named struct {
	name  string
	inner Node
}

// Named is $<name>=n. Named(name, Capture(n)) captures with a scope of its
// own, as $<name>=( ... ) does, and Named(name, Call(rule)) is
// Alias(name, rule), as $<name>=<rule> is <name=rule>. Named(name,
// Words(list)), $<name>=@list, has the canonical value of what it matched
// as its made value, as a rule that's only a list does.
func Named(name string, n Node) Node {
	switch n := n.(type) {
	case *group:
		if n.name == "" {
			return &group{inner: n.inner, sc: n.sc, name: name}
		}
	case *call:
		if n.alias == "" {
			return &call{name: n.name, capture: n.capture, alias: name}
		}
	}
	return &named{name, n}
}

func (n *named) match(st *state, pos int, c *capList, e env, k cont) bool {
	return n.inner.match(st, pos, c, e, func(p int, cc *capList) bool {
		return k(p, &capList{n.name, &mnode{from: pos, to: p, sc: emptyScope, words: wordsBody(n.inner)}, cc})
	})
}

// Lookaround.

type look struct {
	inner  Node
	neg    bool
	behind bool
}

// Before is <?before n>, NotBefore <!before n>, After <?after n> and
// NotAfter <!after n>. They match no text, and keep no captures.
func Before(n Node) Node    { return &look{inner: n} }
func NotBefore(n Node) Node { return &look{inner: n, neg: true} }
func After(n Node) Node     { return &look{inner: n, behind: true} }
func NotAfter(n Node) Node  { return &look{inner: n, neg: true, behind: true} }

func (n *look) match(st *state, pos int, c *capList, e env, k cont) bool {
	var ok bool
	if st.tr != nil {
		st.tr.quiet++ // what's inside a lookaround failing is no news
	}
	if n.behind {
		for from := pos; from >= 0 && !ok; from-- {
			ok = n.inner.match(st, from, c, e, func(p int, _ *capList) bool { return p == pos })
		}
	} else {
		_, _, ok = once(n.inner, st, pos, c, e)
	}
	if st.tr != nil {
		st.tr.quiet--
	}
	if ok == n.neg {
		st.fail(pos, n, e)
		return false
	}
	return k(pos, c)
}

// Adverbs.

type mode struct {
	inner   Node
	ci      *bool
	ratchet *bool
}

// IgnoreCase is :i over n.
func IgnoreCase(n Node) Node { t := true; return &mode{inner: n, ci: &t} }

// Ratchet is :r over n, or :!r when on is false.
func Ratchet(n Node, on bool) Node { return &mode{inner: n, ratchet: &on} }

func (n *mode) match(st *state, pos int, c *capList, e env, k cont) bool {
	if n.ci != nil {
		e.ci = *n.ci
	}
	if n.ratchet != nil {
		e.ratchet = *n.ratchet
	}
	return step(n.inner, st, pos, c, e, k)
}

// commit is a possessive quantifier, such as \d+: — no backtracking into it.
type commit struct{ inner Node }

func (n commit) match(st *state, pos int, c *capList, e env, k cont) bool {
	p, cc, ok := once(n.inner, st, pos, c, e)
	return ok && k(p, cc)
}

// children returns the nodes directly inside n.
func children(n Node) []Node {
	switch n := n.(type) {
	case seq:
		return n
	case alt:
		return n
	case first:
		return n
	case *Quant:
		if n.sep != nil {
			return []Node{n.item, n.sep}
		}
		return []Node{n.item}
	case *group:
		return []Node{n.inner}
	case *named:
		return []Node{n.inner}
	case *look:
		return []Node{n.inner}
	case *mode:
		return []Node{n.inner}
	case commit:
		return []Node{n.inner}
	}
	return nil // leaves, including words
}

// distinct returns n with every capture group in it appearing once: a group
// used again is copied, with what's inside it. A scope numbers its groups by
// node, so one Capture node used twice, as a Go variable can be, would
// otherwise be numbered once and capture once. Nodes are never changed: a
// node with a copied group inside is copied too.
func distinct(n Node, seen map[*group]bool) (Node, bool) {
	kids := children(n)
	changed := false
	copied := make([]Node, len(kids))
	for i, kid := range kids {
		var c bool
		copied[i], c = distinct(kid, seen)
		changed = changed || c
	}
	if g, ok := n.(*group); ok {
		if !changed && !seen[g] {
			seen[g] = true
			return g, false
		}
		ng := &group{inner: copied[0], sc: newScope(copied[0]), name: g.name}
		seen[ng] = true
		return ng, true
	}
	if !changed {
		return n, false
	}
	switch n := n.(type) {
	case seq:
		return seq(copied), true
	case alt:
		return alt(copied), true
	case first:
		return first(copied), true
	case *Quant:
		q := *n
		q.item = copied[0]
		if n.sep != nil {
			q.sep = copied[1]
		}
		return &q, true
	case *named:
		return &named{n.name, copied[0]}, true
	case *look:
		l := *n
		l.inner = copied[0]
		return &l, true
	case *mode:
		m := *n
		m.inner = copied[0]
		return &m, true
	case commit:
		return commit{copied[0]}, true
	}
	return n, false
}

// methodName is the Go method an action for rule is: amount is Amount, and
// pay-range and pay_range are PayRange.
func methodName(rule string) string {
	var b strings.Builder
	for part := range strings.FieldsFuncSeq(rule, func(r rune) bool { return r == '-' || r == '_' }) {
		rs := []rune(part)
		rs[0] = unicode.ToUpper(rs[0])
		b.WriteString(string(rs))
	}
	return b.String()
}
