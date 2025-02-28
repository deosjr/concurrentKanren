package main

import (
    "context"
    "fmt"
    "strings"
    "time"
)

func main() {
    defer stopWorkers()
    startWorkers(100)

/*
    p := pair{variable(1), pair{number(2), pair{variable(3), emptylist}}}
    fmt.Println(p.display())
    s := substitution{}
    s[variable(1)] = number(3)
    out := s.walk(variable(1))
    fmt.Println(out.display())

    g := callfresh(func(q expression) goal { return equalo(q, number(5)) })
    str := g(emptystate)
    fmt.Println(takeAll(str))

    g = callfresh(func(q expression) goal { return disj(equalo(q, number(5)), equalo(q, number(6))) })
    str = g(emptystate)
    fmt.Println(takeAll(str))

    // TODO: takeN isn't closing the stream, so workers are forever producing even when we stopped reading
    // this will impact performance of the next run!
    // Idiomatic way to solve this is using context cancellation
    g = callfresh(func(x expression) goal { return fives(x) })
    str = g(emptystate)
    fmt.Println(takeN(3, str))

    //g = callfresh(func(x expression) goal { return disj(fives(x), disj(sixes(x), sevens(x))) })
    //str = g(emptystate)
    //fmt.Println(takeN(9, str))

    //g = callfresh(func(x expression) goal { return disj_plus(fives(x), sixes(x), sevens(x)) })
    //str = g(emptystate)
    //fmt.Println(takeN(9, str))

    g := callfresh(func(x expression) goal { return disj_conc(fives(x), sixes(x), sevens(x)) })
    str := g(emptystate)
    fmt.Println(takeN(9, str))

    g := callfresh(func(x expression) goal { return disj_conc(nevero(), fives(x), sixes(x), sevens(x)) })
    str := g(emptystate)
    fmt.Println(takeN(9, str))
*/
    g := callfresh(func(x expression) goal { return callfresh(func(y expression) goal { return conj(equalo(x,number(5)), equalo(y,number(6))) }) })
    str := g(emptystate)
    fmt.Println(takeAll(str))
}

type expression interface {
    display() string
}

type number int

func (n number) display() string {
    return fmt.Sprintf("%d", n)
}

type variable int

func (v variable) display() string {
    return fmt.Sprintf("#%d", v)
}

type special uint8

const (
    emptylist special = iota
)

func (s special) display() string {
    switch s {
    case emptylist: return "()"
    default: panic("unknown special")
    }
}

type pair struct {
    car expression
    cdr expression
}

func (p pair) display() string {
    car := p.car.display()
    if p.cdr == emptylist {
        return "(" + car + ")"
    }
    cdr, ok := p.cdr.(pair)
    if !ok { panic("not a list") }
    s := cdr.displayRec(nil)
    return "(" + car + " " + strings.Join(s, " ") + ")"
}

func (p pair) displayRec(s []string) []string {
    car := p.car.display()
    s = append(s, car)
    if p.cdr == emptylist {
        return s
    }
    cdr, ok := p.cdr.(pair)
    if !ok { panic("not a list") }
    return cdr.displayRec(s)
}

type substitution map[variable]expression

func (s substitution) walk(u expression) expression {
    uvar, ok := u.(variable)
    if !ok {
        return u
    }
    e, ok := s[uvar]
    if !ok {
        return u
    }
    return s.walk(e)
}

// TODO: immutable maps
func (s substitution) extend(v variable, e expression) substitution {
    m := substitution{}
    for k, v := range s {
        m[k] = v
    }
    m[v] = e
    return m
}

func (s substitution) unify(u, v expression) (substitution, bool) {
    u0 := s.walk(u)
    v0 := s.walk(v)
    if u0 == v0 {
        return s, true
    }
    uvar, uok := u0.(variable)
    vvar, vok := v0.(variable)
    if uok && vok && uvar == vvar {
        return s, true
    }
    if uok {
        return s.extend(uvar, v0), true
    }
    if vok {
        return s.extend(vvar, u0), true
    }
    upair, uok := u0.(pair)
    vpair, vok := v0.(pair)
    if uok && vok {
        s0, ok := s.unify(upair.car, vpair.car)
        if !ok {
            return nil, false
        }
        s1, ok := s0.unify(upair.cdr, vpair.cdr)
        if !ok {
            return nil, false
        }
        return s1, true
    }
    return nil, false
}

type state struct {
    sub substitution
    vc int
    delayed bool    // signals an immature stream
}

var emptystate = state{
    sub: substitution{},
    vc:  0,
}

type stream struct {
    ch chan state
    ctx context.Context
}

func newStream() stream {
    ch := make(chan state, 1)
    return stream{ch:ch, ctx:nil}
}

type goal func(state) stream

type work func()

var workpool chan work

func startWorkers(numWorkers int) {
    // TODO: does the buffer grow exponentially and thus potentially block?
    workpool = make(chan work, numWorkers * 100)
    for i:=0; i<numWorkers; i++ {
        go worker()
    }
}

func stopWorkers() {
    close(workpool)
}

// worker takes the role of pull and take
func worker() {
    for f := range workpool {
        f()
    }
}

// TODO: without the timeout we can reach deadlock when all workers are trying to
// forward onto or read from streams that aren't making progress.
// There must be a better solution here (increasing workers helps but doesn't scale)
func forward(ch chan state, str stream) {
    select {
    case st, ok := <- str.ch:
        if !ok {
            close(ch)
            return
        }
        ch <- st
        workpool <- func() { forward(ch, str) }
    case <-time.After(1*time.Millisecond):
        workpool <- func() { forward(ch, str) }
    }
/*
    st, ok := <-str.ch
    if !ok {
        close(ch)
        return
    }
    ch <- st
    workpool <- func() { forward(ch, str) }
*/
}

// equalo is the only goal that doesnt queue work
func equalo(u, v expression) goal {
    //time.Sleep(1*time.Second)
    return func(st state) stream {
        str := newStream()
        s, ok := st.sub.unify(u, v)
        if ok {
            str.ch <- state{ sub:s, vc:st.vc }
        }
        close(str.ch)
        return str
    }
}

func callfresh(f func (x expression) goal) goal {
    return func(st state) stream {
        v := variable(st.vc)
        newstate := state{ sub:st.sub, vc:st.vc+1 }
        return f(v)(newstate)
    }
}

func disj(g1, g2 goal) goal {
    return func(st state) stream {
        str := newStream()
        str1 := g1(st)
        str2 := g2(st)
        workpool <- func() {
            mplus(str.ch, str1, str2)
        }
        return str
    }
}

func mplus(ch chan state, str1, str2 stream) {
    v, ok := <-str1.ch
    if !ok {
        workpool <- func() { forward(ch, str2) }
        return
    }
    if !v.delayed {
        ch <- v
    }
    workpool <- func() { mplus(ch, str2, str1) }
}

func conj(g1, g2 goal) goal {
    return func(st state) stream {
        str := newStream()
        str1 := g1(st)
        workpool <- func() {
            bind(str.ch, str1, g2)
        }
        return str
    }
}

func bind(ch chan state, str stream, g goal) {
    v, ok := <-str.ch
    if !ok {
        close(ch)
        return
    }
    if v.delayed {
        workpool <- func() { bind(ch, str, g) }
        return
    }
    bstr := newStream()
    workpool <- func() { bind(bstr.ch, str, g) }
    workpool <- func() { mplus(ch, g(v), bstr) }
}

func takeAll(str stream) []state {
    states := []state{}
    for st := range str.ch {
        states = append(states, st)
    }
    return states
}

func takeN(n int, str stream) []state {
    states := []state{}
    for i:=0; i<n; i++ {
        st, ok := <-str.ch
        if !ok {
            return states
        }
        states = append(states, st)
    }
    return states
}

// NOTE: creates a new stream for each delay, probably not that efficient
func delay(f func() goal) goal {
    return func(st state) stream {
        str := newStream()
        str.ch <- state{delayed:true}
        workpool <- func() { forward(str.ch, f()(st)) }
        return str
    }
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

func disj_plus(goals ...goal) goal {
    if len(goals) == 1 {
        return goals[0]
    }
    return disj(goals[0], disj_plus(goals[1:]...))
}

func disj_conc(goals ...goal) goal {
    return func(st state) stream {
        str := newStream()
        buffer := []state{}
        streams := []stream{}
        for _, g := range goals {
            streams = append(streams, g(st))
        }
        var f func()
        f = func() {
            if len(buffer) > 0 {
                str.ch <- buffer[0]
                buffer = buffer[1:]
                workpool <- f
                return
            }
            if len(streams) == 0 {
                close(str.ch)
                return
            }
            // refill the buffer
            buffer = []state{}
            unproductive := map[int]struct{}{}
            for i, in := range streams {
                x, ok := <- in.ch
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
            workpool <- f
        }
        workpool <- f
        return str
    }
}

func nevero() goal {
    return delay(func() goal { return nevero() })
}
