// disj_par: branch-isolated OR-parallel disjunction.
//
// Direct port of pkanren.erl's pdisj idea (../erlangKanren). Each branch
// runs in its own dedicated goroutine, executing its full machinery
// internally — no shared dispatch through the global worker pool.
// Answers from branches stream back via a Go channel and are forwarded
// to the output stream as they arrive.
//
// This differs from disj_conc, which Applies all branches eagerly and
// then routes their messages through the same global queue, sharing
// GOMAXPROCS workers across branches. The result is that disj_conc
// barely outperforms disj_plus on OR-parallel workloads (we measured
// ~5%). disj_par gives each branch its own goroutine, so the OS
// scheduler distributes branches across cores cleanly.
//
// Branches yield ONE answer at a time and wait on a `done` signal so
// the consumer can stop early (essential for take(N) on potentially
// infinite branch streams). Without this backpressure, recursive goals
// like evalO would have each branch grind through its full stream
// before the consumer ever sees a state.
//
// Synchronous mode is required during disj_par execution so each
// branch's machinery runs inline within its goroutine — see runPar.
package main

import (
	"runtime"
	"sync"
)

type disjParGoal struct {
	goals []goal
}

func disj_par(goals ...goal) goal {
	return disjParGoal{goals: goals}
}

// disjParWG tracks all currently active disj_par branch goroutines so
// runPar/runNPar can wait for them before resetting synchronous mode.
// (Branches that race with the mode reset would otherwise dispatch to
// the now-nil globalQueue.)
var disjParWG sync.WaitGroup

// disjSmartBudget is a buffered channel acting as a counting semaphore.
// Each disj_smart invocation tries to claim one slot at Apply time. If
// it succeeds, the disjunction runs in parallel (one goroutine per
// branch). If the budget is exhausted, it falls back to disj_plus
// (sequential interleaving). The slot is released when the parallel
// branches all finish.
//
// This bounds total parallelism, which is the key insight for making
// disj_smart work on RECURSIVE disjunctions like evalO: the top-level
// call grabs slots; nested recursive evalO calls inside each parallel
// branch find the budget consumed and run sequentially. No goroutine
// explosion.
//
// Budget defaults to GOMAXPROCS. Set explicitly via SetDisjSmartBudget.
var disjSmartBudget chan struct{}

func init() {
	disjSmartBudget = make(chan struct{}, runtime.GOMAXPROCS(0))
}

// SetDisjSmartBudget reconfigures the parallelism budget. Pass 0 to
// disable parallelism entirely (disj_smart always falls back to
// disj_plus). Must be called before any disj_smart invocation.
func SetDisjSmartBudget(n int) {
	if n < 0 {
		n = 0
	}
	disjSmartBudget = make(chan struct{}, n)
}

type disjSmartGoal struct {
	goals []goal
}

// disj_smart picks at runtime: if a parallelism slot is available,
// fans out as disj_par; otherwise falls back to disj_plus. The choice
// is made per Apply call, so deeply-nested recursions naturally cap
// at the budget.
func disj_smart(goals ...goal) goal {
	return disjSmartGoal{goals: goals}
}

func (d disjSmartGoal) Apply(st state) stream {
	select {
	case disjSmartBudget <- struct{}{}:
		// Got a slot — run parallel. Slot released when branches finish.
		return d.applyParallel(st)
	default:
		// Budget exhausted — fall back to sequential interleaving.
		return disj_plus(d.goals...).Apply(st)
	}
}

func (d disjSmartGoal) applyParallel(st state) stream {
	str := newStream()
	answers := make(chan state)
	done := make(chan struct{})

	var wg sync.WaitGroup
	for _, g := range d.goals {
		wg.Add(1)
		disjParWG.Add(1)
		go func(g goal) {
			defer wg.Done()
			defer disjParWG.Done()
			walkAndYield(g.Apply(st), answers, done)
		}(g)
	}
	go func() {
		wg.Wait()
		close(answers)
		<-disjSmartBudget // release slot
	}()

	bridgeChannelToStream(str, answers, done)
	return str
}

func (d disjParGoal) Apply(st state) stream {
	str := newStream()
	answers := make(chan state) // unbuffered: tight backpressure
	done := make(chan struct{})

	var wg sync.WaitGroup
	for _, g := range d.goals {
		wg.Add(1)
		disjParWG.Add(1)
		go func(g goal) {
			defer wg.Done()
			defer disjParWG.Done()
			walkAndYield(g.Apply(st), answers, done)
		}(g)
	}
	go func() {
		wg.Wait()
		close(answers)
	}()

	bridgeChannelToStream(str, answers, done)
	return str
}

// walkAndYield drives a stream synchronously (synchronous mode required),
// sending each state to `out`. Stops when `done` is closed.
//
// On exit, propagates a close request down to the current stream so any
// nested disj_par bridges close their own `done` channels and their
// branch goroutines unblock.
func walkAndYield(str stream, out chan<- state, done <-chan struct{}) {
	outStr := newStream()
	defer func() {
		// `str` is the latest forward target; signal close to it so
		// nested bridges propagate the stop downward.
		request(outStr, str, true)
	}()
	for {
		select {
		case <-done:
			return
		default:
		}
		var got message
		registerReceive(outStr, func(msg message) { got = msg })
		request(outStr, str, false)
		// In synchronous mode the chain unwinds and `got` is set
		// before request returns.
		switch got.msgtype {
		case stateMessage:
			select {
			case out <- got.st:
			case <-done:
				return
			}
		case stateCloseMessage:
			select {
			case out <- got.st:
			case <-done:
			}
			return
		case closeMessage:
			return
		case forwardMessage:
			str = got.fwd
		case forwardWithStateMessage:
			select {
			case out <- got.st:
			case <-done:
				return
			}
			str = got.fwd
		case delayMessage:
			// loop and re-request
		}
	}
}

// bridgeChannelToStream attaches a stream that, on each request, reads
// one value from `answers` and emits it as a state. When the consumer
// signals it's done (via request with done=true, or stream is closed
// from above), close the `done` channel so branch goroutines exit.
func bridgeChannelToStream(str stream, answers <-chan state, done chan<- struct{}) {
	stopped := false
	stop := func() {
		if !stopped {
			stopped = true
			close(done)
		}
	}
	var fn reqFn
	fn = func(sender stream, doneFlag bool) {
		if doneFlag {
			stop()
			// drain remaining answers so producers don't block
			go func() {
				for range answers {
				}
			}()
			return
		}
		s, ok := <-answers
		if !ok {
			stop()
			sendClose(str, sender)
			return
		}
		sendState(str, sender, s)
		registerRequest(str, fn)
	}
	registerRequest(str, fn)
}

// runPar is run() but forces synchronous mode for the duration so
// disj_par's branches run sequentially inside their goroutines (with
// real OS-thread parallelism between branches). Waits for all branch
// goroutines to finish before resetting the mode flag.
func runPar(goals ...goal) []expression {
	prev := synchronous
	synchronous = true
	cancel := startWorkers()
	g := conj_plus(goals...)
	stream := g.Apply(emptystate)
	out := mKreify(takeAll(stream))
	cancel()
	disjParWG.Wait()
	synchronous = prev
	return out
}

// runNPar is runN with synchronous mode forced on (for disj_par).
func runNPar(n int, goals ...goal) []expression {
	prev := synchronous
	synchronous = true
	cancel := startWorkers()
	g := conj_plus(goals...)
	stream := g.Apply(emptystate)
	out := mKreify(takeN(n, stream))
	cancel()
	disjParWG.Wait()
	synchronous = prev
	return out
}
