package main

import (
	"testing"
)

// listOfLen builds a relation: list has exactly N elements (each fresh).
func listOfLen(n int, l expression) goal {
	if n == 0 {
		return equalo(l, emptylist)
	}
	return fresh2(func(head, tail expression) goal {
		return conj(equalo(l, pair(head, tail)), listOfLen(n-1, tail))
	})
}

func TestAbsento(t *testing.T) {
	n5, n6 := number(5), number(6)
	for i, tt := range []struct {
		name string
		goal goal
		want int
	}{
		{"distinct atoms succeed", absentoO(n5, n6), 1},
		{"identical atoms fail", absentoO(n5, n5), 0},
		{"absent in flat list", absentoO(n5, list(number(1), number(2), number(3))), 1},
		{"present in flat list", absentoO(n5, list(number(1), n5, number(3))), 0},
		{"present nested", absentoO(n5, list(number(1), list(number(2), n5), number(3))), 0},
		{"unbound term: pending succeeds",
			callfresh(func(x expression) goal {
				return absentoO(n5, x)
			}), 1},
		{"unbound term, then bind to ground",
			callfresh(func(x expression) goal {
				return conj(absentoO(n5, x), equalo(x, n5))
			}), 0},
		{"unbound term, bound to non-ground",
			callfresh(func(x expression) goal {
				return conj(absentoO(n5, x), equalo(x, n6))
			}), 1},
		{"propagates through pair to var",
			callfresh(func(x expression) goal {
				return conj(absentoO(n5, list(number(1), x, number(3))), equalo(x, n5))
			}), 0},
		{"propagates through pair, var binds to safe",
			callfresh(func(x expression) goal {
				return conj(absentoO(n5, list(number(1), x, number(3))), equalo(x, n6))
			}), 1},
		{"propagates deep into nested fresh",
			fresh2(func(x, y expression) goal {
				return conj_plus(
					absentoO(n5, list(number(1), pair(x, y))),
					equalo(x, number(2)),
					equalo(y, list(n5)),
				)
			}), 0},
		{"bind first, absento second: catches retroactively",
			fresh2(func(x, y expression) goal {
				return conj_plus(
					equalo(x, list(n5, number(2))),
					absentoO(n5, x),
				)
			}), 0},
	} {
		got := run(tt.goal)
		if len(got) != tt.want {
			t.Errorf("%d) %q: got %d answers (%v), want %d", i, tt.name, len(got), got, tt.want)
		}
	}
}

// Real use case: synthesize all 3-element lists over the alphabet {1,2,3}
// that don't contain the value 3. Each slot can be 1, 2, or 3; absento
// must do real work to prune the 3-containing answers. The full search
// has 3^3 = 27 candidates; absento should leave 2^3 = 8.
func TestAbsentoListSynthesis(t *testing.T) {
	got := runN(20, callfresh(func(q expression) goal {
		return conj_plus(
			listOfLen(3, q),
			absentoO(number(3), q),
			fresh3(func(a, b, c expression) goal {
				return conj_plus(
					equalo(q, list(a, b, c)),
					disj_plus(equalo(a, number(1)), equalo(a, number(2)), equalo(a, number(3))),
					disj_plus(equalo(b, number(1)), equalo(b, number(2)), equalo(b, number(3))),
					disj_plus(equalo(c, number(1)), equalo(c, number(2)), equalo(c, number(3))),
				)
			}),
		)
	}))
	if len(got) != 8 {
		t.Errorf("got %d answers, want 8: %v", len(got), got)
	}
	for _, g := range got {
		// Ensure no 3 sneaked through.
		for e := g; e.kind == kindPair; e = e.pair.cdr {
			if e.pair.car.kind == kindNumber && e.pair.car.ival == 3 {
				t.Errorf("answer %v contains forbidden 3", g)
				break
			}
		}
	}
}
