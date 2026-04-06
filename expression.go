package main

import (
	"fmt"
	"strings"
)

// variable is the key type used for substitution lookups in the AVL tree.
// It is not an expression itself; use mkVar to create a variable expression.
type variable int

// exprKind is the type tag for the expression discriminated union.
type exprKind uint8

const (
	kindVariable exprKind = iota
	kindNumber
	kindSpecial
	kindPair
)

// expression is a 16-byte discriminated union — the same width as a Go interface,
// but without boxing overhead. Variables, numbers, and specials are stored inline
// in the ival field (zero heap allocations). Pairs store a pointer to their node;
// the node is allocated exactly once with no convT copy.
//
// Layout (64-bit): pair *pairNode (8) + ival int32 (4) + kind exprKind (1) + 3 pad = 16 bytes.
type expression struct {
	pair *pairNode // non-nil only for kindPair
	ival int32     // variable index, number value, or special constant
	kind exprKind
}

// pairNode holds the car and cdr of a cons cell. Same size as the old pair struct.
type pairNode struct {
	car, cdr expression
}

// emptylist is the empty-list sentinel.
var emptylist = expression{kind: kindSpecial, ival: 0}

// mkVar constructs a variable expression (no allocation).
func mkVar(v variable) expression {
	return expression{kind: kindVariable, ival: int32(v)}
}

// number constructs a number expression (no allocation).
func number(n int) expression {
	return expression{kind: kindNumber, ival: int32(n)}
}

// pair constructs a cons cell (one allocation for the pairNode, no convT copy).
func pair(car, cdr expression) expression {
	return expression{kind: kindPair, pair: &pairNode{car, cdr}}
}

// list constructs a proper list from the given elements.
func list(e ...expression) expression {
	if len(e) == 0 {
		return emptylist
	}
	return pair(e[0], list(e[1:]...))
}

func (e expression) String() string {
	return e.display()
}

func (e expression) display() string {
	switch e.kind {
	case kindVariable:
		return fmt.Sprintf("#%d", e.ival)
	case kindNumber:
		return fmt.Sprintf("%d", e.ival)
	case kindSpecial:
		if e.ival == 0 {
			return "()"
		}
		panic("unknown special")
	case kindPair:
		car := e.pair.car.display()
		if e.pair.cdr == emptylist {
			return "(" + car + ")"
		}
		if e.pair.cdr.kind != kindPair {
			panic("not a list")
		}
		s := displayRec(e.pair.cdr, nil)
		return "(" + car + " " + strings.Join(s, " ") + ")"
	}
	panic("unknown expression kind")
}

func displayRec(e expression, s []string) []string {
	s = append(s, e.pair.car.display())
	if e.pair.cdr == emptylist {
		return s
	}
	if e.pair.cdr.kind != kindPair {
		panic("not a list")
	}
	return displayRec(e.pair.cdr, s)
}
