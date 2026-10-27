package grammar

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/nukopian/scalar"
)

// CheckPath returns an error if path, as scalar's At takes it, can never
// be defined in a match of g, and nil if some text could define it. A
// misspelt capture, as in m.At("per.weak"), reads undef as an optional
// capture that didn't match does, so a typo fails silently; CheckPath says
// which step can't be there, and what could. It's for tests, mostly: a
// check that the paths a program reads are ones its grammar makes.
//
// The path starts at a match of TOP, or of the rule Rule(name) names; the
// other options are ignored. A capture inside $<k>=[ ... ] is beside k,
// not in it, as in Raku; a capture that can match more than once is a
// list, to index; and ~, from and to have nothing inside. Any rule's match
// may have a made value, as an action may make one, and whatever an
// action makes may be anything, so CheckPath takes any steps after made.
// It works from the grammar alone: a path it takes may still be undef in
// a match of a particular text.
//
// A malformed path is scalar.CheckPath's error. Raku has nothing like it.
func (g *Grammar) CheckPath(path string, opts ...Option) error {
	o := options{rule: "TOP"}
	for _, opt := range opts {
		opt(&o)
	}
	if g.role {
		return fmt.Errorf("grammar %s is a role: check the paths of a grammar that does it", g.name)
	}
	if err := g.Check(); err != nil {
		return err
	}
	if g.lookup(o.rule) == nil {
		return fmt.Errorf("grammar %s: no rule %s", g.name, o.rule)
	}
	steps, err := scalar.AtSteps(path)
	if err != nil {
		return err
	}
	pc := &pathCheck{g: g, caps: map[*scope]map[string][]*place{}}
	if err := pc.walk(pc.rule(o.rule), steps, "", nil, ""); err != nil {
		return fmt.Errorf("grammar %s: path %q: %s", g.name, path, err.msg)
	}
	return nil
}

// placeKind is what a place in a match is.
type placeKind int

const (
	aMatch   placeKind = iota // a match, with captures of its own
	aText                     // a match of $<k>=[ ... ]: text, and no captures
	aList                     // a capture that can match more than once
	aLeaf                     // ~, from or to
	anything                  // made, or anything in it
)

// place is somewhere a path may lead in a match: a match, one of its
// captures, or what it holds.
type place struct {
	kind  placeKind
	what  string   // for an error: "rule per", or "( … )"
	sc    *scope   // a match's: what it can capture more than once
	body  Node     // a match's: what makes its captures
	made  bool     // a match's: whether it can have a made value
	elems []*place // a list's: the matches in it
}

// pathCheck walks a path through a grammar's places, keeping each body's
// captures as it finds them, by its scope, as each rule and group has its
// own.
type pathCheck struct {
	g    *Grammar
	caps map[*scope]map[string][]*place
}

// pathError is a step that can't be there: msg says why.
type pathError struct{ msg string }

// rule returns the places a match of rule name may be: the rule's, or for
// a proto, each variant's.
func (pc *pathCheck) rule(name string) []*place {
	r := pc.g.lookup(name)
	if r == nil {
		return nil // a role's rule, which Check doesn't check
	}
	rules := []*rule{r}
	if r.proto {
		rules = r.variants
	}
	what := "rule " + name
	if r.proto {
		what = "proto " + name
	}
	var out []*place
	for _, r := range rules {
		out = append(out, &place{kind: aMatch, what: what, sc: r.sc, body: r.body, made: true})
	}
	return out
}

// captures returns what p's captures may be, by key.
func (pc *pathCheck) captures(p *place) map[string][]*place {
	if p.body == nil {
		return nil
	}
	if c, ok := pc.caps[p.sc]; ok {
		return c
	}
	c := map[string][]*place{}
	pc.collect(p.body, p.sc, c)
	pc.caps[p.sc] = c
	return c
}

// collect adds what n captures to out, as the builder will: the same keys
// as scope.count, each with the places it may lead to.
func (pc *pathCheck) collect(n Node, sc *scope, out map[string][]*place) {
	switch n := n.(type) {
	case *group:
		key := n.name
		what := "a ( … ) capture"
		if key == "" {
			key = sc.posKey[n]
		}
		out[key] = append(out[key], &place{kind: aMatch, what: what, sc: n.sc, body: n.inner})
	case *named:
		pc.collect(n.inner, sc, out) // a capture inside stays in this scope
		out[n.name] = append(out[n.name], &place{kind: aText, what: "$<" + n.name + ">=[ … ]", made: wordsBody(n.inner) != nil})
	case *call:
		if n.capture {
			out[n.name] = append(out[n.name], pc.rule(n.name)...)
		}
		if n.alias != "" {
			out[n.alias] = append(out[n.alias], pc.rule(n.name)...)
		}
	case *look, patterns:
		// They keep none of their captures.
	default:
		for _, c := range children(n) {
			pc.collect(c, sc, out)
		}
	}
}

// walk follows steps from the places here, the path so far being at, and
// returns why the first that can't be there can't. The places before here,
// up, at upAt, are where a capture inside $<k>=[ ... ] really is.
func (pc *pathCheck) walk(here []*place, steps []scalar.AtStep, at string, up []*place, upAt string) *pathError {
	if len(steps) == 0 {
		return nil
	}
	st := steps[0]
	next, err := pc.step(here, st, at)
	if len(next) > 0 {
		return pc.walk(next, steps[1:], stepPath(at, st), here, at)
	}
	if p := here[0]; p.kind == aText && st.Kind != scalar.StepIndex && !slices.Contains(reserved, st.Key) &&
		up != nil && pc.walk(up, steps, upAt, nil, "") == nil {
		read := upAt
		for _, st := range steps {
			read = stepPath(read, st)
		}
		err = &pathError{fmt.Sprintf("%s is text only: a capture inside %s is beside it, so read %q", at, p.what, read)}
	}
	return err
}

// step returns the places st leads to from here, or, if there are none,
// why not, from the first place here.
func (pc *pathCheck) step(here []*place, st scalar.AtStep, at string) ([]*place, *pathError) {
	var next []*place
	seen := map[*place]bool{}
	add := func(ps ...*place) {
		for _, p := range ps {
			if !seen[p] {
				seen[p] = true
				next = append(next, p)
			}
		}
	}
	for _, p := range here {
		switch p.kind {
		case anything:
			add(p)
		case aList:
			if st.Kind != scalar.StepKey {
				add(p.elems...)
			}
		case aMatch, aText:
			if st.Kind == scalar.StepIndex {
				continue
			}
			switch st.Key {
			case "~", "from", "to":
				add(&place{kind: aLeaf, what: st.Key})
			case "made":
				if p.made {
					add(&place{kind: anything})
				}
			default:
				ps := pc.captures(p)[st.Key]
				if len(ps) > 0 && p.sc.many[st.Key] {
					add(&place{kind: aList, elems: ps})
				} else {
					add(ps...)
				}
			}
		}
	}
	if len(next) > 0 {
		return next, nil
	}
	return nil, pc.why(here, st, at)
}

// why says why st can't come after the path so far, at, from here.
func (pc *pathCheck) why(here []*place, st scalar.AtStep, at string) *pathError {
	p := here[0]
	name := at
	if name == "" {
		name = "the match"
	}
	switch {
	case p.kind == aLeaf && p.what == "~":
		return &pathError{fmt.Sprintf("%s is the matched text, with nothing inside", name)}
	case p.kind == aLeaf:
		return &pathError{fmt.Sprintf("%s is a position, with nothing inside", name)}
	case p.kind == aList:
		return &pathError{fmt.Sprintf("%s is a list of matches, as it can capture more than once: index it, as %s[0]", name, name)}
	case st.Kind == scalar.StepIndex:
		return &pathError{fmt.Sprintf("%s is a single match, not a list", name)}
	case st.Key == "made":
		return &pathError{fmt.Sprintf("%s never has a made value: only a rule's match, or a word list's, has one", name)}
	case p.kind == aText:
		return &pathError{fmt.Sprintf("%s is text only, with no captures", name)}
	}
	var keys []string
	for _, p := range here {
		keys = append(keys, slices.Collect(maps.Keys(pc.captures(p)))...)
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	what := p.what
	if strings.HasPrefix(what, "a ") {
		what = name + ", " + what + ","
	}
	msg := fmt.Sprintf("%s has no capture %q", what, st.Key)
	if len(keys) > 0 {
		msg += " (it has: " + strings.Join(keys, ", ") + ")"
	}
	return &pathError{msg}
}

// stepPath is the path so far, at, with st after it, for an error.
func stepPath(at string, st scalar.AtStep) string {
	if st.Kind == scalar.StepIndex {
		return at + "[" + strconv.Itoa(st.Index) + "]"
	}
	if at == "" {
		return st.Key
	}
	return at + "." + st.Key
}
