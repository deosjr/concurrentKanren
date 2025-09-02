package main

// short-circuit evaluation for conj
// does more work, but can deal with infinite streams
// bind can deal with the first goal being unproductive and doesn't evaluate second goal
// the problem is when the first goal is infinitely looping and the second goal already failed
// we test for this case, short-circuiting if we find it, or returning to normal conj if we don't
// TODO: abstract beyond two goals

type conjSCEGoal struct {
	g1, g2 goal
}

func conj_sce(g1, g2 goal) goal {
	return conjSCEGoal{g1, g2}
}

func (c conjSCEGoal) Apply(st state) stream {
	str := newStream()
	str1 := conj(c.g1, c.g2).Apply(st)
	str2 := c.g2.Apply(st)
	conj_sce_request(str, str1, str2)
	return str
}

func conj_sce_request(str, str1, str2 stream) {
	registerRequest(str, func(sender stream, done bool) {
		if done {
			request(str, str1, true) // close
			request(str, str2, true) // close
			return
		}
		request(str, str1, false)
		request(str, str2, false)
		conj_sce_receive(str, str1, str2, sender)
	})
}

func conj_sce_receive(str, str1, str2, sender stream) {
	registerReceive(str, func(msg message) {
		switch msg.sender {
		case str1:
			// we can forward the msg to the original request
			switch msg.msgtype {
			case stateMessage:
				// we have produced at least one result, so give up on short-circuit check
				sendForwardWithState(str, sender, str1, msg.st)
				return
			case closeMessage, stateCloseMessage, forwardWithStateMessage:
				// we have produced at least one result, so give up on short-circuit check
				send(sender, msg)
				return
			case forwardMessage:
				str1 = msg.fwd
			case delayMessage:
			}
			request(str, str1, false)
			conj_sce_receive(str, str1, str2, sender)
		case str2:
			switch msg.msgtype {
			case closeMessage:
				// first result is close: short-circuit!
				sendClose(str, sender)
				return
			case forwardMessage:
				str2 = msg.fwd
			case delayMessage:
			default:
				// we have produced at least one result, so give up on short-circuit check
				conj_sce_forward(str, str1, sender)
				return
			}
			request(str, str2, false)
			conj_sce_receive(str, str1, str2, sender)
		}
	})
}

func conj_sce_forward(str, str1, sender stream) {
	// we have already determined sce is not needed
	// but we have already requested a result from str1
	// wait for that, forward it to whoever requested us,
	registerReceive(str, func(msg message) {
		// guaranteed to receive from str1 here
		switch msg.msgtype {
		case stateMessage:
			sendForwardWithState(str, sender, str1, msg.st)
			return
		case closeMessage, stateCloseMessage, forwardWithStateMessage:
			send(sender, msg)
			return
		case forwardMessage:
			str1 = msg.fwd
		case delayMessage:
		}
		request(str, str1, false)
		conj_sce_forward(str, str1, sender)
	})
}
