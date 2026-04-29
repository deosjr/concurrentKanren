package main

import "fmt"

func nevero() goal {
	return delay(func() goal { return nevero() })
}

func fives(x expression) goal {
	return disj(equalo(x, number(5)), delay(func() goal { return fives(x) }))
}

func sixes(x expression) goal {
	return disj(equalo(x, number(6)), delay(func() goal { return sixes(x) }))
}

func sevens(x expression) goal {
	return disj(equalo(x, number(7)), delay(func() goal { return sevens(x) }))
}

func eights(x expression) goal {
	return disj(equalo(x, number(8)), delay(func() goal { return eights(x) }))
}

// fivesOrSevens produces an infinite stream of x=5 and x=7 (interleaved).
// Used together with sixesOrSevens to construct a "branchy × branchy"
// minimal repro: conj of two infinite-stream goals that share a single
// answer (x=7).
func fivesOrSevens(x expression) goal {
	return disj(
		equalo(x, number(5)),
		disj(equalo(x, number(7)), delay(func() goal { return fivesOrSevens(x) })),
	)
}

func sixesOrSevens(x expression) goal {
	return disj(
		equalo(x, number(6)),
		disj(equalo(x, number(7)), delay(func() goal { return sixesOrSevens(x) })),
	)
}

func main() {
	out := run(fresh3(func(q, x, y expression) goal {
		return conj(
			equalo(q, list(x, y)),
			plusO(x, y, buildNum(10000)),
		)
	}))
	fmt.Println(len(out))
}
