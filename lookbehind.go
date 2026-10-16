package grammar

// <?after …> matches its pattern ending at the current position, by trying
// each start before it. Tried at every start back to the beginning of the
// text, that costs time in proportion to how far into the text it is, and
// FindAll, which tries the grammar at every position, then takes time in
// proportion to the square of the text's length. So a lookbehind narrows
// the starts it tries: only as far back as its pattern can reach, when its
// width is bounded, as for <?after '£'> or <?after \d ** 1..3>, and only
// where its pattern can start, from the characters it can start with, as
// for <?after <[.!?:]> \s+>. What it matches doesn't change.

// boundLookbehind turns the narrowing off, so tests can check that it
// changes no results.
var boundLookbehind = true

// lookKey is a lookbehind as one grammar sees it, with :i or not.
type lookKey struct {
	n  *look
	ci bool
}

// lookInfo is what narrows a lookbehind's starts: the least and most text
// its pattern can match (hi < 0: no limit), and the characters it can
// start with.
type lookInfo struct {
	gen    uint64
	lo, hi int
	s      *starts
	empty  bool
}

// lookbehind returns n's lookInfo in g, worked out once until a grammar
// changes, as the rules its pattern calls may.
func (g *Grammar) lookbehind(n *look, ci bool) *lookInfo {
	gen := generation.Load()
	key := lookKey{n, ci}
	if v, ok := g.afters.Load(key); ok && v.(*lookInfo).gen == gen {
		return v.(*lookInfo)
	}
	info := &lookInfo{gen: gen, s: &starts{runes: map[rune]bool{}}}
	info.lo, info.hi = g.width(n.inner, map[string]bool{})
	info.empty = (&startFinder{g: g, rules: map[string]*ruleStarts{}}).add(n.inner, ci, info.s)
	g.afters.Store(key, info)
	return info
}

// width returns the least and most text n can match, in characters; hi is
// -1 where there's no limit, or no telling, as for a regex terminal or a
// rule that calls itself.
func (g *Grammar) width(n Node, calling map[string]bool) (lo, hi int) {
	switch n := n.(type) {
	case lit:
		return len(n.rs), len(n.rs)
	case class:
		return 1, 1
	case anchor, *look:
		return 0, 0
	case seq:
		for _, c := range n {
			l, h := g.width(c, calling)
			lo += l
			if hi >= 0 {
				hi = sumWidth(hi, h)
			}
		}
		return lo, hi
	case *goal:
		return g.width(n.seq(), calling)
	case alt, first, patterns:
		var kids []Node
		if p, ok := n.(patterns); ok {
			kids = p.l.nodes
		} else {
			kids = children(n)
		}
		if len(kids) == 0 {
			return 0, 0
		}
		lo, hi = -1, 0
		for _, c := range kids {
			l, h := g.width(c, calling)
			if lo < 0 || l < lo {
				lo = l
			}
			if h < 0 || hi < 0 {
				hi = -1
			} else {
				hi = max(hi, h)
			}
		}
		return lo, hi
	case conj:
		// Every branch matches the same text: at least the most any must,
		// and at most the least any can.
		hi = -1
		for _, c := range n {
			l, h := g.width(c, calling)
			lo = max(lo, l)
			if h >= 0 && (hi < 0 || h < hi) {
				hi = h
			}
		}
		return lo, hi
	case *Quant:
		il, ih := g.width(n.item, calling)
		sl, sh := 0, 0
		if n.sep != nil {
			sl, sh = g.width(n.sep, calling)
		}
		lo = n.min*il + max(n.min-1, 0)*sl
		if n.max < 0 || ih < 0 || n.sep != nil && sh < 0 && n.max > 1 {
			return lo, -1
		}
		seps := max(n.max-1, 0)
		if n.trailing {
			seps = n.max
		}
		return lo, n.max*ih + seps*sh
	case *call:
		r := g.lookup(n.name)
		if r == nil || calling[n.name] {
			return 0, -1
		}
		calling[n.name] = true
		defer delete(calling, n.name)
		return g.width(r.body, calling)
	case *group, *named, *mode, commit:
		return g.width(children(n)[0], calling)
	}
	return 0, -1 // a regex, a word list, a backreference: no telling
}

func sumWidth(a, b int) int {
	if a < 0 || b < 0 {
		return -1
	}
	return a + b
}

// lookbehindStarts returns the positions in st's text where info's
// pattern can start, in order, listing them the first time they're asked
// for, so each lookbehind looks at only those, not every position before
// it.
func (st *state) lookbehindStarts(info *lookInfo) []int {
	if starts, ok := st.afterStarts[info]; ok {
		return starts
	}
	var starts []int
	for i, r := range st.in {
		if info.s.has(r) {
			starts = append(starts, i)
		}
	}
	if st.afterStarts == nil {
		st.afterStarts = map[*lookInfo][]int{}
	}
	st.afterStarts[info] = starts
	return starts
}
