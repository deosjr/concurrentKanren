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
	done := str.getRequest()
	if done {
		for _, s := range streams {
			s.sendDone()
		}
		str.close()
		return
	}
	if len(buffer) > 0 {
		str.sendState(buffer[0])
		mplusplus(str, buffer[1:], streams)
		return
	}
	if len(streams) != 0 {
		panic("should never happen: productive streams remain but we didn't find anything to return?")
	}
	str.sendClose()
}

func refillBuffer(str stream, streams []stream) (buffer []state, active []stream) {
	for _, s := range streams {
		s.request()
		rec, ok := s.receive()
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
