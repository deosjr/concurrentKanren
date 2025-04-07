package main

func disj_conc(goals ...goal) goal {
	return func(st state) stream {
		str := newStream()
		streams := []stream{}
		for _, g := range goals {
			streams = append(streams, g(st))
		}
		go mplusplus(str, nil, streams)
		return str
	}
}

func mplusplus(str stream, buffer []state, streams []stream) {
	if len(buffer) == 0 {
		buffer, streams = refillBuffer(str, streams)
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
		mplusplus(str, buffer[1:], streams)
		return
	}
	if len(streams) != 0 {
		panic("should never happen: productive streams remain but we didn't find anything to return?")
	}
	sendClose(req.onto)
	str.close()
}

func refillBuffer(str stream, streams []stream) ([]state, []stream) {
	buffer := []state{}
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
	if len(active) == 0 {
		return buffer, nil
	}
	if len(buffer) == 0 {
		return refillBuffer(str, active)
	}
	return buffer, active
}
