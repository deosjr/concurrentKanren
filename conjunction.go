package main

// short-circuit evaluation for conj
// does more work, but can deal with infinite streams
// bind can deal with the first goal being unproductive and doesn't evaluate second goal
// the problem is when the first goal is infinitely looping and the second goal already failed
// we test for this case, short-circuiting if we find it, or returning to normal conj if we don't
// TODO: abstract beyond two goals
/*
func conj_sce(g1, g2 goal) goal {
	return func(st state) stream {
		str := newStream()
		str1 := conj(g1, g2)(st)
		str2 := g2(st)
		go func() {
			var f func()
			f = func() {
                if !str.more() {
                    *str1.in <- reqMsg{done:true}
                    *str2.in <- reqMsg{done:true}
                    close(*str.out)
                    return
                }
                str1.request()
                str2.request()
				select {
				case msg, ok := <-*str1.out:
					if !ok {
                        *str2.in <- reqMsg{done:true}
						close(*str.out)
						return
					}
	                if msg.out != nil { //link
	                    old := *str1.out
	                    *str1.out = *msg.out
	                    close(old)
                        str1.request()
                        f()
                        return
	                }
					if msg.state.delayed {
						f()
						return
					}
                    *str2.in <- reqMsg{done:true}
                    str.send(msg.state)
					link(str, str1)
					return
				case msg, ok := <-*str2.out:
					if !ok {
						// short-circuit
                        *str1.in <- reqMsg{done:true}
						close(*str.out)
						return
					}
	                if msg.out != nil { //link
	                    old := *str2.out
	                    *str2.out = *msg.out
	                    close(old)
                        str2.request()
                        f()
                        return
	                }
					if msg.state.delayed {
						f()
						return
					}
                    *str2.in <- reqMsg{done:true}
					link(str, str1)
				}
			}
			f()
		}()
		return str
	}
}
*/
