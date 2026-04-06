package main

import "sync"

type disjConcGoal struct {
	goals []goal
}

func disj_conc(goals ...goal) goal {
	return disjConcGoal{goals: goals}
}

func (dc disjConcGoal) Apply(st state) stream {
	str := newStream()
	streams := make([]stream, len(dc.goals))
	for i, g := range dc.goals {
		streams[i] = g.Apply(st)
	}
	rs := getRefillState(str, streams)
	rs.step()
	return str
}

// refillState holds all the mutable context for one round of parallel stream
// draining in disj_conc. Pooling the struct avoids allocation per refill round;
// recvFn holds the pre-baked method value (created once lazily) so that
// each step() call passes it to registerReceive without a funcval allocation.
//
// buffer and active are intentionally reset to length-zero (not nil) before
// pool.Put so that their backing arrays survive for the next round; this means
// recv's append calls never allocate after the first warmup round.  finish()
// copies the data out to independent slices before returning rs to the pool.
type refillState struct {
	str     stream
	streams []stream
	i       int
	buffer  []state
	active  []stream
	recvFn  receiveFn // pre-stored method value, set lazily on first use
}

var refillStatePool = sync.Pool{New: func() any { return &refillState{} }}

func getRefillState(str stream, streams []stream) *refillState {
	rs := refillStatePool.Get().(*refillState)
	if rs.recvFn == nil {
		rs.recvFn = rs.recv
	}
	rs.str = str
	rs.streams = streams
	rs.i = 0
	// buffer and active are already [:0] from the previous finish() call.
	// On first use (new pool object) they are nil; pre-allocate so the
	// first recv call doesn't pay for exponential-growth backing arrays.
	if rs.buffer == nil {
		rs.buffer = make([]state, 0, len(streams))
	}
	if rs.active == nil {
		rs.active = make([]stream, 0, len(streams))
	}
	return rs
}

// step sends a request to the next stream and registers rs.recvFn to handle the
// response. When all streams have been queried (i == len(streams)), it calls
// rs.finish to dispatch the collected buffer and active sets.
func (rs *refillState) step() {
	if rs.i == len(rs.streams) {
		rs.finish()
		return
	}
	s := rs.streams[rs.i]
	request(rs.str, s, false)
	registerReceive(rs.str, rs.recvFn) // pre-stored method value: no funcval allocation
}

// recv is called when a response arrives from streams[i].
func (rs *refillState) recv(msg message) {
	s := rs.streams[rs.i]
	switch msg.msgtype {
	case stateMessage:
		rs.buffer = append(rs.buffer, msg.st)
		rs.active = append(rs.active, s)
	case stateCloseMessage:
		rs.buffer = append(rs.buffer, msg.st)
	case closeMessage:
		// stream exhausted, not added to active
	case forwardMessage:
		rs.active = append(rs.active, msg.fwd)
	case forwardWithStateMessage:
		rs.buffer = append(rs.buffer, msg.st)
		rs.active = append(rs.active, msg.fwd)
	case delayMessage:
		rs.active = append(rs.active, s)
	}
	rs.i++
	rs.step()
}

// finish is called once all streams have responded. It copies buffer/active
// data to independent slices for downstream use, resets rs.buffer and rs.active
// to length-zero (retaining capacity for the next round), returns rs to the
// pool, then dispatches.
//
// Keeping the backing arrays alive in the pool object means recv's append calls
// never allocate on a warm pool, at the cost of one copy per finish() call.
func (rs *refillState) finish() {
	str := rs.str
	nBuf := len(rs.buffer)
	nAct := len(rs.active)

	switch {
	case nAct == 0 && nBuf == 0:
		rs.streams = nil
		refillStatePool.Put(rs)
		fcs := finishCloseReqPool.Get().(*finishCloseReqState)
		if fcs.fn == nil {
			fcs.fn = fcs.reqFn
		}
		fcs.str = str
		registerRequest(str, fcs.fn)

	case nAct == 0:
		// Copy buffer; no active streams to worry about.
		buffer := make([]state, nBuf)
		copy(buffer, rs.buffer)
		for i := 0; i < nBuf; i++ {
			rs.buffer[i] = state{} // release *substitution references
		}
		rs.buffer = rs.buffer[:0] // retain capacity for next round
		rs.streams = nil
		refillStatePool.Put(rs)
		mplusplusWithBuffer(str, buffer, nil)

	case nAct == 1 && nBuf == 0:
		// Single active stream: read the one stream ID; no slice copy needed.
		active0 := rs.active[0]
		rs.active = rs.active[:0]
		rs.streams = nil
		refillStatePool.Put(rs)
		ffs := finishFwdReqPool.Get().(*finishFwdReqState)
		if ffs.fn == nil {
			ffs.fn = ffs.reqFn
		}
		ffs.str, ffs.active0 = str, active0
		registerRequest(str, ffs.fn)

	case nBuf == 0:
		// No buffer, multiple active streams.
		active := make([]stream, nAct)
		copy(active, rs.active)
		rs.active = rs.active[:0]
		rs.streams = nil
		refillStatePool.Put(rs)
		refillBuffer(str, active)

	default:
		// Both buffer and active.
		buffer := make([]state, nBuf)
		copy(buffer, rs.buffer)
		for i := 0; i < nBuf; i++ {
			rs.buffer[i] = state{}
		}
		rs.buffer = rs.buffer[:0]
		active := make([]stream, nAct)
		copy(active, rs.active)
		rs.active = rs.active[:0]
		rs.streams = nil
		refillStatePool.Put(rs)
		mplusplusWithBuffer(str, buffer, active)
	}
}

// finishCloseReqState holds the callback state for the "no results" case in finish().
type finishCloseReqState struct {
	str stream
	fn  reqFn
}

var finishCloseReqPool = sync.Pool{New: func() any { return &finishCloseReqState{} }}

func (fcs *finishCloseReqState) reqFn(sender stream, done bool) {
	str := fcs.str
	fcs.str = 0
	finishCloseReqPool.Put(fcs)
	if done {
		return
	}
	sendClose(str, sender)
}

// finishFwdReqState holds the callback state for the single-active-stream case.
type finishFwdReqState struct {
	str, active0 stream
	fn           reqFn
}

var finishFwdReqPool = sync.Pool{New: func() any { return &finishFwdReqState{} }}

func (ffs *finishFwdReqState) reqFn(sender stream, done bool) {
	str, active0 := ffs.str, ffs.active0
	ffs.str, ffs.active0 = 0, 0
	finishFwdReqPool.Put(ffs)
	if done {
		request(str, active0, true)
		return
	}
	sendForward(str, sender, active0)
}

func refillBuffer(str stream, streams []stream) {
	rs := getRefillState(str, streams)
	rs.step()
}

func mplusplus(str stream, buffer []state, streams []stream) {
	if len(buffer) == 0 {
		refillBuffer(str, streams)
	} else {
		mplusplusWithBuffer(str, buffer, streams)
	}
}

// mplusplusBufferReqState holds the callback state for mplusplusWithBuffer.
// Pooling eliminates the ~72-byte closure allocation on every call.
type mplusplusBufferReqState struct {
	str     stream
	buffer  []state
	streams []stream
	fn      reqFn
}

var mplusplusBufferReqPool = sync.Pool{New: func() any { return &mplusplusBufferReqState{} }}

func (mrs *mplusplusBufferReqState) reqFn(sender stream, done bool) {
	str, buffer, streams := mrs.str, mrs.buffer, mrs.streams
	mrs.str, mrs.buffer, mrs.streams = 0, nil, nil
	mplusplusBufferReqPool.Put(mrs)
	if done {
		for _, s := range streams {
			request(str, s, true) // close
		}
		return
	}
	sendState(str, sender, buffer[0])
	mplusplus(str, buffer[1:], streams)
}

func mplusplusWithBuffer(str stream, buffer []state, streams []stream) {
	mrs := mplusplusBufferReqPool.Get().(*mplusplusBufferReqState)
	if mrs.fn == nil {
		mrs.fn = mrs.reqFn
	}
	mrs.str, mrs.buffer, mrs.streams = str, buffer, streams
	registerRequest(str, mrs.fn)
}
