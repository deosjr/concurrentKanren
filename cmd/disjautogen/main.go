// disjautogen rewrites disjAuto(...) calls into either disj_par(...) or
// disj_plus(...) based on a compile-time cost analysis of each branch.
// Mirrors the Erlang parse transform in ../erlangKanren/disj_auto_pt.erl.
//
// Usage:
//
//	//go:generate go run ./cmd/disjautogen -in foo.go -inplace
//
// or run over a directory (rewrites every *.go that contains disjAuto):
//
//	go run ./cmd/disjautogen -dir . -inplace
//
// Decision rule (same as the Erlang version):
//   - Walk each argument expression in a disjAuto(...) call.
//   - A branch is "heavy" if it contains a call to any function NOT in
//     a small allowlist of structural builtins (equalo, conj, disj,
//     conj_plus, disj_plus, disj_par, disj_conc, callfresh, fresh1/2/3/7,
//     delay, list, pair, number).
//   - If at least 2 branches are heavy, rewrite to disj_par.
//   - Otherwise rewrite to disj_plus (sequential interleaving has
//     less overhead than spawning goroutines for trivial branches).
//
// Idempotent: after rewriting, the file no longer contains disjAuto
// calls, so re-running is a no-op. The disjAuto symbol still exists
// as a runtime fallback that picks disj_plus, in case someone forgets
// to run `go generate`.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

var structural = map[string]bool{
	"equalo":     true,
	"conj":       true,
	"disj":       true,
	"conj_plus":  true,
	"disj_plus":  true,
	"disj_par":   true,
	"disj_smart": true,
	"disj_conc":  true,
	"disjAuto":   true,
	"callfresh":  true,
	"fresh1":     true,
	"fresh2":     true,
	"fresh3":     true,
	"fresh7":     true,
	"delay":      true,
	"list":       true,
	"pair":       true,
	"number":     true,
	"emptylist":  true,
	"buildNum":   true,
	"numLit":     true,
	"symRef":     true,
	"quoteForm":  true,
	"lamForm":    true,
	"appForm":    true,
	"numVal":     true,
	"primVal":    true,
}

const heavyThreshold = 2

func main() {
	dir := flag.String("dir", "", "directory of .go files to rewrite (excluding *_test.go)")
	in := flag.String("in", "", "single input file (mutually exclusive with -dir)")
	inplace := flag.Bool("inplace", false, "overwrite the input file with the rewritten contents")
	flag.Parse()

	if *in != "" {
		processFile(*in, *inplace)
		return
	}
	if *dir == "" {
		flag.Usage()
		os.Exit(2)
	}
	matches, _ := filepath.Glob(filepath.Join(*dir, "*.go"))
	for _, m := range matches {
		base := filepath.Base(m)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		src, err := os.ReadFile(m)
		if err != nil || !strings.Contains(string(src), "disjAuto(") {
			continue
		}
		processFile(m, *inplace)
	}
}

func processFile(inPath string, inplace bool) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, inPath, nil, parser.ParseComments)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", inPath, err)
		os.Exit(1)
	}

	rewrites := 0
	ast.Inspect(file, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := ce.Fun.(*ast.Ident)
		if !ok || ident.Name != "disjAuto" {
			return true
		}
		heavy := 0
		for _, a := range ce.Args {
			if isHeavy(a) {
				heavy++
			}
		}
		if heavy >= heavyThreshold {
			ident.Name = "disj_par"
		} else {
			ident.Name = "disj_plus"
		}
		pos := fset.Position(ce.Pos())
		fmt.Fprintf(os.Stderr, "[disjautogen] %s:%d: %d branches (%d heavy) -> %s\n",
			pos.Filename, pos.Line, len(ce.Args), heavy, ident.Name)
		rewrites++
		return true
	})

	if rewrites == 0 {
		return
	}

	var w *os.File
	if inplace {
		w, err = os.Create(inPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create %s: %v\n", inPath, err)
			os.Exit(1)
		}
		defer w.Close()
	} else {
		w = os.Stdout
	}
	if err := printer.Fprint(w, fset, file); err != nil {
		fmt.Fprintf(os.Stderr, "print %s: %v\n", inPath, err)
		os.Exit(1)
	}
}

// isHeavy reports whether expr's AST contains a call to a function
// outside the structural allowlist.
func isHeavy(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := ce.Fun.(type) {
		case *ast.Ident:
			if !structural[fn.Name] {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			// Remote call (pkg.Func): assume heavy unless package is "math" or similar.
			found = true
			return false
		default:
			// Call through a function literal or variable — assume heavy.
			found = true
			return false
		}
		return true
	})
	return found
}
