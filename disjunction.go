package main

type disjConcNode struct {
    goals []goal
}

func disj_conc(goals ...goal) goal {
    return disjConcNode{goals}
}

// tempted to use a buffered channel, but we want the buffer size dynamic (?)
func (n disjConcNode) apply(st state) stream {
		str := newStream()
		buffer := []state{}
		streams := []stream{}
		for _, g := range n.goals {
			streams = append(streams, g.apply(st))
		}
		refillBuffer := func() {
			buffer = []state{}
			active := []stream{}
			for _, s := range streams {
                s.request()
				x, ok := s.receive()
				if !ok {
					continue
				}
				active = append(active, s)
				if x.delayed {
					continue
				}
				buffer = append(buffer, x)
			}
			streams = active
		}
		var mplusplus func()
		mplusplus = func() {
			for len(buffer) == 0 && len(streams) > 0 {
				refillBuffer()
			}
			if !str.more() {
				for _, s := range streams {
					*s.in <- reqMsg{done: true}
				}
				close(*str.out)
				return
			}
			if len(buffer) > 0 {
				str.send(buffer[0])
				buffer = buffer[1:]
				mplusplus()
				return
			}
			if len(streams) == 0 {
				close(*str.out)
				return
			}
			panic("should never happen: productive streams remain but we didn't find anything to return?")
		}
		go mplusplus()
		return str
}
