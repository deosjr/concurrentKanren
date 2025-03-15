package main

type stream struct {
	in  *chan reqMsg
	out *chan stateMsg
}

type stateMsg struct {
	state state
	out   *chan stateMsg
}

type reqMsg struct {
	done bool
	in   *chan reqMsg
}

func newStream() stream {
	in := make(chan reqMsg, 1)
	out := make(chan stateMsg)
	return stream{
		in:  &in,
		out: &out,
	}
}

func newUnitStream() stream {
	out := make(chan stateMsg, 1)
	return stream{out: &out}
}

func (s stream) unit() bool {
	return cap(*s.out) == 1
}

func (s stream) request() {
	if s.unit() {
		return
	}
	*s.in <- reqMsg{done: false}
}

func (s stream) more() bool {
	if s.unit() {
		return len(*s.out) > 0
	}
	req := <-*s.in
	if req.done {
		return false
	}
	if req.in != nil {
		old := *s.in
		*s.in = *req.in
		close(old)
		return s.more()
	}
	return true
}

func (s stream) send(st state) {
	*s.out <- stateMsg{state: st}
}

func (s stream) receive() (state, bool) {
	msg, ok := <-*s.out
	if ok && msg.out != nil {
		old := *s.out
		*s.out = *msg.out
		close(old)
		return s.receive()
	}
	return msg.state, ok
}

// link two streams
func link(parent, child stream) {
	if !child.unit() {
		*child.in <- reqMsg{in: parent.in}
	}
	*parent.out <- stateMsg{out: child.out}
}

func delay(f func() goal) goal {
	return func(st state) stream {
		str := newStream()
		go func() {
			if !str.more() {
				close(*str.out)
				return
			}
			str.send(state{delayed: true})
			link(str, f()(st))
		}()
		return str
	}
}

func takeAll(str stream) []state {
	states := []state{}
	for {
		str.request()
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
		str.request()
		st, ok := str.receive()
		if !ok {
			if !str.unit() {
				*str.in <- reqMsg{done: true}
			}
			return states
		}
		if st.delayed {
			continue
		}
		states = append(states, st)
		i++
	}
	if !str.unit() {
		*str.in <- reqMsg{done: true}
	}
	str.receive()
	return states
}
