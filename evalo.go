// evalo: relational evaluator for a tagged subset of Scheme.
// Scope (phase 3): quote, numbers, variable references, single-arg lambda,
// application, primitives (cons/car/cdr).
//
// This is a port of faster-minikanren's evalo. The Chez Scheme original
// lives at ../faster-minikanren/evalo-port.scm. Each Go function below has
// the corresponding Scheme definition in a comment immediately above it.
//
// Source-language encoding (every form is a tagged list; no symbolo/numbero):
//
//	(0 n)              literal number n
//	(1 id)             variable/symbol reference; id is a number identifying the symbol
//	(2 val)            (quote val)
//	(3 param body)     (lambda (param) body) — single-arg only
//	(4 rator rands)    application; rator is an expression, rands is a list of expressions
//
// Value encoding:
//
//	(0 n)                       literal number value (self-evaluating)
//	(<tagClosureVal> param body env)  closure
//	(<tagPrimVal> primId)             primitive reference
//	(a . b)                     plain pair built via cons
//
// In the Chez original, closure-tag and prim-tag are symbols (disjoint from
// numbers and any pair user code can construct). In Go we use kindSpecial
// reserved values (tagClosureVal, tagPrimVal) playing the same role.
//
// Symbol ids are arbitrary numbers; we reserve a small range for the
// standard primitives (consId/carId/cdrId) and the user is expected to use
// ids outside that range for their own variables.
package main

// evalOMode selects how evalO combines its 6 cases.
//
//   - modeDisjPlus: sequential interleaving with bind+mplus delay
//     propagation. The default and historically the only mode that
//     works at depth.
//
//   - modeDisjConc: shared-pool fan-out. Hangs at K>=2 because eager
//     fan-out triggers caseAppClosure recursion before productive
//     low-K branches dispatch.
//
//   - modeDisjSmart: budget-bounded disj_par. Top-level evalO claims
//     a parallelism slot; nested recursive evalO calls find the
//     budget exhausted and fall back to disj_plus, naturally
//     capping goroutine count.
type evalOModeT int

const (
	modeDisjPlus  evalOModeT = iota // sequential interleaving (default)
	modeDisjConc                    // shared-pool fan-out (HANGS at K>=2)
	modeDisjSmart                   // budget-bounded disj_par with disj_plus fallback
)

var evalOMode evalOModeT

const (
	tagNum   = 0
	tagSym   = 1
	tagQuote = 2
	tagLam   = 3
	tagApp   = 4
)

// Reserved symbol ids for the standard primitives. The initial env binds these
// to their primitive values.
//
//	(define prim-id-cons 100)
//	(define prim-id-car  101)
//	(define prim-id-cdr  102)
const (
	primIdCons = 100
	primIdCar  = 101
	primIdCdr  = 102
)

// User-defined symbol ids should be >= userSymBase to keep them disjoint from
// reserved primitive ids.
const userSymBase = 1000

// Source-form constructors.
func numLit(n int) expression           { return list(number(tagNum), number(n)) }
func symRef(id int) expression          { return list(number(tagSym), number(id)) }
func quoteForm(val expression) expression { return list(number(tagQuote), val) }
func lamForm(param int, body expression) expression {
	return list(number(tagLam), number(param), body)
}
func appForm(rator expression, rands ...expression) expression {
	rl := emptylist
	for i := len(rands) - 1; i >= 0; i-- {
		rl = pair(rands[i], rl)
	}
	return list(number(tagApp), rator, rl)
}

// Value constructors.
func numVal(n int) expression { return numLit(n) } // numbers self-evaluate
func closureVal(param int, body, env expression) expression {
	return list(tagClosureVal, number(param), body, env)
}
func primVal(id int) expression {
	return list(tagPrimVal, number(id))
}

// initialEnv binds the standard primitives to their primitive values.
//
//	(define (initial-env)
//	  `((,prim-id-cons . (prim-tag ,prim-id-cons))
//	    (,prim-id-car  . (prim-tag ,prim-id-car))
//	    (,prim-id-cdr  . (prim-tag ,prim-id-cdr))))
func initialEnv() expression {
	return list(
		pair(number(primIdCons), primVal(primIdCons)),
		pair(number(primIdCar), primVal(primIdCar)),
		pair(number(primIdCdr), primVal(primIdCdr)),
	)
}

// lookupO: find the first (id . val) pair in env where id matches.
// Uses =/= so shadowed bindings are invisible.
//
//	(define (lookupo id env val)
//	  (fresh (k v rest)
//	    (== env `((,k . ,v) . ,rest))
//	    (conde
//	      [(== id k) (== val v)]
//	      [(=/= id k) (lookupo id rest val)])))
//
// The Go version wraps the body in delay(...) to give mplus a swap point on
// each recursive call (the equivalent of Chez's defrel inverse-eta thunking).
func lookupO(id, env, val expression) goal {
	return delay(func() goal {
		return fresh3(func(k, v, rest expression) goal {
			return conj(
				equalo(env, pair(pair(k, v), rest)),
				disj(
					conj(equalo(id, k), equalo(val, v)),
					conj(neqo(id, k), lookupO(id, rest, val)),
				),
			)
		})
	})
}

// evalListO: relationally evaluate every expression in expr-list against env,
// producing a parallel value-list.
//
//	(define (eval-listo exprs env vals)
//	  (conde
//	    [(== exprs '()) (== vals '())]
//	    [(fresh (e-head e-tail v-head v-tail)
//	       (== exprs (cons e-head e-tail))
//	       (== vals  (cons v-head v-tail))
//	       (eval-expo e-head env v-head)
//	       (eval-listo e-tail env v-tail))]))
//
// The Go version wraps the recursive cons-case in delay(...) so each request
// to evalListO's stream gets a proper swap point in mplus.
func evalListO(exprs, env, vals expression) goal {
	return disj(
		conj(equalo(exprs, emptylist), equalo(vals, emptylist)),
		delay(func() goal {
			return fresh3(func(eHead, eTail, vHead expression) goal {
				return fresh1(func(vTail expression) goal {
					return conj_plus(
						equalo(exprs, pair(eHead, eTail)),
						equalo(vals, pair(vHead, vTail)),
						evalO(eHead, env, vHead),
						evalListO(eTail, env, vTail),
					)
				})
			})
		}),
	)
}

// evalPrimO: dispatch a primitive call. id picks the primitive; argVals is the
// list of evaluated arguments; val is the result.
//
//	(define (eval-primo id arg-vals val)
//	  (conde
//	    [(== id prim-id-cons)
//	     (fresh (a b)
//	       (== arg-vals `(,a ,b))
//	       (== val (cons a b)))]
//	    [(== id prim-id-car)
//	     (fresh (a d)
//	       (== arg-vals `((,a . ,d)))
//	       (== val a))]
//	    [(== id prim-id-cdr)
//	     (fresh (a d)
//	       (== arg-vals `((,a . ,d)))
//	       (== val d))]))
func evalPrimO(id, argVals, val expression) goal {
	return disj_plus(
		// cons: argVals = (a b), val = (a . b)
		conj(
			equalo(id, number(primIdCons)),
			fresh2(func(a, b expression) goal {
				return conj(
					equalo(argVals, list(a, b)),
					equalo(val, pair(a, b)),
				)
			}),
		),
		// car: argVals = ((a . d)), val = a
		conj(
			equalo(id, number(primIdCar)),
			fresh2(func(a, d expression) goal {
				return conj(
					equalo(argVals, list(pair(a, d))),
					equalo(val, a),
				)
			}),
		),
		// cdr: argVals = ((a . d)), val = d
		conj(
			equalo(id, number(primIdCdr)),
			fresh2(func(a, d expression) goal {
				return conj(
					equalo(argVals, list(pair(a, d))),
					equalo(val, d),
				)
			}),
		),
	)
}

// evalO: relational evaluator. expr in env evaluates to val.
//
//	(define (eval-expo expr env val)
//	  (conde
//	    ;; (0 n) → (0 n)
//	    [(fresh (n)
//	       (== expr `(0 ,n))
//	       (== val  `(0 ,n)))]
//	    ;; (1 id) → lookup
//	    [(fresh (id)
//	       (== expr `(1 ,id))
//	       (lookupo id env val))]
//	    ;; (2 v) → v with absento on closure-tag/prim-tag
//	    [(fresh (qval)
//	       (== expr `(2 ,qval))
//	       (== val qval)
//	       (absento 'closure-tag qval)
//	       (absento 'prim-tag qval))]
//	    ;; (3 param body) → closure
//	    [(fresh (param body)
//	       (== expr `(3 ,param ,body))
//	       (== val `(closure-tag ,param ,body ,env)))]
//	    ;; (4 rator (rand)) → closure application (single-arg)
//	    [(fresh (rator rands param body fn-env arg-expr arg-val)
//	       (== expr `(4 ,rator ,rands))
//	       (== rands `(,arg-expr))
//	       (eval-expo rator env `(closure-tag ,param ,body ,fn-env))
//	       (eval-expo arg-expr env arg-val)
//	       (eval-expo body `((,param . ,arg-val) . ,fn-env) val))]
//	    ;; (4 rator rands) → primitive application
//	    [(fresh (rator rands prim-id arg-vals)
//	       (== expr `(4 ,rator ,rands))
//	       (eval-expo rator env `(prim-tag ,prim-id))
//	       (eval-listo rands env arg-vals)
//	       (eval-primo prim-id arg-vals val))]))
//
// In Chez, defrel wraps the conde body in (lambda (s) (lambda () body)),
// giving every recursive call a fresh thunk. In Go, conde branches that
// recurse into evalO are wrapped in delay(...) explicitly so that bind
// and mplus see a delayMessage and propagate it upward (matching Chez's
// procedure→thunk fairness signal).
func evalO(expr, env, val expression) goal {
	caseNumLit := fresh1(func(n expression) goal {
		return conj(
			equalo(expr, list(number(tagNum), n)),
			equalo(val, list(number(tagNum), n)),
		)
	})
	caseVarRef := fresh1(func(id expression) goal {
		return conj(
			equalo(expr, list(number(tagSym), id)),
			lookupO(id, env, val),
		)
	})
	caseQuote := fresh1(func(qval expression) goal {
		return conj_plus(
			equalo(expr, list(number(tagQuote), qval)),
			equalo(val, qval),
			absentoO(tagClosureVal, qval),
			absentoO(tagPrimVal, qval),
		)
	})
	caseLambda := fresh2(func(param, body expression) goal {
		return conj(
			equalo(expr, list(number(tagLam), param, body)),
			equalo(val, list(tagClosureVal, param, body, env)),
		)
	})
	caseAppClosure := delay(func() goal {
		return fresh2(func(rator, rands expression) goal {
			return fresh3(func(param, body, fnEnv expression) goal {
				return fresh2(func(argExpr, argVal expression) goal {
					return conj_plus(
						equalo(expr, list(number(tagApp), rator, rands)),
						equalo(rands, list(argExpr)),
						evalO(rator, env, list(tagClosureVal, param, body, fnEnv)),
						evalO(argExpr, env, argVal),
						evalO(body, pair(pair(param, argVal), fnEnv), val),
					)
				})
			})
		})
	})
	caseAppPrim := delay(func() goal {
		return fresh3(func(rator, rands, argVals expression) goal {
			return fresh1(func(primId expression) goal {
				return conj_plus(
					equalo(expr, list(number(tagApp), rator, rands)),
					evalO(rator, env, list(tagPrimVal, primId)),
					evalListO(rands, env, argVals),
					evalPrimO(primId, argVals, val),
				)
			})
		})
	})
	switch evalOMode {
	case modeDisjConc:
		return disj_conc(
			caseNumLit, caseVarRef, caseQuote, caseLambda,
			caseAppClosure, caseAppPrim,
		)
	case modeDisjSmart:
		return disj_smart(
			caseNumLit, caseVarRef, caseQuote, caseLambda,
			caseAppClosure, caseAppPrim,
		)
	}
	return disj_plus(
		caseNumLit, caseVarRef, caseQuote, caseLambda,
		caseAppClosure, caseAppPrim,
	)
}
