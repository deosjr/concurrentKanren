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
	numWorkers = 4
	mutexShards = 100
)

var (
	reqMuts      = map[int]*sync.Mutex{}
	recMuts      = map[int]*sync.Mutex{}
	requests     = map[int]map[stream]requestWork{}
	suspendedReq = map[int]map[stream]reqFn{}
	inbox        = map[int]map[stream][]message{}
	suspendedRec = map[int]map[stream]receiveFn{}
	out chan any
)

func startWorkers() context.CancelFunc {
	for i:=0; i<mutexShards; i++ {
		// shard by hash: modulo mutexShards
		reqMuts[i] = &sync.Mutex{}
		recMuts[i] = &sync.Mutex{}
		requests[i] = map[stream]requestWork{}
		suspendedReq[i] = map[stream]reqFn{}
		inbox[i] = map[stream][]message{}
		suspendedRec[i] = map[stream]receiveFn{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan any, numWorkers * 10000)
	out = make(chan any, numWorkers * 10000)
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
	hash := int(str % mutexShards)
	reqMuts[hash].Lock()
	req, ok := requests[hash][str]
	if ok {
		delete(requests[hash], str)
	} else {
		suspendedReq[hash][str] = reqFn
	}
	reqMuts[hash].Unlock()
	if !ok {
		return
	}
	req.fn = reqFn
	out <- req
}

// make a request for more work. suspend if no one is waiting for requests
// this needs to be renamed. bool is used to close children as well!
func request(sender, receiver stream, done bool) {
	hash := int(receiver % mutexShards)
	reqMuts[hash].Lock()
	fn, ok := suspendedReq[hash][receiver]
	if ok {
		delete(suspendedReq[hash], receiver)
	} else {
		requests[hash][receiver] = requestWork{sender: sender, done: done}
	}
	reqMuts[hash].Unlock()
	if !ok {
		return
	}
	fn(sender, done)
}

// block waiting to receive a result
func registerReceive(str stream, recFn receiveFn) {
	hash := int(str % mutexShards)
	recMuts[hash].Lock()
	var msg message
	msgs, ok := inbox[hash][str]
	if ok {
		msg = msgs[0]
		if len(msgs) == 1 {
			delete(inbox[hash], str)
		} else {
			inbox[hash][str] = msgs[1:]
		}
	} else {
		suspendedRec[hash][str] = recFn
	}
	recMuts[hash].Unlock()
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
	hash := int(receiver % mutexShards)
	recMuts[hash].Lock()
	fn, ok := suspendedRec[hash][receiver]
	if ok {
		delete(suspendedRec[hash], receiver)
	} else {
		inbox[hash][receiver] = append(inbox[hash][receiver], msg)
	}
	recMuts[hash].Unlock()
	if !ok {
		return
	}
	fn(msg)
}
