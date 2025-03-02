# concurrentKanren

concurrentKanren is a [µKanren](http://webyrd.net/scheme-2013/papers/HemannMuKanren2013.pdf) implementation focused on concurrency.
It is inspired by earlier experimentation in [FLENG](https://gitlab.com/b2495/fleng), see [flengKanren](https://github.com/deosjr/flengKanren).

## Model

In µKanren streams are either lists or functions of zero arguments (thunks): a function signals an immature stream.
In concurrentKanren a stream consists of two channels: one signals more answers are requested from upstream, and one to return answers by.
Streams are managed by goroutines, which need to be cleaned up by context cancellation when less answers are requested than are available.
The implementation of `run` shows what this looks like in practise:

```go
func run(goals ...goal) []expression {
    ctx, cancel := context.WithCancel(context.Background()) 
    globalCtx = ctx

    g := conj_plus(goals...)
    out := mKreify(takeAll(g(emptystate)))
    cancel()
    return out
}
```

## Concurrent disjunction

The original implementation of disjunction uses binary trampolining to alternate between streams:

```go
out := runN(9, callfresh(func(x expression) goal { return disj_plus(fives(x), sixes(x), sevens(x)) }))
fmt.Println(out)    // prints [5 6 5 7 5 6 5 7 5]
```

Concurrent disjunction treats each goal equal in terms of fairness, and uses a buffer to cache evaluations:

```go
out = runN(9, callfresh(func(x expression) goal { return disj_conc(fives(x), sixes(x), sevens(x)) }))
fmt.Println(out)    // prints [5 6 7 5 6 7 5 6 7]
```

Both can deal with unproductive but nonterminating streams.

```go
func nevero() goal {
    return delay(func() goal { return nevero() })
}

// will print the exact same as the previous example
out = runN(9, callfresh(func(x expression) goal { return disj_conc(nevero(), fives(x), sixes(x), sevens(x)) }))
```
