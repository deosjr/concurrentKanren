package main

// tempted to use a buffered channel, but we want the buffer size dynamic (?)
func disj_conc(goals ...goal) goal {
	return func(st state) stream {
		str := newStream()
		buffer := []state{}
		streams := []stream{}
		for _, g := range goals {
			streams = append(streams, g(st))
		}
		refillBuffer := func() {
			buffer = []state{}
			unproductive := map[int]struct{}{}
			for i, s := range streams {
				s.request()
				x, ok := s.receive()
				if !ok {
					unproductive[i] = struct{}{}
					continue
				}
				if x.delayed {
					continue
				}
				buffer = append(buffer, x)
			}
			active := []stream{}
			for i, s := range streams {
				if _, ok := unproductive[i]; ok {
					continue
				}
				active = append(active, s)
			}
			streams = active
		}
		var mplusplus func()
		mplusplus = func() {
			if !str.more() {
				for _, s := range streams {
					*s.in <- reqMsg{done: true}
					//close(*str.out)   // ?
				}
				return
			}
			for len(buffer) == 0 && len(streams) > 0 {
				refillBuffer()
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
}
