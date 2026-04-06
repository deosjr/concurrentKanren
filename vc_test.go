package main

import (
	"fmt"
	"testing"
)

func TestVC(t *testing.T) {
	for _, n := range []int{100, 1000, 10000} {
		cancel := startWorkers()
		maxvc := 0
		stream := fresh3(func(q, x, y expression) goal {
			return conj(
				equalo(q, list(x, y)),
				plusO(x, y, buildNum(n)),
			)
		}).Apply(emptystate)
		states := takeAll(stream)
		for _, st := range states {
			if st.vc > maxvc {
				maxvc = st.vc
			}
		}
		cancel()
		fmt.Printf("plusO(x,y,%d): %d solutions, max vc=%d, avg copy≈%d bytes\n",
			n, len(states), maxvc, maxvc*16)
	}
}
