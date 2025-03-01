package main

import (
    "context"
    "fmt"
    "runtime"
    "strings"
    "time"
)

func main() {
/*
    g := callfresh(func(q expression) goal { return equalo(q, number(5)) })
    str := g(emptystate)
    fmt.Println(takeAll(str))

    g := callfresh(func(q expression) goal { return disj(equalo(q, number(5)), equalo(q, number(6))) })
    str := g(emptystate)
    fmt.Println(takeAll(str))

    g := callfresh(func(x expression) goal { return fives(x) })
    str := g(emptystate)
    fmt.Println(takeN(3, str))

    g := callfresh(func(x expression) goal { return disj(fives(x), disj(sixes(x), sevens(x))) })
    str := g(emptystate)
    fmt.Println(takeN(9, str))

*/
    out := runN(9, callfresh(func(x expression) goal { return disj_plus(fives(x), sixes(x), sevens(x)) }))
    fmt.Println(out)

    out = runN(9, callfresh(func(x expression) goal { return disj_conc(fives(x), sixes(x), sevens(x)) }))
    fmt.Println(out)

/*
    g := callfresh(func(x expression) goal { return disj_conc(nevero(), fives(x), sixes(x), sevens(x)) })
    str := g(emptystate)
    fmt.Println(takeN(9, str))

    g := callfresh(func(x expression) goal { return callfresh(func(y expression) goal { return conj(equalo(x,number(5)), equalo(y,number(6))) }) })
    str := g(emptystate)
    fmt.Println(takeAll(str))
*/
}

func run(goals ...goal) []expression {
    ctx, cancel := context.WithCancel(context.Background()) 
    globalCtx = ctx

    g := conj_plus(goals...)
    out := mKreify(takeAll(g(emptystate)))
    cancel()
    return out
}

func runN(n int, goals ...goal) []expression {
    ctx, cancel := context.WithCancel(context.Background()) 
    globalCtx = ctx

    g := conj_plus(goals...)
    out := mKreify(takeN(n, g(emptystate)))

    fmt.Println(runtime.NumGoroutine())
    cancel()
    time.Sleep(100*time.Millisecond)
    fmt.Println(runtime.NumGoroutine())
    return out
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

func (s substitution) walkstar(u expression) expression {
    v := s.walk(u)
    switch t := v.(type) {
    case variable:
        return t
    case pair:
        return pair{car: s.walkstar(t.car), cdr: s.walkstar(t.cdr)}
    }
    return v
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

var globalCtx context.Context

type stream struct {
    in  chan bool
    out chan state
    ctx context.Context
}

func newStream() stream {
    in := make(chan bool, 1)
    out := make(chan state, 1)
    return stream{in:in, out:out, ctx:globalCtx}
}

func (str stream) more() bool {
    select {
    case _, ok := <-str.in:
        return ok
    case <-str.ctx.Done():
        return false
    }
}

type goal func(state) stream

// link two streams: send in from parent to child, out from child to parent
func link(parent, child stream) {
    go func() {
        for b := range parent.in {
            child.in <- b
        }
        close(child.in)
    }()
    go func() {
        for st := range child.out {
            parent.out <- st
        }
        close(parent.out)
    }()
}

func equalo(u, v expression) goal {
    //time.Sleep(1*time.Second)
    return func(st state) stream {
        str := newStream()
        go func() {
            if !str.more() {
                close(str.out)
                return
            }
            s, ok := st.sub.unify(u, v)
            if ok {
                str.out <- state{ sub:s, vc:st.vc }
            }
            close(str.out)
        }()
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
        go func() {
            mplus(str, g1(st), g2(st))
        }()
        return str
    }
}

func mplus(str, str1, str2 stream) {
    if !str.more() {
        close(str.out)
        close(str1.in)
        close(str2.in)
        return
    }
    str1.in <- true
    v, ok := <-str1.out
    if !ok {
        str2.in <- true
        link(str, str2)
        return
    }
    if v.delayed {
        str.in <- true
    } else {
        str.out <- v
    }
    mplus(str, str2, str1)
}

func conj(g1, g2 goal) goal {
    return func(st state) stream {
        str := newStream()
        go func() {
            bind(str, g1(st), g2)
        }()
        return str
    }
}

func bind(str, str1 stream, g goal) {
    str1.in <- true
    v, ok := <-str1.out
    if !ok {
        close(str.out)
        return
    }
    if v.delayed {
        bind(str, str1, g)
        return
    }
    bstr := newStream()
    go func() {
        bind(bstr, str1, g)
    }()
    mplus(str, g(v), bstr)
}

func takeAll(str stream) []state {
    states := []state{}
    str.in <- true
    for st := range str.out {
        states = append(states, st)
        str.in <- true
    }
    close(str.in)
    return states
}

func takeN(n int, str stream) []state {
    states := []state{}
    str.in <- true
    for i:=0; i<n; i++ {
        st, ok := <-str.out
        if !ok {
            return states
        }
        states = append(states, st)
        str.in <- true
    }
    close(str.in)
    return states
}

func delay(f func() goal) goal {
    return func(st state) stream {
        str := newStream()
        go func() {
            if !str.more() {
                close(str.out)
                return
            }
            str.out <- state{delayed:true}
            link(str, f()(st))
        }()
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

func conj_plus(goals ...goal) goal {
    if len(goals) == 1 {
        return goals[0]
    }
    return conj(goals[0], conj_plus(goals[1:]...))
}

func disj_conc(goals ...goal) goal {
    return func(st state) stream {
        str := newStream()
        buffer := []state{}
        streams := []stream{}
        for _, g := range goals {
            streams = append(streams, g(st))
        }
        refillBuffer := func() {
            buffer = []state{}
            unproductive := map[int]struct{}{}
            for i, s := range streams {
                s.in <- true
                x, ok := <- s.out
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
        }
        var mplusplus func()
        mplusplus = func() {
            if !str.more() {
                for _, s := range streams {
                    close(s.in)
                }
                close(str.out)
                return
            }
            for len(buffer) == 0 && len(streams) > 0 {
                refillBuffer()
            }
            if len(buffer) > 0 {
                str.out <- buffer[0]
                buffer = buffer[1:]
                mplusplus()
                return
            }
            if len(streams) == 0 {
                close(str.out)
                return
            }
            panic("should never happen: productive streams remain but we didn't find anything to return?")
        }
        go mplusplus()
        return str
    }
}

func nevero() goal {
    return delay(func() goal { return nevero() })
}

func mKreify(states []state) []expression {
    exprs := []expression{}
    for _, st := range states {
        exprs = append(exprs, st.sub.walkstar(variable(0)))
    }
    return exprs
}
