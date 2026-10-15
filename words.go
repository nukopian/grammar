package grammar

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
)

// WordList is a list of words and phrases, each with a canonical value: the
// synonyms "Software Developer" and "SWE" can both mean "Software
// Engineer". A grammar matches it with Words, or @name in Raku syntax,
// and a rule whose whole body is a word list has the canonical value of
// what it matched as its made value.
//
// Build a list fully before parsing with it; it isn't safe to change while
// a parse is using it. To refresh a list, build a new one and a grammar
// that uses it.
type WordList struct {
	name   string
	fold   bool
	loose  bool
	root   *trie
	values map[string][]string // a normalised phrase's canonical values, in the order added

	// The same, with case folded, for matching under :i.
	rootFold   *trie
	valuesFold map[string][]string
}

type trie struct {
	next map[rune]*trie
	end  bool
}

// ListOption changes how a WordList matches.
type ListOption func(*WordList)

// FoldCase matches a list's phrases ignoring case, wherever it's used.
// Without it, a list ignores case only under :i, as in Raku.
func FoldCase() ListOption { return func(l *WordList) { l.fold = true } }

// LooseSpacing matches any run of spaces, hyphens, dashes and underscores
// in the text with any such run in a phrase, so "front-end" in the list
// matches "front end" and "front – end" in the text.
func LooseSpacing() ListOption { return func(l *WordList) { l.loose = true } }

// NewWordList returns an empty list called name, which is how Raku syntax
// refers to it: @name.
func NewWordList(name string, opts ...ListOption) *WordList {
	l := &WordList{name: name, root: &trie{}, values: map[string][]string{},
		rootFold: &trie{}, valuesFold: map[string][]string{}}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Name returns the list's name.
func (l *WordList) Name() string { return l.name }

// Len returns how many distinct phrases the list holds.
func (l *WordList) Len() int { return len(l.values) }

// Add adds canonical and each of synonyms to the list, all meaning
// canonical. A phrase added for two canonical values means both; see
// Ambiguous. Empty phrases are skipped.
func (l *WordList) Add(canonical string, synonyms ...string) *WordList {
	for _, phrase := range append([]string{canonical}, synonyms...) {
		key := l.normalise(phrase)
		if key == "" {
			continue
		}
		insert(l.root, l.values, key, canonical)
		insert(l.rootFold, l.valuesFold, strings.ToLower(key), canonical)
	}
	return l
}

func insert(root *trie, values map[string][]string, key, canonical string) {
	if !slices.Contains(values[key], canonical) {
		values[key] = append(values[key], canonical)
	}
	t := root
	for _, r := range key {
		if t.next == nil {
			t.next = map[rune]*trie{}
		}
		if t.next[r] == nil {
			t.next[r] = &trie{}
		}
		t = t.next[r]
	}
	t.end = true
}

// AddRule adds one synonym rule in the format Elasticsearch and Solr use:
//
//	i-pod, i pod => ipod      each phrase on the left means ipod
//	ipod, i-pod, i pod        all mean the first, ipod
//	pm => product manager, project manager
//	                          pm means both; see Ambiguous
//
// Commas and => may be escaped with a backslash. Blank rules and rules
// starting with # are ignored.
func (l *WordList) AddRule(rule string) error {
	rule = strings.TrimSpace(rule)
	if rule == "" || strings.HasPrefix(rule, "#") {
		return nil
	}
	sides := splitRule(rule, "=>")
	switch len(sides) {
	case 1:
		phrases := phrasesOf(sides[0])
		l.Add(phrases[0], phrases[1:]...)
	case 2:
		lhs, rhs := phrasesOf(sides[0]), phrasesOf(sides[1])
		if len(rhs) == 1 && rhs[0] == "" {
			return fmt.Errorf("grammar: synonym rule %q has nothing after =>", rule)
		}
		for _, canonical := range rhs {
			l.Add(canonical, lhs...)
		}
	default:
		return fmt.Errorf("grammar: synonym rule %q has more than one =>", rule)
	}
	return nil
}

// AddRules adds each line of rules as a rule, as AddRule does, and reports
// the first bad one.
func (l *WordList) AddRules(rules string) error {
	for line := range strings.Lines(rules) {
		if err := l.AddRule(line); err != nil {
			return err
		}
	}
	return nil
}

// splitRule splits s at each sep that isn't escaped with a backslash,
// keeping the escapes for phrasesOf.
func splitRule(s, sep string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); {
		switch {
		case s[i] == '\\' && i+1 < len(s):
			i += 2
		case strings.HasPrefix(s[i:], sep):
			parts = append(parts, s[start:i])
			i += len(sep)
			start = i
		default:
			i++
		}
	}
	return append(parts, s[start:])
}

// phrasesOf splits one side of a rule into its phrases, unescaped and
// trimmed.
func phrasesOf(side string) []string {
	parts := splitRule(side, ",")
	for i, p := range parts {
		var b strings.Builder
		for j := 0; j < len(p); j++ {
			if p[j] == '\\' && j+1 < len(p) {
				j++
			}
			b.WriteByte(p[j])
		}
		parts[i] = strings.TrimSpace(b.String())
	}
	return parts
}

// Canonical returns what phrase means: its canonical values, in the order
// they were added, or nil if it isn't in the list. A phrase that's only in
// the list ignoring case, as :i matches, means what its other case does.
func (l *WordList) Canonical(phrase string) []string {
	key := l.normalise(phrase)
	if v, ok := l.values[key]; ok {
		return slices.Clone(v)
	}
	return slices.Clone(l.valuesFold[strings.ToLower(key)])
}

// Ambiguous returns the phrases that mean more than one thing, with what
// each means. A match of one of them has the first as its made value.
func (l *WordList) Ambiguous() map[string][]string {
	out := map[string][]string{}
	for k, v := range l.values {
		if len(v) > 1 {
			out[k] = slices.Clone(v)
		}
	}
	return out
}

// Phrases returns the list's phrases, normalised, in sorted order.
func (l *WordList) Phrases() []string { return slices.Sorted(maps.Keys(l.values)) }

func isListSpace(r rune) bool {
	return unicode.IsSpace(r) || r == '-' || r == '_' || unicode.Is(unicode.Pd, r)
}

// normalise is how the list stores phrase, and how it reads text it
// matched: with case folded and runs of spacing collapsed, if the list says
// so.
func (l *WordList) normalise(phrase string) string {
	if l.fold {
		phrase = strings.ToLower(phrase)
	}
	if l.loose {
		phrase = strings.Join(strings.FieldsFunc(phrase, isListSpace), " ")
	}
	return phrase
}

type words struct{ l *WordList }

// Words matches the longest phrase in l, as @name does. Like Raku's, it's
// one atom: it never gives back part of what it matched, even in a regex.
func Words(l *WordList) Node { return words{l} }

func (n words) match(st *state, pos int, c *capList, e env, k cont) bool {
	in, l := st.in, n.l
	fold := l.fold || e.ci
	t, i, end := l.root, pos, -1
	if fold {
		t = l.rootFold
	}
	for t != nil {
		if t.end {
			end = i
		}
		if i >= len(in) {
			break
		}
		r, width := in[i], 1
		if l.loose && isListSpace(r) {
			for i+width < len(in) && isListSpace(in[i+width]) {
				width++
			}
			r = ' '
		}
		if fold {
			r = unicode.ToLower(r)
		}
		t = t.next[r]
		i += width
	}
	if end < 0 {
		st.fail(pos, n, e)
		return false
	}
	return k(end, c)
}

// wordsBody returns the list a rule body is, if it's nothing else.
func wordsBody(n Node) *WordList {
	for {
		switch b := n.(type) {
		case words:
			return b.l
		case *mode:
			n = b.inner
		default:
			return nil
		}
	}
}
