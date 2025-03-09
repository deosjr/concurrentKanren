package main

import (
	"context"
)

type stream struct {
	out chan state
	ctx context.Context
}

func newStream(ctx context.Context) stream {
	return stream{
		out: make(chan state),
		ctx: ctx,
	}
}

func (s stream) send(st state) bool {
	select {
	case <-s.ctx.Done():
		return false
	case s.out <- st:
		return true
	}
}

func (s stream) receive() (state, bool) {
	select {
	case <-s.ctx.Done():
		return state{}, false
	case v, ok := <-s.out:
		if ok && v.delayed != nil {
			go v.delayed()
		}
		return v, ok
	}
}

// link two streams: send in from parent to child, out from child to parent
func link(parent, child stream) {
Loop:
	for {
		select {
		case <-parent.ctx.Done():
			break Loop
		case st, ok := <-child.out:
			if !ok {
				break Loop
			}
			if !parent.send(st) {
				break Loop
			}
		}
	}
	close(parent.out)
}

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

func takeAll(str stream) []state {
	states := []state{}
	for st := range str.out {
		if st.delayed != nil {
            go st.delayed()
			continue
		}
		states = append(states, st)
	}
	return states
}

func takeN(n int, str stream) []state {
	states := []state{}
	i := 0
	for i < n {
		st, ok := <-str.out
		if !ok {
			return states
		}
		if st.delayed != nil {
            go st.delayed()
			continue
		}
		states = append(states, st)
		i++
	}
	return states
}
