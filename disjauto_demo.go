//go:generate go run ./cmd/disjautogen -in disjauto_demo.go -inplace

package main

// cheapDisj: 4 equalo branches. No user-defined relation calls →
// codegen should pick disj_plus (parallel overhead would dominate).
func cheapDisj() goal {
	return callfresh(func(x expression) goal {
		return disj_plus(
			equalo(x, number(1)),
			equalo(x, number(2)),
			equalo(x, number(3)),
			equalo(x, number(4)),
		)
	})
}

// heavyDisj: 4 plusO branches. Each calls a user-defined relation →
// codegen should pick disj_par.
func heavyDisj(k int) goal {
	return fresh3(func(q, x, y expression) goal {
		return conj(
			equalo(q, list(x, y)),
			disj_par(
				plusO(x, y, buildNum(k)),
				plusO(x, y, buildNum(k+100)),
				plusO(x, y, buildNum(k+200)),
				plusO(x, y, buildNum(k+300)),
			),
		)
	})
}

// mixedDisj: only 1 heavy branch, 2 cheap. Below the threshold of 2
// heavy → codegen should pick disj_plus.
func mixedDisj() goal {
	return callfresh(func(q expression) goal {
		return disj_plus(
			equalo(q, number(1)),
			equalo(q, number(2)),
			conj(equalo(q, number(3)), plusO(number(0), number(0), buildNum(10))),
		)
	})
}
