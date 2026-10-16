package grammar

import (
	"errors"
	"strings"
	"testing"

	"github.com/nukopian/job-meta/internal/scalar"
)

// withStepLimit runs f with StepLimit set to n.
func withStepLimit(t *testing.T, n int, f func()) {
	t.Helper()
	old := StepLimit
	StepLimit = n
	defer func() { StepLimit = old }()
	f()
}

// stepLimitPanic runs f and returns the *StepLimitError it panics with, or
// nil if it doesn't panic.
func stepLimitPanic(t *testing.T, f func()) (e *StepLimitError) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			if e, ok = r.(*StepLimitError); !ok {
				t.Fatalf("panicked with %v, want a *StepLimitError", r)
			}
		}
	}()
	f()
	return nil
}

// Nested repeats in a regex rule backtrack exponentially on a near miss:
// without a limit, 30 characters would take minutes.
var blowup = MustCompile(`grammar Blowup { regex TOP { [ a+ ]+ $ } }`)
var nearMiss = strings.Repeat("a", 30) + "b"

func TestStepLimitStopsBacktracking(t *testing.T) {
	withStepLimit(t, 100_000, func() {
		for name, f := range map[string]func(){
			"Parse":    func() { blowup.Parse(nearMiss) },
			"Subparse": func() { blowup.Subparse(nearMiss) },
			"Find":     func() { blowup.Find(nearMiss) },
			"FindAll":  func() { blowup.FindAll(nearMiss) },
			"Split":    func() { blowup.Split(nearMiss) },
		} {
			e := stepLimitPanic(t, f)
			if e == nil {
				t.Errorf("%s didn't stop", name)
				continue
			}
			if e.Grammar != "Blowup" || e.Limit != 100_000 {
				t.Errorf("%s: %+v", name, e)
			}
			if want := "grammar Blowup: gave up after 100000 steps (see StepLimit)"; e.Error() != want {
				t.Errorf("%s: %q, want %q", name, e.Error(), want)
			}
		}

		// The Err forms return it.
		for name, f := range map[string]func(string, ...Option) (*scalar.SV, error){
			"ParseErr":    blowup.ParseErr,
			"SubparseErr": blowup.SubparseErr,
		} {
			m, err := f(nearMiss)
			var e *StepLimitError
			if !errors.As(err, &e) || m.Defined() {
				t.Errorf("%s = %v, %v; want undef and a *StepLimitError", name, m, err)
			}
		}
	})
}

// Find, FindAll and Split share one count across the positions they try,
// so many short searches can't add up to a long one.
func TestStepLimitCountsWholeCall(t *testing.T) {
	g := MustCompile(`grammar Each { token TOP { [ a | c ] b } }`) // can start at every a
	text := strings.Repeat("a", 1000) + "b"
	withStepLimit(t, 500, func() {
		if stepLimitPanic(t, func() { g.Find(text) }) == nil {
			t.Error("Find over 1001 positions didn't run out of 500 steps")
		}
	})
	withStepLimit(t, 5000, func() {
		if got := g.Find(text).At("~").Str(); got != "ab" {
			t.Errorf("Find = %q, want ab", got)
		}
	})
}

// Zero means no limit; ordinary grammars come nowhere near the default.
func TestStepLimitOff(t *testing.T) {
	withStepLimit(t, 0, func() {
		// About 200,000 steps.
		if stepLimitPanic(t, func() { blowup.Parse(strings.Repeat("a", 16) + "b") }) != nil {
			t.Error("StepLimit 0 still limits")
		}
	})
	if StepLimit != 10_000_000 {
		t.Errorf("default StepLimit = %d", StepLimit)
	}
}

// A panic that isn't a step limit still panics from ParseErr.
func TestParseErrRepanics(t *testing.T) {
	g := New("G").Token("TOP", Call("missing"))
	defer func() {
		if r := recover(); r == nil {
			t.Error("ParseErr swallowed a panic")
		}
	}()
	g.ParseErr("x")
}
