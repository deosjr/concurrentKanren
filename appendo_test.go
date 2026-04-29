package main

import (
	"testing"
)

func TestAppendO(t *testing.T) {
	target := nList(3)
	got := run(fresh3(func(q, x, y expression) goal {
		return conj(
			equalo(q, list(x, y)),
			appendO(x, y, target),
		)
	}))
	if len(got) != 4 {
		t.Errorf("appendo |splits of (1 2 3)|: got %d, want 4", len(got))
	}
}

func benchAppendO(b *testing.B, n int) {
	target := nList(n)
	for b.Loop() {
		run(fresh3(func(q, x, y expression) goal {
			return conj(
				equalo(q, list(x, y)),
				appendO(x, y, target),
			)
		}))
	}
}

func BenchmarkAppendO100(b *testing.B)    { benchAppendO(b, 100) }
func BenchmarkAppendO1000(b *testing.B)   { benchAppendO(b, 1000) }
func BenchmarkAppendO10000(b *testing.B)  { benchAppendO(b, 10000) }

func benchAppendOSync(b *testing.B, n int) {
	synchronous = true
	defer func() { synchronous = false }()
	benchAppendO(b, n)
}

func BenchmarkAppendOSync100(b *testing.B)   { benchAppendOSync(b, 100) }
func BenchmarkAppendOSync1000(b *testing.B)  { benchAppendOSync(b, 1000) }
func BenchmarkAppendOSync10000(b *testing.B) { benchAppendOSync(b, 10000) }
