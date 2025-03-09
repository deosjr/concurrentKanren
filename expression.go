package main

import (
	"fmt"
	"strings"
)

// lack of union types makes us invent things like this
type expression interface {
	display() string
}

type number int

type variable int

type special uint8

const (
	emptylist special = iota
)

type pair struct {
	car expression
	cdr expression
}

func list(e ...expression) expression {
	if len(e) == 0 {
		return emptylist
	}
	if len(e) == 1 {
		return pair{e[0], emptylist}
	}
	return pair{e[0], list(e[1:]...)}
}

// remainder is formatting logic

func (n number) String() string {
	return n.display()
}

func (n number) display() string {
	return fmt.Sprintf("%d", n)
}

func (v variable) String() string {
	return v.display()
}

func (v variable) display() string {
	return fmt.Sprintf("#%d", v)
}

func (s special) String() string {
	return s.display()
}

func (s special) display() string {
	switch s {
	case emptylist:
		return "()"
	default:
		panic("unknown special")
	}
}

func (p pair) String() string {
	return p.display()
}

func (p pair) display() string {
	car := p.car.display()
	if p.cdr == emptylist {
		return "(" + car + ")"
	}
	cdr, ok := p.cdr.(pair)
	if !ok {
		panic("not a list")
	}
	s := cdr.displayRec(nil)
	return "(" + car + " " + strings.Join(s, " ") + ")"
}

func (p pair) displayRec(s []string) []string {
	car := p.car.display()
	s = append(s, car)
	if p.cdr == emptylist {
		return s
	}
	cdr, ok := p.cdr.(pair)
	if !ok {
		panic("not a list")
	}
	return cdr.displayRec(s)
}
