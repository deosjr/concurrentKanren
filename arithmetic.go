// after Appendix B of http://webyrd.net/quines/quines.pdf
package main

import "sync"

var n0 = number(0)
var n1 = number(1)

var p1 = list(n1)

// fullAdderO truth table: right-hand sides are constant (numbers only),
// so we pre-compute them once at init time instead of on every call.
var (
	faRow0 = list(n0, n0, n0, n0, n0)
	faRow1 = list(n1, n0, n0, n1, n0)
	faRow2 = list(n0, n1, n0, n1, n0)
	faRow3 = list(n1, n1, n0, n0, n1)
	faRow4 = list(n0, n0, n1, n1, n0)
	faRow5 = list(n1, n0, n1, n0, n1)
	faRow6 = list(n0, n1, n1, n0, n1)
	faRow7 = list(n1, n1, n1, n1, n1)
)

func buildNum(n int) expression {
	if n < 0 {
		panic("only non-negative integers supported by buildNum")
	}
	if n == 0 {
		return emptylist
	}
	if n%2 == 0 {
		// n is even
		return pair(number(0), buildNum(n/2))
	}
	// n is odd
	return pair(number(1), buildNum((n-1)/2))
}

func parseNum(e expression) int {
	n := 0
	i := 1
	for e != emptylist {
		if e.kind != kindPair {
			panic("not a valid oleg numeral: expected list")
		}
		p := e.pair
		if p.car.kind != kindNumber {
			panic("not a valid oleg numeral: expected number")
		}
		n += int(p.car.ival) * i
		i += i
		e = p.cdr
	}
	return n
}

func zeroO(n expression) goal {
	return equalo(emptylist, n)
}

type posOGoal struct{ n expression }

func posO(n expression) goal { return posOGoal{n} }

func (pg posOGoal) Apply(st state) stream {
	a := mkVar(variable(st.vc))
	d := mkVar(variable(st.vc + 1))
	return equalo(pair(a, d), pg.n).Apply(state{sub: st.sub, vc: st.vc + 2})
}

type gt1OGoal struct{ n expression }

func gt1O(n expression) goal { return gt1OGoal{n} }

func (gg gt1OGoal) Apply(st state) stream {
	a := mkVar(variable(st.vc))
	ad := mkVar(variable(st.vc + 1))
	dd := mkVar(variable(st.vc + 2))
	return equalo(pair(a, pair(ad, dd)), gg.n).Apply(state{sub: st.sub, vc: st.vc + 3})
}

func fullAdderO(b, x, y, r, c expression) goal {
	// Build the LHS list once; reuse it for all 8 equalo goals instead of
	// allocating a fresh pairNode chain for each row.
	lhs := list(b, x, y, r, c)
	return disj_conc(
		equalo(lhs, faRow0),
		equalo(lhs, faRow1),
		equalo(lhs, faRow2),
		equalo(lhs, faRow3),
		equalo(lhs, faRow4),
		equalo(lhs, faRow5),
		equalo(lhs, faRow6),
		equalo(lhs, faRow7),
	)
}

// adderOBodyGoal is the concrete goal struct for the body of adderO (case 5's
// inner fresh2). It stores the two captured expressions and inlines variable
// creation, eliminating the fresh2Goal boxing + closure allocation.
type adderOBodyGoal struct{ r, d expression }

func (fg adderOBodyGoal) Apply(st state) stream {
	a := mkVar(variable(st.vc))
	c := mkVar(variable(st.vc + 1))
	return conj(equalo(list(a, c), fg.r), fullAdderO(fg.d, n1, n1, a, c)).Apply(
		state{sub: st.sub, vc: st.vc + 2})
}

// adderOState is a two-phase pooled callback object for adderOGoal.Apply.
// It replaces the delay(func() goal{...}) closure with a pool-backed struct,
// eliminating the heap-allocation of the captured {d,n,m,r} closure on each call.
// outerReqFn sends a delay and arms innerReqFn; innerReqFn evaluates the body.
type adderOState struct {
	d, n, m, r expression
	str        stream
	st         state
	outerFn    reqFn
	innerFn    reqFn
}

var adderOStatePool = sync.Pool{New: func() any { return &adderOState{} }}

func (as *adderOState) outerReqFn(sender stream, done bool) {
	if done {
		as.str = 0
		as.d, as.n, as.m, as.r, as.st = expression{}, expression{}, expression{}, expression{}, state{}
		adderOStatePool.Put(as)
		return
	}
	sendDelay(as.str, sender)
	if as.innerFn == nil {
		as.innerFn = as.innerReqFn
	}
	registerRequest(as.str, as.innerFn)
}

func (as *adderOState) innerReqFn(sender stream, done bool) {
	str, d, n, m, r, st := as.str, as.d, as.n, as.m, as.r, as.st
	as.str = 0
	as.d, as.n, as.m, as.r, as.st = expression{}, expression{}, expression{}, expression{}, state{}
	adderOStatePool.Put(as)
	if done {
		return
	}
	fwd := disj_conc(
		conj_plus(equalo(n0, d), equalo(emptylist, m), equalo(n, r)),
		conj_plus(equalo(n0, d), equalo(emptylist, n), equalo(m, r), posO(m)),
		conj_plus(equalo(n1, d), equalo(emptylist, m), adderO(n0, n, p1, r)),
		conj_plus(equalo(n1, d), equalo(emptylist, n), posO(m), adderO(n0, p1, m, r)),
		conj_plus(equalo(p1, n), equalo(p1, m), adderOBodyGoal{r: r, d: d}),
		conj(equalo(p1, n), genAdderO(d, n, m, r)),
		conj_plus(equalo(p1, m), gt1O(n), gt1O(r), adderO(d, p1, n, r)),
		conj(gt1O(n), genAdderO(d, n, m, r)),
	).Apply(st)
	sendForward(str, sender, fwd)
}

type adderOGoal struct{ d, n, m, r expression }

func adderO(d, n, m, r expression) goal { return adderOGoal{d, n, m, r} }

func (ag adderOGoal) Apply(st state) stream {
	str := newStream()
	as := adderOStatePool.Get().(*adderOState)
	if as.outerFn == nil {
		as.outerFn = as.outerReqFn
	}
	as.str, as.d, as.n, as.m, as.r, as.st = str, ag.d, ag.n, ag.m, ag.r, st
	registerRequest(str, as.outerFn)
	return str
}

// genAdderOGoal is the concrete goal struct for genAdderO. It inlines the
// fresh7 variable creation, eliminating the fresh7Goal boxing + closure allocation.
type genAdderOGoal struct{ d, n, m, r expression }

func genAdderO(d, n, m, r expression) goal { return genAdderOGoal{d, n, m, r} }

func (gg genAdderOGoal) Apply(st state) stream {
	a := mkVar(variable(st.vc))
	b := mkVar(variable(st.vc + 1))
	c := mkVar(variable(st.vc + 2))
	e := mkVar(variable(st.vc + 3))
	x := mkVar(variable(st.vc + 4))
	y := mkVar(variable(st.vc + 5))
	z := mkVar(variable(st.vc + 6))
	return conj_plus(
		equalo(pair(a, x), gg.n),
		equalo(pair(b, y), gg.m), posO(y),
		equalo(pair(c, z), gg.r), posO(z),
		fullAdderO(gg.d, a, b, c, e),
		adderO(e, x, y, z),
	).Apply(state{sub: st.sub, vc: st.vc + 7})
}

func plusO(n, m, k expression) goal {
	return adderO(n0, n, m, k)
}

func minusO(n, m, k expression) goal {
	return plusO(m, k, n)
}
