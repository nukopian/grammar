package grammar

import (
	"encoding/json"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/nukopian/scalar"
)

// capsTree is a match's captures in order, as Caps gives them, each with
// its own, and its chunks, as Chunks gives them: the shape the Raku script
// below prints, to compare. A capture's chunk has its place too, for
// tieOrder.
func capsTree(m *scalar.SV) map[string]any {
	caps, subs, chunks := []any{}, []any{}, []any{}
	for k, c := range Caps(m) {
		caps = append(caps, []any{k, c.At("from").Int(), c.At("to").Int()})
		subs = append(subs, capsTree(c))
	}
	for k, c := range Chunks(m) {
		if k == "~" {
			chunks = append(chunks, []any{k, c.Str()})
		} else {
			chunks = append(chunks, []any{k, c.At("~").Str(), c.At("from").Int(), c.At("to").Int()})
		}
	}
	return map[string]any{"caps": caps, "subs": subs, "chunks": chunks}
}

// tieOrder puts Raku's captures at the same place in Caps' name order, as
// Raku leaves them in hash order, which changes from run to run. A
// capture's place is its from and to; a "~" chunk stays where it is.
func tieOrder(tree *scalar.SV) {
	if !tree.Defined() {
		return
	}
	place := func(e *scalar.SV) [2]int64 { return [2]int64{e.Get(-2).Int(), e.Get(-1).Int()} }
	for _, list := range []string{"caps", "chunks"} {
		items := tree.At(list).AV()
		// Sort each run of entries at one place: the entries and the
		// trees under them, which are in the same order as caps.
		n := items.Length()
		for i := 0; i < n; {
			j := i + 1
			for j < n && items.Get(i, 0).Str() != "~" && items.Get(j, 0).Str() != "~" &&
				place(items.Get(i)) == place(items.Get(j)) {
				j++
			}
			if j-i > 1 {
				idx := make([]int, j-i)
				for k := range idx {
					idx[k] = i + k
				}
				slices.SortStableFunc(idx, func(a, b int) int {
					return keyCmp(items.Get(a, 0).Str(), items.Get(b, 0).Str())
				})
				reorder := func(av *scalar.AV) {
					vals := make([]*scalar.SV, len(idx))
					for k, from := range idx {
						vals[k] = av.Get(from).Copy()
					}
					for k, v := range vals {
						av.Set(i+k, v)
					}
				}
				if list == "caps" {
					reorder(tree.At("subs").AV())
				}
				reorder(items)
			}
			i = j
		}
	}
	for _, sub := range tree.At("subs").AV().All() {
		tieOrder(sub)
	}
}

// Caps and Chunks give captures in the order Raku's .caps and .chunks do,
// at every level of every Raku case's match.
func TestCapsMatchRaku(t *testing.T) {
	if _, err := exec.LookPath("raku"); err != nil {
		t.Skip("raku isn't installed")
	}
	in, _ := json.Marshal(rakuCases)
	const script = `
use MONKEY-SEE-NO-EVAL;
sub tree($m) {
    my @caps = $m.caps.map({ [ ~.key, .value.from, .value.to ] });
    my @subs = $m.caps.map({ tree(.value) });
    my @chunks = $m.chunks.map({ .key eq '~' ?? [ '~', ~.value ] !! [ ~.key, ~.value, .value.from, .value.to ] });
    { caps => @caps, subs => @subs, chunks => @chunks }
}
my @out;
for @(Rakudo::Internals::JSON.from-json($*IN.slurp)) -> $case {
    my $prelude = (|($case<lists> // {}).pairs, |($case<patterns> // {}).pairs).map(-> $p {
        my ($k, $v) = $p.kv;
        "my \@$k = " ~ ($v.elems ?? $v.map({ "'" ~ .subst("\\", "\\\\", :g).subst("'", "\\'", :g) ~ "'" }).join(", ") !! "()") ~ ";\n"
    }).join;
    my $g = EVAL $prelude ~ $case<src>;
    @out.push: [ $case<inputs>.map: -> $in {
        my $m = $case<sub> ?? $g.subparse($in) !! $g.parse($in);
        $m ?? tree($m) !! Any
    } ];
}
print Rakudo::Internals::JSON.to-json(@out, :!pretty);
`
	cmd := exec.Command("raku", "-e", script)
	cmd.Stdin = strings.NewReader(string(in))
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("raku: %v: %s", err, stderr.String())
	}
	want, err := scalar.FromJSON(out)
	if err != nil {
		t.Fatalf("raku's output: %v", err)
	}
	for i, c := range rakuCases {
		g, err := Compile(c.Src, caseUses(c.Lists, c.Patterns)...)
		if err != nil {
			t.Errorf("case %d: %v", i, err)
			continue
		}
		for j, input := range c.Inputs {
			var m *scalar.SV
			if c.Sub {
				m = g.Subparse(input)
			} else {
				m = g.Parse(input)
			}
			w := want.Get(i, j)
			if !m.Defined() {
				if w.Defined() {
					t.Errorf("%s on %q: no match here", g.Name(), input)
				}
				continue
			}
			got := scalar.Value(capsTree(m))
			tieOrder(w)
			if d := got.Diff(w); d != "" {
				gj, _ := got.MarshalJSON()
				wj, _ := w.MarshalJSON()
				t.Errorf("%s on %q: %s\n got: %s\nwant: %s", g.Name(), input, d, gj, wj)
			}
		}
	}
}

// Caps on no match gives nothing, and so does Chunks.
func TestCapsUndef(t *testing.T) {
	g := MustCompile(`grammar G { token TOP { <a> }; token a { a } }`)
	m := g.Parse("b")
	for range Caps(m) {
		t.Error("Caps gave something for no match")
	}
	for range Chunks(m) {
		t.Error("Chunks gave something for no match")
	}
}

// Stopping a range over Caps or Chunks early is clean.
func TestCapsStop(t *testing.T) {
	g := MustCompile(`grammar G { token TOP { <a>+ % ',' }; token a { a } }`)
	m := g.Parse("a,a,a")
	n := 0
	for range Caps(m) {
		n++
		break
	}
	for range Chunks(m) {
		n++
		break
	}
	if n != 2 {
		t.Errorf("n = %d", n)
	}
}
