package main

import (
	"reflect"
	"testing"
)

func TestKanren(t *testing.T) {
	n5, n6, n7, n8 := number(5), number(6), number(7), number(8)
	for i, tt := range []struct {
		goal goal
		take int
		want []expression
	}{
		{
			goal: callfresh(func(q expression) goal {
				return equalo(q, n5)
			}),
			want: []expression{n5},
		},
		{
			goal: callfresh(func(q expression) goal {
				return equalo(q, n5)
			}),
			take: 3,
			want: []expression{n5},
		},
		{
			goal: callfresh(func(q expression) goal {
				return disj(equalo(q, n5), equalo(q, n6))
			}),
			want: []expression{n5, n6},
		},
		{
			goal: callfresh(func(x expression) goal {
				return fives(x)
			}),
			take: 3,
			want: []expression{n5, n5, n5},
		},
		{
			goal: callfresh(func(x expression) goal {
				return disj(fives(x), disj(sixes(x), sevens(x)))
			}),
			take: 9,
			// Right-leaning binary disj is unfair under mplus delay propagation:
			// deeper branches take more swap hops to surface. Use disj_conc for
			// even round-robin fairness across N branches.
			want: []expression{n5, n6, n7, n5, n5, n6, n7, n5, n5},
		},
		{
			goal: callfresh(func(x expression) goal {
				return disj_plus(fives(x), sixes(x), sevens(x))
			}),
			take: 9,
			// Same right-leaning unfairness as the previous case.
			want: []expression{n5, n6, n7, n5, n5, n6, n7, n5, n5},
		},
		{
			goal: callfresh(func(x expression) goal {
				return disj_plus(fives(x), sixes(x), sevens(x), eights(x))
			}),
			take: 9,
			// Right-leaning unfairness compounds with depth — leftmost branch
			// (fives) dominates as the tree gets deeper.
			want: []expression{n5, n6, n7, n5, n8, n5, n6, n5, n5},
		},
		{
			goal: callfresh(func(x expression) goal {
				return disj_conc(fives(x), sixes(x), sevens(x))
			}),
			take: 9,
			want: []expression{n5, n6, n7, n5, n6, n7, n5, n6, n7},
		},
		{
			goal: callfresh(func(x expression) goal {
				return disj_conc(nevero(), fives(x), sixes(x), sevens(x))
			}),
			take: 9,
			want: []expression{n5, n6, n7, n5, n6, n7, n5, n6, n7},
		},
		{
			goal: fresh2(func(x, y expression) goal {
				return conj(equalo(x, n5), equalo(y, n6))
			}),
			want: []expression{n5},
		},
		{
			goal: fresh3(func(q, x, y expression) goal {
				return conj(
					equalo(q, pair(x, y)),
					conj(equalo(x, n5), equalo(y, n6)),
				)
			}),
			want: []expression{pair(n5, n6)},
		},
		{
			goal: equalo(n5, n6),
			want: []expression{},
		},
		{
			goal: conj_sce(equalo(n5, n6), nevero()),
			want: []expression{},
		},
		{
			goal: conj_sce(nevero(), equalo(n5, n6)),
			want: []expression{},
		},
		{
			goal: fresh2(func(x, y expression) goal {
				return conj_sce(equalo(y, n5), equalo(x, y))
			}),
			want: []expression{n5},
		},
		{
			goal: callfresh(func(x expression) goal {
				return equalo(x, pair(number(1), x))
			}),
			want: []expression{},
		},
		// "Branchy × branchy" — same var in both: passes (downstream
		// fails fast on x bound by upstream), so this isn't the right
		// repro of the evalO synthesis hang.
		{
			goal: callfresh(func(x expression) goal {
				return conj(fivesOrSevens(x), sixesOrSevens(x))
			}),
			take: 1,
			want: []expression{n7},
		},
		// Closer repro: bind chain of three branchy infinite-stream goals
		// over THREE FRESH VARS. K=1 finds answer fast.
		{
			goal: callfresh(func(q expression) goal {
				return fresh3(func(x, y, z expression) goal {
					return conj_plus(
						equalo(q, list(x, y, z)),
						fivesOrSevens(x),
						sixesOrSevens(y),
						fivesOrSevens(z),
					)
				})
			}),
			take: 1,
			want: []expression{list(n5, n6, n5)},
		},
		// Outer disj_plus K=3 with recursive sub-goals wrapped in delay,
		// matching evalO case 5's structure exactly: each recursive call
		// wrapped in delay(func() goal { ... }).
		{
			goal: callfresh(func(q expression) goal {
				return disj_plus(
					equalo(q, n5),
					equalo(q, n6),
					delay(func() goal {
						return fresh3(func(x, y, z expression) goal {
							return conj_plus(
								equalo(q, list(x, y, z)),
								fivesOrSevens(x),
								sixesOrSevens(y),
								fivesOrSevens(z),
							)
						})
					}),
				)
			}),
			take: 3,
			want: []expression{n5, n6, list(n5, n6, n5)},
		},
		// Same repro at K=4: probes whether the minimal pattern explodes
		// at K=4 like evalO does.
		{
			goal: callfresh(func(q expression) goal {
				return disj_plus(
					equalo(q, n5),
					equalo(q, n6),
					delay(func() goal {
						return fresh3(func(x, y, z expression) goal {
							return conj_plus(
								equalo(q, list(x, y, z)),
								fivesOrSevens(x),
								sixesOrSevens(y),
								fivesOrSevens(z),
							)
						})
					}),
				)
			}),
			take: 4,
			want: []expression{n5, n6, list(n5, n6, n5), list(n7, n6, n5)},
		},
	} {
		var got []expression
		if tt.take == 0 {
			got = run(tt.goal)
		} else {
			got = runN(tt.take, tt.goal)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%d) got %v want %v", i, got, tt.want)
		}
	}
}
