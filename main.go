package main

import (
    "context"
    "fmt"
    "maps"
    "strings"
    "time"
)

func main() {
    // modify equalo to sleep for a second, emulating a heavy goal
    slowEqualo := func(u, v expression) goal { 
        return func(ctx context.Context, st state) stream {
            time.Sleep(1*time.Second)
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
    fmt.Println(out)    // prints [5] or [5 6]
}

func run(goals ...goal) []expression {
    ctx, cancel := context.WithCancel(context.Background()) 
    g := conj_plus(goals...)
    out := mKreify(takeAll(g(ctx, emptystate)))
    cancel()
    return out
}

func runN(n int, goals ...goal) []expression {
    ctx, cancel := context.WithCancel(context.Background()) 
    g := conj_plus(goals...)
    out := mKreify(takeN(n, g(ctx, emptystate)))
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
    delayed func()  // signals an immature stream
}

var emptystate = state{
    sub: substitution{},
    vc:  0,
}

type stream struct {
    out chan state
    ctx context.Context
}

func newStream(ctx context.Context) stream {
    return stream{
        out: make(chan state),
        ctx: ctx,
    }
}

type goal func(context.Context, state) stream

// link two streams: send in from parent to child, out from child to parent
func link(parent, child stream) {
Loop:
    for {
        select {
        case <-parent.ctx.Done():
            break Loop
        case st, ok := <-child.out:
            if !ok {
                break Loop
            }
            select {
            case <-parent.ctx.Done():
                break Loop
            case parent.out <- st:
            }
        }
    }
    close(parent.out)
}

func equalo(u, v expression) goal {
    return func(ctx context.Context, st state) stream {
        str := newStream(ctx)
        go func() {
            s, ok := st.sub.unify(u, v)
            if ok {
                select {
                case <-str.ctx.Done():
                    close(str.out)
                    return
                case str.out <- state{ sub:s, vc:st.vc }:
                }
            }
            close(str.out)
        }()
        return str
    }
}

func callfresh(f func (x expression) goal) goal {
    return func(ctx context.Context, st state) stream {
        v := variable(st.vc)
        newstate := state{ sub:st.sub, vc:st.vc+1 }
        return f(v)(ctx, newstate)
    }
}

func disj(g1, g2 goal) goal {
    return func(ctx context.Context, st state) stream {
        str := newStream(ctx)
        go func() {
            mplus(str, g1(ctx, st), g2(ctx, st))
        }()
        return str
    }
}

func mplus(str, str1, str2 stream) {
    var v state
    var ok bool
    select {
    case <-str.ctx.Done():
        close(str.out)
        return
    case v, ok = <-str1.out:
    }
    if !ok {
        go link(str, str2)
        return
    }
    if v.delayed != nil {
        go v.delayed()
    } else {
        select {
        case <-str.ctx.Done():
            close(str.out)
            return
        case str.out <- v:
        }
    }
    mplus(str, str2, str1)
}

// TODO: delay currently relies on receiver to continue the delayed function
// especially if distributed over multiple machines, this moves the calculation
// upwards in a way we do not want. Something to investigate further
func delay(f func() goal) goal {
    return func(ctx context.Context, st state) stream {
        str := newStream(ctx)
        go func() {
            select {
            case <-str.ctx.Done():
                close(str.out)
                return
            case str.out <- state{delayed:func() {
                link(str, f()(ctx, st))
            }}:
            }
        }()
        return str
    }
}

func conj(g1, g2 goal) goal {
    return func(ctx context.Context, st state) stream {
        str := newStream(ctx)
        go func() {
            bind(str, g1(ctx, st), g2)
        }()
        return str
    }
}

func bind(str, str1 stream, g goal) {
    var v state
    var ok bool
    select {
    case <-str.ctx.Done():
        close(str.out)
        return
    case v, ok = <-str1.out:
    }
    if !ok {
        close(str.out)
        return
    }
    if v.delayed != nil {
        go v.delayed()
        bind(str, str1, g)
        return
    }
    bstr := newStream(str.ctx)
    go func() {
        bind(bstr, str1, g)
    }()
    mplus(str, g(str.ctx, v), bstr)
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
    return func(ctx context.Context, st state) stream {
        str := newStream(ctx)
        buffer := []state{}
        streams := []stream{}
        for _, g := range goals {
            streams = append(streams, g(ctx, st))
        }
        refillBuffer := func() {
            buffer = []state{}
            unproductive := map[int]struct{}{}
            for i, s := range streams {
                var x state
                var ok bool
                select {
                case <-str.ctx.Done():
                    return
                case x, ok = <-s.out:
                }
                if !ok {
                    unproductive[i] = struct{}{}
                    continue
                }
                if x.delayed != nil {
                    go x.delayed()
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
            for len(buffer) == 0 && len(streams) > 0 {
                refillBuffer()
            }
            if len(buffer) > 0 {
                select {
                case <-str.ctx.Done():
                    close(str.out)
                    return
                case str.out <- buffer[0]:
                }
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
    return func(ctx context.Context, st state) stream {
        str := newStream(ctx)
        str1 := conj(g1, g2)(ctx, st)
        ctx2, cancelStr2 := context.WithCancel(ctx)
        str2 := g2(ctx2, st)
        go func() {
            var f func()
            f = func() {
                select {
                case <-str.ctx.Done():
                    close(str.out)
                    return
                case st, ok := <-str1.out:
                    if !ok {
                        close(str.out)
                        return
                    }
                    if st.delayed != nil {
                        go st.delayed()
                        go f()
                        return
                    }
                    cancelStr2()
                    select {
                    case <-str.ctx.Done():
                        close(str.out)
                        return
                    case str1.out <- st:
                    }
                    go link(str, str1)
                    return
                case st, ok := <-str2.out:
                    if !ok {
                        // short-circuit
                        close(str.out)
                        return
                    }
                    if st.delayed != nil {
                        go st.delayed()
                        go f()
                        return
                    }
                }
                cancelStr2()
                bind(str, str1, g2)
            }
            f()
        }()
        return str
    }
}

func takeAll(str stream) []state {
    states := []state{}
    for st := range str.out {
        if st.delayed != nil {
            go st.delayed()
            continue
        }
        states = append(states, st)
    }
    return states
}

func takeN(n int, str stream) []state {
    states := []state{}
    for i:=0; i<n; i++ {
        st, ok := <-str.out
        if !ok {
            return states
        }
        if st.delayed != nil {
            go st.delayed()
            continue
        }
        states = append(states, st)
    }
    return states
}

func mKreify(states []state) []expression {
    exprs := []expression{}
    for _, st := range states {
        exprs = append(exprs, st.sub.walkstar(variable(0)))
    }
    return exprs
}

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
