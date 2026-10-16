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

// Matching recurses as it goes, so a long enough match would overflow the
// stack, which kills the program; past DepthLimit, it stops with an error
// that can be recovered instead. The fuzzer found the first of these: an
// empty literal required two million times.
func TestDepthLimit(t *testing.T) {
	withStepLimit(t, 0, func() {
		for name, f := range map[string]func(){
			"empty repeat": func() { MustCompile(`grammar D1 { token TOP { '' ** 2111111 } }`).Parse("") },
			"long repeat":  func() { MustCompile(`grammar D2 { token TOP { a* } }`).Parse(strings.Repeat("a", 3_000_000)) },
			"recursion":    func() { MustCompile(`grammar D3 { token TOP { a <TOP>? } }`).Parse(strings.Repeat("a", 3_000_000)) },
			"heavy regex": func() {
				MustCompile(`grammar D4 { regex TOP { [ <x> [ <y> | <z> ] <.ws> ]* }; token x { a }; token y { b }; token z { c } }`).
					Parse(strings.Repeat("ab ", 1_000_000))
			},
		} {
			e := depthLimitPanic(t, f)
			if e == nil {
				t.Errorf("%s: no DepthLimitError", name)
				continue
			}
			if !strings.Contains(e.Error(), "too deep") {
				t.Errorf("%s: %v", name, e)
			}
		}
	})
	// The Err forms return it.
	_, err := MustCompile(`grammar D5 { token TOP { '' ** 2111111 } }`).ParseErr("")
	var e *DepthLimitError
	if !errors.As(err, &e) {
		t.Errorf("ParseErr = %v, want a *DepthLimitError", err)
	}
	// Ordinary depths are fine.
	if !MustCompile(`grammar D6 { token TOP { a* } }`).Parse(strings.Repeat("a", 50_000)).Defined() {
		t.Error("a* on 50,000 a's didn't match")
	}
}

func depthLimitPanic(t *testing.T, f func()) (e *DepthLimitError) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			if e, ok = r.(*DepthLimitError); !ok {
				panic(r)
			}
		}
	}()
	f()
	return nil
}
