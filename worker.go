package main

import (
	"container/heap"
	"math/rand/v2"
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
	pq = NewPriorityQueue()
	pqIn = make(chan any, 1)
	pqOut = make(chan any)
	pqMut       sync.Mutex
)

func startWorkers() *sync.WaitGroup {
	testDone = false
	var wg sync.WaitGroup
	wg.Add(1)
	ch := make(chan struct{}, numWorkers)
	for range numWorkers {
		ch <- struct{}{}
	}
	go managePriorityQueue(pq)
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
		pqMut.Lock()
		pqIn <- nil
		w := <-pqOut
		pqMut.Unlock()
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
	pqIn <- req
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
	pqIn <- receiveWork{
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

// heap implementation from https://pkg.go.dev/container/heap
// An Item is something we manage in a priority queue.
type Item struct {
	value    any    // The value of the item; arbitrary.
	priority int    // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return pq[i].priority > pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // don't stop the GC from reclaiming the item eventually
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func NewPriorityQueue() *PriorityQueue {
	var pq PriorityQueue
	heap.Init(&pq)
	return &pq
}

func Push(pq *PriorityQueue, work any) {
	item := &Item{
		value:    work,
		priority: rand.Int(),
	}
	heap.Push(pq, item)
}

func Pop(pq *PriorityQueue) (any, bool) {
	if pq.Len() == 0 {
		return nil, false
	}
	item := heap.Pop(pq).(*Item)
	return item.value, true
}

func managePriorityQueue(pq *PriorityQueue) {
	for {
		req := <-pqIn
		if req == nil {
			// request for item
			v, ok := Pop(pq)
			if !ok {
				pqOut <- nil
				continue
			}
			pqOut <- v
			continue
		}
		Push(pq, req)
	}
}
