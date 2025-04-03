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
	return disj_plus(
		conj_plus(equalo(n0, b), equalo(n0, x), equalo(n0, y), equalo(n0, r), equalo(n0, c)),
		conj_plus(equalo(n1, b), equalo(n0, x), equalo(n0, y), equalo(n1, r), equalo(n0, c)),
		conj_plus(equalo(n0, b), equalo(n1, x), equalo(n0, y), equalo(n1, r), equalo(n0, c)),
		conj_plus(equalo(n1, b), equalo(n1, x), equalo(n0, y), equalo(n0, r), equalo(n1, c)),
		conj_plus(equalo(n0, b), equalo(n0, x), equalo(n1, y), equalo(n1, r), equalo(n0, c)),
		conj_plus(equalo(n1, b), equalo(n0, x), equalo(n1, y), equalo(n0, r), equalo(n1, c)),
		conj_plus(equalo(n0, b), equalo(n1, x), equalo(n1, y), equalo(n0, r), equalo(n1, c)),
		conj_plus(equalo(n1, b), equalo(n1, x), equalo(n1, y), equalo(n1, r), equalo(n1, c)),
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
