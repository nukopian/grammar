package grammar

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Compile compiles grammars and roles written in Raku syntax, and returns
// the last one declared:
//
//	role Money {
//	    token money  { '£' \s? <amount> }
//	    ...
//	}
//	grammar Pay does Money {
//	    token TOP    { <min=money> [ <.sep> <max=money> ]? <period>? }
//	    token period { \s* <.per> \s* <unit> }
//	    token unit   { @units }
//	    ...
//	}
//
// The source can refer by name to what's declared earlier in it, and to
// uses: grammars to extend (is), roles to compose (does), and word lists to
// match (@units). The syntax is a subset of Raku's; the package README
// lists what's in it.
func Compile(src string, uses ...Use) (g *Grammar, err error) {
	p := &parser{src: []rune(src), grammars: map[string]*Grammar{}, lists: map[string]*WordList{}}
	for _, u := range uses {
		switch u := u.(type) {
		case *Grammar:
			p.grammars[u.name] = u
		case *WordList:
			p.lists[u.name] = u
		}
	}
	defer func() {
		if r := recover(); r != nil {
			pe, ok := r.(parseError)
			if !ok {
				panic(r)
			}
			g, err = nil, pe
		}
	}()
	for {
		g = p.declaration()
		if err := g.Check(); err != nil {
			return nil, err
		}
		p.grammars[g.name] = g
		p.ws()
		if p.eof() {
			return g, nil
		}
	}
}

// Use is what a grammar's source can refer to by name: a *Grammar, to
// extend or compose, or a *WordList, to match.
type Use interface{ useName() string }

func (g *Grammar) useName() string  { return g.name }
func (l *WordList) useName() string { return l.name }

// MustCompile is Compile, panicking if src doesn't compile. Use it for
// grammars in the program, in package-level variables, so that a bad one
// fails as soon as the program starts.
func MustCompile(src string, uses ...Use) *Grammar {
	g, err := Compile(src, uses...)
	if err != nil {
		panic(err.Error())
	}
	return g
}

type parseError struct {
	line, col int
	msg       string
}

func (e parseError) Error() string {
	return fmt.Sprintf("grammar: line %d, column %d: %s", e.line, e.col, e.msg)
}

type parser struct {
	src      []rune
	i        int
	assert   int // depth of <?before ...> and friends, where > ends the regex
	grammars map[string]*Grammar
	lists    map[string]*WordList

	// adv holds, for each group being parsed, innermost last, the adverbs
	// that cover the rest of it: an adverb covers the rest of its group,
	// later alternatives included, as in Raku. sigs does the same for
	// :sigspace, which changes how the source is read rather than how the
	// text is matched.
	adv  []*mode
	sigs []bool

	// sig is whether whitespace in the source is significant here, as it
	// is in a rule, or after :s; see sequence.
	sig bool
}

func (p *parser) fail(format string, args ...any) {
	line, col := 1, 1
	for _, r := range p.src[:min(p.i, len(p.src))] {
		if r == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	panic(parseError{line, col, fmt.Sprintf(format, args...)})
}

func (p *parser) eof() bool { return p.i >= len(p.src) }

func (p *parser) peek(s string) bool {
	rs := []rune(s)
	return p.i+len(rs) <= len(p.src) && slices.Equal(p.src[p.i:p.i+len(rs)], rs)
}

func (p *parser) eat(s string) bool {
	if p.peek(s) {
		p.i += len([]rune(s))
		return true
	}
	return false
}

func (p *parser) expect(s string) {
	p.ws()
	if !p.eat(s) {
		p.fail("expected %s", s)
	}
}

// space skips whitespace and # comments, as ws does, and reports whether
// there were any, for :sigspace.
func (p *parser) space() bool {
	start := p.i
	p.ws()
	return p.i > start
}

// ws skips whitespace and # comments, which mean nothing in a Raku regex,
// except under :sigspace.
func (p *parser) ws() {
	for !p.eof() {
		switch r := p.src[p.i]; {
		case unicode.IsSpace(r):
			p.i++
		case r == '#':
			for !p.eof() && p.src[p.i] != '\n' {
				p.i++
			}
		default:
			return
		}
	}
}

func (p *parser) ident() string {
	p.ws()
	start := p.i
	for !p.eof() {
		r := p.src[p.i]
		switch {
		case isAlpha(r), p.i > start && isDigit(r):
		case (r == '-' || r == '\'') && p.i > start && p.i+1 < len(p.src) && isAlpha(p.src[p.i+1]):
		case r == ':' && p.peek("::") && p.i > start:
			p.i++
		default:
			if p.i == start {
				p.fail("expected a name")
			}
			return string(p.src[start:p.i])
		}
		p.i++
	}
	if p.i == start {
		p.fail("expected a name")
	}
	return string(p.src[start:p.i])
}

// keyword reports whether the next word is kw, and skips it if so.
func (p *parser) keyword(kw string) bool {
	p.ws()
	if p.peek(kw) {
		end := p.i + len([]rune(kw))
		if end == len(p.src) || !isWord(p.src[end]) && p.src[end] != '-' {
			p.i = end
			return true
		}
	}
	return false
}

// declaration reads a grammar or a role.
func (p *parser) declaration() *Grammar {
	var role bool
	switch {
	case p.keyword("grammar"):
	case p.keyword("role"):
		role = true
	default:
		p.fail("expected grammar or role")
	}
	name := p.ident()
	g := New(name)
	g.role = role
	var roles []*Grammar
	for {
		switch {
		case !role && p.keyword("is"):
			if g.parent != builtins {
				p.fail("grammar %s can only extend one grammar", name)
			}
			parent := p.named()
			if parent.role {
				p.fail("%s is a role: use does", parent.name)
			}
			g.parent = parent
			continue
		case p.keyword("does"):
			r := p.named()
			if !r.role {
				p.fail("%s is a grammar, not a role: use is", r.name)
			}
			roles = append(roles, r)
			continue
		}
		break
	}
	p.rules(g)
	defer func() {
		if r := recover(); r != nil {
			if msg, ok := r.(string); ok {
				p.fail("%s", strings.TrimPrefix(msg, "grammar "+name+": "))
			}
			panic(r)
		}
	}()
	g.Does(roles...)
	return g
}

// named reads the name of a grammar or role, declared earlier or passed to
// Compile.
func (p *parser) named() *Grammar {
	name := p.ident()
	g, ok := p.grammars[name]
	if !ok {
		p.fail("no grammar or role %s: declare it earlier in the source, or pass it to Compile", name)
	}
	return g
}

// rules reads the rules of a grammar or role, between its braces.
func (p *parser) rules(g *Grammar) {
	p.expect("{")
	for {
		p.ws()
		if p.eat("}") {
			break
		}
		var ratchet, sig bool
		switch {
		case p.keyword("token"):
			ratchet = true
		case p.keyword("regex"):
		case p.keyword("rule"):
			ratchet, sig = true, true // a rule is a token with :sigspace
		default:
			p.fail("expected token, regex or rule")
		}
		rname := p.ident()
		if _, dup := g.rules[rname]; dup {
			p.fail("%s is declared twice", rname)
		}
		p.expect("{")
		p.sig = sig
		body := p.regex()
		p.sig = false
		p.expect("}")
		p.add(g, rname, body, ratchet)
		p.ws()
		p.eat(";")
	}
	p.ws()
	p.eat(";")
}

// add adds a rule to g, turning a mistake that add panics on, such as a
// capture with a reserved name, into a syntax error.
func (p *parser) add(g *Grammar, name string, body Node, ratchet bool) {
	defer func() {
		if r := recover(); r != nil {
			if msg, ok := r.(string); ok {
				p.fail("in %s: %s", name, strings.TrimPrefix(msg, "grammar: "))
			}
			panic(r)
		}
	}()
	g.add(name, body, ratchet)
}

// regex is a whole regex, or the inside of a group: alternatives, each a
// sequence.
func (p *parser) regex() Node {
	p.adv = append(p.adv, nil)
	p.sigs = append(p.sigs, p.sig)
	outer := p.sig
	defer func() {
		p.adv = p.adv[:len(p.adv)-1]
		p.sigs = p.sigs[:len(p.sigs)-1]
		p.sig = outer
	}()
	p.ws()
	if !p.eat("||") {
		p.eat("|")
	}
	branches := []Node{p.alts()}
	for p.ws(); p.eat("||"); p.ws() {
		branches = append(branches, p.alts())
	}
	if len(branches) == 1 {
		return branches[0]
	}
	return first(branches)
}

func (p *parser) alts() Node {
	branches := []Node{p.branch()}
	for p.ws(); p.peek("|") && !p.peek("||"); p.ws() {
		p.i++
		branches = append(branches, p.branch())
	}
	if len(branches) == 1 {
		return branches[0]
	}
	return alt(branches)
}

// branch is one alternative of a group. Adverbs from earlier in the group
// cover it.
func (p *parser) branch() Node {
	adv := p.adv[len(p.adv)-1]
	p.sig = p.sigs[len(p.sigs)-1]
	n := p.sequence()
	if adv != nil {
		n = &mode{inner: n, ci: adv.ci, ratchet: adv.ratchet}
	}
	return n
}

// sequence reads terms one after another. Under :sigspace, whitespace
// after a term matches whitespace in the text: it becomes <.ws>, as in
// Raku. Whitespace at the start of a sequence (of a rule, a group or an
// alternative) never does.
func (p *parser) sequence() Node {
	var items []Node
	p.ws()
	for {
		if p.eof() || p.peek("|") || p.peek("]") || p.peek(")") || p.peek("}") ||
			p.assert > 0 && p.peek(">") {
			break
		}
		if p.peek(":") {
			m, sig := p.adverbs()
			if sig != nil {
				p.sig = *sig
				p.sigs[len(p.sigs)-1] = *sig
			}
			p.ws()
			// They cover the rest of this sequence, and the rest of the group.
			top := &p.adv[len(p.adv)-1]
			merged := &mode{}
			if *top != nil {
				*merged = **top
			}
			if m.ci != nil {
				merged.ci = m.ci
			}
			if m.ratchet != nil {
				merged.ratchet = m.ratchet
			}
			*top = merged
			m.inner = p.sequence()
			if m.ci == nil && m.ratchet == nil {
				items = append(items, m.inner) // only :s, which is done with
			} else {
				items = append(items, m)
			}
			break
		}
		sig := p.sig // as it is where the whitespace after the term is
		term, spaced := p.quantified()
		items = append(items, term)
		if spaced && sig {
			items = append(items, Silent("ws"))
		}
	}
	return Seq(items...)
}

// adverbs reads :i, :r, :s and the like: a mode for the caller to fill in,
// and :s or :!s, if given, which the parser applies itself.
func (p *parser) adverbs() (*mode, *bool) {
	m := &mode{}
	var sig *bool
	for p.ws(); p.eat(":"); p.ws() {
		on := !p.eat("!")
		switch name := p.ident(); name {
		case "i", "ignorecase":
			m.ci = &on
		case "r", "ratchet":
			m.ratchet = &on
		case "s", "sigspace":
			sig = &on
		default:
			p.fail("unknown adverb :%s", name)
		}
	}
	return m, sig
}

// quantified reads a term: an atom, perhaps named or quantified. It
// reports whether whitespace follows the term, which matters under
// :sigspace.
func (p *parser) quantified() (Node, bool) {
	var capName string
	if p.eat("$<") {
		capName = p.ident()
		p.expect(">")
		p.expect("=")
	}
	n, spaced := p.quantifier(p.atom())
	if capName != "" {
		n = Named(capName, n)
	}
	return n, spaced
}

// quantifier reads any quantifier after atom, and reports whether
// whitespace follows. Under :sigspace, as in Raku, whitespace between an
// atom and its quantifier is part of what's repeated (a + is [a <.ws>]+),
// and whitespace after a separator is part of the separator.
func (p *parser) quantifier(atom Node) (Node, bool) {
	sig := p.sig
	spaced := p.space()
	if !p.peek("**") && !p.peek("?") && !p.peek("*") && !p.peek("+") {
		return atom, spaced
	}
	if spaced && sig {
		atom = Seq(atom, Silent("ws"))
	}
	var q *Quant
	switch {
	case p.eat("**"):
		p.ws()
		lo := p.number()
		hi := lo
		if p.eat("..") {
			if p.eat("*") {
				hi = -1
			} else {
				hi = p.number()
			}
		}
		if hi >= 0 && hi < lo {
			p.fail("empty range %d..%d can't be a quantifier", lo, hi)
		}
		q = Repeat(atom, lo, hi)
	case p.eat("?"):
		q = Opt(atom)
	case p.eat("*"):
		q = Star(atom)
	case p.eat("+"):
		q = Plus(atom)
	}
	frugal := p.eat("?")
	possessive := !frugal && p.eat(":")
	if !frugal && !possessive {
		p.eat("!") // explicitly greedy, as it is anyway
	}
	spaced = p.space()
	if trailing := p.eat("%%"); trailing || p.eat("%") {
		sep := p.atom()
		if p.space() && sig {
			sep = Seq(sep, Silent("ws"))
		}
		spaced = false // the separator took it
		if trailing {
			q = q.SepTrailing(sep)
		} else {
			q = q.Sep(sep)
		}
	}
	if frugal {
		q = q.Frugal()
	}
	if possessive {
		return commit{q}, spaced
	}
	return q, spaced
}

func (p *parser) number() int {
	start := p.i
	for !p.eof() && p.src[p.i] >= '0' && p.src[p.i] <= '9' {
		p.i++
	}
	if start == p.i {
		p.fail("expected a number")
	}
	n, err := strconv.Atoi(string(p.src[start:p.i]))
	if err != nil {
		p.fail("%s is too large a count", string(p.src[start:p.i]))
	}
	return n
}

func (p *parser) atom() Node {
	p.ws()
	if p.eof() {
		p.fail("unexpected end of grammar")
	}
	switch r := p.src[p.i]; {
	case r == '\'' || r == '"':
		return Lit(p.quoted())
	case p.eat("["):
		n := p.regex()
		p.expect("]")
		return n
	case p.eat("("):
		n := p.regex()
		p.expect(")")
		return Capture(n)
	case p.eat("."):
		return AnyChar
	case p.eat(`\`):
		return Class(p.escape())
	case p.eat("^^"):
		return LineStart
	case p.eat("^"):
		return Start
	case p.eat("$$"):
		return LineEnd
	case p.eat("$"):
		return End
	case p.eat("<<"), p.eat("«"):
		return WordStart
	case p.eat(">>"), p.eat("»"):
		return WordEnd
	case p.eat("<"):
		return p.assertion()
	case p.eat("@"):
		name := p.ident()
		l, ok := p.lists[name]
		if !ok {
			p.fail("no word list @%s: pass it to Compile", name)
		}
		return Words(l)
	case isWord(r):
		p.i++
		return Lit(string(r))
	case r == '{':
		p.fail("code blocks aren't supported: use actions")
	case r == '%':
		p.fail("%% needs a quantifier before it, as in <item>+ %% ','")
	}
	p.fail("%q must be quoted, as '%c', or escaped, as \\%c", p.src[p.i], p.src[p.i], p.src[p.i])
	return nil
}

func (p *parser) quoted() string {
	q := p.src[p.i]
	p.i++
	var b strings.Builder
	for {
		if p.eof() {
			p.fail("unterminated string")
		}
		r := p.src[p.i]
		p.i++
		switch {
		case r == q:
			return b.String()
		case r == '\\' && !p.eof():
			e := p.src[p.i]
			p.i++
			switch {
			case q == '"' && e == 'n':
				b.WriteRune('\n')
			case q == '"' && e == 't':
				b.WriteRune('\t')
			case e == q || e == '\\':
				b.WriteRune(e)
			default:
				b.WriteRune('\\')
				b.WriteRune(e)
			}
		default:
			b.WriteRune(r)
		}
	}
}

// escape reads what follows a backslash, as a class of one or more
// characters. Any other punctuation is itself, as \- and \] are.
func (p *parser) escape() func(rune) bool {
	if p.eof() {
		p.fail("unterminated escape")
	}
	r := p.src[p.i]
	p.i++
	classes := map[rune]func(rune) bool{
		'd': isDigit, 'w': isWord, 's': isSpace, 'h': isHSpace, 'v': isVSpace,
		'n': func(r rune) bool { return r == '\n' }, 't': func(r rune) bool { return r == '\t' },
		'r': func(r rune) bool { return r == '\r' }, 'e': func(r rune) bool { return r == 0x1b },
	}
	if f, ok := classes[unicode.ToLower(r)]; ok {
		if unicode.IsUpper(r) {
			return not(f)
		}
		return f
	}
	if isWord(r) {
		p.fail(`\%c isn't supported`, r)
	}
	return func(c rune) bool { return c == r }
}

// assertion reads what follows a <.
func (p *parser) assertion() Node {
	switch {
	case p.peek("[") || p.peek("-") || p.peek("+"):
		f := p.classExpr()
		p.expect(">")
		return Class(f)
	case p.peek("?before") || p.peek("!before") || p.peek("?after") || p.peek("!after"):
		neg := p.eat("!")
		p.eat("?")
		behind := p.keyword("after")
		if !behind {
			p.keyword("before")
		}
		p.assert++
		n := p.regex()
		p.assert--
		p.expect(">")
		return &look{inner: n, neg: neg, behind: behind}
	case p.peek("?") || p.peek("!"):
		neg := p.eat("!")
		p.eat("?")
		name := p.ident()
		p.expect(">")
		return &look{inner: Silent(name), neg: neg}
	}
	if p.peek("@") {
		p.fail("<@name> matches each entry as a regex, which isn't supported: use @name, which matches them as text")
	}
	silent := p.eat(".")
	name := p.ident()
	p.ws()
	var n Node
	switch {
	case !silent && p.eat("="):
		p.ws()
		if p.eat(".") {
			n = AliasOnly(name, p.ident())
		} else {
			n = Alias(name, p.ident())
		}
	case silent:
		n = Silent(name)
	default:
		n = Call(name)
	}
	p.expect(">")
	return n
}

// classExpr reads a character class: [ ... ] sets and names of built-in
// classes, joined with + and -, as in <[a..z] - [aeiou]>, <-[\n]> or
// <+alpha +[-]>. As in Raku, a class that starts with a name starts with a
// sign, so that <alpha> stays a call.
func (p *parser) classExpr() func(rune) bool {
	var f func(rune) bool
	for {
		p.ws()
		sign := '+'
		switch {
		case p.eat("+"):
		case p.eat("-"):
			sign = '-'
		default:
			if f != nil {
				return f
			}
		}
		p.ws()
		var g func(rune) bool
		if p.eat("[") {
			g = p.set()
		} else {
			name := p.ident()
			named := map[string]func(rune) bool{
				"alpha": isAlpha, "digit": isDigit, "space": isSpace, "upper": unicode.IsUpper,
				"lower": unicode.IsLower, "punct": unicode.IsPunct, "alnum": func(r rune) bool { return isAlpha(r) || isDigit(r) },
			}
			if g = named[name]; g == nil {
				p.fail("<%s> can't be used in a character class", name)
			}
		}
		switch {
		case f == nil && sign == '-':
			f = not(g)
		case f == nil:
			f = g
		case sign == '-':
			f0 := f
			f = func(r rune) bool { return f0(r) && !g(r) }
		default:
			f0 := f
			f = func(r rune) bool { return f0(r) || g(r) }
		}
	}
}

// set reads the inside of [ ... ]: characters, ranges such as a..z, and
// escapes, with whitespace ignored.
func (p *parser) set() func(rune) bool {
	var parts []func(rune) bool
	for {
		p.ws()
		if p.eof() {
			p.fail("unterminated character class")
		}
		if p.eat("]") {
			break
		}
		var lo rune
		if p.eat(`\`) {
			f := p.escape()
			parts = append(parts, f)
			continue
		}
		lo = p.src[p.i]
		p.i++
		p.ws()
		if p.eat("..") {
			p.ws()
			p.eat(`\`)
			if p.eof() {
				p.fail("unterminated character class")
			}
			hi := p.src[p.i]
			p.i++
			parts = append(parts, func(r rune) bool { return r >= lo && r <= hi })
			continue
		}
		parts = append(parts, func(r rune) bool { return r == lo })
	}
	return func(r rune) bool {
		return slices.ContainsFunc(parts, func(f func(rune) bool) bool { return f(r) })
	}
}
