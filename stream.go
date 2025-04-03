package main

// a stream is smth you can request an answer from by
// sending it the return channel, or close it by sending done
// goal go func that maintains the stream closes the stream
// note we are not using channel close to signal end of stream;
// that takes another request/inspection which causes complexity
type stream struct {
	req chan reqMsg
	rec chan stateMsg
}

type stateMsg struct {
	st      state
	fwd     stream
	ok      bool
	delayed bool
	done    bool
}

type reqMsg struct {
	onto chan stateMsg
	done bool
}

func newStream() stream {
	req := make(chan reqMsg, 1)
	rec := make(chan stateMsg)
	return stream{req, rec}
}

func (s stream) close() {
	close(s.req)
	close(s.rec)
}

func (sender stream) request(s stream) {
	s.req <- reqMsg{onto: sender.rec}
}

func sendDone(ch chan reqMsg) {
	ch <- reqMsg{done: true}
}

func sendState(ch chan stateMsg, st state) {
	ch <- stateMsg{st: st, ok: true}
}

func sendStateAndClose(ch chan stateMsg, st state) {
	ch <- stateMsg{st: st, ok: true, done: true}
}

func sendClose(ch chan stateMsg) {
	ch <- stateMsg{done: true}
}

func sendDelay(ch chan stateMsg) {
	ch <- stateMsg{delayed: true}
}

func sendForward(ch chan stateMsg, fwd stream) {
	ch <- stateMsg{fwd: fwd}
}

func sendForwardWithState(ch chan stateMsg, fwd stream, st state) {
	ch <- stateMsg{fwd: fwd, st: st, ok: true}
}

func (m stateMsg) isState() bool {
	return m.ok && !m.done && m.fwd.req == nil
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
	return m.fwd.req != nil && !m.ok
}

func (m stateMsg) isForwardWithState() bool {
	return m.fwd.req != nil && m.ok
}

func delay(f func() goal) goal {
	return func(st state) stream {
		str := newStream()
		go func() {
			req := <-str.req
			if req.done {
				str.close()
				return
			}
			sendDelay(req.onto)
			req = <-str.req
			if req.done {
				str.close()
				return
			}
			sendForward(req.onto, f()(st))
			str.close()
		}()
		return str
	}
}

func takeAll(str stream) []state {
	states := []state{}
	out := newStream()
	for {
		out.request(str)
		rec, ok := <-out.rec
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
	out := newStream()
	for len(states) < n {
		out.request(str)
		rec, ok := <-out.rec
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
	sendDone(str.req)
	return states
}
