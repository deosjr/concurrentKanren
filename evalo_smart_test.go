package main

import (
	"testing"
	"time"
)

// TestEvaloDisjSmartCorrectness — modeDisjSmart with budget=1 must
// produce the same number of answers as the default modeDisjPlus on a
// few representative K values. (Answer-set equality would be cleaner
// but the runtime's String() doesn't handle cons-produced dotted
// pairs; count + completion is enough as a smoke check.)
func TestEvaloDisjSmartCorrectness(t *testing.T) {
	for _, k := range []int{1, 5, 50} {
		evalOMode = modeDisjPlus
		seq := runN(k, callfresh(func(q expression) goal {
			return evalO(q, initialEnv(), numVal(5))
		}))
		evalOMode = modeDisjSmart
		SetDisjSmartBudget(1)
		par := runNPar(k, callfresh(func(q expression) goal {
			return evalO(q, initialEnv(), numVal(5))
		}))
		evalOMode = modeDisjPlus
		SetDisjSmartBudget(10)

		if len(seq) != k || len(par) != k {
			t.Errorf("K=%d: disj_plus produced %d, disj_smart produced %d", k, len(seq), len(par))
		}
	}
}

// TestEvaloDisjSmartBench — performance comparison vs disj_plus on
// evalO synthesis. Skipped under -short.
//
// Apple M4 (10 schedulers), target=(0 5):
//
//   K       Chez       disj_plus    disj_smart(b=1)  speedup vs plus  vs Chez
//   ----    -----      ----------   ---------------  ---------------  ---------------
//   100     3.34ms     14.9ms       3.14ms (b=2)     4.7x             tied
//   1000    111ms      336ms        158ms            2.1x             0.70x (Chez ahead)
//   5000    1.69s      3.08s        1.43s            2.2x             1.18x (Go ahead)
//   10000   5.01s      9.88s        4.28s            2.3x             1.17x (Go ahead)
//   20000   15.7s      26.1s        13.1s            2.0x             1.20x (Go ahead)
//
// The runtime budget mechanism is the key insight: top-level evalO
// claims a parallelism slot, recursive calls find it consumed and
// fall back to disj_plus. No goroutine explosion, no deadlock,
// natural granularity control.
//
// Crossover with Chez moved from K~50000 (with plain disj_plus) down
// to between K=1000 and K=5000 — Go is faster than Chez across the
// entire interesting range of synthesis workloads.
func TestEvaloDisjSmartBench(t *testing.T) {
	if testing.Short() {
		t.Skip("long-running benchmark; skipped under -short")
	}
	for _, k := range []int{100, 1000, 5000} {
		k := k
		t.Run("disj_plus", func(t *testing.T) {
			evalOMode = modeDisjPlus
			start := time.Now()
			got := runN(k, callfresh(func(q expression) goal {
				return evalO(q, initialEnv(), numVal(5))
			}))
			if len(got) != k {
				t.Errorf("K=%d: got %d answers, want %d", k, len(got), k)
			}
			t.Logf("[disj_plus]       K=%d: %v", k, time.Since(start))
		})
		t.Run("disj_smart", func(t *testing.T) {
			evalOMode = modeDisjSmart
			SetDisjSmartBudget(1)
			defer func() {
				evalOMode = modeDisjPlus
				SetDisjSmartBudget(10)
			}()
			start := time.Now()
			got := runNPar(k, callfresh(func(q expression) goal {
				return evalO(q, initialEnv(), numVal(5))
			}))
			if len(got) != k {
				t.Errorf("K=%d: got %d answers, want %d", k, len(got), k)
			}
			t.Logf("[disj_smart b=1]  K=%d: %v", k, time.Since(start))
		})
	}
}
