// after Appendix B of http://webyrd.net/quines/quines.pdf
package main

var n0 = number(0)
var n1 = number(1)

var p1 = list(n1)

// fullAdderO truth table: right-hand sides are constant (numbers only),
// so we pre-compute them once at init time instead of on every call.
var (
	faRow0 = list(n0, n0, n0, n0, n0)
	faRow1 = list(n1, n0, n0, n1, n0)
	faRow2 = list(n0, n1, n0, n1, n0)
	faRow3 = list(n1, n1, n0, n0, n1)
	faRow4 = list(n0, n0, n1, n1, n0)
	faRow5 = list(n1, n0, n1, n0, n1)
	faRow6 = list(n0, n1, n1, n0, n1)
	faRow7 = list(n1, n1, n1, n1, n1)
)

func buildNum(n int) expression {
	if n < 0 {
		panic("only non-negative integers supported by buildNum")
	}
	if n == 0 {
		return emptylist
	}
	if n%2 == 0 {
		// n is even
		return pair(number(0), buildNum(n/2))
	}
	// n is odd
	return pair(number(1), buildNum((n-1)/2))
}

func parseNum(e expression) int {
	n := 0
	i := 1
	for e != emptylist {
		if e.kind != kindPair {
			panic("not a valid oleg numeral: expected list")
		}
		p := e.pair
		if p.car.kind != kindNumber {
			panic("not a valid oleg numeral: expected number")
		}
		n += int(p.car.ival) * i
		i += i
		e = p.cdr
	}
	return n
}

func zeroO(n expression) goal {
	return equalo(emptylist, n)
}

func posO(n expression) goal {
	return fresh2(func(a, d expression) goal {
		return equalo(pair(a, d), n)
	})
}

func gt1O(n expression) goal {
	return fresh3(func(a, ad, dd expression) goal {
		return equalo(pair(a, pair(ad, dd)), n)
	})
}

func fullAdderO(b, x, y, r, c expression) goal {
	// Build the LHS list once; reuse it for all 8 equalo goals instead of
	// allocating a fresh pairNode chain for each row.
	lhs := list(b, x, y, r, c)
	return disj_conc(
		equalo(lhs, faRow0),
		equalo(lhs, faRow1),
		equalo(lhs, faRow2),
		equalo(lhs, faRow3),
		equalo(lhs, faRow4),
		equalo(lhs, faRow5),
		equalo(lhs, faRow6),
		equalo(lhs, faRow7),
	)
}

func adderO(d, n, m, r expression) goal {
	return delay(func() goal {
		return disj_conc(
			conj_plus(equalo(n0, d), equalo(emptylist, m), equalo(n, r)),
			conj_plus(equalo(n0, d), equalo(emptylist, n), equalo(m, r), posO(m)),
			conj_plus(equalo(n1, d), equalo(emptylist, m), adderO(n0, n, p1, r)),
			conj_plus(equalo(n1, d), equalo(emptylist, n), posO(m), adderO(n0, p1, m, r)),
			conj_plus(equalo(p1, n), equalo(p1, m), fresh2(func(a, c expression) goal {
				return conj(equalo(list(a, c), r), fullAdderO(d, n1, n1, a, c))
			})),
			conj(equalo(p1, n), genAdderO(d, n, m, r)),
			conj_plus(equalo(p1, m), gt1O(n), gt1O(r), adderO(d, p1, n, r)),
			conj(gt1O(n), genAdderO(d, n, m, r)),
		)
	})
}

func genAdderO(d, n, m, r expression) goal {
	return fresh7(func(a, b, c, e, x, y, z expression) goal {
		return conj_plus(
			equalo(pair(a, x), n),
			equalo(pair(b, y), m), posO(y),
			equalo(pair(c, z), r), posO(z),
			fullAdderO(d, a, b, c, e),
			adderO(e, x, y, z),
		)
	})
}

func plusO(n, m, k expression) goal {
	return adderO(n0, n, m, k)
}

func minusO(n, m, k expression) goal {
	return plusO(m, k, n)
}
