package main

import (
	"testing"
)

func BenchmarkPlusO(b *testing.B) {
	for b.Loop() {
		run(fresh3(func(q, x, y expression) goal {
			return conj(
				equalo(q, list(x, y)),
				plusO(x, y, buildNum(10000)),
			)
		}))
	}
}

func BenchmarkPlusOSmall(b *testing.B) {
	for b.Loop() {
		run(fresh3(func(q, x, y expression) goal {
			return conj(
				equalo(q, list(x, y)),
				plusO(x, y, buildNum(100)),
			)
		}))
	}
}
