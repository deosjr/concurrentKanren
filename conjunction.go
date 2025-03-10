package main

import (
	"context"
)

// short-circuit evaluation for conj
// does more work, but can deal with infinite streams
// bind can deal with the first goal being unproductive and doesn't evaluate second goal
// the problem is when the first goal is infinitely looping and the second goal already failed
// we test for this case, short-circuiting if we find it, or returning to normal conj if we don't
// TODO: abstract beyond two goals
func conj_sce(g1, g2 goal) goal {
	return func(ctx context.Context, st state) stream {
		str := newStream(ctx)
		str1 := conj(g1, g2)(ctx, st)
		ctx2, cancelStr2 := context.WithCancel(ctx)
		str2 := g2(ctx2, st)
		go func() {
			var f func()
			f = func() {
				select {
				case st, ok := <-str1.out:
					if !ok {
						close(str.out)
						return
					}
					if st.delayed != nil {
						go st.delayed()
						f()
						return
					}
					cancelStr2()
					select {
					case <-str.ctx.Done():
						close(str.out)
						return
					case str.out <- st:
					}
					link(str, str1)
					return
				case st, ok := <-str2.out:
					if !ok {
						// short-circuit
						close(str.out)
						return
					}
					if st.delayed != nil {
						go st.delayed()
						f()
						return
					}
					cancelStr2()
					link(str, str1)
				}
			}
			f()
		}()
		return str
	}
}
