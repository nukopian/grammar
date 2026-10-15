// Package grammar brings Raku-style grammars to Go: named rules that call
// each other, parse text into a tree of matches, and hand each match to an
// action that turns it into a value. Matches come back as scalar values, so
// At, Or and To work on them as on any other.
//
// Write a grammar in Raku syntax and Compile it, or build one in Go with
// New, Token, Regex and this package's node functions. Both give the same
// Grammar.
//
// A match is a hash of its captures, by name, with its own text under the
// key "~", as Raku's ~$/ is the text of a match. So m.At("unit") is the
// <unit> match, and m.At("unit.~") the text it matched.
//
// Nothing here is safe for concurrent use while a grammar is being built;
// a finished grammar may be used to parse from several goroutines.
package grammar

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/nukopian/job-meta/internal/scalar"
)

// Grammar is a set of named rules, and the grammar it extends, if any.
// Rules are looked up by name when they're called, starting from the
// grammar being parsed with, so a rule in an extension overrides the rule of
// the same name wherever its parent calls it.
type Grammar struct {
	name     string
	parent   *Grammar
	rules    map[string]*rule
	role     bool
	fromRole map[string]string // rule name → the role it was composed from
	checked  atomic.Uint64     // the generation at which Check last passed
}

// generation counts changes to any grammar's rules. A grammar's Check
// result stands until it changes, as a grammar's rules include its parents'
// and its roles'.
var generation atomic.Uint64

type rule struct {
	name    string
	body    Node
	ratchet bool
	sc      *scope
	words   *WordList // the list the body is, if it's nothing else
}

// New returns an empty grammar called name. Raku's built-in rules, such as
// <ws>, <alpha> and <digit>, are already there.
func New(name string) *Grammar {
	return &Grammar{name: name, parent: builtins, rules: map[string]*rule{}, fromRole: map[string]string{}}
}

// Extend returns an empty grammar called name that inherits g's rules, as
// grammar name is g { ... } does.
func (g *Grammar) Extend(name string) *Grammar {
	return &Grammar{name: name, parent: g, rules: map[string]*rule{}, fromRole: map[string]string{}}
}

// NewRole returns an empty role called name: a library of rules to compose
// into grammars with Does, as Raku's role is. A role's rules may call rules
// it doesn't have, for the grammars it's composed into to provide.
func NewRole(name string) *Grammar {
	g := New(name)
	g.role = true
	return g
}

// IsRole reports whether g is a role.
func (g *Grammar) IsRole() bool { return g.role }

// Does composes roles into g, as grammar g does role { ... } does: their
// rules become g's own, so they can call g's rules and g's extensions can
// override them. A rule g defines itself, before or after, wins over a
// role's. Two roles with a rule of the same name panic, unless g defines
// that rule itself, as Raku requires.
func (g *Grammar) Does(roles ...*Grammar) *Grammar {
	for _, role := range roles {
		if !role.role {
			panic(fmt.Sprintf("grammar %s: %s is a grammar, not a role; use Extend", g.name, role.name))
		}
		for _, name := range slices.Sorted(maps.Keys(role.rules)) {
			from, composed := g.fromRole[name]
			if _, own := g.rules[name]; own && !composed {
				continue
			}
			if composed && from != role.name {
				panic(fmt.Sprintf("grammar %s: rule %s is in both %s and %s; define it in %s to choose",
					g.name, name, from, role.name, g.name))
			}
			g.rules[name] = role.rules[name]
			g.fromRole[name] = role.name
		}
	}
	generation.Add(1)
	return g
}

// Name returns the grammar's name.
func (g *Grammar) Name() string { return g.name }

// Token adds a rule that doesn't backtrack, as Raku's token does.
func (g *Grammar) Token(name string, body Node) *Grammar { return g.add(name, body, true) }

// Regex adds a rule that backtracks, as Raku's regex does.
func (g *Grammar) Regex(name string, body Node) *Grammar { return g.add(name, body, false) }

func (g *Grammar) add(name string, body Node, ratchet bool) *Grammar {
	body, _ = distinct(body, map[*group]bool{})
	g.rules[name] = &rule{name: name, body: body, ratchet: ratchet, sc: newScope(body), words: wordsBody(body)}
	delete(g.fromRole, name)
	generation.Add(1)
	return g
}

func (g *Grammar) lookup(name string) *rule {
	for ; g != nil; g = g.parent {
		if r, ok := g.rules[name]; ok {
			return r
		}
	}
	return nil
}

// Check reports the first call to a rule that doesn't exist, and the first
// left-recursive rule: one that can call itself again, directly or through
// other rules, before matching any text, and so would recurse for ever. A
// role may call rules it doesn't have, so Check doesn't check roles.
func (g *Grammar) Check() error {
	if g.role {
		return nil
	}
	names := g.ruleNames()
	for _, name := range names {
		var err error
		walk(g.lookup(name).body, func(n Node) {
			if c, ok := n.(*call); ok && err == nil && g.lookup(c.name) == nil {
				err = fmt.Errorf("grammar %s: rule %s calls <%s>, which doesn't exist", g.name, name, c.name)
			}
		})
		if err != nil {
			return err
		}
	}
	return g.leftRecursion(names)
}

// ruleNames returns the names of every rule g has, its parents' included,
// sorted.
func (g *Grammar) ruleNames() []string {
	var names []string
	for gg := g; gg != nil; gg = gg.parent {
		for name := range gg.rules {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// leftRecursion reports a cycle of rules that call each other without
// matching any text in between. It works out which rules can match empty
// text, then which rules each rule can call at the position it starts, and
// looks for a cycle among those calls.
func (g *Grammar) leftRecursion(names []string) error {
	empty := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, name := range names {
			if !empty[name] && canBeEmpty(g.lookup(name).body, empty) {
				empty[name], changed = true, true
			}
		}
	}
	const (
		unvisited = iota
		visiting
		visited
	)
	state := map[string]int{}
	var path []string
	var visit func(name string) []string
	visit = func(name string) []string {
		switch state[name] {
		case visiting:
			return append(slices.Clone(path[slices.Index(path, name):]), name)
		case visited:
			return nil
		}
		state[name] = visiting
		path = append(path, name)
		for _, next := range leftCalls(g.lookup(name).body, empty, nil) {
			if cycle := visit(next); cycle != nil {
				return cycle
			}
		}
		path = path[:len(path)-1]
		state[name] = visited
		return nil
	}
	for _, name := range names {
		if cycle := visit(name); cycle != nil {
			return fmt.Errorf("grammar %s: rule %s is left-recursive: %s calls itself again before matching any text",
				g.name, cycle[0], strings.Join(cycle, " → "))
		}
	}
	return nil
}

// canBeEmpty reports whether n can match empty text, given which rules can.
// For a regex, it asks whether the regex matches an empty string.
func canBeEmpty(n Node, empty map[string]bool) bool {
	switch n := n.(type) {
	case lit:
		return len(n.rs) == 0
	case re:
		ok, _ := n.rx.MatchString("")
		return ok
	case anchor, *look:
		return true
	case seq:
		return !slices.ContainsFunc(n, func(c Node) bool { return !canBeEmpty(c, empty) })
	case alt, first:
		return slices.ContainsFunc(children(n), func(c Node) bool { return canBeEmpty(c, empty) })
	case *Quant:
		return n.min == 0 || canBeEmpty(n.item, empty) && (n.sep == nil || n.min == 1 || canBeEmpty(n.sep, empty))
	case *call:
		return empty[n.name]
	case *group, *named, *mode, commit:
		return canBeEmpty(children(n)[0], empty)
	}
	return false // class, words
}

// leftCalls appends the rules n can call at the position it starts.
func leftCalls(n Node, empty map[string]bool, out []string) []string {
	switch n := n.(type) {
	case *call:
		return append(out, n.name)
	case seq:
		for _, c := range n {
			out = leftCalls(c, empty, out)
			if !canBeEmpty(c, empty) {
				break
			}
		}
	case *Quant:
		out = leftCalls(n.item, empty, out)
		if n.sep != nil && canBeEmpty(n.item, empty) {
			out = leftCalls(n.sep, empty, out)
		}
	default:
		for _, c := range children(n) {
			out = leftCalls(c, empty, out)
		}
	}
	return out
}

func walk(n Node, f func(Node)) {
	f(n)
	for _, c := range children(n) {
		walk(c, f)
	}
}

// Option changes how Parse, Subparse, Find and FindAll match.
type Option func(*options)

type options struct {
	rule    string
	actions *actions
}

// Rule starts the match at the named rule instead of TOP, as :rule does.
func Rule(name string) Option { return func(o *options) { o.rule = name } }

// Actions has a's methods turn matches into values, as :actions does. When
// a rule matches, the method named after it is called with the match, and
// what it returns is the match's made value: rule amount calls Amount, and
// pay-range and pay_range call PayRange. Methods must be
// func(*scalar.SV) any; rules without a method are left alone. A
// map[string]func(*scalar.SV) any, keyed by rule name, works too, and nil
// means no actions. Methods may have value or pointer receivers, whether a
// is a value or a pointer.
//
// Actions run once the whole match has succeeded, innermost first, so a
// method can read its subrules' made values. For a rule whose body is a word
// list, made already holds the canonical value when its action runs, and
// the action may replace it. Raku runs them as each rule
// matches, even for matches it later backtracks out of; that's only
// visible to actions with side effects.
func Actions(a any) Option { return func(o *options) { o.actions = newActions(a) } }

// Parse matches the whole of text, as Raku's .parse does, and returns the
// match, or undef if text doesn't match.
func (g *Grammar) Parse(text string, opts ...Option) *scalar.SV {
	o, in := g.prepare(opts), []rune(text)
	return o.result(in, g.run(in, 0, true, o))
}

// Subparse matches from the start of text, without needing to reach the
// end, as Raku's .subparse does.
func (g *Grammar) Subparse(text string, opts ...Option) *scalar.SV {
	o, in := g.prepare(opts), []rune(text)
	return o.result(in, g.run(in, 0, false, o))
}

// Find returns the first match anywhere in text, or undef.
func (g *Grammar) Find(text string, opts ...Option) *scalar.SV {
	o, in := g.prepare(opts), []rune(text)
	for from := 0; from <= len(in); from++ {
		if m := g.run(in, from, false, o); m != nil {
			return o.result(in, m)
		}
	}
	return &scalar.SV{}
}

// FindAll returns every match in text, left to right and not overlapping.
func (g *Grammar) FindAll(text string, opts ...Option) *scalar.AV {
	o, in := g.prepare(opts), []rune(text)
	all := scalar.Array()
	for from := 0; from <= len(in); {
		m := g.run(in, from, false, o)
		if m == nil {
			from++
			continue
		}
		all.Push(o.result(in, m))
		from = max(m.to, from+1)
	}
	return all
}

// prepare applies opts, and panics if Check finds a problem, as a bad regex
// panics in scalar. Check's result is kept until a grammar changes.
func (g *Grammar) prepare(opts []Option) options {
	o := options{rule: "TOP"}
	for _, opt := range opts {
		opt(&o)
	}
	if gen := generation.Load(); g.checked.Load() != gen {
		if err := g.Check(); err != nil {
			panic(err.Error())
		}
		g.checked.Store(gen)
	}
	if g.lookup(o.rule) == nil {
		panic(fmt.Sprintf("grammar %s: no rule %s", g.name, o.rule))
	}
	return o
}

func (g *Grammar) run(in []rune, from int, whole bool, o options) *mnode {
	st := &state{in: in, g: g}
	var m *mnode
	(&call{name: o.rule, capture: true}).match(st, from, nil, env{}, func(p int, c *capList) bool {
		if whole && p != len(in) {
			return false
		}
		m = c.m
		return true
	})
	return m
}

func (o options) result(in []rune, m *mnode) *scalar.SV {
	if m == nil {
		return &scalar.SV{}
	}
	b := &builder{in: in, act: o.actions, built: map[*mnode]*scalar.SV{}}
	return b.build(m)
}

// The shape of a match.
//
// A match is a hash. Its text is under "~" (Raku's ~$/; read a capture's
// text as m.At("name.~"), as m.At("name") is its match), and its position
// under "from" and "to", counted in characters (runes). Named captures are
// under their names, positional captures under "0", "1" and so on, and an
// action's value under "made". A capture that may match more than once, by
// being quantified or by appearing twice, is always an array, empty if it
// didn't match; any other capture is a match, or missing if it didn't
// match. That's how Raku decides too.

// reserved are the keys a match uses for itself.
var reserved = []string{"~", "from", "to", "made"}

// builder turns a finished match into scalar data. A match captured under
// two names, as <min=money> is, is built once and shared by both, as Raku's
// $<min> and $<money> are the same match; so its action runs once.
type builder struct {
	in    []rune
	act   *actions
	built map[*mnode]*scalar.SV
}

func (b *builder) build(m *mnode) *scalar.SV {
	if sv, ok := b.built[m]; ok {
		return sv
	}
	in, act := b.in, b.act
	h := scalar.Hash("~", string(in[m.from:m.to]), "from", m.from, "to", m.to)
	for _, k := range m.sc.manyKeys {
		h.Set(k, scalar.Array())
	}
	var caps []*capList
	for c := m.caps; c != nil; c = c.next {
		caps = append(caps, c)
	}
	for _, c := range slices.Backward(caps) {
		v := b.build(c.m)
		if m.sc.many[c.key] {
			h.Get(c.key).AV().Push(v)
		} else {
			h.Set(c.key, v)
		}
	}
	sv := scalar.Scalar(h)
	if m.words != nil {
		if values := m.words.Canonical(string(in[m.from:m.to])); len(values) > 0 {
			h.Set("made", values[0])
		}
	}
	if act != nil && m.rule != "" {
		if made, ok := act.call(m.rule, sv); ok {
			h.Set("made", made)
		}
	}
	b.built[m] = sv
	return sv
}

// Capture scopes.

// scope is what a rule body or a ( ... ) group can capture: which keys may
// capture more than once, and the number of each positional group in it.
// Raku works these out from the regex, not from what matched, and so does
// newScope.
type scope struct {
	many     map[string]bool
	manyKeys []string
	posKey   map[*group]string
}

var emptyScope = &scope{}

func newScope(body Node) *scope {
	sc := &scope{many: map[string]bool{}, posKey: map[*group]string{}}
	counts, _ := sc.count(body, 0)
	for k, n := range counts {
		if slices.Contains(reserved, k) {
			panic(fmt.Sprintf("grammar: %q is reserved for the match itself, so it can't name a capture", k))
		}
		if n > 1 {
			sc.many[k] = true
			sc.manyKeys = append(sc.manyKeys, k)
		}
	}
	slices.Sort(sc.manyKeys)
	return sc
}

// count returns how many times each key can be captured by n, with 2
// meaning more than once, and the next positional number after n.
func (sc *scope) count(n Node, next int) (map[string]int, int) {
	out := map[string]int{}
	add := func(m map[string]int) {
		for k, v := range m {
			out[k] = min(out[k]+v, 2)
		}
	}
	switch n := n.(type) {
	case seq:
		for _, c := range n {
			var m map[string]int
			m, next = sc.count(c, next)
			add(m)
		}
	case alt, first:
		// Each branch numbers its positional captures from the same place,
		// and a key captures as often as its busiest branch.
		end := next
		for _, b := range children(n) {
			m, e := sc.count(b, next)
			for k, v := range m {
				out[k] = max(out[k], v)
			}
			end = max(end, e)
		}
		next = end
	case *Quant:
		for _, c := range children(n) {
			var m map[string]int
			m, next = sc.count(c, next)
			add(m)
		}
		if n.max != 1 {
			for k := range out {
				out[k] = 2
			}
		}
	case *group:
		key := n.name
		if key == "" {
			key = strconv.Itoa(next)
			next++
			sc.posKey[n] = key
		}
		out[key] = 1
	case *named:
		var m map[string]int
		m, next = sc.count(n.inner, next)
		add(m)
		add(map[string]int{n.name: 1})
	case *call:
		if n.capture {
			out[n.name] = 1
		}
		if n.alias != "" {
			add(map[string]int{n.alias: 1})
		}
	case *look:
		sc.count(n.inner, next) // number its groups, but keep none of its captures
	case *mode, commit:
		return sc.count(children(n)[0], next)
	}
	return out, next
}

// Actions.

// actions are a set of action functions, keyed by rule name (from a map) or
// by method name (from a value's methods), looked up once.
type actions struct {
	funcs    map[string]func(*scalar.SV) any
	byMethod bool
	wrong    map[string]reflect.Type // methods of the wrong type, by name
}

func newActions(a any) *actions {
	switch a := a.(type) {
	case nil:
		return nil
	case map[string]func(*scalar.SV) any:
		return &actions{funcs: a}
	}
	v := reflect.ValueOf(a)
	if v.Kind() != reflect.Pointer {
		// A pointer has the methods of both receivers, so a value with
		// methods on *T works as well as a *T.
		p := reflect.New(v.Type())
		p.Elem().Set(v)
		v = p
	}
	acts := &actions{funcs: map[string]func(*scalar.SV) any{}, byMethod: true, wrong: map[string]reflect.Type{}}
	for i := range v.NumMethod() {
		name, meth := v.Type().Method(i).Name, v.Method(i)
		if f, ok := meth.Interface().(func(*scalar.SV) any); ok {
			acts.funcs[name] = f
		} else {
			acts.wrong[name] = meth.Type()
		}
	}
	return acts
}

func (a *actions) call(rule string, m *scalar.SV) (any, bool) {
	key := rule
	if a.byMethod {
		key = methodName(rule)
		if t, ok := a.wrong[key]; ok {
			panic(fmt.Sprintf("grammar: action %s is %s, not func(*scalar.SV) any", key, t))
		}
	}
	f, ok := a.funcs[key]
	if !ok {
		return nil, false
	}
	return f(m), true
}

// Raku's built-in rules.
var builtins = &Grammar{name: "Grammar", rules: map[string]*rule{}}

func init() {
	tokens := map[string]Node{
		"ws":    Seq(aNotWithinWord, Star(Space)), // token ws { <!ww> \s* }
		"alpha": Class(isAlpha),
		"digit": Digit,
		"alnum": Class(func(r rune) bool { return isAlpha(r) || isDigit(r) }),
		"upper": Class(unicode.IsUpper),
		"lower": Class(unicode.IsLower),
		"space": Space,
		"punct": Class(unicode.IsPunct),
	}
	for name, body := range tokens {
		builtins.Token(name, body)
	}
}
