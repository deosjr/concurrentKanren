package main

import (
	"fmt"
    "runtime"
)

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

func main() {
    out := runN(9, callfresh(func(x expression) goal {
				return disj(fives(x), disj(sixes(x), sevens(x)))
			}))

/*
	// actual heavy goal to benchmark concurrency with
	out := run(fresh3(func(q, x, y expression) goal {
		return conj(
			equalo(q, list(x, y)),
			plusO(x, y, buildNum(10000)),
		)
	}))
*/
	fmt.Println(len(out))

    b := make([]byte, 10000)
    runtime.Stack(b, true)
	fmt.Println(string(b))
}
