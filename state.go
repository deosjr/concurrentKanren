package main

type state struct {
	sub     *substitution
	vc      int
	delayed bool // signals an immature stream if true
}

var emptystate = state{sub: nil, vc: 0}

func (s *substitution) get(v variable) (expression, bool) {
	return s.Lookup(v)
}

func (s *substitution) put(v variable, e expression) *substitution {
	return s.Insert(v, e)
}

func (s *substitution) walk(u expression) expression {
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

func (s *substitution) walkstar(u expression) expression {
	v := s.walk(u)
	switch t := v.(type) {
	case variable:
		return t
	case pair:
		return pair{car: s.walkstar(t.car), cdr: s.walkstar(t.cdr)}
	}
	return v
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

func (s *substitution) occursCheck(v variable, e expression) bool {
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
