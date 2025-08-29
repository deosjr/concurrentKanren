package main

import (
	"fmt"
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

func eights(x expression) goal {
	return disj(equalo(x, number(8)), delay(func() goal { return eights(x) }))
}

func testList(length int) expression {
	l := make([]expression, length)
	for i:=0; i<length; i++ {
		l[i] = number(i)
	}
	return list(l...)
}

func main() {
	/*
		out := run(fresh1(func(x expression) goal {
			return disj(equalo(x, number(5)), equalo(x, number(6)))
		}))
		fmt.Println(out)
	*/
	// actual heavy goal to benchmark concurrency with
	/*
		out := run(fresh3(func(q, x, y expression) goal {
			return conj(
				equalo(q, list(x, y)),
				plusO(x, y, buildNum(10000)),
			)
		}))
	*/
		out := run(fresh3(func(q, x, y expression) goal {
			return conj(
				equalo(q, list(x, y)),
				timesO(x, y, buildNum(6)),
			)
		}))
	fmt.Println(out)
	//fmt.Println(len(out))
}
