package main

import (
    "reflect"
    "runtime"
    "testing"
    "time"
)

func TestMain(t *testing.T) {
    n5, n6, n7 := number(5), number(6), number(7)
    waitTime := time.Millisecond * 100
    for i, tt := range []struct{
        goal goal
        take int
        want []expression
    }{
        {
            goal: callfresh(func(q expression) goal {
                return equalo(q, n5)
            }),
            want: []expression{ n5 },
        },
        {
            goal: callfresh(func(q expression) goal {
                return disj(equalo(q, n5), equalo(q, n6))
            }),
            want: []expression{ n5, n6 },
        },
        {
            goal: callfresh(func(x expression) goal {
                return fives(x)
            }),
            take: 3,
            want: []expression{ n5, n5, n5 },
        },
        {
            goal: callfresh(func(x expression) goal {
                return disj(fives(x), disj(sixes(x), sevens(x)))
            }),
            take: 9,
            want: []expression{ n5, n6, n5, n7, n5, n6, n5, n7, n5 },
        },
        {
            goal: callfresh(func(x expression) goal {
                return disj_plus(fives(x), sixes(x), sevens(x))
            }),
            take: 9,
            want: []expression{ n5, n6, n5, n7, n5, n6, n5, n7, n5 },
        },
        {
            goal: callfresh(func(x expression) goal {
                return disj_conc(fives(x), sixes(x), sevens(x))
            }),
            take: 9,
            want: []expression{ n5, n6, n7, n5, n6, n7, n5, n6, n7 },
        },
        {
            goal: callfresh(func(x expression) goal {
                return disj_conc(nevero(), fives(x), sixes(x), sevens(x))
            }),
            take: 9,
            want: []expression{ n5, n6, n7, n5, n6, n7, n5, n6, n7 },
        },
        {
            goal: callfresh(func(x expression) goal {
                return callfresh(func(y expression) goal {
                    return conj(equalo(x, n5), equalo(y, n6))
                })
            }),
            want: []expression{ n5 },
        },
        {
            goal: callfresh(func(q expression) goal {
                    return callfresh(func(x expression) goal {
                        return callfresh(func(y expression) goal {
                            return conj(
                                equalo(q, pair{x, y}),
                                conj(equalo(x, n5), equalo(y, n6)),
                            )
                    })
                })
            }),
            want: []expression{ pair{n5, n6} },
        },
        {
            goal: equalo(n5, n6),
            want: []expression{},
        },
        {
            goal: conj_sce(equalo(n5, n6), nevero()),
            want: []expression{},
        },
        {
            goal: conj_sce(nevero(), equalo(n5, n6)),
            want: []expression{},
        },
    }{
        n := runtime.NumGoroutine()
        var got []expression
        if tt.take == 0 {
            got = run(tt.goal)
        } else {
            got = runN(tt.take, tt.goal)
        }
        if !reflect.DeepEqual(got, tt.want) {
            t.Errorf("%d) got %v want %v", i, got, tt.want)
        }
        // we need some time for goroutines to close down
        time.Sleep(waitTime)
        if n != runtime.NumGoroutine() {
            t.Errorf("%d) number of goroutines has changed!", i)
        }
    }
}
