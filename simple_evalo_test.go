package main

import (
	"testing"
	"time"
)

// Identity closure value: (closure-tag idX (1 idX) ()) — equivalent to
// what (lambda (x) x) evaluates to in the empty env.
func simpleIdentityClosure() expression {
	return list(tagClosureVal, number(idX), symRef(idX), emptylist)
}

// Forward sanity: (lambda (x) x) evaluates to identity closure.
func TestSimpleEvalOForwardLambda(t *testing.T) {
	expr := lamForm(idX, symRef(idX))
	got := run(callfresh(func(q expression) goal {
		return simpleEvalO(expr, emptylist, q)
	}))
	want := simpleIdentityClosure()
	if len(got) != 1 || got[0].String() != want.String() {
		t.Errorf("got %v, want [%v]", got, want)
	}
}

// Forward: ((lambda (x) x) (lambda (y) y)) evaluates to identity closure
// (the inner lambda evaluated to its own closure with empty env).
func TestSimpleEvalOForwardSelfApp(t *testing.T) {
	id := lamForm(idX, symRef(idX))
	expr := simpleAppForm(id, lamForm(idY, symRef(idY)))
	got := run(callfresh(func(q expression) goal {
		return simpleEvalO(expr, emptylist, q)
	}))
	wantStr := list(tagClosureVal, number(idY), symRef(idY), emptylist).String()
	if len(got) != 1 || got[0].String() != wantStr {
		t.Errorf("got %v, want [%v]", got, wantStr)
	}
}

// Synthesis at K up to 10. Targets: identity closure.
func TestSimpleEvalOSynth(t *testing.T) {
	for _, k := range []int{1, 2, 3, 5, 10} {
		k := k
		t.Run("K_"+map[int]string{1: "01", 2: "02", 3: "03", 5: "05", 10: "10"}[k], func(t *testing.T) {
			start := time.Now()
			done := make(chan []expression, 1)
			go func() {
				done <- runN(k, callfresh(func(q expression) goal {
					return simpleEvalO(q, emptylist, simpleIdentityClosure())
				}))
			}()
			select {
			case got := <-done:
				t.Logf("K=%d: %d answers in %v", k, len(got), time.Since(start))
				for i, a := range got {
					t.Logf("  [%d] %v", i+1, a)
				}
			case <-time.After(1 * time.Second):
				t.Errorf("K=%d: timeout 1s", k)
			}
		})
	}
}
