package main

type goal interface {
	Init(str stream, st state)
}

type equaloGoal struct {
	u, v expression
}

func equalo(u, v expression) goal {
	return equaloGoal{u: u, v: v}
}

func (e equaloGoal) Init(str stream, st state) {
	s, ok := st.sub.unify(e.u, e.v)
	registerRequest(str, func(sender stream, done bool) {
		if done {
			return
		}
		if ok {
			sendStateAndClose(str, sender, state{sub: s, vc: st.vc})
		} else {
			sendClose(str, sender)
		}
	})
}

type callfreshGoal struct {
	f func(expression) goal
}

func callfresh(f func(x expression) goal) goal {
	return callfreshGoal{f}
}

func (cf callfreshGoal) Init(str stream, st state) {
	v := variable(st.vc)
	newstate := state{sub: st.sub, vc: st.vc + 1}
	cf.f(v).Init(str, newstate)
}

type disjGoal struct {
	g1, g2 goal
}

func disj(g1, g2 goal) goal {
	return disjGoal{g1, g2}
}

func (d disjGoal) Init(str stream, st state) {
	str1 := registerInit(d.g1, st)
	str2 := registerInit(d.g2, st)
	mplus(str, str1, str2)
}

func mplus(str, str1, str2 stream) {
	registerRequest(str, func(sender stream, done bool) {
		if done {
			request(str, str1, true) // close
			request(str, str2, true) // close
			return
		}
		mplus_(sender, str, str1, str2)
	})
}

func mplus_(sender, str, str1, str2 stream) {
	request(str, str1, false)
	registerReceive(str, func(msg Message) {
		switch t := msg.(type) {
		case stateMessage:
			sendState(str, sender, t.st)
			mplus(str, str2, str1)
		case stateCloseMessage:
			sendForwardWithState(str, sender, str2, t.st)
		case closeMessage:
			sendForward(str, sender, str2)
		case forwardMessage:
			mplus_(sender, str, t.fwd, str2)
		case forwardWithStateMessage:
			sendState(str, sender, t.st)
			mplus(str, str2, t.fwd)
		case delayMessage:
			mplus_(sender, str, str2, str1)
		}
	})

	/*
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
	*/
}

/*
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
		mplus_(req, str, g(rec.st), bstr)
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
*/

func disj_plus(goals ...goal) goal {
	if len(goals) == 1 {
		return goals[0]
	}
	return disj(goals[0], disj_plus(goals[1:]...))
}

/*
func conj_plus(goals ...goal) goal {
	if len(goals) == 1 {
		return goals[0]
	}
	return conj(goals[0], conj_plus(goals[1:]...))
}
*/

func run(goals ...goal) []expression {
	//g := conj_plus(goals...)
	g := goals[0]
	stream := registerInit(g, emptystate)
	wg := startWorkers()
	out := mKreify(takeAll(stream))
	awaitWorkers(wg)
	return out
}

func runN(n int, goals ...goal) []expression {
	wg := startWorkers()
	//g := conj_plus(goals...)
	g := goals[0]
	out := mKreify(takeN(n, registerInit(g, emptystate)))
	awaitWorkers(wg)
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

/*
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
*/
