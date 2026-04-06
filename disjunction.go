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
// recvFn holds the pre-baked method value (created once in pool.New) so that
// each step() call passes it to registerReceive without a funcval allocation.
type refillState struct {
	str     stream
	streams []stream
	i       int
	buffer  []state
	active  []stream
	recvFn  receiveFn // pre-stored method value, set once in pool.New
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
	rs.buffer = rs.buffer[:0]
	rs.active = rs.active[:0]
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

// finish is called once all streams have responded. It returns the rs struct to
// the pool (after extracting the data it needs) and dispatches the buffer/active
// sets to the appropriate continuation.
func (rs *refillState) finish() {
	str := rs.str
	buffer := rs.buffer
	active := rs.active

	// Nil out slice fields before returning to pool so future rounds get a
	// clean struct; we keep the backing arrays alive via the local variables.
	rs.buffer = nil
	rs.active = nil
	rs.streams = nil
	refillStatePool.Put(rs)

	switch {
	case len(active) == 0 && len(buffer) == 0:
		registerRequest(str, func(sender stream, done bool) {
			if done {
				return
			}
			sendClose(str, sender)
		})

	case len(active) == 0:
		mplusplusWithBuffer(str, buffer, nil)

	case len(active) == 1 && len(buffer) == 0:
		active0 := active[0]
		registerRequest(str, func(sender stream, done bool) {
			if done {
				request(str, active0, true)
				return
			}
			sendForward(str, sender, active0)
		})

	case len(buffer) == 0:
		refillBuffer(str, active)

	default:
		mplusplusWithBuffer(str, buffer, active)
	}
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

func mplusplusWithBuffer(str stream, buffer []state, streams []stream) {
	registerRequest(str, func(sender stream, done bool) {
		if done {
			for _, s := range streams {
				request(str, s, true) // close
			}
			return
		}
		sendState(str, sender, buffer[0])
		mplusplus(str, buffer[1:], streams)
	})
}
