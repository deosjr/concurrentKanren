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
// Pooling avoids allocation per equalo application; fn holds the pre-baked
// method value (created once in pool.New, reused across pool Get/Put cycles).
type equaloReqState struct {
	str stream
	s   *substitution
	vc  int
	ok  bool
	fn  reqFn // pre-stored method value, set once in pool.New
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
	if ers.fn == nil {
		ers.fn = ers.reqFn // set once per pool-object lifetime, reused on every subsequent Get
	}
	ers.str, ers.s, ers.vc, ers.ok = str, s, st.vc, ok
	registerRequest(str, ers.fn)
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

// mplusReqState holds the captured state for mplus's registerRequest callback.
// fn is pre-stored in pool.New to avoid a funcval allocation on each mplus call.
type mplusReqState struct {
	str, str1, str2 stream
	fn              reqFn
}

var mplusReqPool = sync.Pool{New: func() any { return &mplusReqState{} }}

func (mrs *mplusReqState) mplusReqFn(sender stream, done bool) {
	str, str1, str2 := mrs.str, mrs.str1, mrs.str2
	mrs.str, mrs.str1, mrs.str2 = 0, 0, 0
	mplusReqPool.Put(mrs)
	if done {
		request(str, str1, true) // close
		request(str, str2, true) // close
		return
	}
	mplus_(sender, str, str1, str2)
}

func mplus(str, str1, str2 stream) {
	mrs := mplusReqPool.Get().(*mplusReqState)
	if mrs.fn == nil {
		mrs.fn = mrs.mplusReqFn
	}
	mrs.str, mrs.str1, mrs.str2 = str, str1, str2
	registerRequest(str, mrs.fn)
}

// mplusRecState holds the captured state for mplus_'s registerReceive callback.
// For forwardMessage and delayMessage (which loop), the struct is reused in-place.
// For all terminal message types, fields are cleared and the struct returned to pool.
type mplusRecState struct {
	sender, str, str1, str2 stream
	fn                      receiveFn
}

var mplusRecPool = sync.Pool{New: func() any { return &mplusRecState{} }}

func (mrs *mplusRecState) mplusRecFn(msg message) {
	switch msg.msgtype {
	case stateMessage:
		str, sender, str1, str2 := mrs.str, mrs.sender, mrs.str1, mrs.str2
		mrs.sender, mrs.str, mrs.str1, mrs.str2 = 0, 0, 0, 0
		mplusRecPool.Put(mrs)
		sendState(str, sender, msg.st)
		mplus(str, str2, str1)
	case stateCloseMessage:
		str, sender, str2 := mrs.str, mrs.sender, mrs.str2
		mrs.sender, mrs.str, mrs.str1, mrs.str2 = 0, 0, 0, 0
		mplusRecPool.Put(mrs)
		sendForwardWithState(str, sender, str2, msg.st)
	case closeMessage:
		str, sender, str2 := mrs.str, mrs.sender, mrs.str2
		mrs.sender, mrs.str, mrs.str1, mrs.str2 = 0, 0, 0, 0
		mplusRecPool.Put(mrs)
		sendForward(str, sender, str2)
	case forwardMessage:
		// Reuse mrs for the hop; just update str1.
		mrs.str1 = msg.fwd
		request(mrs.str, msg.fwd, false)
		registerReceive(mrs.str, mrs.fn)
	case forwardWithStateMessage:
		str, sender, str2 := mrs.str, mrs.sender, mrs.str2
		mrs.sender, mrs.str, mrs.str1, mrs.str2 = 0, 0, 0, 0
		mplusRecPool.Put(mrs)
		sendState(str, sender, msg.st)
		mplus(str, str2, msg.fwd)
	case delayMessage:
		// Swap str1/str2 and retry from str1 (the new lead stream).
		mrs.str1, mrs.str2 = mrs.str2, mrs.str1
		request(mrs.str, mrs.str1, false)
		registerReceive(mrs.str, mrs.fn)
	}
}

func mplus_(sender, str, str1, str2 stream) {
	mrs := mplusRecPool.Get().(*mplusRecState)
	if mrs.fn == nil {
		mrs.fn = mrs.mplusRecFn
	}
	mrs.sender, mrs.str, mrs.str1, mrs.str2 = sender, str, str1, str2
	request(str, str1, false)
	registerReceive(str, mrs.fn)
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
// fn is pre-stored in pool.New to avoid a funcval allocation on each bind call.
type bindReqState struct {
	str, str1 stream
	g         goal
	fn        reqFn
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
	if bs.fn == nil {
		bs.fn = bs.reqFn
	}
	bs.str, bs.str1, bs.g = str, str1, g
	registerRequest(str, bs.fn)
}

// bindRecState holds the captured variables for the bind_ registerReceive callback.
// fn is pre-stored in pool.New to avoid a funcval allocation on each bind_ call.
// The struct is returned to the pool after each terminal message type.
// For forwardMessage and delayMessage (which loop), the same struct is reused.
type bindRecState struct {
	sender, str, str1 stream
	g                 goal
	fn                receiveFn
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
		registerReceive(bs.str, bs.fn)
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
		registerReceive(bs.str, bs.fn)
	}
}

func bind_(sender, str, str1 stream, g goal) {
	bs := bindRecPool.Get().(*bindRecState)
	if bs.fn == nil {
		bs.fn = bs.recFn
	}
	bs.sender, bs.str, bs.str1, bs.g = sender, str, str1, g
	request(str, str1, false)
	registerReceive(str, bs.fn)
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
