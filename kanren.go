package main

type goal interface {
	Apply(st state) stream
}

type equaloGoal struct {
	u, v expression
}

func equalo(u, v expression) goal {
	return equaloGoal{u: u, v: v}
}

func (e equaloGoal) Apply(st state) stream {
	str := newStream()
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
	return str
}

type callfreshGoal struct {
	f func(expression) goal
}

func callfresh(f func(x expression) goal) goal {
	return callfreshGoal{f}
}

func (cf callfreshGoal) Apply(st state) stream {
	v := variable(st.vc)
	newstate := state{sub: st.sub, vc: st.vc + 1}
	return cf.f(v).Apply(newstate)
}

type disjGoal struct {
	g1, g2 goal
}

func disj(g1, g2 goal) goal {
	return disjGoal{g1, g2}
}

func (d disjGoal) Apply(st state) stream {
	str := newStream()
	str1 := d.g1.Apply(st)
	str2 := d.g2.Apply(st)
	mplus(str, str1, str2)
	return str
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
}

type conjGoal struct {
	g1, g2 goal
}

func conj(g1, g2 goal) goal {
	return conjGoal{g1, g2}
}

func (c conjGoal) Apply(st state) stream {
	str := newStream()
	str1 := c.g1.Apply(st)
	bind(str, str1, c.g2)
	return str
}

func bind(str, str1 stream, g goal) {
	registerRequest(str, func(sender stream, done bool) {
		if done {
			request(str, str1, true) // close
			return
		}
		bind_(sender, str, str1, g)
	})
}

func bind_(sender, str, str1 stream, g goal) {
	request(str, str1, false)
	registerReceive(str, func(msg Message) {
		switch t := msg.(type) {
		case stateMessage:
			bstr := newStream()
			bind(bstr, str1, g)
			conjStr := g.Apply(t.st)
			mplus_(sender, str, conjStr, bstr)
		case stateCloseMessage:
			conjStr := g.Apply(t.st)
			sendForward(str, sender, conjStr)
		case closeMessage:
			sendClose(str, sender)
		case forwardMessage:
			bind_(sender, str, t.fwd, g)
		case forwardWithStateMessage:
			bstr := newStream()
			bind(bstr, t.fwd, g)
			conjStr := g.Apply(t.st)
			mplus_(sender, str, conjStr, bstr)
		case delayMessage:
			bind_(sender, str, str1, g)
		}
	})
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
	wg := startWorkers()
	g := conj_plus(goals...)
	stream := g.Apply(emptystate)
	out := mKreify(takeAll(stream))
	awaitWorkers(wg)
	return out
}

func runN(n int, goals ...goal) []expression {
	wg := startWorkers()
	g := conj_plus(goals...)
	stream := g.Apply(emptystate)
	out := mKreify(takeN(n, stream))
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

type fresh1Goal struct {
	f func(expression) goal
}

func fresh1(f func(x expression) goal) goal {
	return fresh1Goal{f}
}
func (f fresh1Goal) Apply(st state) stream {
	x := variable(st.vc)
	newstate := state{sub: st.sub, vc: st.vc + 1}
	return f.f(x).Apply(newstate)
}

type fresh2Goal struct {
	f func(expression, expression) goal
}

func fresh2(f func(x, y expression) goal) goal {
	return fresh2Goal{f}
}
func (f fresh2Goal) Apply(st state) stream {
	x := variable(st.vc)
	y := variable(st.vc + 1)
	newstate := state{sub: st.sub, vc: st.vc + 2}
	return f.f(x, y).Apply(newstate)
}

type fresh3Goal struct {
	f func(expression, expression, expression) goal
}

func fresh3(f func(x, y, z expression) goal) goal {
	return fresh3Goal{f}
}
func (f fresh3Goal) Apply(st state) stream {
	x := variable(st.vc)
	y := variable(st.vc + 1)
	z := variable(st.vc + 2)
	newstate := state{sub: st.sub, vc: st.vc + 3}
	return f.f(x, y, z).Apply(newstate)
}

type fresh7Goal struct {
	f func(expression, expression, expression, expression, expression, expression, expression) goal
}

func fresh7(f func(x, y, z, a, b, c, d expression) goal) goal {
	return fresh7Goal{f}
}
func (f fresh7Goal) Apply(st state) stream {
	x := variable(st.vc)
	y := variable(st.vc + 1)
	z := variable(st.vc + 2)
	a := variable(st.vc + 3)
	b := variable(st.vc + 4)
	c := variable(st.vc + 5)
	d := variable(st.vc + 6)
	newstate := state{sub: st.sub, vc: st.vc + 7}
	return f.f(x, y, z, a, b, c, d).Apply(newstate)
}
