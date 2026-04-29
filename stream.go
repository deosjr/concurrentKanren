package main

import "sync/atomic"

var streamcounter atomic.Int64

// a stream is a coroutine ID, yielding messages
// a stream is guaranteed to have a single requesting parent
// and will always send a message back upon request
type stream int64

func newStream() stream {
	return stream(streamcounter.Add(1))
}

type reqFn func(sender stream, done bool)

type receiveFn func(msg message)

type message struct {
	sender  stream
	msgtype msgtype
	st      state
	fwd     stream
}

type msgtype uint8

const (
	stateMessage msgtype = iota
	stateCloseMessage
	closeMessage
	forwardMessage
	forwardWithStateMessage
	delayMessage
)

func sendState(sender, receiver stream, st state) {
	m := message{sender: sender, msgtype: stateMessage, st: st}
	send(receiver, m)
}

func sendStateAndClose(sender, receiver stream, st state) {
	m := message{sender: sender, msgtype: stateCloseMessage, st: st}
	send(receiver, m)
}

func sendClose(sender, receiver stream) {
	m := message{sender: sender, msgtype: closeMessage}
	send(receiver, m)
}

func sendForward(sender, receiver, fwd stream) {
	m := message{sender: sender, msgtype: forwardMessage, fwd: fwd}
	send(receiver, m)
}

func sendForwardWithState(sender, receiver, fwd stream, st state) {
	m := message{sender: sender, msgtype: forwardWithStateMessage, fwd: fwd, st: st}
	send(receiver, m)
}

func sendDelay(sender, receiver stream) {
	m := message{sender: sender, msgtype: delayMessage}
	send(receiver, m)
}

type delayGoal struct {
	f func() goal
}

func delay(f func() goal) goal {
	return delayGoal{f: f}
}

func (d delayGoal) Apply(st state) stream {
	str := newStream()
	registerRequest(str, func(sender stream, done bool) {
		if done {
			return
		}
		sendDelay(str, sender)
		registerRequest(str, func(sender stream, done bool) {
			if done {
				return
			}
			fwd := d.f().Apply(st)
			sendForward(str, sender, fwd)
		})
	})
	return str
}

func takeAll(str stream) []state {
	states := []state{}
	out := newStream()
	done := make(chan bool, 1)
	var takeFn receiveFn
	takeFn = func(msg message) {
		switch msg.msgtype {
		case stateMessage:
			states = append(states, msg.st)
		case stateCloseMessage:
			states = append(states, msg.st)
			done <- true
			return
		case closeMessage:
			done <- true
			return
		case forwardMessage:
			str = msg.fwd
		case forwardWithStateMessage:
			states = append(states, msg.st)
			str = msg.fwd
		case delayMessage:
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
	done := make(chan bool, 1)
	var takeFn receiveFn
	takeFn = func(msg message) {
		switch msg.msgtype {
		case stateMessage:
			states = append(states, msg.st)
		case stateCloseMessage:
			states = append(states, msg.st)
			done <- true
			return
		case closeMessage:
			done <- true
			return
		case forwardMessage:
			str = msg.fwd
		case forwardWithStateMessage:
			states = append(states, msg.st)
			str = msg.fwd
		case delayMessage:
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
