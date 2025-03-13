package main

type goal func(state) stream

func equalo(u, v expression) goal {
	return func(st state) stream {
		str := newStream()
		go func() {
			s, ok := st.sub.unify(u, v)
			if !str.more() {
				close(*str.out)
				return
			}
			if ok {
				str.send(state{sub: s, vc: st.vc})
			}
			close(*str.out)
		}()
		return str
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
		str := newStream()
		go func() {
			mplus(str, g1(st), g2(st))

		}()
		return str
	}
}

func mplus(str, str1, str2 stream) {
	str1.request()
	if !str.more() {
		*str1.in <- reqMsg{done: true}
		*str2.in <- reqMsg{done: true}
		close(*str.out)
		return
	}
	st, ok := str1.receive()
	if !ok {
		str.request()
		link(str, str2)
		return
	}
	if st.delayed {
		str.request()
	} else {
		str.send(st)
	}
	mplus(str, str2, str1)
}

func conj(g1, g2 goal) goal {
	return func(st state) stream {
		str := newStream()
		go func() {
			bind(str, g1(st), g2)
		}()
		return str
	}
}

func bind(str, str1 stream, g goal) {
	str1.request()
	if !str.more() {
		*str1.in <- reqMsg{done: true}
		close(*str.out)
	}
	st, ok := str1.receive()
	if !ok {
		close(*str.out)
		return
	}
	str.request()
	if st.delayed {
		bind(str, str1, g)
		return
	}
	bstr := newStream()
	go func() {
		bind(bstr, str1, g)
	}()
	mplus(str, g(st), bstr)
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
