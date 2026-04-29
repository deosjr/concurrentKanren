package main

func appendO(l1, l2, l3 expression) goal {
	return delay(func() goal {
		return disj(
			conj(equalo(l1, emptylist), equalo(l2, l3)),
			fresh3(func(a, d, res expression) goal {
				return conj_plus(
					equalo(l1, pair(a, d)),
					equalo(l3, pair(a, res)),
					appendO(d, l2, res),
				)
			}),
		)
	})
}

// nList builds (1 2 ... n) as a Go expression for use as a fixed appendo target.
func nList(n int) expression {
	out := emptylist
	for i := n; i >= 1; i-- {
		out = pair(number(i), out)
	}
	return out
}
