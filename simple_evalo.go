// simpleEvalO: faithful port of faster-miniKanren's simple-interp.scm.
// Pure lambda calculus + variable lookup. No primitives, no quote, no numbers.
//
// Source-language encoding (mirrors our main evalo's tagging):
//
//	(1 id)             variable/symbol reference
//	(3 param body)     (lambda (param) body) — single-arg only
//	(4 rator rand)     application (single-arg)
//
// Note: our application here uses a single rand (not a list) — closer to
// simple-interp's `(,rator ,rand)` shape than our main evalO's `(4 rator rands)`.
//
// Value encoding:
//
//	(<tagClosureVal> param body env)  closure
//
// Symbol ids are arbitrary numbers (we don't have symbolo).
package main

// simpleAppForm constructs a single-arg application: (4 rator rand).
func simpleAppForm(rator, rand expression) expression {
	return list(number(tagApp), rator, rand)
}

// simpleLookupO: same as our main lookupO — first-match with =/= guard.
func simpleLookupO(id, env, val expression) goal {
    return delay(func() goal {
		return fresh3(func(k, v, rest expression) goal {
			return conj(
				equalo(env, pair(pair(k, v), rest)),
				disj(
					conj(equalo(id, k), equalo(val, v)),
					conj(neqo(id, k), simpleLookupO(id, rest, val)),
				),
			)
		})
    })
}

// simpleEvalO: lambda calculus only, 3 cases. No quote, no primitives.
func simpleEvalO(expr, env, val expression) goal {
	caseVarRef := fresh1(func(id expression) goal {
		return conj(
			equalo(expr, list(number(tagSym), id)),
			simpleLookupO(id, env, val),
		)
	})
	caseLambda := fresh2(func(param, body expression) goal {
		return conj(
			equalo(expr, list(number(tagLam), param, body)),
			equalo(val, list(tagClosureVal, param, body, env)),
		)
	})
	caseApp := delay(func() goal {
        return fresh2(func(rator, rand expression) goal {
			return fresh3(func(param, body, fnEnv expression) goal {
				return fresh1(func(argVal expression) goal {
					return conj_plus(
						equalo(expr, list(number(tagApp), rator, rand)),
						simpleEvalO(rator, env, list(tagClosureVal, param, body, fnEnv)),
						simpleEvalO(rand, env, argVal),
						simpleEvalO(body, pair(pair(param, argVal), fnEnv), val),
					)
				})
			})
        })
	})
	return disj_plus(caseVarRef, caseLambda, caseApp)
}
