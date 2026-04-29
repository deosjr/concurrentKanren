package main

import (
	"runtime"
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
	mutexShards = 100
)

// synchronous, when true, makes registerRequest/registerReceive bypass the
// work queue and invoke the callback inline. Used for benchmarking / when the
// caller knows the goal tree has no real parallelism to exploit.
var synchronous bool

// Arrays instead of maps: shard index is always 0..mutexShards-1,
// so direct array access is faster (no hash, no pointer chase, no GC overhead).
var (
	reqMuts      [mutexShards]sync.Mutex
	recMuts      [mutexShards]sync.Mutex
	requests     [mutexShards]map[stream]requestWork
	suspendedReq [mutexShards]map[stream]reqFn
	// inbox stores the first pending message per stream as a value (no heap
	// allocation for the message itself). Zero sender indicates "no message".
	// Streams start at 1, so sender==0 is a reliable absent sentinel.
	inbox     [mutexShards]map[stream]message
	// inboxFull holds additional messages for streams that accumulate >1
	// before being consumed. Only conj_sce triggers this (rare).
	inboxFull    [mutexShards]map[stream][]message
	suspendedRec [mutexShards]map[stream]receiveFn
	globalQueue  *workQueue
)

// workQueue holds two typed slices behind a single mutex+cond.
// Storing requestWork and receiveWork by value (not as interface{})
// avoids the per-item heap allocation that any-boxing would require.
type workQueue struct {
	mu       sync.Mutex
	cond     *sync.Cond
	reqItems []requestWork
	recItems []receiveWork
	closed   bool
}

func newWorkQueue() *workQueue {
	q := &workQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *workQueue) pushReq(w requestWork) {
	q.mu.Lock()
	q.reqItems = append(q.reqItems, w)
	q.cond.Signal()
	q.mu.Unlock()
}

func (q *workQueue) pushRec(w receiveWork) {
	q.mu.Lock()
	q.recItems = append(q.recItems, w)
	q.cond.Signal()
	q.mu.Unlock()
}

// pop returns the next work item. isReq distinguishes which kind was returned.
// Items are popped LIFO (from the end) so that the backing array can be
// properly truncated: old elements fall off the slice and their closure
// references become reclaimable by the GC immediately.
func (q *workQueue) pop() (isReq bool, req requestWork, rec receiveWork, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.reqItems) == 0 && len(q.recItems) == 0 && !q.closed {
		q.cond.Wait()
	}
	if q.closed {
		return false, req, rec, false
	}
	if len(q.reqItems) > 0 {
		n := len(q.reqItems) - 1
		req = q.reqItems[n]
		q.reqItems[n] = requestWork{} // zero to release the fn closure reference
		q.reqItems = q.reqItems[:n]
		return true, req, rec, true
	}
	n := len(q.recItems) - 1
	rec = q.recItems[n]
	q.recItems[n] = receiveWork{} // zero to release the fn closure reference
	q.recItems = q.recItems[:n]
	return false, req, rec, true
}

func (q *workQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

func startWorkers() func() {
	// Allocate fresh per-shard maps each invocation. Re-using maps via clear()
	// races with orphan workers from a previous run that hasn't been cancelled
	// (e.g. test timeouts, panics): they still hold the per-shard mutex and
	// write to the maps, while clear() writes from this goroutine without the
	// lock. Fresh maps give orphans a dead map to scribble on; we get a clean
	// one. Cost is ~5 small map allocations per shard per run — negligible.
	for i := 0; i < mutexShards; i++ {
		requests[i] = map[stream]requestWork{}
		suspendedReq[i] = map[stream]reqFn{}
		inbox[i] = map[stream]message{}
		inboxFull[i] = map[stream][]message{}
		suspendedRec[i] = map[stream]receiveFn{}
	}
	if synchronous {
		return func() {}
	}
	q := newWorkQueue()
	globalQueue = q
	numWorkers := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			work(q)
		}()
	}
	return func() {
		q.close()
		wg.Wait()
	}
}

func work(q *workQueue) {
	for {
		isReq, req, rec, ok := q.pop()
		if !ok {
			return
		}
		if isReq {
			req.fn(req.sender, req.done)
		} else {
			rec.fn(rec.msg)
		}
	}
}

// block waiting for more requests for work
func registerRequest(str stream, fn reqFn) {
	hash := int(str % mutexShards)
	reqMuts[hash].Lock()
	req, ok := requests[hash][str]
	if ok {
		delete(requests[hash], str)
	} else {
		suspendedReq[hash][str] = fn
	}
	reqMuts[hash].Unlock()
	if !ok {
		return
	}
	if synchronous {
		fn(req.sender, req.done)
		return
	}
	req.fn = fn
	globalQueue.pushReq(req)
}

// make a request for more work. suspend if no one is waiting for requests
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
	first, hasFirst := inbox[hash][str]
	if hasFirst {
		msg = first
		// promote the first overflow message (if any) into the primary slot
		if extra := inboxFull[hash][str]; len(extra) > 0 {
			inbox[hash][str] = extra[0]
			if len(extra) == 1 {
				delete(inboxFull[hash], str)
			} else {
				inboxFull[hash][str] = extra[1:]
			}
		} else {
			delete(inbox[hash], str)
		}
	} else {
		suspendedRec[hash][str] = recFn
	}
	recMuts[hash].Unlock()
	if !hasFirst {
		return
	}
	if synchronous {
		recFn(msg)
		return
	}
	globalQueue.pushRec(receiveWork{msg: msg, fn: recFn})
}

func send(receiver stream, msg message) {
	hash := int(receiver % mutexShards)
	recMuts[hash].Lock()
	fn, ok := suspendedRec[hash][receiver]
	if ok {
		delete(suspendedRec[hash], receiver)
	} else if _, hasFirst := inbox[hash][receiver]; !hasFirst {
		// Common path: no existing message — store inline, zero allocation.
		inbox[hash][receiver] = msg
	} else {
		// Rare path (conj_sce): stream already has a pending message.
		inboxFull[hash][receiver] = append(inboxFull[hash][receiver], msg)
	}
	recMuts[hash].Unlock()
	if !ok {
		return
	}
	fn(msg)
}
