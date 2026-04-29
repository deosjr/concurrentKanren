package main

type state struct {
	sub   *substitution
	vc    int
	diseq []diseq    // disequality constraints; nil/empty when none exist
	abs   []absento  // absento constraints; nil/empty when none exist
}

var emptystate = state{}

// diseq represents one (=/= u v) constraint as the set of bindings whose
// simultaneous entailment would make u = v. The constraint is violated iff
// every binding in `bindings` is already entailed by the current substitution.
type diseq struct {
	bindings []diseqBinding
}

type diseqBinding struct {
	v variable
	e expression
}

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
	s2, _, ok := s.unifyTrack(u, v, nil)
	return s2, ok
}

// unifyTrack is like unify, but if `added` is non-nil, every binding
// installed during this unification is appended to it. Used by =/= solving
// to discover which bindings would be needed to entail u = v.
func (s *substitution) unifyTrack(u, v expression, added *[]diseqBinding) (*substitution, *[]diseqBinding, bool) {
	u0 := s.walk(u)
	v0 := s.walk(v)
	if u0 == v0 {
		return s, added, true
	}
	if u0.kind == kindVariable {
		return s.extendTrack(variable(u0.ival), v0, added)
	}
	if v0.kind == kindVariable {
		return s.extendTrack(variable(v0.ival), u0, added)
	}
	if u0.kind == kindPair && v0.kind == kindPair {
		s0, added, ok := s.unifyTrack(u0.pair.car, v0.pair.car, added)
		if !ok {
			return nil, nil, false
		}
		s1, added, ok := s0.unifyTrack(u0.pair.cdr, v0.pair.cdr, added)
		if !ok {
			return nil, nil, false
		}
		return s1, added, true
	}
	return nil, nil, false
}

func (s *substitution) extendTrack(v variable, e expression, added *[]diseqBinding) (*substitution, *[]diseqBinding, bool) {
	if s.occursCheck(v, e) {
		return nil, nil, false
	}
	if added != nil {
		*added = append(*added, diseqBinding{v: v, e: e})
	}
	return s.put(v, e), added, true
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
