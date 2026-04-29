package main

import (
	"testing"
	"time"
)

// User-defined symbol ids start at userSymBase. We use idX, idY for params.
const (
	idX = userSymBase + 0
	idY = userSymBase + 1
)

// runEval evaluates expr in the initial env and returns the (one) value.
func runEval(t *testing.T, expr expression) expression {
	t.Helper()
	got := run(callfresh(func(q expression) goal {
		return evalO(expr, initialEnv(), q)
	}))
	if len(got) != 1 {
		t.Fatalf("eval %v: got %d answers (%v), want 1", expr, len(got), got)
	}
	return got[0]
}

func TestEvaloForwardNumber(t *testing.T) {
	got := runEval(t, numLit(5))
	if got.String() != numLit(5).String() {
		t.Errorf("got %v, want %v", got, numLit(5))
	}
}

func TestEvaloForwardLambda(t *testing.T) {
	// (lambda (x) x) → closure
	expr := lamForm(idX, symRef(idX))
	want := list(tagClosureVal, number(idX), symRef(idX), initialEnv())
	got := runEval(t, expr)
	if got.String() != want.String() {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEvaloForwardIdentityApp(t *testing.T) {
	// ((lambda (x) x) 5) → 5
	expr := appForm(lamForm(idX, symRef(idX)), numLit(5))
	got := runEval(t, expr)
	if got.String() != numLit(5).String() {
		t.Errorf("got %v, want %v", got, numLit(5))
	}
}

func TestEvaloForwardCons(t *testing.T) {
	// (cons 1 2) → (1 . 2)
	expr := appForm(symRef(primIdCons), numLit(1), numLit(2))
	want := pair(numLit(1), numLit(2))
	got := runEval(t, expr)
	if got.String() != want.String() {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEvaloForwardCar(t *testing.T) {
	// (car (cons 1 2)) → 1
	expr := appForm(symRef(primIdCar),
		appForm(symRef(primIdCons), numLit(1), numLit(2)))
	got := runEval(t, expr)
	if got.String() != numLit(1).String() {
		t.Errorf("got %v, want %v", got, numLit(1))
	}
}

func TestEvaloForwardCdr(t *testing.T) {
	// (cdr (cons 1 2)) → 2
	expr := appForm(symRef(primIdCdr),
		appForm(symRef(primIdCons), numLit(1), numLit(2)))
	got := runEval(t, expr)
	if got.String() != numLit(2).String() {
		t.Errorf("got %v, want %v", got, numLit(2))
	}
}

func TestEvaloForwardQuote(t *testing.T) {
	// (quote (1 2 3)) → (1 2 3)
	val := list(numLit(1), numLit(2), numLit(3))
	expr := quoteForm(val)
	got := runEval(t, expr)
	if got.String() != val.String() {
		t.Errorf("got %v, want %v", got, val)
	}
}

func TestEvaloForwardConsList(t *testing.T) {
	// (cons 1 (cons 2 (quote ()))) → (1 2)
	expr := appForm(symRef(primIdCons), numLit(1),
		appForm(symRef(primIdCons), numLit(2), quoteForm(emptylist)))
	want := pair(numLit(1), pair(numLit(2), emptylist))
	got := runEval(t, expr)
	if got.String() != want.String() {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestEvaloForwardAbsentoBlocksFakeClosure verifies absento prevents
// `(quote (closure-tag ...))` from forging a closure. Without absento, this
// would succeed and yield the fake closure as a value. With absento active,
// caseQuote's absentoO check fails on the closure-tag head and the entire
// evaluation has zero answers. This is the canonical reason absento exists.
func TestEvaloForwardAbsentoBlocksFakeClosure(t *testing.T) {
	fakeClosure := list(tagClosureVal, number(idX), numLit(5), emptylist)
	expr := quoteForm(fakeClosure)
	got := run(callfresh(func(q expression) goal {
		return evalO(expr, initialEnv(), q)
	}))
	if len(got) != 0 {
		t.Errorf("expected 0 answers (absento should block fake closure quote), got %d: %v", len(got), got)
	}
}

// TestEvaloForwardLookupShadowing verifies lookupO's =/= guard is genuinely
// required: with a manually shadowed env (two entries for the same key),
// lookup must return only the first binding. Without =/= the recursion
// would proceed past the matching first entry and ALSO match the shadowed
// second entry, yielding two answers.
func TestEvaloForwardLookupShadowing(t *testing.T) {
	dupEnv := list(
		pair(number(idX), numLit(1)),
		pair(number(idX), numLit(2)),
	)
	expr := symRef(idX)
	got := run(callfresh(func(q expression) goal {
		return evalO(expr, dupEnv, q)
	}))
	want := numLit(1).String()
	if len(got) != 1 || got[0].String() != want {
		t.Errorf("expected single answer %v (shadowing), got %v", want, got)
	}
}

// TestEvaloSynth: tractable synthesis cases — find the trivial program(s)
// that evaluate to a target. K=1 returns the literal/quote answer; K=2 also
// returns the second trivial answer when the target is a numeric value.
func TestEvaloSynth(t *testing.T) {
	t.Run("number target K=1: literal", func(t *testing.T) {
		got := runN(1, callfresh(func(q expression) goal {
			return evalO(q, initialEnv(), numVal(5))
		}))
		want := numLit(5).String()
		if len(got) != 1 || got[0].String() != want {
			t.Errorf("got %v, want [%v]", got, want)
		}
	})
	t.Run("number target K=2: literal + quote", func(t *testing.T) {
		got := runN(2, callfresh(func(q expression) goal {
			return evalO(q, initialEnv(), numVal(5))
		}))
		if len(got) != 2 {
			t.Fatalf("got %d answers, want 2: %v", len(got), got)
		}
	})
	t.Run("list target K=1: quote", func(t *testing.T) {
		target := list(numLit(1), numLit(2))
		got := runN(1, callfresh(func(q expression) goal {
			return evalO(q, initialEnv(), target)
		}))
		want := quoteForm(target).String()
		if len(got) != 1 || got[0].String() != want {
			t.Errorf("got %v, want [%v]", got, want)
		}
	})
}

// TestEvaloSynthHighK probes higher K to verify that the search scales.
// Pre-delay-propagation this hung at K=4; with bind+mplus delay propagation
// in place, K=20 completes in milliseconds.
func TestEvaloSynthHighK(t *testing.T) {
	for _, k := range []int{1, 2, 3, 5, 10, 20} {
		k := k
		t.Run("K_"+map[int]string{1: "1", 2: "2", 3: "3", 5: "5", 10: "10", 20: "20"}[k], func(t *testing.T) {
			start := time.Now()
			done := make(chan []expression, 1)
			go func() {
				done <- runN(k, callfresh(func(q expression) goal {
					return evalO(q, initialEnv(), numVal(5))
				}))
			}()
			select {
			case got := <-done:
				if len(got) != k {
					t.Errorf("K=%d: got %d answers, want %d", k, len(got), k)
				}
				t.Logf("K=%d in %v", k, time.Since(start))
			case <-time.After(5 * time.Second):
				t.Errorf("K=%d: timeout 5s", k)
			}
		})
	}
}
