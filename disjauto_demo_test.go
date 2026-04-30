package main

import (
	"testing"
)

// Sanity that the disjauto demo's rewritten functions still work.
func TestDisjAutoDemo(t *testing.T) {
	if got := run(cheapDisj()); len(got) != 4 {
		t.Errorf("cheapDisj: got %d answers, want 4: %v", len(got), got)
	}
	if got := runPar(heavyDisj(50)); len(got) == 0 {
		t.Errorf("heavyDisj: got 0 answers")
	}
	if got := run(mixedDisj()); len(got) != 2 {
		t.Errorf("mixedDisj: got %d answers, want 2: %v", len(got), got)
	}
}
