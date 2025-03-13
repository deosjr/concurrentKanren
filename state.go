package main

import (
	"slices"
)

type state struct {
	sub     substitution
	vc      int
	delayed bool        // signals an immature stream if true
	link    *chan state // if not nil, signals chan link
}

var emptystate = state{sub: nil, vc: 0}

type substitution []expression

func (s substitution) get(v variable) (expression, bool) {
	key := int(v)
	if key >= len(s) {
		return v, false
	}
	e := s[key]
	return e, e != nil
}

func (s substitution) put(v variable, e expression) substitution {
	var news substitution
	key := int(v)
	if len(s) <= key {
		news = slices.Concat(s, make(substitution, key-len(s)+1))
	} else {
		news = slices.Clone(s)
	}
	news[key] = e
	return news
}

func (s substitution) walk(u expression) expression {
	uvar, ok := u.(variable)
	if !ok {
		return u
	}
	e, ok := s.get(uvar)
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

// TODO: immutable data structure with reuse, such as an AVL tree or HAMT
func (s substitution) extend(v variable, e expression) (substitution, bool) {
	if s.occursCheck(v, e) {
		return nil, false
	}
	return s.put(v, e), true
}

func (s substitution) unify(u, v expression) (substitution, bool) {
	u0 := s.walk(u)
	v0 := s.walk(v)
	if u0 == v0 {
		return s, true
	}
	uvar, uok := u0.(variable)
	if uok {
		return s.extend(uvar, v0)
	}
	vvar, vok := v0.(variable)
	if vok {
		return s.extend(vvar, u0)
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

func (s substitution) occursCheck(v variable, e expression) bool {
	e0 := s.walk(e)
	if evar, ok := e0.(variable); ok {
		return v == evar
	}
	epair, ok := e0.(pair)
	if !ok {
		return false
	}
	return s.occursCheck(v, epair.car) || s.occursCheck(v, epair.cdr)
}
