// Copyright 2018 The CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cue

import (
	"bytes"
	"math/big"
	"sort"
	"strings"
	"time"

	"cuelang.org/go/cue/token"
	"github.com/cockroachdb/apd"
)

// binSrc returns a baseValue representing a binary expression of the given
// values.
func binSrc(pos token.Pos, op op, a, b value) baseValue {
	return baseValue{&computedSource{pos, op, a, b}}
}

func unify(ctx *context, src source, left, right evaluated) evaluated {
	return binOp(ctx, src, opUnify, left, right)
}

func binOp(ctx *context, src source, op op, left, right evaluated) (result evaluated) {
	if err := firstBottom(left, right); err != nil {
		return err
	}

	leftKind := left.kind()
	rightKind := right.kind()
	kind, invert := matchBinOpKind(op, leftKind, rightKind)
	if kind == bottomKind {
		return ctx.mkIncompatible(src, op, left, right)
	}
	if kind.hasReferences() {
		panic("unexpected references in expression")
	}
	if invert {
		left, right = right, left
	}
	if op != opUnify {
		v := left.binOp(ctx, src, op, right) // may return incomplete
		return v
	}

	// op == opUnify

	// TODO: unify type masks.
	if left == right {
		return left
	}
	if isTop(left) {
		return right
	}
	if isTop(right) {
		return left
	}

	if dl, ok := left.(*disjunction); ok {
		return distribute(ctx, src, dl, right, false)
	} else if dr, ok := right.(*disjunction); ok {
		return distribute(ctx, src, dr, left, false)
	}

	// TODO: value may be incomplete if there is a cycle. Instead of an error
	// schedule an assert and return the atomic value, if applicable.
	v := left.binOp(ctx, src, op, right)
	if isBottom(v) {
		v := right.binOp(ctx, src, op, left)
		// Return the original failure if both fail, as this will result in
		// better error messages.
		if !isBottom(v) {
			return v
		}
	}
	return v
}

type mVal struct {
	val  evaluated
	mark bool
}

// distribute distributes a value over the element of a disjunction in a
// unification operation. If allowCycle is true, references that resolve
// to a cycle are dropped.
func distribute(ctx *context, src source, x *disjunction, y evaluated, allowCycle bool) evaluated {
	return dist(ctx, src, x, mVal{y, false}, false).val
}

func dist(ctx *context, src source, dx *disjunction, y mVal, allowCycle bool) mVal {
	dn := &disjunction{src.base(), make([]dValue, 0, len(dx.values))}
	for _, dv := range dx.values {
		x := mVal{dv.val.evalPartial(ctx), dv.marked}
		src := binSrc(src.Pos(), opUnify, x.val, y.val)

		var v mVal
		if dy, ok := y.val.(*disjunction); ok {
			v = dist(ctx, src, dy, x, allowCycle)
		} else if ddv, ok := dv.val.(*disjunction); ok {
			v = dist(ctx, src, ddv, y, allowCycle)
		} else {
			v = mVal{binOp(ctx, src, opUnify, x.val, y.val), x.mark || y.mark}
		}
		dn.add(ctx, v.val, v.mark)
	}

	return dn.normalize(ctx, src)
}

func (x *disjunction) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	panic("unreachable: special-cased")
}

func (x *bottom) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	panic("unreachable: special-cased")
}

func (x *top) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	switch op {
	case opUnify:
		return other
	}
	src = mkBin(ctx, src.Pos(), op, x, other)
	return ctx.mkErr(src, "binary operation on non-ground top value")
}

func (x *basicType) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	k := unifyType(x.kind(), other.kind())
	switch y := other.(type) {
	case *basicType:
		switch op {
		// TODO: other types.
		case opUnify:
			if k&typeKinds != bottomKind {
				return &basicType{binSrc(src.Pos(), op, x, other), k & typeKinds}
			}
		}
	case *rangeLit:
		src = mkBin(ctx, src.Pos(), op, x, other)
		return ctx.mkErr(src, codeIncomplete, "%s with incomplete values", op)
	case *numLit:
		if op == opUnify {
			if k == y.k {
				return y
			}
			i := *y
			i.k = k
			return &i
		}
		src = mkBin(ctx, src.Pos(), op, x, other)
		return ctx.mkErr(src, codeIncomplete, "%s with incomplete values", op)
	default:
		if k&typeKinds != bottomKind {
			return other
		}
	}
	return ctx.mkIncompatible(src, op, x, other)
}

// unifyFrom determines the maximum value of a and b.
func unifyFrom(ctx *context, src source, a, b evaluated) evaluated {
	if a.kind().isGround() && b.kind().isGround() {
		if leq(ctx, src, a, b) {
			return b
		}
		return a
	}
	if isTop(a) {
		return b
	}
	if isTop(b) {
		return a
	}
	if x, ok := a.(*rangeLit); ok {
		return unifyFrom(ctx, src, x.from.(evaluated), b)
	}
	if x, ok := b.(*rangeLit); ok {
		return unifyFrom(ctx, src, a, x.from.(evaluated))
	}
	src = mkBin(ctx, src.Pos(), opUnify, a, b)
	return ctx.mkErr(src, "incompatible types %v and %v", a.kind(), b.kind())
}

// unifyTo determines the minimum value of a and b.
func unifyTo(ctx *context, src source, a, b evaluated) evaluated {
	if a.kind().isGround() && b.kind().isGround() {
		if leq(ctx, src, a, b) {
			return a
		}
		return b
	}
	if isTop(a) {
		return b
	}
	if isTop(b) {
		return a
	}
	if x, ok := a.(*rangeLit); ok {
		return unifyTo(ctx, src, x.to.(evaluated), b)
	}
	if x, ok := b.(*rangeLit); ok {
		return unifyTo(ctx, src, a, x.to.(evaluated))
	}
	src = mkBin(ctx, src.Pos(), opUnify, a, b)
	return ctx.mkErr(src, "incompatible types %v and %v", a.kind(), b.kind())
}

func errInRange(ctx *context, pos token.Pos, r *rangeLit, v evaluated) *bottom {
	if pos == token.NoPos {
		pos = r.Pos()
	}
	const msgInRange = "value %v not in range %v"
	e := mkBin(ctx, pos, opUnify, r, v)
	return ctx.mkErr(e, msgInRange, v.strValue(), debugStr(ctx, r))
}

func (x *rangeLit) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	combine := func(x, y evaluated) evaluated {
		if _, ok := x.(*numLit); !ok {
			return x
		}
		if _, ok := y.(*numLit); !ok {
			return y
		}
		return binOp(ctx, src, op, x, y)
	}
	from := x.from.(evaluated)
	to := x.to.(evaluated)
	newSrc := mkBin(ctx, src.Pos(), op, x, other)
	switch op {
	case opUnify:
		k := unifyType(x.kind(), other.kind())
		if k&comparableKind != bottomKind {
			switch y := other.(type) {
			case *basicType:
				from := unify(ctx, src, x.from.(evaluated), y)
				to := unify(ctx, src, x.to.(evaluated), y)
				if from == x.from && to == x.to {
					return x
				}
				return &rangeLit{newSrc.base(), from, to}
			case *rangeLit:
				from := unifyFrom(ctx, src, x.from.(evaluated), y.from.(evaluated))
				to := unifyTo(ctx, src, x.to.(evaluated), y.to.(evaluated))
				if from.kind().isGround() && to.kind().isGround() && !leq(ctx, src, from, to) {
					r1 := debugStr(ctx, x)
					r2 := debugStr(ctx, y)
					return ctx.mkErr(newSrc, "non-overlapping ranges %s and %s", r1, r2)
				}
				return ctx.manifest(&rangeLit{newSrc.base(), from, to})

			case *numLit:
				if !leq(ctx, src, x.from.(evaluated), y) || !leq(ctx, src, y, x.to.(evaluated)) {
					return errInRange(ctx, src.Pos(), x, y)
				}
				if y.k != k {
					n := *y
					n.k = k
					return &n
				}
				return other

			case *durationLit, *stringLit:
				if !leq(ctx, src, x.from.(evaluated), y) || !leq(ctx, src, y, x.to.(evaluated)) {
					return errInRange(ctx, src.Pos(), x, y)
				}
				return other
			}
		}
	// See https://en.wikipedia.org/wiki/Interval_arithmetic.
	case opAdd:
		switch x.kind() & typeKinds {
		case stringKind:
			if !x.from.kind().isGround() || !x.to.kind().isGround() {
				// TODO: return regexp
				return ctx.mkErr(newSrc, codeIncomplete, "cannot add incomplete values")
			}
			combine := func(x, y evaluated) evaluated {
				if _, ok := x.(*basicType); ok {
					return ctx.mkErr(newSrc, "adding string to non-concrete type")
				}
				if _, ok := y.(*basicType); ok {
					return x
				}
				return binOp(ctx, src, opAdd, x, y)
			}
			return &rangeLit{
				baseValue: binSrc(src.Pos(), op, x, other),
				from:      combine(minNum(from), minNum(other)),
				to:        combine(maxNum(to), maxNum(other)),
			}

		case intKind, numKind, floatKind:
			return &rangeLit{
				baseValue: binSrc(src.Pos(), op, x, other),
				from:      combine(minNum(from), minNum(other)),
				to:        combine(maxNum(to), maxNum(other)),
			}

		default:
			return ctx.mkErrUnify(src, x, other)
		}

	case opSub:
		return &rangeLit{
			baseValue: binSrc(src.Pos(), op, x, other),
			from:      combine(minNum(from), maxNum(other)),
			to:        combine(maxNum(to), minNum(other)),
		}

	case opQuo:
		// See https://en.wikipedia.org/wiki/Interval_arithmetic.
		// TODO: all this is strictly not correct. To do it right we need to
		// have non-inclusive ranges at the least. So for now we just do this.
		var from, to evaluated
		if max := maxNum(other); !max.kind().isGround() {
			from = newNum(other, max.kind()) // 1/infinity is 0
		} else if num, ok := max.(*numLit); ok && num.v.IsZero() {
			from = &basicType{num.baseValue, num.kind()} // div by 0
		} else {
			one := newNum(other, max.kind())
			one.v.SetInt64(1)
			from = combine(one, max)
		}

		if _, ok := other.(*rangeLit); !ok {
			other = from
		} else {
			if min := minNum(other); !min.kind().isGround() {
				to = newNum(other, min.kind()) // 1/infinity is 0
			} else if num, ok := min.(*numLit); ok && num.v.IsZero() {
				to = &basicType{num.baseValue, num.kind()} // div by 0
			} else {
				one := newNum(other, min.kind())
				one.v.SetInt64(1)
				to = combine(one, min)
			}

			if !from.kind().isGround() && !to.kind().isGround() {
				other = from
			} else if leq(ctx, src, from, to) && leq(ctx, src, to, from) {
				other = from
			} else {
				other = &rangeLit{newSrc.base(), from, to}
			}
		}
		fallthrough

	case opMul:
		xMin, xMax := minNum(from), maxNum(to)
		yMin, yMax := minNum(other), maxNum(other)

		var from, to evaluated
		negMax := func(from, to *evaluated, val, sign evaluated) {
			if !val.kind().isGround() {
				*from = val
				if num, ok := sign.(*numLit); ok && num.v.Negative {
					*to = val
				}
			}
		}
		negMax(&from, &to, yMin, xMax)
		negMax(&to, &from, yMax, xMin)
		negMax(&from, &to, xMin, yMax)
		negMax(&to, &from, xMax, yMin)
		if from != nil && to != nil {
			return binOp(ctx, src, opUnify, from, to)
		}

		values := []evaluated{}
		add := func(a, b evaluated) {
			if a.kind().isGround() && b.kind().isGround() {
				values = append(values, combine(a, b))
			}
		}
		add(xMin, yMin)
		add(xMax, yMin)
		add(xMin, yMax)
		add(xMax, yMax)
		sort.Slice(values, func(i, j int) bool {
			return !leq(ctx, src, values[j], values[i])
		})

		r := &rangeLit{baseValue: binSrc(src.Pos(), op, x, other), from: from, to: to}
		if from == nil {
			r.from = values[0]
		}
		if to == nil {
			r.to = values[len(values)-1]
		}
		return r
	}
	return ctx.mkIncompatible(src, op, x, other)
}

func evalLambda(ctx *context, a value) (l *lambdaExpr, err evaluated) {
	if a == nil {
		return nil, nil
	}
	// NOTE: the values of a lambda might still be a disjunction
	e := ctx.manifest(a)
	if isBottom(e) {
		return nil, e
	}
	l, ok := e.(*lambdaExpr)
	if !ok {
		return nil, ctx.mkErr(a, "value must be lambda")
	}
	return ctx.deref(l).(*lambdaExpr), nil
}

func (x *structLit) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	y, ok := other.(*structLit)
	if !ok || op != opUnify {
		return ctx.mkIncompatible(src, op, x, other)
	}

	// TODO: unify emit

	x = ctx.deref(x).(*structLit)
	y = ctx.deref(y).(*structLit)
	if x == y {
		return x
	}
	arcs := make(arcs, 0, len(x.arcs)+len(y.arcs))
	obj := &structLit{binSrc(src.Pos(), op, x, other), x.emit, nil, nil, arcs}
	defer ctx.pushForwards(x, obj, y, obj).popForwards()

	tx, ex := evalLambda(ctx, x.template)
	ty, ey := evalLambda(ctx, y.template)

	var t *lambdaExpr
	switch {
	case ex != nil:
		return ex
	case ey != nil:
		return ey
	case tx != nil:
		t = tx
	case ty != nil:
		t = ty
	}
	if tx != ty && tx != nil && ty != nil {
		v := binOp(ctx, src, opUnify, tx, ty)
		if isBottom(v) {
			return v
		}
		t = v.(*lambdaExpr)
	}
	if t != nil {
		obj.template = ctx.copy(t)
	}

	sz := len(x.comprehensions) + len(y.comprehensions)
	obj.comprehensions = make([]*fieldComprehension, sz)
	for i, c := range x.comprehensions {
		obj.comprehensions[i] = ctx.copy(c).(*fieldComprehension)
	}
	for i, c := range y.comprehensions {
		obj.comprehensions[i+len(x.comprehensions)] = ctx.copy(c).(*fieldComprehension)
	}

	for _, a := range x.arcs {
		cp := ctx.copy(a.v)
		obj.arcs = append(obj.arcs, arc{a.feature, cp, nil})
	}
outer:
	for _, a := range y.arcs {
		v := ctx.copy(a.v)
		for i, b := range obj.arcs {
			if a.feature == b.feature {
				v = mkBin(ctx, src.Pos(), opUnify, b.v, v)
				obj.arcs[i].v = v
				continue outer
			}
		}
		obj.arcs = append(obj.arcs, arc{feature: a.feature, v: v})
	}
	sort.Stable(obj)

	return obj
}

func (x *nullLit) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	// TODO: consider using binSrc instead of src.base() for better traceability.
	switch op {
	case opEql:
		return &boolLit{baseValue: src.base(), b: true}
	case opNeq:
		return &boolLit{baseValue: src.base(), b: false}
	case opUnify:
		return x
	default:
		panic("unimplemented")
	}
}

func (x *boolLit) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	switch y := other.(type) {
	case *basicType:
		// range math
		return x

	case *boolLit:
		switch op {
		case opUnify:
			if x.b != y.b {
				return ctx.mkErr(x, "failed to unify: %v != %v", x.b, y.b)
			}
			return x
		case opLand:
			return boolTonode(src, x.b && y.b)
		case opLor:
			return boolTonode(src, x.b || y.b)
		case opEql:
			return boolTonode(src, x.b == y.b)
		case opNeq:
			return boolTonode(src, x.b != y.b)
		}
	}
	return ctx.mkIncompatible(src, op, x, other)
}

func (x *stringLit) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	switch other.(type) {
	// case *basicType:
	// 	return x

	// TODO: rangelit

	case *stringLit:
		str := other.strValue()
		switch op {
		case opUnify:
			str := other.strValue()
			if x.str != str {
				src := mkBin(ctx, src.Pos(), op, x, other)
				return ctx.mkErr(src, "failed to unify: %v != %v", x.str, str)
			}
			return x
		case opLss, opLeq, opEql, opNeq, opGeq, opGtr:
			return cmpTonode(src, op, strings.Compare(string(x.str), string(str)))
		case opAdd:
			return &stringLit{binSrc(src.Pos(), op, x, other), x.str + str}
		}
	}
	return ctx.mkIncompatible(src, op, x, other)
}

func (x *bytesLit) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	switch y := other.(type) {
	// case *basicType:
	// 	return x

	// TODO: rangelit

	case *bytesLit:
		b := y.b
		switch op {
		case opUnify:
			if !bytes.Equal(x.b, b) {
				return ctx.mkErr(x, "failed to unify: %v != %v", x.b, b)
			}
			return x
		case opLss, opLeq, opEql, opNeq, opGeq, opGtr:
			return cmpTonode(src, op, bytes.Compare(x.b, b))
		case opAdd:
			copy := append([]byte(nil), x.b...)
			copy = append(copy, b...)
			return &bytesLit{binSrc(src.Pos(), op, x, other), copy}
		}
	}
	return ctx.mkIncompatible(src, op, x, other)
}

func leq(ctx *context, src source, a, b evaluated) bool {
	if isTop(a) || isTop(b) {
		return true
	}
	v := binOp(ctx, src, opLeq, a, b)
	if isBottom(v) {
		return false
	}
	return v.(*boolLit).b
}

func maxNum(v evaluated) evaluated {
	switch x := v.(type) {
	case *numLit:
		return x
	case *rangeLit:
		return maxNum(x.to.(evaluated))
	}
	return v
}

func minNum(v evaluated) evaluated {
	switch x := v.(type) {
	case *numLit:
		return x
	case *rangeLit:
		return minNum(x.from.(evaluated))
	}
	return v
}

func maxNumRaw(v value) value {
	switch x := v.(type) {
	case *numLit:
		return x
	case *rangeLit:
		return maxNumRaw(x.to)
	}
	return v
}

func minNumRaw(v value) value {
	switch x := v.(type) {
	case *numLit:
		return x
	case *rangeLit:
		return minNumRaw(x.from)
	}
	return v
}

func cmpTonode(src source, op op, r int) evaluated {
	result := false
	switch op {
	case opLss:
		result = r == -1
	case opLeq:
		result = r != 1
	case opEql, opUnify:
		result = r == 0
	case opNeq:
		result = r != 0
	case opGeq:
		result = r != -1
	case opGtr:
		result = r == 1
	}
	return boolTonode(src, result)
}

func (x *numLit) updateNumInfo(a, b *numLit) {
	x.numInfo = unifyNuminfo(a.numInfo, b.numInfo)
}

func (x *numLit) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	switch y := other.(type) {
	case *basicType:
		if op == opUnify {
			return y.binOp(ctx, src, op, x)
		}
		// infinity math
		// 4 * int = int
	case *rangeLit:
		if op == opUnify {
			return y.binOp(ctx, src, op, x)
		}
		// 5..7 - 8 = -3..4
	case *numLit:
		k := unifyType(x.kind(), y.kind())
		n := newNumBin(k, x, y)
		switch op {
		case opUnify:
			if x.v.Cmp(&y.v) != 0 {
				src = mkBin(ctx, src.Pos(), op, x, other)
				return ctx.mkErr(src, "cannot unify numbers %v and %v", x.strValue(), y.strValue())
			}
			if k != x.k {
				n.v = x.v
				return n
			}
			return x
		case opLss, opLeq, opEql, opNeq, opGeq, opGtr:
			return cmpTonode(src, op, x.v.Cmp(&y.v))
		case opAdd:
			ctx.Add(&n.v, &x.v, &y.v)
		case opSub:
			ctx.Sub(&n.v, &x.v, &y.v)
		case opMul:
			ctx.Mul(&n.v, &x.v, &y.v)
		case opQuo:
			ctx.Quo(&n.v, &x.v, &y.v)
			ctx.Reduce(&n.v, &n.v)
			n.k = floatKind
		case opRem:
			ctx.Rem(&n.v, &x.v, &y.v)
			n.k = floatKind
		case opIDiv:
			intOp(ctx, n, (*big.Int).Div, x, y)
		case opIMod:
			intOp(ctx, n, (*big.Int).Mod, x, y)
		case opIQuo:
			intOp(ctx, n, (*big.Int).Quo, x, y)
		case opIRem:
			intOp(ctx, n, (*big.Int).Rem, x, y)
		}
		return n

	case *durationLit:
		if op == opMul {
			fd := float64(y.d)
			// TODO: check range
			f, _ := x.v.Float64()
			d := time.Duration(f * fd)
			return &durationLit{binSrc(src.Pos(), op, x, other), d}
		}
	}
	return ctx.mkIncompatible(src, op, x, other)
}

type intFunc func(z, x, y *big.Int) *big.Int

func intOp(ctx *context, n *numLit, fn intFunc, a, b *numLit) {
	var x, y apd.Decimal
	ctx.RoundToIntegralValue(&x, &a.v)
	if x.Negative {
		x.Coeff.Neg(&x.Coeff)
	}
	ctx.RoundToIntegralValue(&y, &b.v)
	if y.Negative {
		y.Coeff.Neg(&y.Coeff)
	}
	fn(&n.v.Coeff, &x.Coeff, &y.Coeff)
	if n.v.Coeff.Sign() < 0 {
		n.v.Coeff.Neg(&n.v.Coeff)
		n.v.Negative = true
	}
	n.k = intKind
}

// TODO: check overflow

func (x *durationLit) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	switch y := other.(type) {
	case *basicType:
		// infinity math

	case *durationLit:
		switch op {
		case opUnify:
			if x.d != y.d {
				return ctx.mkIncompatible(src, op, x, other)
			}
			return other
		case opLss:
			return boolTonode(src, x.d < y.d)
		case opLeq:
			return boolTonode(src, x.d <= y.d)
		case opEql:
			return boolTonode(src, x.d == y.d)
		case opNeq:
			return boolTonode(src, x.d != y.d)
		case opGeq:
			return boolTonode(src, x.d >= y.d)
		case opGtr:
			return boolTonode(src, x.d > y.d)
		case opAdd:
			return &durationLit{binSrc(src.Pos(), op, x, other), x.d + y.d}
		case opSub:
			return &durationLit{binSrc(src.Pos(), op, x, other), x.d - y.d}
		case opQuo:
			n := &numLit{
				numBase: newNumBase(nil, newNumInfo(floatKind, 0, 10, false)),
			}
			n.v.SetInt64(int64(x.d))
			d := apd.New(int64(y.d), 0)
			ctx.Quo(&n.v, &n.v, d)
			return n
		case opRem:
			n := &numLit{
				numBase: newNumBase(nil, newNumInfo(intKind, 0, 10, false)),
			}
			n.v.SetInt64(int64(x.d % y.d))
			n.v.Exponent = -9
			return n
		}

	case *numLit:
		switch op {
		case opMul:
			// TODO: check range
			f, _ := y.v.Float64()
			d := time.Duration(float64(x.d) * f)
			return &durationLit{binSrc(src.Pos(), op, x, other), d}
		case opQuo:
			// TODO: check range
			f, _ := y.v.Float64()
			d := time.Duration(float64(x.d) * f)
			return &durationLit{binSrc(src.Pos(), op, x, other), d}
		case opRem:
			d := x.d % time.Duration(y.intValue(ctx))
			return &durationLit{binSrc(src.Pos(), op, x, other), d}
		}
	}
	return ctx.mkIncompatible(src, op, x, other)
}

func (x *list) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	switch op {
	case opUnify:
		y, ok := other.(*list)
		if !ok {
			break
		}
		n := unify(ctx, src, x.len.(evaluated), y.len.(evaluated))
		if isBottom(n) {
			src = mkBin(ctx, src.Pos(), op, x, other)
			return ctx.mkErr(src, "incompatible list lengths: %v", n)
		}
		var a, rest []value
		var rtyp value
		nx, ny := len(x.a), len(y.a)
		if nx < ny {
			a = make([]value, nx, ny)
			rest = y.a[nx:]
			rtyp = x.typ

		} else {
			a = make([]value, ny, nx)
			rest = x.a[ny:]
			rtyp = y.typ
		}
		typ := x.typ
		max, ok := n.(*numLit)
		if !ok || len(a)+len(rest) < max.intValue(ctx) {
			typ = unify(ctx, src, x.typ.(evaluated), y.typ.(evaluated))
			if isBottom(typ) {
				src = mkBin(ctx, src.Pos(), op, x, other)
				return ctx.mkErr(src, "incompatible list types: %v: ", typ)
			}
		}

		for i := range a {
			ai := unify(ctx, src, x.at(ctx, i).evalPartial(ctx), y.at(ctx, i).evalPartial(ctx))
			if isBottom(ai) {
				return ai
			}
			a[i] = ai
		}
		for _, n := range rest {
			an := unify(ctx, src, n.evalPartial(ctx), rtyp.(evaluated))
			if isBottom(an) {
				return an
			}
			a = append(a, an)
		}
		return &list{baseValue: binSrc(src.Pos(), op, x, other), a: a, typ: typ, len: n}

	case opMul:
		k := other.kind()
		if !k.isAnyOf(intKind) {
			panic("multiplication must be int type")
		}
		typ := x.typ.(evaluated)
		ln := x.len.(evaluated)
		n := &list{baseValue: binSrc(src.Pos(), op, x, other), typ: x.typ}
		switch len(x.a) {
		case 0:
		case 1:
			n.typ = binOp(ctx, src, opUnify, typ, x.a[0].evalPartial(ctx))
		default:
			if !k.isGround() {
				return x
			}
			if ln := other.(*numLit).intValue(ctx); ln > 0 {
				// TODO: check error
				for i := 0; i < ln; i++ {
					// TODO: copy values
					n.a = append(n.a, x.a...)
				}
			}
		}
		switch v := x.len.(type) {
		case *top, *basicType:
			n.len = other
		case *numLit:
			switch v.intValue(ctx) {
			case 0:
				n.len = x.len
			case 1:
				n.len = other
			default:
				n.len = binOp(ctx, src, opMul, ln, other)
			}
		default:
			n.len = binOp(ctx, src, opMul, ln, other)
		}
		return n
	}
	return ctx.mkIncompatible(src, op, x, other)
}

func (x *lambdaExpr) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	if y, ok := other.(*lambdaExpr); ok && op == opUnify {
		x = ctx.deref(x).(*lambdaExpr)
		y = ctx.deref(y).(*lambdaExpr)
		n, m := len(x.params.arcs), len(y.params.arcs)
		if n != m {
			src = mkBin(ctx, src.Pos(), op, x, other)
			return ctx.mkErr(src, "number of params of params should match in unification (%d != %d)", n, m)
		}
		arcs := make([]arc, len(x.arcs))
		lambda := &lambdaExpr{binSrc(src.Pos(), op, x, other), &params{arcs}, nil}
		defer ctx.pushForwards(x, lambda, y, lambda).popForwards()

		xVal := ctx.copy(x.value)
		yVal := ctx.copy(y.value)
		lambda.value = mkBin(ctx, src.Pos(), opUnify, xVal, yVal)

		for i := range arcs {
			xArg := ctx.copy(x.at(ctx, i)).(evaluated)
			yArg := ctx.copy(y.at(ctx, i)).(evaluated)
			v := binOp(ctx, src, op, xArg, yArg)
			if isBottom(v) {
				return v
			}
			arcs[i] = arc{x.arcs[i].feature, v, nil}
		}

		return lambda
	}
	return ctx.mkIncompatible(src, op, x, other)
}

func (x *builtin) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	if op == opUnify && evaluated(x) == other {
		return x
	}
	return ctx.mkIncompatible(src, op, x, other)
}

func (x *feed) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	return ctx.mkIncompatible(src, op, x, other)
}

func (x *guard) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	return ctx.mkIncompatible(src, op, x, other)
}

func (x *yield) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	return ctx.mkIncompatible(src, op, x, other)
}

func (x *fieldComprehension) binOp(ctx *context, src source, op op, other evaluated) evaluated {
	return ctx.mkIncompatible(src, op, x, other)
}
