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
	joined bool
	whole  bool
	plural bool
	root   *trie
	values map[string][]string // a normalised phrase's canonical values, in the order added

	// The same, with case folded, for matching under :i.
	rootFold   *trie
	valuesFold map[string][]string

	// made are the keys of values that only Plurals made, and madeFold the
	// same for valuesFold, so Phrases, Len and Ambiguous can leave them out,
	// and a phrase added as itself can take one over.
	made, madeFold map[string]bool
}

type trie struct {
	next map[rune]*trie
	end  bool
	key  string // at an end, the phrase as stored, to find its values

	// spacing are the nodes after a space, hyphen, dash or underscore, the
	// ones JoinedSpacing may skip to, kept apart so the search doesn't
	// have to look through next for them.
	spacing []*trie
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

// JoinedSpacing lets the text leave out the spacing in a phrase, so "on
// site" in the list matches "onsite" too, and "front-end" matches
// "frontend". Job postings write the same term run together, hyphenated and
// spaced, and without this a list must spell out every form.
//
// It works one way only: write the spaced form in the list. A phrase
// written "onsite" can't match "on site", as there's no telling where a
// space would go. Each space, hyphen, dash or underscore in a phrase may be
// left out, and each separately: "full stack web developer" matches
// "fullstack web developer" and "fullstackwebdeveloper". With LooseSpacing
// as well, the text can have any spacing between the words, or none.
//
// A phrase with a long run of spacing, without LooseSpacing, can be
// matched or skipped a character at a time, which can take a while; the
// search counts against StepLimit, as backtracking does.
//
// Run together, two words can make a different word: "pen is" would match
// "penis". A phrase in the list as written wins over a joined reading of
// another, so add the joined word too where it means something else.
func JoinedSpacing() ListOption { return func(l *WordList) { l.joined = true } }

// Plurals lets each phrase match its plural too, as postings write "Care
// Assistants" for a list's "Care Assistant": the canonical value is the
// same. Without it, a list must spell out both forms, or use a stem such
// as "opportunit" that matches neither word whole.
//
// Only the last word changes ("sales assistants", not "saleses
// assistants"), by the regular English rules: -es after s, x, z, ch and
// sh ("coaches"), -ies for a y after a consonant ("secretaries"), and -s
// otherwise ("engineers", "days"). The ending is lower case, as for
// acronyms ("PMs"); with FoldCase, case doesn't matter anyway. A phrase
// that doesn't end with a letter, such as "level 2" or "c++", has no
// plural. Irregular plurals ("people", "analyses") are ordinary synonyms:
// add them.
//
// The plurals are made, not added: Phrases, Len and Ambiguous show only
// what was added, while Canonical and matching know the plurals. A phrase
// added as itself beats a plural made from another, whichever came first,
// so with "new" and "news" both in the list, "news" means news.
func Plurals() ListOption { return func(l *WordList) { l.plural = true } }

// WholeWords matches a phrase only where it isn't part of a longer word:
// "java" doesn't match the start of "JavaScript", nor "lead" the start of
// "leadership". It's << @list >> without having to write the boundaries,
// except that a phrase beginning or ending with punctuation, such as "c++",
// can end where punctuation does. The longest phrase that fits still wins.
//
// Digits are word characters, as in Raku, so "pa" in "£45000pa" isn't a
// whole word: the 0 before it makes it part of one. Where a phrase may
// follow a figure directly, as a unit of pay does, leave this off and
// check only the end, with <!before \w> after the list.
func WholeWords() ListOption { return func(l *WordList) { l.whole = true } }

// NewWordList returns an empty list called name, which is how Raku syntax
// refers to it: @name.
func NewWordList(name string, opts ...ListOption) *WordList {
	l := &WordList{name: name, root: &trie{}, values: map[string][]string{},
		rootFold: &trie{}, valuesFold: map[string][]string{}, made: map[string]bool{}, madeFold: map[string]bool{}}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Name returns the list's name.
func (l *WordList) Name() string { return l.name }

// Len returns how many distinct phrases the list holds, not counting
// plurals that Plurals made.
func (l *WordList) Len() int { return len(l.values) - len(l.made) }

// Add adds canonical and each of synonyms to the list, all meaning
// canonical. A phrase added for two canonical values means both; see
// Ambiguous. Empty phrases are skipped.
func (l *WordList) Add(canonical string, synonyms ...string) *WordList {
	return l.AddAs(canonical, append([]string{canonical}, synonyms...)...)
}

// AddAs adds each of phrases to the list, all meaning value, without adding
// value itself as a phrase. Use it when the value is a code or an ID rather
// than words, such as an occupation code: AddAs("2134", "Software engineer")
// matches "Software engineer" but not "2134". Empty phrases are skipped.
func (l *WordList) AddAs(value string, phrases ...string) *WordList {
	for _, phrase := range phrases {
		key := l.normalise(phrase)
		if key == "" {
			continue
		}
		l.addKey(key, value, false)
		if plural := pluralOf(key); l.plural && plural != "" {
			l.addKey(plural, value, true)
		}
	}
	return l
}

// addKey adds key, meaning canonical, to the list, and its case-folded
// form to the folded trie. A made plural doesn't replace a phrase added as
// itself, and a phrase added as itself replaces a made plural.
func (l *WordList) addKey(key, canonical string, made bool) {
	add := func(root *trie, values map[string][]string, madeKeys map[string]bool, key string) {
		_, exists := values[key]
		switch {
		case made && exists && !madeKeys[key]:
			return
		case made:
			madeKeys[key] = true
		case madeKeys[key]:
			delete(values, key)
			delete(madeKeys, key)
		}
		insert(root, values, key, canonical)
	}
	add(l.root, l.values, l.made, key)
	add(l.rootFold, l.valuesFold, l.madeFold, strings.ToLower(key))
}

// pluralOf returns phrase with its last word made plural by the regular
// English rules, or "" if it doesn't end with a letter.
func pluralOf(phrase string) string {
	rs := []rune(phrase)
	n := len(rs)
	if n == 0 || !unicode.IsLetter(rs[n-1]) {
		return ""
	}
	lower := strings.ToLower(phrase)
	switch {
	case strings.HasSuffix(lower, "s") || strings.HasSuffix(lower, "x") || strings.HasSuffix(lower, "z") ||
		strings.HasSuffix(lower, "ch") || strings.HasSuffix(lower, "sh"):
		return phrase + "es"
	case n >= 2 && unicode.ToLower(rs[n-1]) == 'y' && unicode.IsLetter(rs[n-2]) &&
		!strings.ContainsRune("aeiou", unicode.ToLower(rs[n-2])):
		return string(rs[:n-1]) + "ies"
	}
	return phrase + "s"
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
			if isListSpace(r) {
				t.spacing = append(t.spacing, t.next[r])
			}
		}
		t = t.next[r]
	}
	t.end, t.key = true, key
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
// the first bad one. A line ending with a comma carries on onto the next,
// so a long rule can be wrapped:
//
//	annum, year, yr, pa,
//	    per annum, p.a.
func (l *WordList) AddRules(rules string) error {
	var rule strings.Builder
	for line := range strings.Lines(rules) {
		line = strings.TrimSpace(line)
		rule.WriteString(line)
		if strings.HasSuffix(line, ",") && !strings.HasSuffix(line, `\,`) {
			rule.WriteString(" ")
			continue
		}
		if err := l.AddRule(rule.String()); err != nil {
			return err
		}
		rule.Reset()
	}
	return l.AddRule(rule.String())
}

// WordLists makes a word list of each set of synonym rules in lists, named by
// its key and read as AddRules reads them, with opts for every list. The
// result goes straight to Compile:
//
//	uses, err := grammar.WordLists(map[string]string{
//	    "units":  "Year, annum, pa\nWeek, wk",
//	    "levels": "Senior, snr, sr",
//	}, grammar.FoldCase())
//	g, err := grammar.Compile(src, uses...)
func WordLists(lists map[string]string, opts ...ListOption) ([]Use, error) {
	var uses []Use
	for _, name := range slices.Sorted(maps.Keys(lists)) {
		l := NewWordList(name, opts...)
		if err := l.AddRules(lists[name]); err != nil {
			return nil, fmt.Errorf("word list %s: %w", name, err)
		}
		uses = append(uses, l)
	}
	return uses, nil
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
//
// With JoinedSpacing, a phrase written without the spacing of one in the
// list means what that one does: "onsite" means what "on site" does.
func (l *WordList) Canonical(phrase string) []string {
	key := l.normalise(phrase)
	if v, ok := l.values[key]; ok {
		return slices.Clone(v)
	}
	if v, ok := l.valuesFold[strings.ToLower(key)]; ok || !l.joined {
		return slices.Clone(v)
	}
	in := []rune(key)
	for _, fold := range []bool{false, true} {
		if end, t := l.longest(in, 0, fold, nil); end == len(in) {
			return slices.Clone(l.valuesIn(fold)[t.key])
		}
	}
	return nil
}

func (l *WordList) valuesIn(fold bool) map[string][]string {
	if fold {
		return l.valuesFold
	}
	return l.values
}

// Ambiguous returns the phrases that mean more than one thing, with what
// each means. A match of one of them has the first as its made value.
func (l *WordList) Ambiguous() map[string][]string {
	out := map[string][]string{}
	for k, v := range l.values {
		if len(v) > 1 && !l.made[k] {
			out[k] = slices.Clone(v)
		}
	}
	return out
}

// Phrases returns the list's phrases, normalised, in sorted order, without
// the plurals that Plurals made.
func (l *WordList) Phrases() []string {
	var out []string
	for k := range l.values {
		if !l.made[k] {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

func isListSpace(r rune) bool {
	return unicode.IsSpace(r) || r == '-' || r == '_' || unicode.Is(unicode.Pd, r)
}

// normalise is how the list stores phrase, and how it reads text it
// matched: with case folded and runs of spacing collapsed, if the list says
// so.
func (l *WordList) normalise(phrase string) string {
	phrase = nfc(phrase)
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
	if l.joined {
		end := -1
		if !(l.whole && midWord(in, pos)) {
			end, _ = l.longest(in, pos, fold, st)
		}
		if end < 0 {
			st.fail(pos, n, e)
			return false
		}
		return k(end, c)
	}
	t, i, end := l.root, pos, -1
	if fold {
		t = l.rootFold
	}
	if l.whole && midWord(in, pos) {
		st.fail(pos, n, e)
		return false
	}
	for t != nil {
		if t.end && !(l.whole && midWord(in, i)) {
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

// longest returns where the longest phrase starting at in[pos] ends, and
// the trie node it ends at, or -1 and nil. It's the loop in words.match,
// searching every way through the trie, as JoinedSpacing lets a phrase's
// spacing be skipped. (Canonical looks a phrase up as written before
// calling it, which is how an exact phrase beats a joined reading of
// another.)
//
// Each spacing character can be matched or skipped, so a run of n of them
// has 2ⁿ ways through. A node reached at a position always leads to the
// same ends, so the search remembers where it's been, and does no more work
// than nodes × positions. It only starts remembering at the first skip:
// until then there's one way through, as without JoinedSpacing.
//
// That's still a lot for a phrase with a long run of spacing, which can be
// matched or skipped a character at a time, so in a match each place the
// search visits is a step, counted against StepLimit. Canonical passes no
// state: it reads text that has already matched, which searched as far.
func (l *WordList) longest(in []rune, pos int, fold bool, st *state) (int, *trie) {
	root := l.root
	if fold {
		root = l.rootFold
	}
	type place struct {
		t *trie
		i int
	}
	end, at := -1, (*trie)(nil)
	var seen map[place]bool
	var walk func(t *trie, i int)
	walk = func(t *trie, i int) {
		if seen != nil {
			if seen[place{t, i}] {
				return
			}
			seen[place{t, i}] = true
		}
		if st != nil {
			st.tick()
		}
		if t.end && i > end && !(l.whole && midWord(in, i)) {
			end, at = i, t
		}
		if i < len(in) {
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
			if next := t.next[r]; next != nil {
				walk(next, i+width)
			}
		}
		for _, next := range t.spacing {
			if seen == nil {
				seen = map[place]bool{}
			}
			walk(next, i) // the text leaves this spacing out
		}
	}
	walk(root, pos)
	return end, at
}

// midWord reports whether position at is inside a word: between two word
// characters, where WholeWords won't start or end a phrase.
func midWord(in []rune, at int) bool {
	return at > 0 && at < len(in) && isWord(in[at-1]) && isWord(in[at])
}

// wordsBody returns the list a rule body or named capture is, if it's
// nothing else: the list itself, under an adverb, or quantified, as in
// $<level>=@levels?. What matched is looked up in the list, so it has a
// canonical value only when it's one of the list's phrases.
func wordsBody(n Node) *WordList {
	for {
		switch b := n.(type) {
		case words:
			return b.l
		case *mode:
			n = b.inner
		case *Quant:
			n = b.item
		case commit:
			n = b.inner
		default:
			return nil
		}
	}
}
