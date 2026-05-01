package main

import (
	"testing"
	"time"
)

// TestEvaloDisjSmartCorrectness — modeDisjSmart must produce the
// same number of answers as the default modeDisjPlus on a few
// representative K values. Budget=6 fits evalO's 6 cases exactly
// (one top-level fan-out, recursive disjs fall back).
func TestEvaloDisjSmartCorrectness(t *testing.T) {
	for _, k := range []int{1, 5, 50} {
		evalOMode = modeDisjPlus
		seq := runN(k, callfresh(func(q expression) goal {
			return evalO(q, initialEnv(), numVal(5))
		}))
		evalOMode = modeDisjSmart
		SetDisjSmartBudget(6)
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
// Budget counts goroutines via all-or-nothing claim of N slots for
// an N-way disj. For evalo's 6-way disj at every level:
//   budget=6  → top-level fans out, recursion serializes (sweet spot)
//   budget=12 → top + first recursive fan-out, mild thrash
//   budget=48 → many recursive fan-outs, heavy thrash
//
// Apple M4, 10 cores, isolated process per measurement. Wall-clock:
//
//   K       Go disj_plus   Go disj_smart(b=6)   Chez conde   Chez smart(b=6)
//   ----    ------------   ------------------   ----------   ---------------
//   100     20.2ms         5.5ms      (3.7x)    3.4ms        1.4ms     (2.4x)
//   1000    358ms          140ms      (2.6x)    136ms        76ms      (1.8x)
//   5000    3.15s          1.53s      (2.1x)    1.93s        1.14s     (1.7x)
//   10000   9.80s          4.61s      (2.1x)    5.07s        3.34s     (1.5x)
//   20000   26.3s          14.2s      (1.9x)    16.8s        11.5s     (1.5x)
//
// disj_smart gives ~2x consistent speedup over disj_plus in Go and
// ~1.5-2.4x in Chez. Across runtimes: Chez sequential beats Go
// disj_plus throughout — Chez has faster per-op machinery (var-val
// mutation, scope-based commits). Adding parallelism on top of either
// runtime keeps that runtime ahead — Chez disj_smart is the fastest
// implementation across all K, ahead of Go disj_smart by 1.2-1.6x.
//
// Earlier (before the thread-counting budget fix) we observed Go
// disj_smart catching Chez conde at K~5000+; with corrected
// semantics that still holds, but Chez disj_smart pulls back ahead.
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
			SetDisjSmartBudget(6)
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
			t.Logf("[disj_smart b=6]  K=%d: %v", k, time.Since(start))
		})
	}
}
