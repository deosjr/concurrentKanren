package main

import (
	"testing"
)

func benchPlusO(b *testing.B, n int) {
	for b.Loop() {
		run(fresh3(func(q, x, y expression) goal {
			return conj(
				equalo(q, list(x, y)),
				plusO(x, y, buildNum(n)),
			)
		}))
	}
}

func BenchmarkPlusO100(b *testing.B)    { benchPlusO(b, 100) }
func BenchmarkPlusO1000(b *testing.B)   { benchPlusO(b, 1000) }
func BenchmarkPlusO10000(b *testing.B)  { benchPlusO(b, 10000) }
func BenchmarkPlusO100000(b *testing.B) { benchPlusO(b, 100000) }
