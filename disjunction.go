package main

type disjConcGoal struct {
	goals []goal
}

func disj_conc(goals ...goal) goal {
	return disjConcGoal{goals: goals}
}

func (dc disjConcGoal) Apply(st state) stream {
	str := newStream()
	streams := []stream{}
	for _, g := range dc.goals {
		s := g.Apply(st)
		streams = append(streams, s)
	}
	mplusplus(str, nil, streams)
	return str
}

func mplusplus(str stream, buffer []state, streams []stream) {
	if len(buffer) == 0 {
		refillBuffer(str, streams)
	} else {
		mplusplusWithBuffer(str, buffer, streams)
	}
}

func mplusplusWithBuffer(str stream, buffer []state, streams []stream) {
	registerRequest(str, func(sender stream, done bool) {
		if done {
			for _, s := range streams {
				request(str, s, true) // close
			}
			return
		}
		sendState(str, sender, buffer[0])
		mplusplus(str, buffer[1:], streams)
	})
}

func refillBuffer(str stream, streams []stream) {
	refillBufferN(str, streams, 0, nil, nil)
}

func refillBufferN(str stream, streams []stream, i int, buffer []state, active []stream) {
	if i == len(streams) {
		// done filling the buffer
		if len(active) == 0 && len(buffer) == 0 {
			registerRequest(str, func(sender stream, done bool) {
				if done {
					return
				}
				sendClose(str, sender)
			})
			return
		}
		if len(active) == 0 {
			mplusplusWithBuffer(str, buffer, nil)
			return
		}
		if len(buffer) == 0 {
			refillBuffer(str, active)
			return
		}
		mplusplusWithBuffer(str, buffer, active)
		return
	}
	s := streams[i]
	request(str, s, false)
	registerReceive(str, func(msg Message) {
		switch t := msg.(type) {
		case stateMessage:
			buffer = append(buffer, t.st)
			active = append(active, s)
		case stateCloseMessage:
			buffer = append(buffer, t.st)
		case closeMessage:
			break
		case forwardMessage:
			active = append(active, t.fwd)
		case forwardWithStateMessage:
			buffer = append(buffer, t.st)
			active = append(active, t.fwd)
		case delayMessage:
			active = append(active, s)
		}
		refillBufferN(str, streams, i+1, buffer, active)
	})
}
