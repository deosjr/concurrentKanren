package main

import (
	"sync/atomic"
)

var streamcounter atomic.Int64

// a stream is a coroutine ID, yielding messages
// a stream is guaranteed to have a single requesting parent
// and will always send a message back upon request
type stream int64

func newStream() stream {
	return stream(streamcounter.Add(1))
}

type reqFn func(sender stream, done bool)

type receiveFn func(msg Message)

type Message interface {
	Done() bool
	Sender() stream
}

type message struct {
	sender stream
}

func (m message) Done() bool {
	return false
}

func (m message) Sender() stream {
	return m.sender
}

type stateMessage struct {
	message
	st state
}

func sendState(sender, receiver stream, st state) {
	m := stateMessage{message: message{sender}, st: st}
	send(receiver, m)
}

type stateCloseMessage struct {
	message
	st state
}

func sendStateAndClose(sender, receiver stream, st state) {
	m := stateCloseMessage{message: message{sender}, st: st}
	send(receiver, m)
}

type closeMessage struct {
	message
}

func sendClose(sender, receiver stream) {
	m := closeMessage{message: message{sender}}
	send(receiver, m)
}

type forwardWithStateMessage struct {
	message
	st state
	fwd stream
}

func sendForwardWithState(sender, receiver, fwd stream, st state) {
	m := forwardWithStateMessage{message: message{sender}, st:st, fwd:fwd}
	send(receiver, m)
}

/*
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
*/

/*
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
*/

func takeAll(str stream) []state {
	states := []state{}
	out := newStream()
	done := make(chan bool)
	var takeFn receiveFn
	takeFn = func(msg Message) {
		switch t := msg.(type) {
		case stateMessage:
			states = append(states, t.st)
		case stateCloseMessage:
			states = append(states, t.st)
			done <- true
			return
		case closeMessage:
			done <- true
			return
		case forwardWithStateMessage:
			states = append(states, t.st)
			str = t.fwd
		}
		request(out, str, false)
		registerReceive(out, takeFn)
	}
	request(out, str, false)
	registerReceive(out, takeFn)
	<-done
	return states
}

func takeN(n int, str stream) []state {
	states := []state{}
	out := newStream()
	done := make(chan bool)
	var takeFn receiveFn
	takeFn = func(msg Message) {
		switch t := msg.(type) {
		case stateMessage:
			states = append(states, t.st)
		case stateCloseMessage:
			states = append(states, t.st)
			done <- true
			return
		case closeMessage:
			done <- true
			return
		case forwardWithStateMessage:
			states = append(states, t.st)
			str = t.fwd
		}
		if len(states) == n {
			request(out, str, true)
			done <- true
			return
		}
		request(out, str, false)
		registerReceive(out, takeFn)
	}
	request(out, str, false)
	registerReceive(out, takeFn)
	<-done
	return states
}
