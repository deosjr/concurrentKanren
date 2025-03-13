package main

import (
	"context"
)

type stream struct {
	out *chan state
	ctx context.Context
}

func newStream(ctx context.Context) stream {
	ch := make(chan state)
	return stream{
		out: &ch,
		ctx: ctx,
	}
}

func (s stream) send(st state) bool {
	select {
	case <-s.ctx.Done():
		close(*s.out)
		return false
	case *s.out <- st:
		return true
	}
}

func (s stream) receive() (state, bool) {
	st, ok := <-*s.out
	if ok && st.link != nil {
		old := *s.out
		*s.out = *st.link
		close(old)
		return s.receive()
	}
	return st, ok
}

// link two streams: send out from child to parent
func link(parent, child stream) {
	parent.send(state{link: child.out})
}

func delay(f func() goal) goal {
	return func(ctx context.Context, st state) stream {
		str := newStream(ctx)
		go func() {
			if !str.send(state{delayed: true}) {
				return
			}
			link(str, f()(ctx, st))
		}()
		return str
	}
}

func takeAll(str stream) []state {
	states := []state{}
	for {
		st, ok := str.receive()
		if !ok {
			break
		}
		if st.delayed {
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
		st, ok := str.receive()
		if !ok {
			return states
		}
		if st.delayed {
			continue
		}
		states = append(states, st)
		i++
	}
	return states
}
