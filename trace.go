package grammar

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Trace writes a trace of the parse to w, in the style of Raku's
// Grammar::Tracer: each rule's name as it's entered, indented by depth,
// then "* MATCH" and the text it matched, or "* FAIL", at its caller's
// depth.
//
//	TOP
//	|  amount
//	|  * MATCH "£30,000"
//	|  period
//	|  * FAIL
//	* FAIL
//
// It traces what the grammar does, so a rule tried by more than one
// branch of an alternation, or by each variant of a proto, is traced each
// time. As in Grammar::Tracer, a rule is traced once each time it's
// called: backtracking into a rule that has matched prints nothing more
// for it, though the rules it calls as it tries again are traced, beside
// it; and built-in rules, such as <ws> and <alpha>, aren't traced unless
// the grammar defines its own. Find, FindAll and Split trace each
// position they try, which leaves out those where no match can start.
// Tracing changes no result. A match stopped by StepLimit or DepthLimit
// leaves the rule it was in open at the end of the trace, and while
// tracing, the error names the innermost traced rule and where its match
// started, as ParseErr's names the innermost rule; ParseErr's can be
// <ws>, which isn't traced.
// What w returns is ignored, so an error writing the trace doesn't stop
// the parse.
func Trace(w io.Writer) Option { return func(o *options) { o.trace = w } }

// tracer writes a trace; see Trace. calls are the traced rules matching,
// outermost first, kept as the failure tracker keeps its own: a rule is
// off the stack while its caller goes on. The last is where a limit error
// says it was reached. depth is how far the trace is indented: the rules
// that haven't matched yet. A rule that has matched stays at its caller's
// depth when matching backtracks into it, so the rules it calls as it
// tries again are traced beside it, as Raku's tracer traces them.
type tracer struct {
	w     io.Writer
	calls []frame
	depth int
	b     strings.Builder
}

func (t *tracer) enter(f frame) { t.calls = append(t.calls, f) }
func (t *tracer) leave()        { t.calls = t.calls[:len(t.calls)-1] }

func (t *tracer) line(s string) {
	t.b.Reset()
	t.b.WriteString(strings.Repeat("|  ", t.depth))
	t.b.WriteString(s)
	t.b.WriteByte('\n')
	io.WriteString(t.w, t.b.String())
}

// matched prints a rule's match of text, cut, as Grammar::Tracer cuts it,
// to fit 60 characters less three for each level of depth.
func (t *tracer) matched(text []rune) {
	n := 60 - 3*t.depth
	if n <= 0 {
		t.line("* MATCH")
		return
	}
	if len(text) > n {
		text = text[:n]
	}
	t.line("* MATCH " + rakuQuote(text))
}

// traced reports whether r is traced: built-in rules aren't, as Raku's
// aren't methods Grammar::Tracer wraps.
func traced(r *rule) bool { return builtins.rules[r.name] != r }

// matchTraced is call.match with r traced: its name on entry, and
// whether it matched once its body has.
func (n *call) matchTraced(st *state, pos int, c *capList, e env, k cont, r *rule) bool {
	t := st.trace
	f := frame{r.name, pos}
	t.line(r.name)
	t.enter(f)
	t.depth++
	done := false
	ok := n.run(st, pos, c, e, func(p int, cc *capList) bool {
		t.leave()
		if !done {
			done = true
			t.depth--
			t.matched(st.in[pos:p])
		}
		res := k(p, cc)
		t.enter(f)
		return res
	}, r)
	t.leave()
	if !done {
		t.depth--
		t.line("* FAIL")
	}
	return ok
}

// rakuQuote quotes s as Raku's Str.raku does: in double quotes, with what
// would interpolate and control characters escaped, and a combining mark
// at the start, which would otherwise join the quote, written as a code.
func rakuQuote(s []rune) string {
	var b strings.Builder
	b.WriteByte('"')
	for i, r := range s {
		switch r {
		case '"', '\\', '$', '@', '%', '&', '{':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case 0:
			b.WriteString(`\0`)
		case '\b':
			b.WriteString(`\b`)
		default:
			if unicode.IsControl(r) || (i == 0 && unicode.Is(unicode.M, r)) {
				fmt.Fprintf(&b, `\x[%X]`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
