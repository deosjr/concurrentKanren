package main

import (
	"maps"
)

type state struct {
	sub     substitution
	vc      int
	delayed func() // signals an immature stream
}

var emptystate = state{
	sub: substitution{},
	vc:  0,
}

type substitution map[variable]expression

func (s substitution) walk(u expression) expression {
	uvar, ok := u.(variable)
	if !ok {
		return u
	}
	e, ok := s[uvar]
	if !ok {
		return u
	}
	return s.walk(e)
}

func (s substitution) walkstar(u expression) expression {
	v := s.walk(u)
	switch t := v.(type) {
	case variable:
		return t
	case pair:
		return pair{car: s.walkstar(t.car), cdr: s.walkstar(t.cdr)}
	}
	return v
}

// TODO: immutable maps
func (s substitution) extend(v variable, e expression) substitution {
	m := maps.Clone(s)
	m[v] = e
	return m
}

func (s substitution) unify(u, v expression) (substitution, bool) {
	u0 := s.walk(u)
	v0 := s.walk(v)
	if u0 == v0 {
		return s, true
	}
	uvar, uok := u0.(variable)
	vvar, vok := v0.(variable)
	if uok && vok && uvar == vvar {
		return s, true
	}
	if uok {
		return s.extend(uvar, v0), true
	}
	if vok {
		return s.extend(vvar, u0), true
	}
	upair, uok := u0.(pair)
	vpair, vok := v0.(pair)
	if uok && vok {
		s0, ok := s.unify(upair.car, vpair.car)
		if !ok {
			return nil, false
		}
		s1, ok := s0.unify(upair.cdr, vpair.cdr)
		if !ok {
			return nil, false
		}
		return s1, true
	}
	return nil, false
}
