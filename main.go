package main

import (
	"context"
	"fmt"
	"time"
)

func nevero() goal {
	return delay(func() goal { return nevero() })
}

func fives(x expression) goal {
	return disj(equalo(x, number(5)), delay(func() goal { return fives(x) }))
}

func sixes(x expression) goal {
	return disj(equalo(x, number(6)), delay(func() goal { return sixes(x) }))
}

func sevens(x expression) goal {
	return disj(equalo(x, number(7)), delay(func() goal { return sevens(x) }))
}

func main() {
	// modify equalo to sleep for a second, emulating a heavy goal
	slowEqualo := func(u, v expression) goal {
		return func(ctx context.Context, st state) stream {
			time.Sleep(1 * time.Second)
			return equalo(u, v)(ctx, st)
		}
	}

	// modify goal to give up after 100ms
	timeout100ms := func(g goal) goal {
		return func(ctx context.Context, st state) stream {
			// TODO: is cancel needed here?
			ctx, _ = context.WithTimeout(ctx, 100*time.Millisecond)
			return g(ctx, st)
		}
	}

	// second goal is cancelled after 100ms and starts cleanup early
	// it might still return x=6, as select is nondeterministic
	out := run(callfresh(func(x expression) goal {
		return disj(
			slowEqualo(x, number(5)),
			timeout100ms(slowEqualo(x, number(6))),
		)
	}))
	fmt.Println(out) // prints [5] or [5 6]
}
