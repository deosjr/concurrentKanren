package main

type goal func(state) stream

func equalo(u, v expression) goal {
	return func(st state) stream {
		str := newStream()
		go func() {
			s, ok := st.sub.unify(u, v)
			req := <-str.req
			if req.done {
				str.close()
				return
			}
			if ok {
				sendStateAndClose(req.onto, state{sub: s, vc: st.vc})
			} else {
				sendClose(req.onto)
			}
			str.close()
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
	req := <-str.req
	if req.done {
		sendDone(str1.req)
		sendDone(str2.req)
		str.close()
		return
	}
	mplus_(req.onto, str, str1, str2)
}

func mplus_(req chan stateMsg, str, str1, str2 stream) {
	str.request(str1)
	rec, ok := <-str.rec
	if !ok {
		panic("mplus tried to read from closed channel")
	}
	switch {
	case rec.isState():
		sendState(req, rec.st)
		mplus(str, str2, str1)
	case rec.isStateAndClose():
		sendForwardWithState(req, str2, rec.st)
		str.close()
	case rec.isClose():
		sendForward(req, str2)
		str.close()
	case rec.isForward():
		mplus_(req, str, rec.fwd, str2)
	case rec.isForwardWithState():
		sendState(req, rec.st)
		mplus(str, str2, rec.fwd)
	case rec.isDelay():
		mplus_(req, str, str2, str1)
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
	req := <-str.req
	if req.done {
		sendDone(str1.req)
		str.close()
		return
	}
	bind_(req.onto, str, str1, g)
}

func bind_(req chan stateMsg, str, str1 stream, g goal) {
	str.request(str1)
	rec, ok := <-str.rec
	if !ok {
		panic("bind tried to read from closed channel")
	}
	switch {
	case rec.isState():
		bstr := newStream()
		go bind(bstr, str1, g)
		mplus(str, g(rec.st), bstr)
	case rec.isStateAndClose():
		s := g(rec.st)
		sendForward(req, s)
		str.close()
	case rec.isClose():
		sendClose(req)
		str.close()
	case rec.isForward():
		bind_(req, str, rec.fwd, g)
	case rec.isForwardWithState():
		bstr := newStream()
		go bind(bstr, rec.fwd, g)
		mplus_(req, str, g(rec.st), bstr)
	case rec.isDelay():
		bind_(req, str, str1, g)
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
