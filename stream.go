package main

import (
	"context"
	"math/rand/v2"
	"sync"
)

// functionality of a stream is mostly subsumed by work
// what remains is a unique identifier, more akin to a pid
type stream int64

// TODO: something less collision-prone
func newStream() stream {
	return stream(rand.Int64())
}

func delay(f func() goal) goal {
	return func(ctx context.Context, st state) stream {
		str := newStream()
		delayed := state{delayed: func() {
			reqch <- wForward(f()(ctx, st), str)
		}}
		reqch <- wSend(ctx, str, delayed, func() {})
		return str
	}
}

/*
// TODO: delay currently relies on receiver to continue the delayed function
// especially if distributed over multiple machines, this moves the calculation
// upwards in a way we do not want. Something to investigate further
func delay(f func() goal) goal {
	return func(ctx context.Context, st state) stream {
		str := newStream(ctx)
		go func() {
			if !str.send(state{delayed: func() {
				link(str, f()(ctx, st))
			}}) {
				close(str.out)
				return
			}
		}()
		return str
	}
}
*/

func takeAll(ctx context.Context, str stream) []state {
	states := []state{}
	var wg sync.WaitGroup
	var f func(message)
	f = func(m message) {
		if !m.ok {
			wg.Done()
			return
		}
		// delay?
		states = append(states, m.st)
		reqch <- wReceive(ctx, str, f)
	}
	wg.Add(1)
	reqch <- wReceive(ctx, str, f)
	wg.Wait()
	return states
}

func takeN(ctx context.Context, n int, str stream) []state {
	states := []state{}
	var wg sync.WaitGroup
	var f func(message)
	f = func(m message) {
		if !m.ok || n == 0 {
			wg.Done()
			return
		}
		// delay?
		states = append(states, m.st)
		n = n - 1
		reqch <- wReceive(ctx, str, f)
	}
	wg.Add(1)
	reqch <- wReceive(ctx, str, f)
	wg.Wait()
	return states
}
