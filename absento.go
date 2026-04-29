package main

// absento: a constraint that `ground` must not occur as a sub-term of `term`.
// In faster-miniKanren absento is generalized to allow non-ground term1, but
// the canonical use case (and our only need for evalo) is a ground atom such
// as the 'closure or 'prim tag. We keep the impl general — non-ground ground
// stays in pending state until enough variables bind to decide.
//
// Algorithm: at each check, walk both sides through the substitution.
//   - If `ground == term` under the sub with no new bindings: violated (occurrence).
//   - If `ground == term` would require new bindings: pending.
//   - Else if term is a pair, recurse into car and cdr.
//   - Else (term atomic and != ground): satisfied.
type absento struct {
	ground expression
	term   expression
}

// checkAbsento tests whether ground occurs inside term under sub.
// Returns the (possibly simplified) constraint and its status.
func checkAbsento(sub *substitution, a absento) (absento, diseqStatus) {
	g := sub.walk(a.ground)
	t := sub.walk(a.term)

	var added []diseqBinding
	_, _, eq := sub.unifyTrack(g, t, &added)
	if eq && len(added) == 0 {
		// g equals t exactly under current sub: an occurrence at this level.
		return absento{}, diseqViolated
	}
	if eq {
		// Equality possible only with further bindings: keep pending.
		return absento{ground: g, term: t}, diseqPending
	}

	// g cannot equal t at this level. Recurse into pair components.
	if t.kind == kindPair {
		_, statusCar := checkAbsento(sub, absento{ground: g, term: t.pair.car})
		if statusCar == diseqViolated {
			return absento{}, diseqViolated
		}
		_, statusCdr := checkAbsento(sub, absento{ground: g, term: t.pair.cdr})
		if statusCdr == diseqViolated {
			return absento{}, diseqViolated
		}
		if statusCar == diseqSatisfied && statusCdr == diseqSatisfied {
			return absento{}, diseqSatisfied
		}
		// At least one half pending: keep the original constraint pending.
		return absento{ground: g, term: t}, diseqPending
	}

	// t is atomic and not equal to g: this level is fine and there are no
	// more sub-terms to check.
	return absento{}, diseqSatisfied
}

// checkAllAbsento renormalizes every absento constraint against newSub.
// Returns the new list of pending constraints, or false if any is violated.
func checkAllAbsento(sub *substitution, abs []absento) ([]absento, bool) {
	if len(abs) == 0 {
		return abs, true
	}
	out := make([]absento, 0, len(abs))
	for _, a := range abs {
		simplified, status := checkAbsento(sub, a)
		switch status {
		case diseqViolated:
			return nil, false
		case diseqSatisfied:
			continue
		default:
			out = append(out, simplified)
		}
	}
	return out, true
}

type absentoGoal struct {
	ground, term expression
}

// absentoO is the (absento ground term) goal: ground must not appear as a
// sub-term of term under any extension of the substitution.
func absentoO(ground, term expression) goal {
	return absentoGoal{ground: ground, term: term}
}

func (a absentoGoal) Apply(st state) stream {
	str := newStream()
	simplified, status := checkAbsento(st.sub, absento{ground: a.ground, term: a.term})

	var newSt state
	var success bool
	switch status {
	case diseqViolated:
		success = false
	case diseqSatisfied:
		newSt, success = st, true
	default:
		newAbs := make([]absento, len(st.abs), len(st.abs)+1)
		copy(newAbs, st.abs)
		newAbs = append(newAbs, simplified)
		newSt = state{sub: st.sub, vc: st.vc, diseq: st.diseq, abs: newAbs}
		success = true
	}

	registerRequest(str, func(sender stream, done bool) {
		if done {
			return
		}
		if success {
			sendStateAndClose(str, sender, newSt)
		} else {
			sendClose(str, sender)
		}
	})
	return str
}
