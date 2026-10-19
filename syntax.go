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
// match (@units). The syntax is a subset of Raku's; the reference,
// docs/REFERENCE.md, lists what's in it.
func Compile(src string, uses ...Use) (g *Grammar, err error) {
	p := &parser{src: []rune(nfc(src)), grammars: map[string]*Grammar{}, lists: map[string]*WordList{},
		patterns: map[string]*PatternList{}}
	for _, u := range uses {
		switch u := u.(type) {
		case *Grammar:
			p.grammars[u.name] = u
		case *WordList:
			p.lists[u.name] = u
		case *PatternList:
			p.patterns[u.name] = u
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
	patterns map[string]*PatternList

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

	// single is whether the next atom is just one character of a bare
	// word, as after $<x>= and as a separator, which Raku binds to one
	// letter; see atom.
	single bool
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
	var parents, roles []*Grammar
	for {
		switch {
		case !role && p.keyword("is"):
			parent := p.named()
			if parent.role {
				p.fail("%s is a role: use does", parent.name)
			}
			if slices.Contains(parents, parent) {
				p.fail("grammar %s already has parent %s", name, parent.name)
			}
			parents = append(parents, parent)
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
	g := p.newGrammar(name, parents)
	g.role = role
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

// newGrammar makes the grammar a declaration declares, extending parents,
// or Raku's built-in rules if there are none, failing as Extend panics.
func (p *parser) newGrammar(name string, parents []*Grammar) (g *Grammar) {
	if len(parents) == 0 {
		return New(name)
	}
	defer func() {
		if r := recover(); r != nil {
			msg, ok := r.(string)
			if !ok {
				panic(r)
			}
			p.fail("%s", msg)
		}
	}()
	return parents[0].Extend(name, parents[1:]...)
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
		if p.keyword("proto") {
			p.ws()
			if !p.keyword("token") && !p.keyword("regex") && !p.keyword("rule") {
				p.fail("expected token, regex or rule after proto")
			}
			pname := p.ident()
			if _, dup := g.rules[pname]; dup {
				p.fail("%s is declared twice", pname)
			}
			if !p.protoBody() {
				p.fail("a proto's body is {*}: its variants are rules named %s:sym<…>", pname)
			}
			p.protect(pname, func() { g.Proto(pname) })
			p.ws()
			p.eat(";")
			continue
		}
		switch {
		case p.keyword("token"):
			ratchet = true
		case p.keyword("regex"):
		case p.keyword("rule"):
			ratchet, sig = true, true // a rule is a token with :sigspace
		default:
			p.fail("expected token, regex or rule")
		}
		rname := p.ident() + p.symSuffix()
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
	p.protect(name, func() { g.add(name, body, ratchet) })
}

// protect runs f, which adds rule name to a grammar, failing as the
// grammar panics.
func (p *parser) protect(name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			if msg, ok := r.(string); ok {
				p.fail("in %s: %s", name, strings.TrimPrefix(msg, "grammar: "))
			}
			panic(r)
		}
	}()
	f()
}

// protoBody reads a proto's body, { * }, and reports whether it was one.
func (p *parser) protoBody() bool {
	for _, part := range []string{"{", "*", "}"} {
		p.ws()
		if !p.eat(part) {
			return false
		}
	}
	return true
}

// symSuffix reads a variant's :sym<…> or :sym«…» after its proto's name,
// if there is one, and returns it.
func (p *parser) symSuffix() string {
	if !p.peek(":sym<") && !p.peek(":sym«") {
		return ""
	}
	start := p.i
	p.i += len(":sym")
	closer := ">"
	if p.eat("«") {
		closer = "»"
	} else {
		p.eat("<")
	}
	for !p.eof() && !p.peek(closer) {
		p.i++
	}
	if p.eof() || p.i == start+len(":sym")+1 {
		p.fail("a :sym needs a name between its brackets, as in :sym<hour>")
	}
	p.expect(closer)
	return string(p.src[start:p.i])
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
	// As in Raku, any of the operators may come first, with nothing before.
	_ = p.eat("||") || p.eat("&&") || p.eat("|") || p.eat("&")
	branches := []Node{p.conjs()}
	for p.ws(); p.eat("||"); p.ws() {
		branches = append(branches, p.conjs())
	}
	if len(branches) == 1 {
		return branches[0]
	}
	return first(branches)
}

// The operators between branches, loosest first, as in Raku: ||, &&, |,
// then &.

// conjs reads branches joined by &&.
func (p *parser) conjs() Node {
	branches := []Node{p.alts()}
	for p.ws(); p.eat("&&"); p.ws() {
		branches = append(branches, p.alts())
	}
	if len(branches) == 1 {
		return branches[0]
	}
	return conj(branches)
}

// alts reads branches joined by |.
func (p *parser) alts() Node {
	branches := []Node{p.tightConjs()}
	for p.ws(); p.peek("|") && !p.peek("||"); p.ws() {
		p.i++
		branches = append(branches, p.tightConjs())
	}
	if len(branches) == 1 {
		return branches[0]
	}
	return alt(branches)
}

// tightConjs reads branches joined by &.
func (p *parser) tightConjs() Node {
	branches := []Node{p.branch()}
	for p.ws(); p.peek("&") && !p.peek("&&"); p.ws() {
		p.i++
		branches = append(branches, p.branch())
	}
	if len(branches) == 1 {
		return branches[0]
	}
	return conj(branches)
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
		if p.atSequenceEnd() {
			break
		}
		if p.commitColon() {
			p.fail("a : on its own stops backtracking into the atom before it, and there's no atom before this one")
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
		term := p.spacedTerm()
		if p.eat("~") {
			// Goal matching: open ~ close inner is open inner close, each
			// with the space that followed it in the source.
			p.ws()
			if p.atSequenceEnd() {
				p.fail("~ needs a closer and something to go between, as in '(' ~ ')' <inner>")
			}
			close := p.spacedTerm()
			if p.atSequenceEnd() || p.peek("~") {
				p.fail("~ needs something to go between the brackets, as in '(' ~ ')' <inner>")
			}
			term = Goal(term, close, p.spacedTerm())
		}
		items = append(items, term)
	}
	return Seq(items...)
}

// atSequenceEnd reports whether the sequence being read ends here: at the
// end of the source, a branch's | or &, a group's closing ] or ), the end
// of the rule, or the > that ends a lookaround, which >>, a word boundary,
// isn't, as in Raku.
func (p *parser) atSequenceEnd() bool {
	return p.eof() || p.peek("|") || p.peek("&") || p.peek("]") || p.peek(")") || p.peek("}") ||
		p.assert > 0 && p.peek(">") && !p.peek(">>")
}

// spacedTerm reads a quantified term and, in a rule, the space after it.
func (p *parser) spacedTerm() Node {
	sig := p.sig // as it is where the whitespace after the term is
	term, spaced := p.quantified()
	if spaced && sig {
		return Seq(term, Silent("ws"))
	}
	return term
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
		name := p.ident()
		p.expect(">")
		// $<name> = …, with or without spaces, is a capture; on its own, a
		// backreference, whose spaces are left for quantifier to read, as
		// they matter in a rule.
		after := p.i
		p.ws()
		if !p.eat("=") {
			p.i = after
			return p.quantifier(Backref(name))
		}
		capName = name
		p.single = true
	}
	var atom Node
	if capName != "" && p.peek("$<") {
		// $<r>=$<q>: a backreference, captured.
		p.eat("$<")
		atom = Backref(p.ident())
		p.expect(">")
		p.single = false
	} else {
		atom = p.atom()
	}
	n, spaced := p.quantifier(atom)
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
	if p.peek("::") {
		p.fail("::, ::: and the like aren't supported, as Rakudo doesn't support them either: use : after an atom, or a token")
	}
	if p.commitColon() {
		// atom: never gives back what it first matched, as in a token.
		p.eat(":")
		return commit{atom}, p.space()
	}
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
		p.single = true
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

// commitColon reports whether a : comes next on its own, as in [ a | ab ]:
// or <w> : x, which stops backtracking into the atom before it, rather
// than starting an adverb such as :i or :!r, which has its name straight
// after the colon, as in Raku.
func (p *parser) commitColon() bool {
	if !p.peek(":") || p.peek("::") {
		return false
	}
	if p.i+1 >= len(p.src) {
		return true
	}
	next := p.src[p.i+1]
	return !unicode.IsLetter(next) && next != '_' && next != '!'
}

// quantifierAt reports whether a quantifier follows position at, after any
// whitespace.
func (p *parser) quantifierAt(at int) bool {
	saved := p.i
	defer func() { p.i = saved }()
	p.i = at
	p.ws()
	return p.peek("**") || p.peek("?") || p.peek("*") || p.peek("+")
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
	single := p.single
	p.single = false
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
		start := p.i - 1
		in := p.escape()
		return class{in: in, desc: string(p.src[start:p.i])}
	case p.eat("^^"):
		return LineStart
	case p.eat("^"):
		return Start
	case p.peek("$") && p.i+1 < len(p.src) && unicode.IsDigit(p.src[p.i+1]):
		p.eat("$")
		return Backref(strconv.Itoa(p.number()))
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
		// A run of letters is one literal, so a failure report names the
		// whole word. Raku matches it a letter at a time, which matches the
		// same text, except where it binds to one letter: the last, before
		// a quantifier (ab* is a b*), and the first, after $<x>= or as a
		// separator.
		start, end := p.i, p.i+1
		if !single {
			for end < len(p.src) && isWord(p.src[end]) {
				end++
			}
			if end-start > 1 && p.quantifierAt(end) {
				end--
			}
		}
		p.i = end
		return Lit(string(p.src[start:end]))
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
	case p.peek("[") || p.peek("-") || p.peek("+") || p.peek(":"):
		start := p.i - 1 // the <
		f, fci := p.classExpr()
		p.expect(">")
		return class{in: f, desc: string(p.src[start:p.i]), inCI: fci}
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
		if p.eat(">") {
			// <?> always matches, and <!> never does.
			return &look{inner: seq(nil), neg: neg}
		}
		name := p.ident()
		p.expect(">")
		return &look{inner: Silent(name), neg: neg}
	}
	if p.eat("@") {
		name := p.ident()
		p.expect(">")
		if l, ok := p.patterns[name]; ok {
			return patterns{l}
		}
		if _, ok := p.lists[name]; ok {
			p.fail("@%s is a word list, of phrases to match as text, with @%s; <@%s> is for a grammar.PatternList", name, name, name)
		}
		p.fail("there's no pattern list @%s: pass a grammar.PatternList to Compile", name)
	}
	silent := p.eat(".")
	name := p.ident()
	if name == "sym" && !silent && p.peek(">") {
		p.expect(">")
		return Sym()
	}
	if variant := name + p.symSuffix(); variant != name {
		// Raku reads this as a call to the proto, the :sym<…> ignored,
		// which is never what it looks like.
		p.fail("<%s> calls the proto, not the variant, in Raku: write <%s>, or call the variant's rules by names of their own", variant, name)
	}
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
//
// It returns the class, and the class under :i, which folds the case of
// its [ ... ] sets but not of its properties or named classes, as Raku
// does.
func (p *parser) classExpr() (f, fci func(rune) bool) {
	for {
		p.ws()
		sign := '+'
		switch {
		case p.eat("+"):
		case p.eat("-"):
			sign = '-'
		default:
			if f != nil {
				return f, fci
			}
		}
		p.ws()
		var g, gci func(rune) bool
		switch {
		case p.eat("["):
			g = p.set()
			gci = folded(g)
		case p.eat(":"):
			g = p.property()
			gci = g
		default:
			name := p.ident()
			named := map[string]func(rune) bool{
				"alpha": isAlpha, "digit": isDigit, "space": isSpace, "upper": unicode.IsUpper,
				"lower": unicode.IsLower, "punct": unicode.IsPunct, "alnum": func(r rune) bool { return isAlpha(r) || isDigit(r) },
			}
			if g = named[name]; g == nil {
				p.fail("<%s> can't be used in a character class", name)
			}
			gci = g
		}
		f, fci = combine(f, g, sign), combine(fci, gci, sign)
	}
}

// combine adds g to the class f, or takes it away, by sign; f is nil at
// the start of a class.
func combine(f, g func(rune) bool, sign rune) func(rune) bool {
	switch {
	case f == nil && sign == '-':
		return not(g)
	case f == nil:
		return g
	case sign == '-':
		return func(r rune) bool { return f(r) && !g(r) }
	}
	return func(r rune) bool { return f(r) || g(r) }
}

// folded is f ignoring case: a character is in it if any case of it is in f.
func folded(f func(rune) bool) func(rune) bool {
	return func(r rune) bool {
		if f(r) {
			return true
		}
		for c := unicode.SimpleFold(r); c != r; c = unicode.SimpleFold(c) {
			if f(c) {
				return true
			}
		}
		return false
	}
}

// property reads a Unicode property after its colon: Lu, !Lu (not Lu),
// Letter, Latin, or Script<Latin>.
func (p *parser) property() func(rune) bool {
	neg := p.eat("!")
	name := p.ident()
	var f func(rune) bool
	var ok bool
	if looseName(name) == "script" && p.eat("<") {
		script := p.ident()
		p.expect(">")
		if f, ok = unicodeScript(script); !ok {
			p.fail("there's no Unicode script %s, as in <:Script<%s>>: see Go's unicode.Scripts", script, script)
		}
	} else if f, ok = unicodeProperty(name); !ok {
		p.fail("there's no Unicode property %s, as in <:%s>: use a category (Lu, Letter), a script (Latin) or a binary property (White_Space, Alpha)", name, name)
	}
	if neg {
		return not(f)
	}
	return f
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
