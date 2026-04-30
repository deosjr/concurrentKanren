package main

// disjAuto is a marker that the cmd/disjautogen tool rewrites at build
// time into either disj_par(...) or disj_plus(...) based on a static
// cost analysis of each branch.
//
// If you forget to run `go generate`, this fallback runs at runtime —
// it conservatively picks disj_plus, which is correct but won't
// extract OR-parallelism. The codegen output replaces the call entirely
// so this body is never reached after generation.
//
// See cmd/disjautogen/main.go for the rewriting rules. The Erlang
// equivalent is ../erlangKanren/disj_auto_pt.erl (parse transform).
func disjAuto(goals ...goal) goal {
	return disj_plus(goals...)
}
