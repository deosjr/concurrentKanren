package main

import (
	"context"
)

type goal func(context.Context, state) stream

func equalo(u, v expression) goal {
	return func(ctx context.Context, st state) stream {
		str := newStream()
		reqch <- wInit(ctx, func() {
			s, ok := st.sub.unify(u, v)
			if !ok {
				reqch <- wClose(str)
				return
			}
			newst := state{sub: s, vc: st.vc}
			reqch <- wSend(ctx, str, newst, func() {
				reqch <- wClose(str)
			})
		})
		return str
	}
}

func callfresh(f func(x expression) goal) goal {
	return func(ctx context.Context, st state) stream {
		v := variable(st.vc)
		newstate := state{sub: st.sub, vc: st.vc + 1}
		return f(v)(ctx, newstate)
	}
}

func disj(g1, g2 goal) goal {
	return func(ctx context.Context, st state) stream {
		str := newStream()
		reqch <- mplus(ctx, str, g1(ctx, st), g2(ctx, st))
		return str
	}
}

func mplus(ctx context.Context, str, str1, str2 stream) work {
	return wReceive(ctx, str1, func(m message) {
		if !m.ok {
			reqch <- wForward(str2, str)
			return
		}
		if m.st.delayed != nil {
			m.st.delayed()
			reqch <- mplus(ctx, str, str2, str1)
		} else {
			reqch <- wSend(ctx, str, m.st, func() {
				reqch <- mplus(ctx, str, str2, str1)
			})
		}
	})
}

func conj(g1, g2 goal) goal {
	return func(ctx context.Context, st state) stream {
		str := newStream()
		reqch <- bind(ctx, str, g1(ctx, st), g2)
		return str
	}
}

func bind(ctx context.Context, str, str1 stream, g goal) work {
    return wReceive(ctx, str1, func(m message) {
        if !m.ok {
            reqch <- wClose(str)
            return
        }
        if m.st.delayed != nil {
            reqch <- bind(ctx, str, str1, g)
            return
        }
        bstr := newStream()
        reqch <- bind(ctx, bstr, str1, g)
        reqch <- mplus(ctx, str, g(ctx, m.st), bstr)
    })
}

func disj_plus(goals ...goal) goal {
	if len(goals) == 1 {
		return goals[0]
	}
	return disj(goals[0], disj_plus(goals[1:]...))
}

func conj_plus(goals ...goal) goal {
	if len(goals) == 1 {
		return goals[0]
	}
	return conj(goals[0], conj_plus(goals[1:]...))
}

func run(goals ...goal) []expression {
	ctx, cancel := context.WithCancel(context.Background())
	g := conj_plus(goals...)
	out := mKreify(takeAll(ctx, g(ctx, emptystate)))
	cancel()
	return out
}

func runN(n int, goals ...goal) []expression {
	ctx, cancel := context.WithCancel(context.Background())
	g := conj_plus(goals...)
	out := mKreify(takeN(ctx, n, g(ctx, emptystate)))
	cancel()
	return out
}

func mKreify(states []state) []expression {
	exprs := []expression{}
	for _, st := range states {
		exprs = append(exprs, st.sub.walkstar(variable(0)))
	}
	return exprs
}

// missing macros here. go:generate could be used perhaps
// for now we duplicate the implementation of callfresh

func fresh1(f func(expression) goal) goal {
	return func(ctx context.Context, st state) stream {
		x := variable(st.vc)
		newstate := state{sub: st.sub, vc: st.vc + 1}
		return f(x)(ctx, newstate)
	}
}

func fresh2(f func(expression, expression) goal) goal {
	return func(ctx context.Context, st state) stream {
		x := variable(st.vc)
		y := variable(st.vc + 1)
		newstate := state{sub: st.sub, vc: st.vc + 2}
		return f(x, y)(ctx, newstate)
	}
}

func fresh3(f func(expression, expression, expression) goal) goal {
	return func(ctx context.Context, st state) stream {
		x := variable(st.vc)
		y := variable(st.vc + 1)
		z := variable(st.vc + 2)
		newstate := state{sub: st.sub, vc: st.vc + 3}
		return f(x, y, z)(ctx, newstate)
	}
}
