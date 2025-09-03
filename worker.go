package main

import (
	"context"
	"sync"
)

// two kinds of work: request and receive
// sender of request will suspend on receive
// upon request, further requests of subgoals may be needed before we can send
// receive resumes when a value exists (can be immediate without suspend!)
// sending does _not_ yield, only resume some more tasks

type requestWork struct {
	sender stream
	done   bool
	fn     reqFn
}

type receiveWork struct {
	msg message
	fn  receiveFn
}

const (
	numWorkers = 10
)

var (
	reqMut       sync.Mutex
	recMut       sync.Mutex
	requests     = map[stream]requestWork{}
	suspendedReq = map[stream]reqFn{}
	inbox        = map[stream][]message{}
	suspendedRec = map[stream]receiveFn{}
	out chan any
)

func startWorkers() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan any, numWorkers)
	out = make(chan any, numWorkers)
	for range numWorkers {
		go work(in)
	}
	go manageWorkers(ctx, in, out)
	return cancel 
}

func manageWorkers(ctx context.Context, in, out chan any) {
	q := []any{}
	for {
		select {
		case <-ctx.Done():
			close(in)
			return
		case w := <-out:
			q = append(q, w)
		default:
		}
		if len(q) == 0 {
			continue
		}
		w := q[0]
		select {
		case <-ctx.Done():
			close(in)
			return
		case in<-w:
			q = q[1:]
		default:
		}
	}
}

func work(in chan any) {
	for w := range in {
		switch t := w.(type) {
		case requestWork:
			t.fn(t.sender, t.done)
		case receiveWork:
			t.fn(t.msg)
		}
	}
}

// block waiting for more requests for work
func registerRequest(str stream, reqFn reqFn) {
	reqMut.Lock()
	req, ok := requests[str]
	if ok {
		delete(requests, str)
	} else {
		suspendedReq[str] = reqFn
	}
	reqMut.Unlock()
	if !ok {
		return
	}
	req.fn = reqFn
	out <- req
}

// make a request for more work. suspend if no one is waiting for requests
// this needs to be renamed. bool is used to close children as well!
func request(sender, receiver stream, done bool) {
	reqMut.Lock()
	fn, ok := suspendedReq[receiver]
	if ok {
		delete(suspendedReq, receiver)
	} else {
		requests[receiver] = requestWork{sender: sender, done: done}
	}
	reqMut.Unlock()
	if !ok {
		return
	}
	fn(sender, done)
}

// block waiting to receive a result
func registerReceive(str stream, recFn receiveFn) {
	recMut.Lock()
	var msg message
	msgs, ok := inbox[str]
	if ok {
		msg = msgs[0]
		if len(msgs) == 1 {
			delete(inbox, str)
		} else {
			inbox[str] = msgs[1:]
		}
	} else {
		suspendedRec[str] = recFn
	}
	recMut.Unlock()
	if !ok {
		return
	}
	rec := receiveWork{
		msg: msg,
		fn:  recFn,
	}
	out <- rec
}

func send(receiver stream, msg message) {
	recMut.Lock()
	fn, ok := suspendedRec[receiver]
	if ok {
		delete(suspendedRec, receiver)
	} else {
		inbox[receiver] = append(inbox[receiver], msg)
	}
	recMut.Unlock()
	if !ok {
		return
	}
	fn(msg)
}
