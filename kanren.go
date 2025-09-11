package main

type goal func(state) stream

func equalo(u, v expression) goal {
	return func(st state) stream {
		str := newStream()
		go func() {
			s, ok := st.sub.unify(u, v)
			sender, more := str.getRequest()
			if !more {
				str.close()
				return
			}
			if ok {
				str.sendStateAndClose(sender, state{sub: s, vc: st.vc})
			} else {
				str.sendClose(sender)
			}
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
		go mplus(str, g1(st), g2(st))
		return str
	}
}

func mplus(str, str1, str2 stream) {
	sender, more := str.getRequest()
	if !more {
		str1.sendDone()
		str2.sendDone()
		str.close()
		return
	}
	mplus_(sender, str, str1, str2)
}

func mplus_(sender inbox, str, str1, str2 stream) {
	str1.request(str)
	rec, ok := str.receive()
	if !ok {
		panic("mplus tried to read from closed channel")
	}
	switch {
	case rec.isState():
		str.sendState(sender, rec.st)
		mplus(str, str2, str1)
	case rec.isStateAndClose():
		str.sendForwardWithState(sender, str2, rec.st)
	case rec.isClose():
		str.sendForward(sender, str2)
	case rec.isForward():
		mplus_(sender, str, rec.fwd, str2)
	case rec.isForwardWithState():
		str.sendState(sender, rec.st)
		mplus(str, str2, rec.fwd)
	case rec.isDelay():
		mplus_(sender, str, str2, str1)
	}
}

func conj(g1, g2 goal) goal {
	return func(st state) stream {
		str := newStream()
		go bind(str, g1(st), g2)
		return str
	}
}

func bind(str, str1 stream, g goal) {
	sender, more := str.getRequest()
	if !more {
		str1.sendDone()
		str.close()
		return
	}
	bind_(sender, str, str1, g)
}

func bind_(sender inbox, str, str1 stream, g goal) {
	str1.request(str)
	rec, ok := str.receive()
	if !ok {
		panic("bind tried to read from closed channel")
	}
	switch {
	case rec.isState():
		bstr := newStream()
		go bind(bstr, str1, g)
		mplus_(sender, str, g(rec.st), bstr)
	case rec.isStateAndClose():
		s := g(rec.st)
		str.sendForward(sender, s)
	case rec.isClose():
		str.sendClose(sender)
	case rec.isForward():
		bind_(sender, str, rec.fwd, g)
	case rec.isForwardWithState():
		bstr := newStream()
		go bind(bstr, rec.fwd, g)
		mplus_(sender, str, g(rec.st), bstr)
	case rec.isDelay():
		bind_(sender, str, str1, g)
	}
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

func fresh4(f func(expression, expression, expression, expression) goal) goal {
	return func(st state) stream {
		x := variable(st.vc)
		y := variable(st.vc + 1)
		z := variable(st.vc + 2)
		a := variable(st.vc + 3)
		newstate := state{sub: st.sub, vc: st.vc + 4}
		return f(x, y, z, a)(newstate)
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
