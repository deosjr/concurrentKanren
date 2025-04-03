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
			active := []stream{}
			for _, s := range streams {
				str.request(s)
				rec, ok := <-str.rec
				if !ok {
					panic("disj_conc read on closed channel")
				}
				switch {
				case rec.isState():
					buffer = append(buffer, rec.st)
					active = append(active, s)
				case rec.isStateAndClose():
					buffer = append(buffer, rec.st)
				case rec.isClose():
					continue
				case rec.isForward():
					active = append(active, rec.fwd)
				case rec.isForwardWithState():
					buffer = append(buffer, rec.st)
					active = append(active, rec.fwd)
				case rec.isDelay():
					active = append(active, s)
				}
			}
			streams = active
		}
		var mplusplus func()
		mplusplus = func() {
			for len(buffer) == 0 && len(streams) > 0 {
				refillBuffer()
			}
			req := <-str.req
			if req.done {
				for _, s := range streams {
					sendDone(s.req)
				}
				str.close()
				return
			}
			if len(buffer) > 0 {
				sendState(req.onto, buffer[0])
				buffer = buffer[1:]
				mplusplus()
				return
			}
			if len(streams) == 0 {
				sendClose(req.onto)
				str.close()
				return
			}
			panic("should never happen: productive streams remain but we didn't find anything to return?")
		}
		go mplusplus()
		return str
	}
}
