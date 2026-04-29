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
// method value (set once lazily on first Get, then reused across Put/Get cycles).
type equaloReqState struct {
	str stream
	st  state
	ok  bool
	fn  reqFn // pre-stored method value, set once in pool.New
}

var equaloReqPool = sync.Pool{New: func() any { return &equaloReqState{} }}

func (ers *equaloReqState) reqFn(sender stream, done bool) {
	str, st, ok := ers.str, ers.st, ers.ok
	ers.str, ers.st, ers.ok = 0, state{}, false
	equaloReqPool.Put(ers)
	if done {
		return
	}
	if ok {
		sendStateAndClose(str, sender, st)
	} else {
		sendClose(str, sender)
	}
}

func (e equaloGoal) Apply(st state) stream {
	str := newStream()
	s, ok := st.sub.unify(e.u, e.v)
	var newSt state
	if ok {
		newSt, ok = extendStateAndCheck(st, s)
	}
	ers := equaloReqPool.Get().(*equaloReqState)
	if ers.fn == nil {
		ers.fn = ers.reqFn // set once per pool-object lifetime, reused on every subsequent Get
	}
	ers.str, ers.st, ers.ok = str, newSt, ok
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
	newstate := state{sub: st.sub, vc: st.vc + 1, diseq: st.diseq, abs: st.abs}
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
// fn is pre-stored (set once lazily on first Get) to avoid a funcval allocation on each mplus call.
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
	resumeFn                reqFn
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
		// Propagate delay upward AND swap. Mirrors Chez's
		//   ((procedure? s) (lambda () (mplus s2 (s))))
		// where returning a thunk surfaces the delay to the consumer, and
		// (mplus s2 (s)) does the swap when forced.
		str, sender := mrs.str, mrs.sender
		mrs.sender = 0
		mrs.str1, mrs.str2 = mrs.str2, mrs.str1
		sendDelay(str, sender)
		registerRequest(str, mrs.resumeFn)
	}
}

// resumeReqFn is mplus's request handler after sending a delay upward. On the
// next request from our consumer, request from the new str1 (post-swap) and
// re-register the receive callback.
func (mrs *mplusRecState) resumeReqFn(sender stream, done bool) {
	str, str1, str2 := mrs.str, mrs.str1, mrs.str2
	if done {
		mrs.sender, mrs.str, mrs.str1, mrs.str2 = 0, 0, 0, 0
		mplusRecPool.Put(mrs)
		request(str, str1, true)
		request(str, str2, true)
		return
	}
	mrs.sender = sender
	request(str, str1, false)
	registerReceive(str, mrs.fn)
}

func mplus_(sender, str, str1, str2 stream) {
	mrs := mplusRecPool.Get().(*mplusRecState)
	if mrs.fn == nil {
		mrs.fn = mrs.mplusRecFn
		mrs.resumeFn = mrs.resumeReqFn
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
// fn is pre-stored (set once lazily on first Get) to avoid a funcval allocation on each bind call.
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
// fn is pre-stored (set once lazily on first Get) to avoid a funcval allocation on each bind_ call.
// The struct is returned to the pool after each terminal message type.
// For forwardMessage and delayMessage (which loop), the same struct is reused.
type bindRecState struct {
	sender, str, str1 stream
	g                 goal
	fn                receiveFn
	resumeFn          reqFn
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
		// Propagate delay upward instead of absorbing it locally. mplus
		// upstream gets a chance to swap. Re-arm so that on the next request
		// from our consumer, we re-request str1 (which is mid-delay).
		str, sender := bs.str, bs.sender
		bs.sender = 0
		sendDelay(str, sender)
		registerRequest(str, bs.resumeFn)
	}
}

// resumeReqFn is bind's request handler after sending a delay upward. When
// the consumer sends its next request, we re-request str1 (which will now
// move past its delay) and re-register the receive callback.
func (bs *bindRecState) resumeReqFn(sender stream, done bool) {
	str, str1 := bs.str, bs.str1
	if done {
		bs.g, bs.str, bs.str1, bs.sender = nil, 0, 0, 0
		bindRecPool.Put(bs)
		request(str, str1, true)
		return
	}
	bs.sender = sender
	request(str, str1, false)
	registerReceive(str, bs.fn)
}

func bind_(sender, str, str1 stream, g goal) {
	bs := bindRecPool.Get().(*bindRecState)
	if bs.fn == nil {
		bs.fn = bs.recFn
		bs.resumeFn = bs.resumeReqFn
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

// fresh1/fresh2/fresh3/fresh7 duplicate callfresh's logic for N variables.
// Go has no macros, so each arity is written out by hand.
// fresh7 is needed by genAdderO in arithmetic.go; arities 4-6 are not currently used.

type fresh1Goal struct {
	f func(expression) goal
}

func fresh1(f func(x expression) goal) goal {
	return fresh1Goal{f}
}
func (f fresh1Goal) Apply(st state) stream {
	x := mkVar(variable(st.vc))
	newstate := state{sub: st.sub, vc: st.vc + 1, diseq: st.diseq, abs: st.abs}
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
	newstate := state{sub: st.sub, vc: st.vc + 2, diseq: st.diseq, abs: st.abs}
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
	newstate := state{sub: st.sub, vc: st.vc + 3, diseq: st.diseq, abs: st.abs}
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
	newstate := state{sub: st.sub, vc: st.vc + 7, diseq: st.diseq, abs: st.abs}
	return f.f(x, y, z, a, b, c, d).Apply(newstate)
}
