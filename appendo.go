package main

func appendo(x, y, xy expression) goal {
	return disj(
		conj(
			equalo(x, emptylist),
			equalo(y, xy)),
		fresh3(func(e, xs, xys expression) goal {
			return conj_plus(
				equalo(x, pair{e, xs}),
				equalo(xy, pair{e, xys}),
				appendo(xs, y, xys))
		}))
}

func reverso(x, y expression) goal {
	return disj(
		conj_plus(
			equalo(x, emptylist),
			equalo(y, emptylist)),
		fresh3(func(e, xs, ys expression) goal {
			return conj_plus(
				equalo(x, pair{e, xs}),
				reverso(xs, ys),
				appendo(ys, pair{e, emptylist}, y),
			)
		}))
}
