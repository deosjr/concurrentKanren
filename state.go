package main

type state struct {
	sub *substitution
	vc  int
}

var emptystate = state{sub: nil, vc: 0}

func (s *substitution) get(v variable) (expression, bool) {
	return s.Lookup(v)
}

func (s *substitution) put(v variable, e expression) *substitution {
	return s.Insert(v, e)
}

func (s *substitution) walk(u expression) expression {
	for u.kind == kindVariable {
		e, ok := s.get(variable(u.ival))
		if !ok {
			return u
		}
		u = e
	}
	return u
}

func (s *substitution) walkstar(u expression) expression {
	v := s.walk(u)
	if v.kind != kindPair {
		return v
	}
	return pair(s.walkstar(v.pair.car), s.walkstar(v.pair.cdr))
}

func (s *substitution) extend(v variable, e expression) (*substitution, bool) {
	if s.occursCheck(v, e) {
		return nil, false
	}
	return s.put(v, e), true
}

func (s *substitution) unify(u, v expression) (*substitution, bool) {
	u0 := s.walk(u)
	v0 := s.walk(v)
	if u0 == v0 {
		return s, true
	}
	if u0.kind == kindVariable {
		return s.extend(variable(u0.ival), v0)
	}
	if v0.kind == kindVariable {
		return s.extend(variable(v0.ival), u0)
	}
	if u0.kind == kindPair && v0.kind == kindPair {
		s0, ok := s.unify(u0.pair.car, v0.pair.car)
		if !ok {
			return nil, false
		}
		s1, ok := s0.unify(u0.pair.cdr, v0.pair.cdr)
		if !ok {
			return nil, false
		}
		return s1, true
	}
	return nil, false
}

func (s *substitution) occursCheck(v variable, e expression) bool {
	e0 := s.walk(e)
	if e0.kind == kindVariable {
		return v == variable(e0.ival)
	}
	if e0.kind != kindPair {
		return false
	}
	return s.occursCheck(v, e0.pair.car) || s.occursCheck(v, e0.pair.cdr)
}
