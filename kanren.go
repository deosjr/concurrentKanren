package main

import "sync"

type goal interface {
    apply(state) stream
}

type equaloNode struct {
    u, v expression
}

type eqKey struct { u, v expression }

var mem sync.Map

func equalo(u, v expression) goal {
    if e, ok := mem.Load(eqKey{u, v}); ok {
        return e.(*equaloNode)
    }
    n := &equaloNode{u, v}
    mem.Store(eqKey{u, v}, n)
    return n
}

func (n *equaloNode) apply(st state) stream {
	str := newBoundedStream(1)
	s, ok := st.sub.unify(n.u, n.v)
	if ok {
		str.send(state{sub: s, vc: st.vc})
	}
	close(*str.out)
	return str
}

type callfreshNode struct {
    f func(expression) goal
}

func callfresh(f func(x expression) goal) goal {
    return callfreshNode{f}
}

func (n callfreshNode) apply(st state) stream {
	v := variable(st.vc)
	newstate := state{sub: st.sub, vc: st.vc + 1}
	return n.f(v).apply(newstate)
}

type disjNode struct {
    g1, g2 goal
}

func disj(g1, g2 goal) goal {
    return disjNode{g1, g2}
}

func (n disjNode) apply(st state) stream {
	str := newStream()
	go mplus(str, n.g1.apply(st), n.g2.apply(st))
	return str
}

func mplus(str, str1, str2 stream) {
	if str1.bounded() {
		if !str.more() {
			*str2.in <- reqMsg{done: true}
			close(*str.out)
			return
		}
		st, ok := str1.receive()
		if !ok {
			str.request()
			link(str, str2)
		} else {
			sendAndLink(str, str2, st)
		}
		return
	}
	if !str.more() {
		*str1.in <- reqMsg{done: true}
		*str2.in <- reqMsg{done: true}
		close(*str.out)
		return
	}
	str1.request()
	st, ok := str1.receive()
	if !ok {
		str.request()
		link(str, str2)
		return
	}
	if st.delayed {
		str.request()
	} else {
		str.send(st)
	}
	mplus(str, str2, str1)
}

type conjNode struct {
    g1, g2 goal
}

func conj(g1, g2 goal) goal {
    return conjNode{g1, g2}
}

func (n conjNode) apply(st state) stream {
	str := newStream()
	go bind(str, n.g1.apply(st), n.g2)
	return str
}

func bind(str, str1 stream, g goal) {
	if str1.bounded() {
		if !str.more() {
			close(*str.out)
			return
		}
		st, ok := str1.receive()
		if !ok {
			close(*str.out)
			return
		}
		str.request()
		link(str, g.apply(st))
		return
	}
	str1.request()
	if !str.more() {
		*str1.in <- reqMsg{done: true}
		close(*str.out)
	}
	st, ok := str1.receive()
	if !ok {
		close(*str.out)
		return
	}
	str.request()
	if st.delayed {
		bind(str, str1, g)
		return
	}
	bstr := newStream()
	go func() {
		bind(bstr, str1, g)
	}()
	mplus(str, g.apply(st), bstr)
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
	g := conj_plus(goals...)
	out := mKreify(takeAll(g.apply(emptystate)))
	return out
}

func runN(n int, goals ...goal) []expression {
	g := conj_plus(goals...)
	out := mKreify(takeN(n, g.apply(emptystate)))
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

type fresh1Node struct {
    f func(expression) goal
}

func fresh1(f func(expression) goal) goal {
    return fresh1Node{f}
}

func (n fresh1Node) apply(st state) stream {
	x := variable(st.vc)
	newstate := state{sub: st.sub, vc: st.vc + 1}
	return n.f(x).apply(newstate)
}

type fresh2Node struct {
    f func(expression, expression) goal
}

func fresh2(f func(expression, expression) goal) goal {
    return fresh2Node{f}
}

func (n fresh2Node) apply(st state) stream {
	x := variable(st.vc)
	y := variable(st.vc + 1)
	newstate := state{sub: st.sub, vc: st.vc + 2}
	return n.f(x, y).apply(newstate)
}

type fresh3Node struct {
    f func(expression, expression, expression) goal
}

func fresh3(f func(expression, expression, expression) goal) goal {
    return fresh3Node{f}
}

func (n fresh3Node) apply(st state) stream {
	x := variable(st.vc)
	y := variable(st.vc + 1)
	z := variable(st.vc + 2)
	newstate := state{sub: st.sub, vc: st.vc + 3}
	return n.f(x, y, z).apply(newstate)
}

type fresh7Node struct {
    f func(expression, expression, expression, expression, expression, expression, expression) goal
}

func fresh7(f func(expression, expression, expression, expression, expression, expression, expression) goal) goal {
    return fresh7Node{f}
}

func (n fresh7Node) apply(st state) stream {
	x1 := variable(st.vc)
	x2 := variable(st.vc + 1)
	x3 := variable(st.vc + 2)
	x4 := variable(st.vc + 3)
	x5 := variable(st.vc + 4)
	x6 := variable(st.vc + 5)
	x7 := variable(st.vc + 6)
	newstate := state{sub: st.sub, vc: st.vc + 7}
	return n.f(x1, x2, x3, x4, x5, x6, x7).apply(newstate)
}
