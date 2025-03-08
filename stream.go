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
			select {
			case <-parent.ctx.Done():
				break Loop
			case parent.out <- st:
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
			select {
			case <-str.ctx.Done():
				close(str.out)
				return
			case str.out <- state{delayed: func() {
				link(str, f()(ctx, st))
			}}:
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
