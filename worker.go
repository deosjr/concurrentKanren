package main

import (
	"fmt"
	"sync"
)

// three kinds of work: init, request and receive
// when a goal is applied to a state, it spawns init work
// this eventually results in a yield, when waiting for request
// sender of request will suspend on receive
// upon request, further requests of subgoals may be needed before we can send
// receive resumes when a value exists (can be immediate without suspend!)
// sending does _not_ yield, only resume some more tasks

type initWork struct {
	goal goal
	str  stream
	st   state
}

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
	pool         sync.Pool
	reqMut       sync.Mutex
	recMut       sync.Mutex
	requests     = map[stream]requestWork{}
	suspendedReq = map[stream]reqFn{}
	inbox        = map[stream]Message{}
	suspendedRec = map[stream]receiveFn{}
)

func startWorkers() *sync.WaitGroup {
	var wg sync.WaitGroup
	ch := make(chan struct{}, numWorkers)
	for range numWorkers {
		ch <- struct{}{}
	}
	go manageWorkers(ch, &wg)
	return &wg
}

func manageWorkers(ch chan struct{}, wg *sync.WaitGroup) {
	for {
		<-ch
		wg.Add(1)
		go work(ch, wg)
	}
}

func awaitWorkers(wg *sync.WaitGroup) {
	wg.Wait()
}

func work(ch chan struct{}, wg *sync.WaitGroup) {
	for {
		w := pool.Get()
		if w == nil {
			ch <- struct{}{}
			wg.Done()
			return
		}
		fmt.Printf("WORK %#v\n", w)
		switch t := w.(type) {
		case initWork:
			t.goal.Init(t.str, t.st)
		case requestWork:
			t.fn(t.sender, t.done)
		case receiveWork:
			t.fn(t.msg)
		}
	}
}

// goal application spawns work
// todo: does this really have to exist? cant this be done on goal creation
// in the creating thread?
func registerInit(g goal, st state) stream {
	str := newStream()
	pool.Put(initWork{
		goal: g,
		str:  str,
		st:   st,
	})
	return str
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
	fmt.Printf("REG REQ %d %#v %t\n", str, req, ok)
	if !ok {
		return
	}
	req.fn = reqFn
	pool.Put(req)
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
	fmt.Printf("REQ %d %d %t\n", sender, receiver, ok)
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
	fmt.Printf("REG REC %d %#v %t\n", str, msg, ok)
	if !ok {
		return
	}
	pool.Put(receiveWork{
		msg: msg,
		fn:  recFn,
	})
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
	fmt.Printf("SEND %d %d %t\n", msg.Sender(), receiver, ok)
	if !ok {
		return
	}
	fn(msg)
}
