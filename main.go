package main

import (
    "context"
    "fmt"
    "maps"
    "strings"
    //"time"
)

func main() {
    failo := func() goal { return equalo(number(1), number(2)) }

    out := run(failo())
    fmt.Println(out)    // prints []

    out = run(conj_sce(failo(), nevero()))
    fmt.Println(out)    // prints []

    out = run(conj_sce(nevero(), failo()))
    fmt.Println(out)    // conj diverges, conj_sce prints []
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

    cancel()
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
    m := maps.Clone(s)
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
        select {
        case str.in <- true:
        default:
            return
        }
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

// TODO: bind should still check context?
// in case str1 is a nondelayed nevero...
func bind(str, str1 stream, g goal) {
    str1.in <- true
    v, ok := <-str1.out
    if !ok {
        close(str.out)
        close(str1.in)
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

// short-circuit evaluation for conj
// does more work, but can deal with infinite streams
// bind can deal with the first goal being unproductive and doesn't evaluate second goal
// the problem is when the first goal is infinitely looping and the second goal already failed
// we test for this case, short-circuiting if we find it, or returning to normal conj if we don't
// TODO: abstract beyond two goals
func conj_sce(g1, g2 goal) goal {
    return func(st state) stream {
        str := newStream()
        str1 := conj(g1, g2)(st)
        str2 := g2(st)
        go func() {
            str1.in <- true
            str2.in <- true
            var f func()
            f = func() {
                select {
                case <-str.ctx.Done():
                    close(str.out)
                    close(str1.in)
                    close(str2.in)
                    return
                case st, ok := <-str1.out:
                    if !ok {
                        close(str.out)
                        close(str1.in)
                        close(str2.in)
                        return
                    }
                    if st.delayed {
                        str1.in <- true
                        go f()
                        return
                    }
                    close(str2.in)
                    str1.out <- st
                    link(str, str1)
                    return
                case st, ok := <-str2.out:
                    if !ok {
                        // short-circuit
                        close(str.out)
                        close(str1.in)
                        close(str2.in)
                        return
                    }
                    if st.delayed {
                        str2.in <- true
                        go f()
                        return
                    }
                }
                close(str2.in)
                bind(str, str1, g2)
            }
            f()
        }()
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
