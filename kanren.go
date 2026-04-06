package main

import "sync"

type goal interface {
	Apply(st state) stream
}

type equaloGoal struct {
	u, v expression
}

func equalo(u, v expression) goal {
	return equaloGoal{u: u, v: v}
}

// equaloReqState holds the registerRequest callback state for equaloGoal.Apply.
// Pooling avoids a ~40-byte closure allocation per equalo application; the
// method value ers.reqFn is only 16 bytes (funcval code ptr + ers pointer).
type equaloReqState struct {
	str stream
	s   *substitution
	vc  int
	ok  bool
}

var equaloReqPool = sync.Pool{New: func() any { return &equaloReqState{} }}

func (ers *equaloReqState) reqFn(sender stream, done bool) {
	str, s, vc, ok := ers.str, ers.s, ers.vc, ers.ok
	ers.str, ers.s, ers.vc, ers.ok = 0, nil, 0, false
	equaloReqPool.Put(ers)
	if done {
		return
	}
	if ok {
		sendStateAndClose(str, sender, state{sub: s, vc: vc})
	} else {
		sendClose(str, sender)
	}
}

func (e equaloGoal) Apply(st state) stream {
	str := newStream()
	s, ok := st.sub.unify(e.u, e.v)
	ers := equaloReqPool.Get().(*equaloReqState)
	ers.str, ers.s, ers.vc, ers.ok = str, s, st.vc, ok
	registerRequest(str, ers.reqFn)
	return str
}

type callfreshGoal struct {
	f func(expression) goal
}

func callfresh(f func(x expression) goal) goal {
	return callfreshGoal{f}
}

func (cf callfreshGoal) Apply(st state) stream {
	v := mkVar(variable(st.vc))
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
	registerReceive(str, func(msg message) {
		switch msg.msgtype {
		case stateMessage:
			sendState(str, sender, msg.st)
			mplus(str, str2, str1)
		case stateCloseMessage:
			sendForwardWithState(str, sender, str2, msg.st)
		case closeMessage:
			sendForward(str, sender, str2)
		case forwardMessage:
			mplus_(sender, str, msg.fwd, str2)
		case forwardWithStateMessage:
			sendState(str, sender, msg.st)
			mplus(str, str2, msg.fwd)
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

// bindReqState holds the captured variables for the bind registerRequest callback.
// Pooling avoids a ~40-byte heap allocation per bind call; the method value
// bs.reqFn is only 16 bytes (funcval code ptr + bs pointer).
type bindReqState struct {
	str, str1 stream
	g         goal
}

var bindReqPool = sync.Pool{New: func() any { return &bindReqState{} }}

func (bs *bindReqState) reqFn(sender stream, done bool) {
	str, str1, g := bs.str, bs.str1, bs.g
	bs.str, bs.str1, bs.g = 0, 0, nil
	bindReqPool.Put(bs)
	if done {
		request(str, str1, true) // close
		return
	}
	bind_(sender, str, str1, g)
}

func bind(str, str1 stream, g goal) {
	bs := bindReqPool.Get().(*bindReqState)
	bs.str, bs.str1, bs.g = str, str1, g
	registerRequest(str, bs.reqFn)
}

// bindRecState holds the captured variables for the bind_ registerReceive callback.
// Pooling avoids a ~48-byte heap allocation per bind_ call; the method value
// bs.recFn is only 16 bytes.
// The struct is returned to the pool after each terminal message type.
// For forwardMessage and delayMessage (which loop), the same struct is reused.
type bindRecState struct {
	sender, str, str1 stream
	g                 goal
}

var bindRecPool = sync.Pool{New: func() any { return &bindRecState{} }}

func (bs *bindRecState) recFn(msg message) {
	switch msg.msgtype {
	case stateMessage:
		g, str1, str, sender := bs.g, bs.str1, bs.str, bs.sender
		bs.g, bs.str1, bs.str, bs.sender = nil, 0, 0, 0
		bindRecPool.Put(bs)
		bstr := newStream()
		bind(bstr, str1, g)
		conjStr := g.Apply(msg.st)
		mplus_(sender, str, conjStr, bstr)
	case stateCloseMessage:
		g, str, sender := bs.g, bs.str, bs.sender
		bs.g, bs.str, bs.sender = nil, 0, 0
		bindRecPool.Put(bs)
		conjStr := g.Apply(msg.st)
		sendForward(str, sender, conjStr)
	case closeMessage:
		str, sender := bs.str, bs.sender
		bs.g, bs.str, bs.sender = nil, 0, 0
		bindRecPool.Put(bs)
		sendClose(str, sender)
	case forwardMessage:
		// Reuse bs for the recursive call; just update str1.
		bs.str1 = msg.fwd
		request(bs.str, bs.str1, false)
		registerReceive(bs.str, bs.recFn)
	case forwardWithStateMessage:
		g, str, sender := bs.g, bs.str, bs.sender
		bs.g, bs.str, bs.sender = nil, 0, 0
		bindRecPool.Put(bs)
		bstr := newStream()
		bind(bstr, msg.fwd, g)
		conjStr := g.Apply(msg.st)
		mplus_(sender, str, conjStr, bstr)
	case delayMessage:
		// Reuse bs for the retry; str1 is unchanged.
		request(bs.str, bs.str1, false)
		registerReceive(bs.str, bs.recFn)
	}
}

func bind_(sender, str, str1 stream, g goal) {
	bs := bindRecPool.Get().(*bindRecState)
	bs.sender, bs.str, bs.str1, bs.g = sender, str, str1, g
	request(str, str1, false)
	registerReceive(str, bs.recFn)
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
	cancel := startWorkers()
	g := conj_plus(goals...)
	stream := g.Apply(emptystate)
	out := mKreify(takeAll(stream))
	cancel()
	return out
}

func runN(n int, goals ...goal) []expression {
	cancel := startWorkers()
	g := conj_plus(goals...)
	stream := g.Apply(emptystate)
	out := mKreify(takeN(n, stream))
	cancel()
	return out
}

func mKreify(states []state) []expression {
	exprs := []expression{}
	for _, st := range states {
		exprs = append(exprs, st.sub.walkstar(mkVar(0)))
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
	x := mkVar(variable(st.vc))
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
	x := mkVar(variable(st.vc))
	y := mkVar(variable(st.vc + 1))
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
	x := mkVar(variable(st.vc))
	y := mkVar(variable(st.vc + 1))
	z := mkVar(variable(st.vc + 2))
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
	x := mkVar(variable(st.vc))
	y := mkVar(variable(st.vc + 1))
	z := mkVar(variable(st.vc + 2))
	a := mkVar(variable(st.vc + 3))
	b := mkVar(variable(st.vc + 4))
	c := mkVar(variable(st.vc + 5))
	d := mkVar(variable(st.vc + 6))
	newstate := state{sub: st.sub, vc: st.vc + 7}
	return f.f(x, y, z, a, b, c, d).Apply(newstate)
}
