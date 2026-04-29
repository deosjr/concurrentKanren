package main

// disequality (=/=) and the constraint-store machinery that supports it.
//
// A (=/= u v) constraint is stored as the set of bindings whose simultaneous
// entailment by the substitution would make u = v. The constraint:
//
//   - is satisfied (drop) if any binding is contradicted by the substitution,
//   - is violated (fail) if every binding is already entailed,
//   - otherwise simplifies to the still-pending bindings.
//
// After every successful unification that extends the substitution, every
// stored disequality is walked through the new substitution to update its
// status.

type diseqStatus int

const (
	diseqPending diseqStatus = iota
	diseqSatisfied
	diseqViolated
)

// normalizeDiseq walks d's bindings through sub. Returns the simplified
// disequality (only the bindings still needed to entail it) along with its
// status after this round of substitution extension.
func normalizeDiseq(sub *substitution, d diseq) (diseq, diseqStatus) {
	s := sub
	var added []diseqBinding
	for _, b := range d.bindings {
		var stepAdded []diseqBinding
		s2, _, ok := s.unifyTrack(mkVar(b.v), b.e, &stepAdded)
		if !ok {
			return diseq{}, diseqSatisfied
		}
		added = append(added, stepAdded...)
		s = s2
	}
	if len(added) == 0 {
		return diseq{}, diseqViolated
	}
	return diseq{bindings: added}, diseqPending
}

// extendStateAndCheck builds a new state with newSub as its substitution and
// renormalizes every stored constraint. Returns ok=false if any disequality
// or absento is violated by newSub. Cheap fast-path when no constraints exist.
func extendStateAndCheck(st state, newSub *substitution) (state, bool) {
	if len(st.diseq) == 0 && len(st.abs) == 0 {
		return state{sub: newSub, vc: st.vc}, true
	}
	var newDiseqs []diseq
	if len(st.diseq) > 0 {
		newDiseqs = make([]diseq, 0, len(st.diseq))
		for _, d := range st.diseq {
			simplified, status := normalizeDiseq(newSub, d)
			switch status {
			case diseqViolated:
				return state{}, false
			case diseqSatisfied:
				continue
			default:
				newDiseqs = append(newDiseqs, simplified)
			}
		}
	}
	newAbs, ok := checkAllAbsento(newSub, st.abs)
	if !ok {
		return state{}, false
	}
	return state{sub: newSub, vc: st.vc, diseq: newDiseqs, abs: newAbs}, true
}

type neqGoal struct {
	u, v expression
}

// neqo is the (=/= u v) goal: u and v must remain distinct under any
// extension of the substitution.
func neqo(u, v expression) goal {
	return neqGoal{u: u, v: v}
}

func (n neqGoal) Apply(st state) stream {
	str := newStream()
	var added []diseqBinding
	_, _, ok := st.sub.unifyTrack(n.u, n.v, &added)

	var newSt state
	var success bool
	switch {
	case !ok:
		// u and v cannot be unified given the current substitution: they are
		// already distinct, so the constraint trivially holds.
		newSt, success = st, true
	case len(added) == 0:
		// u and v are already equal under the current substitution: the
		// constraint is immediately violated.
		success = false
	default:
		// Store the bindings that would entail u = v as a new disequality.
		newDiseqs := make([]diseq, len(st.diseq), len(st.diseq)+1)
		copy(newDiseqs, st.diseq)
		newDiseqs = append(newDiseqs, diseq{bindings: added})
		newSt = state{sub: st.sub, vc: st.vc, diseq: newDiseqs, abs: st.abs}
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
