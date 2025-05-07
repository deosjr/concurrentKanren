package main

type goal func(state) stream

func equalo(u, v expression) goal {
	return func(st state) stream {
		s, ok := st.sub.unify(u, v)
		if ok {
			return stream{state{sub: s, vc: st.vc}}
		}
		return nil
	}
}

func callfresh(f func(x expression) goal) goal {
	return func(st state) stream {
		v := variable(st.vc)
		newstate := state{sub: st.sub, vc: st.vc + 1}
		return f(v)(newstate)
	}
}

func disj(g1, g2 goal) goal {
	return func(st state) stream {
		return mplus(g1(st), g2(st))
	}
}

func mplus(str1, str2 stream) stream {
	if len(str1) == 0 {
		return str2
	}
	st := str1[0]
	rem1 := str1[1:]
	if st.delayed != nil {
		return stream{state{delayed: func() stream {
			s1 := append(st.delayed(), rem1...)
			return mplus(str2, s1)
		}}}
	}
	return append([]state{st}, mplus(str2, rem1)...)
}

func conj(g1, g2 goal) goal {
	return func(st state) stream {
		return bind(g1(st), g2)
	}
}

func bind(str stream, g goal) stream {
	if len(str) == 0 {
		return nil
	}
	st := str[0]
	rem1 := str[1:]
	if st.delayed != nil {
		return stream{state{delayed: func() stream {
			s1 := append(st.delayed(), rem1...)
			return bind(s1, g)
		}}}
	}
	return mplus(g(st), bind(rem1, g))
}

func disj_plus(goals ...goal) goal {
	if len(goals) == 1 {
		return goals[0]
	}
	return disj(goals[0], disj_plus(goals[1:]...))
}

func conj_plus(goals ...goal) goal {
	if len(goals) == 1 {
		return goals[0]
	}
	return conj(goals[0], conj_plus(goals[1:]...))
}

func run(goals ...goal) []expression {
	g := conj_plus(goals...)
	out := mKreify(takeAll(g(emptystate)))
	return out
}

func runN(n int, goals ...goal) []expression {
	g := conj_plus(goals...)
	out := mKreify(takeN(n, g(emptystate)))
	return out
}

func mKreify(states []state) []expression {
	exprs := []expression{}
	for _, st := range states {
		exprs = append(exprs, st.sub.walkstar(variable(0)))
	}
	return exprs
}

// missing macros here. go:generate could be used perhaps
// for now we duplicate the implementation of callfresh

func fresh1(f func(expression) goal) goal {
	return func(st state) stream {
		x := variable(st.vc)
		newstate := state{sub: st.sub, vc: st.vc + 1}
		return f(x)(newstate)
	}
}

func fresh2(f func(expression, expression) goal) goal {
	return func(st state) stream {
		x := variable(st.vc)
		y := variable(st.vc + 1)
		newstate := state{sub: st.sub, vc: st.vc + 2}
		return f(x, y)(newstate)
	}
}

func fresh3(f func(expression, expression, expression) goal) goal {
	return func(st state) stream {
		x := variable(st.vc)
		y := variable(st.vc + 1)
		z := variable(st.vc + 2)
		newstate := state{sub: st.sub, vc: st.vc + 3}
		return f(x, y, z)(newstate)
	}
}

func fresh7(f func(expression, expression, expression, expression, expression, expression, expression) goal) goal {
	return func(st state) stream {
		x1 := variable(st.vc)
		x2 := variable(st.vc + 1)
		x3 := variable(st.vc + 2)
		x4 := variable(st.vc + 3)
		x5 := variable(st.vc + 4)
		x6 := variable(st.vc + 5)
		x7 := variable(st.vc + 6)
		newstate := state{sub: st.sub, vc: st.vc + 7}
		return f(x1, x2, x3, x4, x5, x6, x7)(newstate)
	}
}
