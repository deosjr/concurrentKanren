// Einstein's puzzle, ported from the Chez Scheme original.
// Faithful translation: uses only disj/disj_plus/conj/conj_plus, no disj_conc.
package main

import "sync/atomic"

// All atoms are unique int32 values across categories so accidental
// cross-slot unifications fail. Positions are reused by left-of/right-of.
const (
	posOne   = 1
	posTwo   = 2
	posThree = 3
	posFour  = 4
	posFive  = 5

	natBrit      = 10
	natSwede     = 11
	natDane      = 12
	natNorwegian = 13
	natGerman    = 14

	colRed    = 20
	colWhite  = 21
	colGreen  = 22
	colYellow = 23
	colBlue   = 24

	petDog   = 30
	petBirds = 31
	petCats  = 32
	petHorse = 33
	petFish  = 34

	drnTea    = 40
	drnCoffee = 41
	drnMilk   = 42
	drnBeer   = 43
	drnWater  = 44

	smkPallMall   = 50
	smkDunhill    = 51
	smkBlend      = 52
	smkBluemaster = 53
	smkPrince     = 54
)

// gensym for anonymous fresh vars created outside callfresh.
// Starts above any plausible per-run state.vc to avoid collisions.
var anonCounter atomic.Int32

func init() {
	anonCounter.Store(1 << 20)
}

func anon() expression {
	return mkVar(variable(anonCounter.Add(1)))
}

// rightO: x is one position right of y. Faithful to the Scheme conde.
func rightO(x, y expression) goal {
	return disj_plus(
		conj(equalo(x, number(posTwo)), equalo(y, number(posOne))),
		conj(equalo(x, number(posThree)), equalo(y, number(posTwo))),
		conj(equalo(x, number(posFour)), equalo(y, number(posThree))),
		conj(equalo(x, number(posFive)), equalo(y, number(posFour))),
	)
}

func leftO(x, y expression) goal { return rightO(y, x) }

func nextTo(x, y expression) goal {
	return disj_plus(leftO(x, y), rightO(x, y))
}

// streetDef builds the 5-row list-of-lists with the position column fixed
// and the other 5 columns filled with fresh anon vars. Called once per run
// so each run uses its own anon vars.
func streetDef() expression {
	row := func(pos int) expression {
		return list(number(pos), anon(), anon(), anon(), anon(), anon())
	}
	return list(row(posOne), row(posTwo), row(posThree), row(posFour), row(posFive))
}

// memberoUnrolled precompiles a known list into a function that unifies x
// against each element via disj_plus. Mirrors the Scheme membero-unrolled.
func memberoUnrolled(ylist expression) func(x expression) goal {
	rows := []expression{}
	for e := ylist; e.kind == kindPair; e = e.pair.cdr {
		rows = append(rows, e.pair.car)
	}
	return func(x expression) goal {
		goals := make([]goal, len(rows))
		for i, r := range rows {
			goals[i] = equalo(x, r)
		}
		return disj_plus(goals...)
	}
}

// einsteinGoal returns the full puzzle goal. q binds to (fishowner street).
func einsteinGoal(q expression) goal {
	return fresh1(func(fishowner expression) goal {
		return fresh1(func(s expression) goal {
			street := streetDef()
			unrolled := memberoUnrolled(street)
			return fresh7(func(a, b, c, d, e, f, g expression) goal {
				return fresh3(func(h, i, j expression) goal {
					return conj_plus(
						equalo(q, list(fishowner, s)),
						equalo(s, street),
						unrolled(list(anon(), number(natBrit), number(colRed), anon(), anon(), anon())),
						unrolled(list(anon(), number(natSwede), anon(), number(petDog), anon(), anon())),
						unrolled(list(anon(), number(natDane), anon(), anon(), number(drnTea), anon())),
						unrolled(list(a, anon(), number(colGreen), anon(), anon(), anon())),
						unrolled(list(b, anon(), number(colWhite), anon(), anon(), anon())),
						leftO(a, b),
						unrolled(list(anon(), anon(), number(colGreen), anon(), number(drnCoffee), anon())),
						unrolled(list(anon(), anon(), anon(), number(petBirds), anon(), number(smkPallMall))),
						unrolled(list(anon(), anon(), number(colYellow), anon(), anon(), number(smkDunhill))),
						unrolled(list(number(posThree), anon(), anon(), anon(), number(drnMilk), anon())),
						unrolled(list(number(posOne), number(natNorwegian), anon(), anon(), anon(), anon())),
						unrolled(list(c, anon(), anon(), anon(), anon(), number(smkBlend))),
						unrolled(list(d, anon(), anon(), number(petCats), anon(), anon())),
						nextTo(c, d),
						unrolled(list(e, anon(), anon(), number(petHorse), anon(), anon())),
						unrolled(list(f, anon(), anon(), anon(), anon(), number(smkDunhill))),
						nextTo(e, f),
						unrolled(list(anon(), anon(), anon(), anon(), number(drnBeer), number(smkBluemaster))),
						unrolled(list(anon(), number(natGerman), anon(), anon(), anon(), number(smkPrince))),
						unrolled(list(g, number(natNorwegian), anon(), anon(), anon(), anon())),
						unrolled(list(h, anon(), number(colBlue), anon(), anon(), anon())),
						nextTo(g, h),
						unrolled(list(i, anon(), anon(), anon(), anon(), number(smkBlend))),
						unrolled(list(j, anon(), anon(), anon(), number(drnWater), anon())),
						nextTo(i, j),
						unrolled(list(anon(), fishowner, anon(), number(petFish), anon(), anon())),
					)
				})
			})
		})
	})
}
