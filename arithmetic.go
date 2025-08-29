// after Appendix B of http://webyrd.net/quines/quines.pdf
package main

const n0 = number(0)
const n1 = number(1)

var p1 = list(n1)

func buildNum(n int) expression {
	if n < 0 {
		panic("only non-negative integers supported by buildNum")
	}
	if n == 0 {
		return emptylist
	}
	if n%2 == 0 {
		// n is even
		return pair{
			car: number(0),
			cdr: buildNum(n / 2),
		}
	}
	// n is odd
	return pair{
		car: number(1),
		cdr: buildNum((n - 1) / 2),
	}
}

func parseNum(e expression) int {
	n := 0
	i := 1
	for e != emptylist {
		p, ok := e.(pair)
		if !ok {
			panic("not a valid oleg numeral: expected list")
		}
		x, ok := p.car.(number)
		if !ok {
			panic("not a valid oleg numeral: expected number")
		}
		n += int(x) * i
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
		return equalo(pair{a, d}, n)
	})
}

func gt1O(n expression) goal {
	return fresh3(func(a, ad, dd expression) goal {
		return equalo(pair{a, pair{ad, dd}}, n)
	})
}

func fullAdderO(b, x, y, r, c expression) goal {
	return disj_conc(
		equalo(list(b, x, y, r, c), list(n0, n0, n0, n0, n0)),
		equalo(list(b, x, y, r, c), list(n1, n0, n0, n1, n0)),
		equalo(list(b, x, y, r, c), list(n0, n1, n0, n1, n0)),
		equalo(list(b, x, y, r, c), list(n1, n1, n0, n0, n1)),
		equalo(list(b, x, y, r, c), list(n0, n0, n1, n1, n0)),
		equalo(list(b, x, y, r, c), list(n1, n0, n1, n0, n1)),
		equalo(list(b, x, y, r, c), list(n0, n1, n1, n0, n1)),
		equalo(list(b, x, y, r, c), list(n1, n1, n1, n1, n1)),
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
			equalo(pair{a, x}, n),
			equalo(pair{b, y}, m), posO(y),
			equalo(pair{c, z}, r), posO(z),
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

func timesO(n, m, p expression) goal {
	return delay(func() goal {
		return disj_conc(
			conj(equalo(emptylist, n), equalo(emptylist, p)),
			conj_plus(posO(n), equalo(emptylist, m), equalo(emptylist, p)),
			conj_plus(equalo(p1, n), posO(m), equalo(m, p)),
			conj_plus(gt1O(n), equalo(p1, m), equalo(n, p)),
			fresh2(func(x, z expression) goal {
				return conj_plus(
					equalo(pair{n0, x}, n), posO(x),
					equalo(pair{n0, z}, p), posO(z),
					gt1O(m),
					timesO(x, m, z))
			}),
			fresh2(func(x, y expression) goal {
				return conj_plus(
					equalo(pair{n1, x}, n), posO(x),
					equalo(pair{n0, y}, m), posO(y),
					timesO(m, n, p))
			}),
			fresh2(func(x, y expression) goal {
				return conj_plus(
					equalo(pair{n1, x}, n), posO(x),
					equalo(pair{n1, y}, m), posO(y),
					oddTimesO(x, n, m, p))
			}),
		)
	})
}

func oddTimesO(x, n, m, p expression) goal {
	return fresh1(func(q expression) goal {
		return conj_plus(
			boundTimesO(q, p, n, m),
			timesO(x, m, q),
			plusO(pair{n0, q}, m, p))
	})
}

func boundTimesO(q, p, n, m expression) goal {
	return disj(
		conj(equalo(emptylist, q), posO(p)),
		fresh7(func(a0, a1, a2, a3, x, y, z expression) goal {
			return conj_plus(
				equalo(pair{a0, x}, q),
				equalo(pair{a1, y}, p),
				disj(
					conj_plus(
						equalo(emptylist, n),
						equalo(pair{a2, z}, m),
						boundTimesO(x, y, z, emptylist)),
					conj(equalo(pair{a3, z}, n), boundTimesO(x, y, z, m))))
		}),
	)
}
