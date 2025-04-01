package main

import (
	"context"
	"sync"
)

// there are four types of work:
// - init, work without prerequisite
// - send, continues once state is sent
// - receive, continues once state is received
// - close, closing a stream
// - forward, setting str=fwd
// most of work is a one-to-one channel with continuation

type work struct {
	ctx     context.Context
	str     stream
	init    func()
	send    func()
	receive func(message)
	closed  bool
	fwd     stream
	m       message
}

type message struct {
	st state
	ok bool
}

const (
	numWorkers      = 10
	numCoordinators = 1
)

var (
	reqch  = make(chan work, numWorkers*100000)
	workch = make(chan work, numWorkers*10)

	receiving = map[stream]work{}
	sending   = map[stream]work{}
	closed    = map[stream]struct{}{}
	forward   = map[stream]stream{}
)

func startWorkers() *sync.WaitGroup {
	var wg sync.WaitGroup
	for range numWorkers {
		go worker(&wg)
	}
	for range numCoordinators {
		go coordinate()
	}
	return &wg
}

func (w work) ctxdone() bool {
	select {
	case <-w.ctx.Done():
		return true
	default:
		return false
	}
}

func worker(wg *sync.WaitGroup) {
	for w := range workch {
		if w.ctxdone() {
			continue
		}
		switch {
		case w.init != nil:
			w.init()
		case w.send != nil:
			w.send()
		case w.receive != nil:
			w.receive(w.m)
		}
	}
	wg.Done()
}

func coordinate() {
	for w := range reqch {
		if w.init != nil {
			workch <- w
			continue
		}
		if w.closed {
			str := getFwd(w.str)
			coordinateClose(str)
			continue
		}
		// TODO: collision!
		if w.fwd != stream(0) {
			str := getFwd(w.str)
			forward[w.fwd] = str
			if w, ok := receiving[w.fwd]; ok {
				coordinateReceive(w)
				delete(receiving, w.fwd)
			}
			if w, ok := sending[w.fwd]; ok {
				coordinateSend(w)
				delete(sending, w.fwd)
			}
			if _, ok := closed[w.fwd]; ok {
				coordinateClose(str)
			}
			continue
		}
		if w.send != nil {
			coordinateSend(w)
			continue
		}
		if w.receive != nil {
			coordinateReceive(w)
		}
	}
}

func getFwd(str stream) stream {
	for {
		s, ok := forward[str]
		if !ok {
			return str
		}
		str = s
	}
}

func wInit(ctx context.Context, f func()) {
	reqch <- work{ctx: ctx, init: f}
}

func wClose(str stream) {
	reqch <- work{str: str, closed: true}
}

func wSend(ctx context.Context, str stream, st state, f func()) {
	reqch <- work{ctx: ctx, str: str, m: message{st: st, ok: true}, send: f}
}

func wReceive(ctx context.Context, str stream, f func(message)) {
	reqch <- work{ctx: ctx, str: str, receive: f}
}

// forwarding closes a stream, so we have to only replace references one-way
func wForward(from, to stream) {
	reqch <- work{str: from, fwd: to}
}

func coordinateClose(str stream) {
	closed[str] = struct{}{}
	v, ok := receiving[str]
	if !ok {
		return
	}
	delete(receiving, str)
	v.m = message{ok: false}
	workch <- v
}

func coordinateSend(w work) {
	str := getFwd(w.str)
	if _, ok := closed[str]; ok {
		panic("send on closed stream!")
	}
	v, ok := receiving[str]
	if !ok {
		sending[str] = w
		return
	}
	delete(receiving, str)
	v.m = w.m
	workch <- w
	workch <- v
}

func coordinateReceive(w work) {
	str := getFwd(w.str)
	_, ok := closed[str]
	if ok {
		w.m = message{ok: false}
		workch <- w
		return
	}
	v, ok := sending[str]
	if !ok {
		receiving[str] = w
		return
	}
	delete(sending, str)
	w.m = v.m
	workch <- w
	workch <- v
}
