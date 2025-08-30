package main

import (
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
	msg Message
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
	inbox        = map[stream]Message{}
	suspendedRec = map[stream]receiveFn{}
	qIn = make(chan any, 100)
	qOut = make(chan any, 100)
	q = []any{}
)

func startWorkers() *sync.WaitGroup {
	testDone = false
	var wg sync.WaitGroup
	wg.Add(1)
	ch := make(chan struct{}, numWorkers)
	for range numWorkers {
		ch <- struct{}{}
	}
	go manageQueue(q)
	go manageWorkers(ch, &wg)
	return &wg
}

func manageWorkers(ch chan struct{}, wg *sync.WaitGroup) {
	for {
		<-ch
		if testDone {
			break
		}
		wg.Add(1)
		go work(ch, wg)
	}
}

// todo: replace by context?
var testDone bool

func awaitWorkers(wg *sync.WaitGroup) {
	testDone = true
	wg.Done()
	wg.Wait()
}

func work(ch chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		// doesnt matter if req/resp gets swapped between workers!
		qIn <- nil
		w := <-qOut
		if w == nil {
			ch <- struct{}{}
			return
		}
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
	qIn <- req
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
	msg, ok := inbox[str]
	if ok {
		delete(inbox, str)
	} else {
		suspendedRec[str] = recFn
	}
	recMut.Unlock()
	if !ok {
		return
	}
	qIn <- receiveWork{
		msg: msg,
		fn:  recFn,
	}
}

// guarantee: there will not be another message still waiting to be received
// because we only send upon request
func send(receiver stream, msg Message) {
	recMut.Lock()
	fn, ok := suspendedRec[receiver]
	if ok {
		delete(suspendedRec, receiver)
	} else {
		inbox[receiver] = msg
	}
	recMut.Unlock()
	if !ok {
		return
	}
	fn(msg)
}

func manageQueue(q []any) {
	for {
		req := <-qIn
		if req == nil {
			// request for item
			if len(q) == 0 {
				qOut <- nil
				continue
			}
			v := q[0]
			q = q[1:]
			qOut <- v
			continue
		}
		q = append(q, req)
	}
}
