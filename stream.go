package main

// a stream is smth you can request an answer from by
// sending it the return channel, or close it by sending done
// goal go func that maintains the stream closes the stream
// note we are not using channel close to signal end of stream;
// that takes another request/inspection which causes complexity
type stream struct {
	in chan bool // true means done, ie no more requests coming
	out chan stateMsg
}

type stateMsg struct {
	st      state
	fwd     stream
	ok      bool
	delayed bool
	done    bool
}

func newStream() stream {
	in := make(chan bool, 1)
	out := make(chan stateMsg)
	return stream{in, out}
}

func (str stream) getRequest() bool {
	return <-str.in
}

func (s stream) close() {
	close(s.in)
	close(s.out)
}

func (str stream) request() {
	str.in <- false
}

func (str stream) receive() (stateMsg, bool) {
	msg, ok := <-str.out
	return msg, ok
}

func (str stream) sendDone() {
	str.in <- true
}

func (str stream) sendState(st state) {
	str.out <- stateMsg{st: st, ok: true}
}

func (str stream) sendStateAndClose(st state) {
	str.out <- stateMsg{st: st, ok: true, done: true}
}

func (str stream) sendClose() {
	str.out <- stateMsg{done: true}
	str.close()
}

func (str stream) sendDelay() {
	str.out <- stateMsg{delayed: true}
}

func (str stream) sendForward(fwd stream) {
	str.out <- stateMsg{fwd: fwd}
	str.close()
}

func (str stream) sendForwardWithState(fwd stream, st state) {
	str.out <- stateMsg{fwd: fwd, st: st, ok: true}
	str.close()
}

func (m stateMsg) isState() bool {
	return m.ok && !m.done && m.fwd.in == nil
}

func (m stateMsg) isStateAndClose() bool {
	return m.ok && m.done
}

func (m stateMsg) isClose() bool {
	return !m.ok && m.done
}

func (m stateMsg) isDelay() bool {
	return m.delayed
}

func (m stateMsg) isForward() bool {
	return m.fwd.in != nil && !m.ok
}

func (m stateMsg) isForwardWithState() bool {
	return m.fwd.in != nil && m.ok
}

func delay(f func() goal) goal {
	return func(st state) stream {
		str := newStream()
		go func() {
			done := str.getRequest()
			if done {
				str.close()
				return
			}
			str.sendDelay()
			done = str.getRequest()
			if done {
				str.close()
				return
			}
			str.sendForward(f()(st))
		}()
		return str
	}
}

func takeAll(str stream) []state {
	states := []state{}
	for {
		str.request()
		rec, ok := str.receive()
		if !ok {
			panic("takeAll read on closed channel")
		}
		switch {
		case rec.isState():
			states = append(states, rec.st)
		case rec.isStateAndClose():
			return append(states, rec.st)
		case rec.isClose():
			return states
		case rec.isForward():
			str = rec.fwd
		case rec.isForwardWithState():
			states = append(states, rec.st)
			str = rec.fwd
		case rec.isDelay():
			continue
		}
	}
}

func takeN(n int, str stream) []state {
	states := []state{}
	for len(states) < n {
		str.request()
		rec, ok := str.receive()
		if !ok {
			panic("takeN read on closed channel")
		}
		switch {
		case rec.isState():
			states = append(states, rec.st)
		case rec.isStateAndClose():
			return append(states, rec.st)
		case rec.isClose():
			return states
		case rec.isForward():
			str = rec.fwd
		case rec.isForwardWithState():
			states = append(states, rec.st)
			str = rec.fwd
		case rec.isDelay():
			continue
		}
	}
	str.sendDone()
	return states
}
