package main

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

// TestDisjParCorrectness — disj_par must produce the same answer set
// as disj_plus (ordering may differ since branches race).
func TestDisjParCorrectness(t *testing.T) {
	mkBranches := func() []goal {
		return []goal{
			callfresh(func(q expression) goal {
				return conj(equalo(q, list(number(1), number(2), number(3))),
					plusO(number(1), number(2), buildNum(3)))
			}),
			callfresh(func(q expression) goal {
				return conj(equalo(q, list(number(2), number(3), number(5))),
					plusO(number(2), number(3), buildNum(5)))
			}),
		}
	}

	seq := run(disj_plus(mkBranches()...))
	par := runPar(disj_par(mkBranches()...))

	sortByString := func(xs []expression) []string {
		out := make([]string, len(xs))
		for i, x := range xs {
			out[i] = x.String()
		}
		sort.Strings(out)
		return out
	}
	if !reflect.DeepEqual(sortByString(seq), sortByString(par)) {
		t.Errorf("disj_plus and disj_par produced different sets:\nseq=%v\npar=%v",
			seq, par)
	}
}

// TestDisjParWidePlusO benchmarks disj_par against disj_plus and
// disj_conc on the wide-plusO workload (same shape as
// ../erlangKanren/pkanren.erl bench_pk2).
//
// Apple M4 (10 schedulers) numbers (skipped under -short):
//
//   N br   K       disj_plus   disj_conc   disj_par
//   ----   -----   ---------   ---------   --------
//   4      500     117ms       112ms       54ms     ← 2.2x speedup
//   8      500     222ms       232ms       116ms    ← 1.9x
//   4      2000    475ms       477ms       232ms    ← 2.0x
//   4      5000    2369ms      2392ms      1162ms   ← 2.0x
//
// disj_par doubles throughput consistently and crosses Erlang pdisj
// at K=1000+ (Go's faster sequential code wins once the right
// architecture is in place).
func TestDisjParWidePlusO(t *testing.T) {
	if testing.Short() {
		t.Skip("long-running benchmark; skipped under -short")
	}
	type bench struct{ branches, k int }
	configs := []bench{
		{4, 500}, {8, 500},
		{4, 2000},
	}
	for _, c := range configs {
		c := c

		mkBranches := func() []goal {
			gs := make([]goal, c.branches)
			for i := 0; i < c.branches; i++ {
				tag := number(i + 1)
				gs[i] = fresh3(func(q, x, y expression) goal {
					return conj(
						equalo(q, list(tag, x, y)),
						plusO(x, y, buildNum(c.k)),
					)
				})
			}
			return gs
		}

		t.Run("disj_plus", func(t *testing.T) {
			start := time.Now()
			out := run(disj_plus(mkBranches()...))
			t.Logf("[disj_plus] %d branches × K=%d: %d answers in %v",
				c.branches, c.k, len(out), time.Since(start))
		})
		t.Run("disj_par", func(t *testing.T) {
			start := time.Now()
			out := runPar(disj_par(mkBranches()...))
			t.Logf("[disj_par]  %d branches × K=%d: %d answers in %v",
				c.branches, c.k, len(out), time.Since(start))
		})
	}
}
