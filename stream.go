package main

// a stream is smth you can request an answer from by
// sending your inbox as destination on its inbound channel

type inbox chan stateMsg

type stream struct {
	req chan inbox	// closed by parent
	inbox inbox	// owned by stream
}

type stateMsg struct {
	sender inbox
	st      state
	fwd     stream
	ok      bool
	delayed bool
	done    bool
}

func newStream() stream {
	req := make(chan inbox, 1)
	inbox := make(inbox)
	return stream{req, inbox}
}

func (str stream) getRequest() (inbox, bool) {
	in, ok := <-str.req
	return in, ok
}

func (s stream) close() {
	close(s.inbox)
}

func (str stream) request(sender stream) {
	str.req <- sender.inbox
}

func (str stream) receive() (stateMsg, bool) {
	msg, ok := <-str.inbox
	return msg, ok
}

// note: confusing sender/receiver
func (str stream) sendDone() {
	close(str.req)
}

func (str stream) sendState(receiver inbox, st state) {
	receiver <- stateMsg{sender: str.inbox, st: st, ok: true}
}

func (str stream) sendStateAndClose(receiver inbox, st state) {
	receiver <- stateMsg{sender: str.inbox, st: st, ok: true, done: true}
}

func (str stream) sendClose(receiver inbox) {
	receiver <- stateMsg{sender: str.inbox, done: true}
	str.close()
}

func (str stream) sendDelay(receiver inbox) {
	receiver <- stateMsg{sender: str.inbox, delayed: true}
}

func (str stream) sendForward(receiver inbox, fwd stream) {
	receiver <- stateMsg{sender: str.inbox, fwd: fwd}
	str.close()
}

func (str stream) sendForwardWithState(receiver inbox, fwd stream, st state) {
	receiver <- stateMsg{sender: str.inbox, fwd: fwd, st: st, ok: true}
	str.close()
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
			sender, more := str.getRequest()
			if !more {
				str.close()
				return
			}
			str.sendDelay(sender)
			sender, more = str.getRequest()
			if !more {
				str.close()
				return
			}
			str.sendForward(sender, f()(st))
		}()
		return str
	}
}

func takeAll(str stream) []state {
	states := []state{}
	out := newStream()
	for {
		str.request(out)
		rec, ok := out.receive()
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
		str.request(out)
		rec, ok := out.receive()
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
