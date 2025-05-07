package main

// difference between muKanren in Scheme and this:
// delay is tracked on a state in the stream, not the stream itself
type stream []state

// needs to already be passed a thunk because we dont have macros
func delay(f func() goal) goal {
	return func(st state) stream {
		return stream{state{delayed: func() stream {
			return f()(st)
		}}}
	}
}

func pull(str stream) (stream, bool) {
	out := stream{}
	mature := false
	for _, s := range str {
		if s.delayed == nil {
			mature = true
		}
		if !mature {
			substr, ok := pull(s.delayed())
			if ok {
				mature = true
			}
			out = append(out, substr...)
			continue
		}
		out = append(out, s)
	}
	return out, mature
}

func takeAll(str stream) []state {
	states := []state{}
	for {
		matured, ok := pull(str)
		if !ok {
			return states
		}
		st := matured[0]
		str = matured[1:]
		states = append(states, st)
	}
}

func takeN(n int, str stream) []state {
	states := []state{}
	for len(states) < n {
		matured, ok := pull(str)
		if !ok {
			return states
		}
		st := matured[0]
		str = matured[1:]
		states = append(states, st)
	}
	return states
}
