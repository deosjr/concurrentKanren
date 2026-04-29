package main

import (
	"reflect"
	"testing"
)

// nameLookupO is the canonical name-based env lookup. With =/= guarding
// the recursive case, the relation returns only the *first* (most recent)
// binding for x — proper shadowing semantics. Without =/= the recursive
// branch would also fire on the matching entry, returning every shadowed
// binding. This is the smallest realistic use of disequality.
func nameLookupO(x, env, val expression) goal {
	return delay(func() goal {
		return fresh3(func(y, b, rest expression) goal {
			return conj(
				equalo(env, pair(pair(y, b), rest)),
				disj(
					conj(equalo(x, y), equalo(val, b)),
					conj(neqo(x, y), nameLookupO(x, rest, val)),
				),
			)
		})
	})
}

func TestNameLookupShadowing(t *testing.T) {
	a, b := number(1), number(2)
	v1, v2 := number(10), number(20)

	// env = ((a . v1) (b . v2) (a . v2))  -- a is shadowed
	env := list(pair(a, v1), pair(b, v2), pair(a, v2))

	// Looking up `a` must yield v1 only, not v2.
	got := run(callfresh(func(q expression) goal {
		return nameLookupO(a, env, q)
	}))
	if !reflect.DeepEqual(got, []expression{v1}) {
		t.Errorf("lookup a: got %v, want [%v]", got, v1)
	}

	// Reverse query: which key maps to v1? Only a, not b — and a appears once
	// (the shadowed binding is invisible by =/= guard).
	got = run(callfresh(func(q expression) goal {
		return nameLookupO(q, env, v1)
	}))
	if !reflect.DeepEqual(got, []expression{a}) {
		t.Errorf("reverse v1: got %v, want [%v]", got, a)
	}

	// Reverse query for v2: only b — the shadowed (a . v2) is invisible.
	got = run(callfresh(func(q expression) goal {
		return nameLookupO(q, env, v2)
	}))
	if !reflect.DeepEqual(got, []expression{b}) {
		t.Errorf("reverse v2: got %v, want [%v] (shadowing leak if got [%v %v])", got, b, b, a)
	}
}

func TestDisequality(t *testing.T) {
	n5, n6 := number(5), number(6)
	for i, tt := range []struct {
		name string
		goal goal
		want int // expected number of answers
	}{
		{"diff atoms succeed", neqo(n5, n6), 1},
		{"same atoms fail", neqo(n5, n5), 0},
		{"diseq then equal fails",
			callfresh(func(x expression) goal {
				return conj(neqo(x, n5), equalo(x, n5))
			}), 0},
		{"equal then diseq fails",
			callfresh(func(x expression) goal {
				return conj(equalo(x, n5), neqo(x, n5))
			}), 0},
		{"two vars tied to same value",
			fresh2(func(x, y expression) goal {
				return conj_plus(neqo(x, y), equalo(x, n5), equalo(y, n5))
			}), 0},
		{"two vars tied to different values",
			fresh2(func(x, y expression) goal {
				return conj_plus(neqo(x, y), equalo(x, n5), equalo(y, n6))
			}), 1},
		{"unbound diseq survives",
			callfresh(func(x expression) goal {
				return neqo(x, n5)
			}), 1},
		{"pair diseq entailed by binding",
			callfresh(func(x expression) goal {
				return conj(neqo(pair(number(1), x), pair(number(1), number(2))), equalo(x, number(2)))
			}), 0},
		{"pair diseq simplifies and survives",
			fresh2(func(x, y expression) goal {
				return conj(
					neqo(pair(x, number(1)), pair(number(2), y)),
					equalo(x, number(2)),
				)
			}), 1},
		{"pair diseq fully entailed after both bindings",
			fresh2(func(x, y expression) goal {
				return conj_plus(
					neqo(pair(x, number(1)), pair(number(2), y)),
					equalo(x, number(2)),
					equalo(y, number(1)),
				)
			}), 0},
	} {
		got := run(tt.goal)
		if len(got) != tt.want {
			t.Errorf("%d) %q: got %d answers (%v), want %d", i, tt.name, len(got), got, tt.want)
		}
	}
}
