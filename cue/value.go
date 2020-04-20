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
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/cockroachdb/apd/v2"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/token"
)

type value interface {
	source

	rewrite(*context, rewriteFunc) value

	// evalPartial evaluates a value without choosing default values.
	evalPartial(*context) evaluated

	kind() kind

	// subsumesImpl is only defined for non-reference types.
	// It should only be called by the subsumes function.
	subsumesImpl(*subsumer, value) bool
}

type evaluated interface {
	value
	binOp(*context, source, op, evaluated) evaluated
	strValue() string
}

type scope interface {
	value
	lookup(*context, label) arc
}

type atter interface {
	// at returns the evaluated and its original value at the given position.
	// If the original could not be found, it returns an error and nil.
	at(*context, int) evaluated
}

type iterAtter interface {
	// at returns the evaluated and its original value at the given position.
	// If the original could not be found, it returns an error and nil.
	iterAt(*context, int) arc
}

// caller must be implemented by any concrete lambdaKind
type caller interface {
	call(ctx *context, src source, args ...evaluated) value
	returnKind() kind
}

func checkKind(ctx *context, x value, want kind) *bottom {
	if b, ok := x.(*bottom); ok {
		return b
	}
	got := x.kind()
	if got&want&concreteKind == bottomKind && want != bottomKind {
		return ctx.mkErr(x, "cannot use value %v (type %s) as %s", ctx.str(x), got, want)
	}
	if !got.isGround() {
		return ctx.mkErr(x, codeIncomplete,
			"non-concrete value %v", got)
	}
	return nil
}

func newDecl(n ast.Decl) baseValue {
	if n == nil {
		panic("empty node")
	}
	return baseValue{n}
}

func newExpr(n ast.Expr) baseValue {
	if n == nil {
		panic("empty node")
	}
	return baseValue{n}
}

func newNode(n ast.Node) baseValue {
	if n == nil {
		panic("empty node")
	}
	return baseValue{n}
}

type source interface {
	// syntax returns the parsed file of the underlying node or a computed
	// node indicating that it is a computed binary expression.
	syntax() ast.Node
	computed() *computedSource
	Pos() token.Pos
	base() baseValue
}

type computedSource struct {
	pos token.Pos
	op  op
	x   value
	y   value
}

func (s *computedSource) Pos() token.Pos {
	return s.pos
}

type posser interface {
	Pos() token.Pos
}

type baseValue struct {
	pos posser
}

func (b baseValue) Pos() token.Pos {
	if b.pos == nil {
		return token.NoPos
	}
	return b.pos.Pos()
}

func (b baseValue) computed() *computedSource {
	switch x := b.pos.(type) {
	case *computedSource:
		return x
	}
	return nil
}

func (b baseValue) syntax() ast.Node {
	switch x := b.pos.(type) {
	case ast.Node:
		return x
	}
	return nil
}

func (b baseValue) base() baseValue {
	return b
}

func (b baseValue) strValue() string { panic("unimplemented") }
func (b baseValue) returnKind() kind { panic("unimplemented") }

// top is the top of the value lattice. It subsumes all possible values.
type top struct{ baseValue }

func (x *top) kind() kind { return topKind }

// basicType represents the root class of any specific type.
type basicType struct {
	baseValue
	k kind
}

func (x *basicType) kind() kind { return x.k | nonGround }

// Literals

type nullLit struct{ baseValue }

func (x *nullLit) kind() kind { return nullKind }

type boolLit struct {
	baseValue
	b bool
}

func (x *boolLit) kind() kind { return boolKind }

func boolTonode(src source, b bool) evaluated {
	return &boolLit{src.base(), b}
}

type bytesLit struct {
	baseValue
	b  []byte
	re *regexp.Regexp // only set if needed
}

func (x *bytesLit) kind() kind       { return bytesKind }
func (x *bytesLit) strValue() string { return string(x.b) }

func (x *bytesLit) iterAt(ctx *context, i int) arc {
	if i >= len(x.b) {
		return arc{}
	}
	v := x.at(ctx, i)
	return arc{v: v, cache: v}
}

func (x *bytesLit) at(ctx *context, i int) evaluated {
	if i < 0 || i >= len(x.b) {
		return ctx.mkErr(x, "index %d out of bounds", i)
	}
	// TODO: this is incorrect.
	return newInt(x, 0).setUInt64(uint64(x.b[i]))
}

func (x *bytesLit) len() int { return len(x.b) }

func (x *bytesLit) slice(ctx *context, lo, hi *numLit) evaluated {
	lox := 0
	hix := len(x.b)
	if lo != nil {
		lox = lo.intValue(ctx)
	}
	if hi != nil {
		hix = hi.intValue(ctx)
	}
	if lox < 0 {
		return ctx.mkErr(x, "invalid slice index %d (must be non-negative)", lox)
	}
	if hix < 0 {
		return ctx.mkErr(x, "invalid slice index %d (must be non-negative)", hix)
	}
	if hix < lox {
		return ctx.mkErr(x, "invalid slice index: %d > %d", lox, hix)
	}
	if len(x.b) < hix {
		return ctx.mkErr(hi, "slice bounds out of range")
	}
	return &bytesLit{x.baseValue, x.b[lox:hix], nil}
}

type stringLit struct {
	baseValue
	str string
	re  *regexp.Regexp // only set if needed

	// TODO: maintain extended grapheme index cache.
}

func (x *stringLit) kind() kind       { return stringKind }
func (x *stringLit) strValue() string { return x.str }

func (x *stringLit) iterAt(ctx *context, i int) arc {
	runes := []rune(x.str)
	if i >= len(runes) {
		return arc{}
	}
	v := x.at(ctx, i)
	return arc{v: v, cache: v}
}

func (x *stringLit) at(ctx *context, i int) evaluated {
	runes := []rune(x.str)
	if i < 0 || i >= len(runes) {
		return ctx.mkErr(x, "index %d out of bounds", i)
	}
	// TODO: this is incorrect.
	return &stringLit{x.baseValue, string(runes[i : i+1]), nil}
}
func (x *stringLit) len() int { return len([]rune(x.str)) }

func (x *stringLit) slice(ctx *context, lo, hi *numLit) evaluated {
	runes := []rune(x.str)
	lox := 0
	hix := len(runes)
	if lo != nil {
		lox = lo.intValue(ctx)
	}
	if hi != nil {
		hix = hi.intValue(ctx)
	}
	if lox < 0 {
		return ctx.mkErr(x, "invalid slice index %d (must be non-negative)", lox)
	}
	if hix < 0 {
		return ctx.mkErr(x, "invalid slice index %d (must be non-negative)", hix)
	}
	if hix < lox {
		return ctx.mkErr(x, "invalid slice index: %d > %d", lox, hix)
	}
	if len(runes) < hix {
		return ctx.mkErr(hi, "slice bounds out of range")
	}
	return &stringLit{x.baseValue, string(runes[lox:hix]), nil}
}

type numLit struct {
	baseValue
	rep literal.Multiplier
	k   kind
	v   apd.Decimal
}

func newNum(src source, k kind, rep literal.Multiplier) *numLit {
	if k&numKind == 0 {
		panic("not a number")
	}
	return &numLit{baseValue: src.base(), rep: rep, k: k}
}

func newInt(src source, rep literal.Multiplier) *numLit {
	return newNum(src, intKind, rep)
}

func newFloat(src source, rep literal.Multiplier) *numLit {
	return newNum(src, floatKind, rep)
}

func (n numLit) specialize(k kind) *numLit {
	n.k = k
	return &n
}

func (n *numLit) set(d *apd.Decimal) *numLit {
	n.v.Set(d)
	return n
}

func (n *numLit) setInt(x int) *numLit {
	n.v.SetInt64(int64(x))
	return n
}

func (n *numLit) setInt64(x int64) *numLit {
	n.v.SetInt64(x)
	return n
}

func (n *numLit) setUInt64(x uint64) *numLit {
	n.v.Coeff.SetUint64(x)
	return n
}

func (n *numLit) setString(s string) *numLit {
	_, _, _ = n.v.SetString(s)
	return n
}

func (n *numLit) String() string {
	if n.k&intKind != 0 {
		return n.v.Text('f') // also render info
	}
	s := n.v.Text('g')
	if len(s) == 1 {
		s += "."
	}
	return s // also render info
}

func parseInt(k kind, s string) *numLit {
	num := newInt(newExpr(ast.NewLit(token.INT, s)), 0)
	_, _, err := num.v.SetString(s)
	if err != nil {
		panic(err)
	}
	return num
}

func parseFloat(s string) *numLit {
	num := newFloat(newExpr(ast.NewLit(token.FLOAT, s)), 0)
	_, _, err := num.v.SetString(s)
	if err != nil {
		panic(err)
	}
	return num
}

var ten = big.NewInt(10)

var one = parseInt(intKind, "1")

func (x *numLit) kind() kind       { return x.k }
func (x *numLit) strValue() string { return x.v.String() }

func (x *numLit) isInt(ctx *context) bool {
	return x.kind()&intKind != 0
}

func (x *numLit) intValue(ctx *context) int {
	v, err := x.v.Int64()
	if err != nil {
		return 0
	}
	return int(v)
}

type durationLit struct {
	baseValue
	d time.Duration
}

func (x *durationLit) kind() kind       { return durationKind }
func (x *durationLit) strValue() string { return x.d.String() }

type bound struct {
	baseValue
	op    op   // opNeq, opLss, opLeq, opGeq, or opGtr
	k     kind // mostly used for number kind
	value value
}

func newBound(ctx *context, base baseValue, op op, k kind, v value) evaluated {
	kv := v.kind()
	if kv.isAnyOf(numKind) {
		kv |= numKind
	} else if op == opNeq && kv&atomKind == nullKind {
		kv = typeKinds &^ nullKind
	}
	if op == opMat || op == opNMat {
		v = compileRegexp(ctx, v)
		if isBottom(v) {
			return v.(*bottom)
		}
	}
	return &bound{base, op, unifyType(k&topKind, kv) | nonGround, v}
}

func (x *bound) kind() kind {
	return x.k
}

func mkIntRange(a, b string) evaluated {
	from := newBound(nil, baseValue{}, opGeq, intKind, parseInt(intKind, a))
	to := newBound(nil, baseValue{}, opLeq, intKind, parseInt(intKind, b))
	e := &unification{
		binSrc(token.NoPos, opUnify, from, to),
		[]evaluated{from, to},
	}
	// TODO: make this an integer
	// int := &basicType{k: intKind}
	// e = &unification{
	// 	binSrc(token.NoPos, opUnify, int, e),
	// 	[]evaluated{int, e},
	// }
	return e
}

func mkFloatRange(a, b string) evaluated {
	from := newBound(nil, baseValue{}, opGeq, numKind, parseFloat(a))
	to := newBound(nil, baseValue{}, opLeq, numKind, parseFloat(b))
	e := &unification{
		binSrc(token.NoPos, opUnify, from, to),
		[]evaluated{from, to},
	}
	// TODO: make this an integer
	// int := &basicType{k: intKind}
	// e = &unification{
	// 	binSrc(token.NoPos, opUnify, int, e),
	// 	[]evaluated{int, e},
	// }
	return e
}

var predefinedRanges = map[string]evaluated{
	"rune":  mkIntRange("0", strconv.Itoa(0x10FFFF)),
	"int8":  mkIntRange("-128", "127"),
	"int16": mkIntRange("-32768", "32767"),
	"int32": mkIntRange("-2147483648", "2147483647"),
	"int64": mkIntRange("-9223372036854775808", "9223372036854775807"),
	"int128": mkIntRange(
		"-170141183460469231731687303715884105728",
		"170141183460469231731687303715884105727"),

	// Do not include an alias for "byte", as it would be too easily confused
	// with the builtin "bytes".
	"uint":    newBound(nil, baseValue{}, opGeq, intKind, parseInt(intKind, "0")),
	"uint8":   mkIntRange("0", "255"),
	"uint16":  mkIntRange("0", "65535"),
	"uint32":  mkIntRange("0", "4294967295"),
	"uint64":  mkIntRange("0", "18446744073709551615"),
	"uint128": mkIntRange("0", "340282366920938463463374607431768211455"),

	// 2**127 * (2**24 - 1) / 2**23
	"float32": mkFloatRange(
		"-3.40282346638528859811704183484516925440e+38",
		"+3.40282346638528859811704183484516925440e+38",
	),
	// 2**1023 * (2**53 - 1) / 2**52
	"float64": mkFloatRange(
		"-1.797693134862315708145274237317043567981e+308",
		"+1.797693134862315708145274237317043567981e+308",
	),
}

type interpolation struct {
	baseValue
	k     kind    // string or bytes
	parts []value // odd: strings, even expressions
}

func (x *interpolation) kind() kind { return x.k | nonGround }

type list struct {
	baseValue
	elem *structLit

	typ value

	// TODO: consider removing len. Currently can only be len(a) or >= len(a)
	// and could be replaced with a bool.
	len value
}

// initLit initializes a literal list.
func (x *list) initLit() {
	x.len = newInt(x, 0).setInt(len(x.elem.arcs))
	x.typ = &top{x.baseValue}
}

func (x *list) manifest(ctx *context) evaluated {
	if x.kind().isGround() {
		return x
	}
	// A list is ground if its length is ground, or if the current length
	// meets matches the cap.
	n := newInt(x, 0).setInt(len(x.elem.arcs))
	if n := binOp(ctx, x, opUnify, n, x.len.evalPartial(ctx)); !isBottom(n) {
		return &list{
			baseValue: x.baseValue,
			elem:      x.elem,
			len:       n,
			typ:       &top{x.baseValue},
		}
	}
	return x
}

func (x *list) kind() kind {
	k := listKind
	if _, ok := x.len.(*numLit); ok {
		return k
	}
	return k | nonGround
}

// at returns the evaluated and original value of position i. List x must
// already have been evaluated. It returns an error and nil if there was an
// issue evaluating the list itself.
func (x *list) at(ctx *context, i int) evaluated {
	arc := x.iterAt(ctx, i)
	if arc.cache == nil {
		return ctx.mkErr(x, "index %d out of bounds", i)
	}
	return arc.cache
}

// iterAt returns the evaluated and original value of position i. List x must
// already have been evaluated. It returns an error and nil if there was an
// issue evaluating the list itself.
func (x *list) iterAt(ctx *context, i int) arc {
	if i < 0 {
		v := ctx.mkErr(x, "index %d out of bounds", i)
		return arc{cache: v}
	}
	if i < len(x.elem.arcs) {
		a := x.elem.iterAt(ctx, i)
		a.feature = 0
		return a
	}
	max := maxNum(x.len.(evaluated))
	if max.kind().isGround() {
		if max.kind()&intKind == bottomKind {
			v := ctx.mkErr(max, "length indicator of list not of type int")
			return arc{cache: v}
		}
		n := max.(*numLit).intValue(ctx)
		if i >= n {
			return arc{}
		}
	}

	// XXX need to return attrs here?
	return arc{cache: x.typ.evalPartial(ctx), v: x.typ}
}

func (x *list) isOpen() bool {
	return !x.len.kind().isGround()
}

// lo and hi must be nil or a ground integer.
func (x *list) slice(ctx *context, lo, hi *numLit) evaluated {
	a := x.elem.arcs
	max := maxNum(x.len).evalPartial(ctx)
	if hi != nil {
		n := hi.intValue(ctx)
		if n < 0 {
			return ctx.mkErr(x, "negative slice index")
		}
		if max.kind().isGround() && !leq(ctx, hi, hi, max) {
			return ctx.mkErr(hi, "slice bounds out of range")
		}
		max = hi
		if n < len(a) {
			a = a[:n]
		}
	}

	if lo != nil {
		n := lo.intValue(ctx)
		if n < 0 {
			return ctx.mkErr(x, "negative slice index")
		}
		if n > 0 && max.kind().isGround() {
			if !leq(ctx, lo, lo, max) {
				max := max.(*numLit).intValue(ctx)
				return ctx.mkErr(x, "invalid slice index: %v > %v", n, max)
			}
			max = binOp(ctx, lo, opSub, max, lo)
		}
		if n < len(a) {
			a = a[n:]
		} else {
			a = []arc{}
		}
	}
	arcs := make([]arc, len(a))
	for i, a := range a {
		arcs[i] = arc{feature: label(i), v: a.v, docs: a.docs}
	}
	s := &structLit{baseValue: x.baseValue, arcs: arcs}
	return &list{baseValue: x.baseValue, elem: s, typ: x.typ, len: max}
}

// An structLit is a single structLit in the configuration tree.
//
// An structLit may have multiple arcs. There may be only one arc per label. Use
// insertRaw to insert arcs to ensure this invariant holds.
type structLit struct {
	baseValue

	// TODO(perf): separate out these infrequent values to save space.
	emit value // currently only supported at top level.
	// TODO: make this a list of templates and don't unify until templates are
	// applied. This allows generalization of having different constraints
	// for different field sets. This could also be used to mark closedness:
	// use [string]: _ for fully open. This could be a sentinel value.
	// For now we use a boolean for closedness.

	// NOTE: must be conjunction of lists.
	// For lists originating from closed structs,
	// there must be at least one match.
	// templates [][]value
	// catch_all: value

	// optionals holds pattern-constraint pairs that
	// are applied to all concrete values in this struct.
	optionals   *optionals
	closeStatus closeMode

	comprehensions []compValue

	// TODO: consider hoisting the template arc to its own value.
	arcs     []arc
	expanded evaluated
}

// compValue is a temporary stop-gap until the correct unification algorithm is
// implemented. This implementation is more strict than should be. When two
// structs, of which at least one is closed, are unified, the fields resolving
// later from unresolved comprehensions should match the closedness constraints.
// To relax this constraint, unification could follow the lines of
// traditional unification with bookkeeping of which fields are
// allowed, to be applied as constraints after full unification.

type compValue struct {
	checked bool
	comp    value
}

// optionals holds a set of key pattern-constraint pairs, where constraints are
// to be applied to concrete fields of which the label matches the key pattern.
//
// optionals will either hold concrete fields or a couple of nested optional
// structs combined based on the op type, but not both.
type optionals struct {
	closed closeMode
	op     op
	left   *optionals // nil means empty closed struct
	right  *optionals // nil means empty closed struct
	fields []optionalSet
}

type optionalSet struct {
	// A key filter may be nil, in which case it means all strings, or _.
	key value

	// constraint must evaluate to a lambda and is applied to any concrete
	// value for which the key matches key.
	value value
}

func newOptional(key, value value) *optionals {
	return &optionals{
		fields: []optionalSet{{key, value}},
	}
}

// isClosed mirrors the closed status of the struct to which
// this optionals belongs.
func (o *optionals) isClosed() bool {
	if o == nil {
		return true
	}
	return o.closed.isClosed()
}

func (o *optionals) close() *optionals {
	if o == nil {
		return nil
	}
	o.closed |= isClosed
	return o
}

// isEmpty reports whether this optionals may report true for match. Even if an
// optionals is empty, it may still hold constraints to be applied to already
// existing concrete fields.
func (o *optionals) isEmpty() bool {
	if o == nil {
		return true
	}
	le := o.left.isEmpty()
	re := o.right.isEmpty()

	if o.op == opUnify {
		if le && o.left.isClosed() {
			return true
		}
		if re && o.right.isClosed() {
			return true
		}
	}
	return le && re && len(o.fields) == 0
}

// isFull reports whether match reports true for all fields.
func (o *optionals) isFull() bool {
	found, _ := o.match(nil, nil)
	return found
}

// match reports whether a field with the given name may be added in the
// associated struct as a new field. ok is false if there was any closed
// struct that failed to match. Even if match returns false, there may still be
// constraints represented by optionals that are to be applied to existing
// concrete fields.
func (o *optionals) match(ctx *context, str *stringLit) (found, ok bool) {
	if o == nil {
		return false, true
	}

	found1, ok := o.left.match(ctx, str)
	if !ok && o.op == opUnify {
		return false, false
	}

	found2, ok := o.right.match(ctx, str)
	if !ok && o.op == opUnify {
		return false, false
	}

	if found1 || found2 {
		return true, true
	}

	for _, f := range o.fields {
		if f.key == nil {
			return true, true
		}
		if str != nil {
			v := binOp(ctx, f.value, opUnify, f.key.evalPartial(ctx), str)
			if !isBottom(v) {
				return true, true
			}
		}
	}

	return false, !o.closed.isClosed()
}

func (o *optionals) allows(ctx *context, f label) bool {
	if o == nil {
		return false
	}

	str := ctx.labelStr(f)
	arg := &stringLit{str: str}

	found, ok := o.match(ctx, arg)
	return found && ok
}

func (o *optionals) add(ctx *context, key, value value) {
	for i, b := range o.fields {
		if b.key == key {
			o.fields[i].value = mkBin(ctx, token.NoPos, opUnify, b.value, value)
			return
		}
	}
	o.fields = append(o.fields, optionalSet{key, value})
}

// isDotDotDot reports whether optionals only contains fully-qualified
// constraints. This is useful for some optimizations.
func (o *optionals) isDotDotDot() bool {
	if o == nil {
		return false
	}
	if len(o.fields) > 1 {
		return false
	}
	if len(o.fields) == 1 {
		f := o.fields[0]
		if f.key != nil {
			return false
		}
		lambda, ok := f.value.(*lambdaExpr)
		if ok {
			if _, ok = lambda.value.(*top); ok {
				return true
			}
		}
		return false
	}
	if o.left == nil {
		return o.right.isDotDotDot()
	}
	if o.right == nil {
		return o.left.isDotDotDot()
	}
	return o.left.isDotDotDot() && o.right.isDotDotDot()
}

// constraint returns the unification of all constraints for which arg matches
// the key filter. doc contains the documentation of all applicable fields.
func (o *optionals) constraint(ctx *context, label evaluated) (u value, doc *docNode) {
	if o == nil {
		return nil, nil
	}
	add := func(v value) {
		if v != nil {
			if u == nil {
				u = v
			} else {
				u = mkBin(ctx, token.NoPos, opUnify, u, v)
			}
		}
	}
	v, doc1 := o.left.constraint(ctx, label)
	add(v)
	v, doc2 := o.right.constraint(ctx, label)
	add(v)

	if doc1 != nil || doc2 != nil {
		doc = &docNode{left: doc1, right: doc2}
	}

	arg := label
	if arg == nil {
		arg = &basicType{k: stringKind}
	}

	for _, s := range o.fields {
		if s.key != nil {
			if label == nil {
				continue
			}
			key := s.key.evalPartial(ctx)
			if v := binOp(ctx, label, opUnify, key, label); isBottom(v) {
				continue
			}
		}
		fn, ok := ctx.manifest(s.value).(*lambdaExpr)
		if !ok {
			// create error
			continue
		}
		add(fn.call(ctx, s.value, arg))
		if f, _ := s.value.base().syntax().(*ast.Field); f != nil {
			doc = &docNode{n: f, left: doc}
		}
	}
	return u, doc
}

func (o *optionals) rewrite(fn func(value) value) (c *optionals, err evaluated) {
	if o == nil {
		return nil, nil
	}

	left, err := o.left.rewrite(fn)
	if err != nil {
		return nil, err
	}
	right, err := o.right.rewrite(fn)
	if err != nil {
		return nil, err
	}

	fields := make([]optionalSet, len(o.fields))
	for i, s := range o.fields {
		if s.key != nil {
			s.key = fn(s.key)
			if b, ok := s.key.(*bottom); ok {
				return nil, b
			}
		}
		s.value = fn(s.value)
		if b, ok := s.value.(*bottom); ok {
			return nil, b
		}
		fields[i] = s
	}

	return &optionals{o.closed, o.op, left, right, fields}, nil
}

type closeMode byte

const (
	shouldFinalize closeMode = 1 << iota
	toClose
	isClosed
)

func (m closeMode) shouldFinalize() bool {
	return m&shouldFinalize != 0
}

func (m *closeMode) unclose() {
	*m &^= (toClose | isClosed)
}

func (m closeMode) isClosed() bool {
	return m&isClosed != 0
}

func (m closeMode) shouldClose() bool {
	return m >= toClose
}

func (x *structLit) isClosed() bool {
	return x.closeStatus.isClosed()
}

func (x *structLit) addTemplate(ctx *context, pos token.Pos, key, value value) {
	if x.optionals == nil {
		x.optionals = &optionals{}
	}
	x.optionals.add(ctx, key, value)
}

func (x *structLit) allows(ctx *context, f label) bool {
	return !x.closeStatus.isClosed() ||
		f&hidden != 0 ||
		x.optionals.allows(ctx, f)
}

func newStruct(src source) *structLit {
	return &structLit{baseValue: src.base()}
}

func (x *structLit) kind() kind { return structKind }

type arcs []arc

func (x *structLit) Len() int           { return len(x.arcs) }
func (x *structLit) Less(i, j int) bool { return x.arcs[i].feature < x.arcs[j].feature }
func (x *structLit) Swap(i, j int)      { x.arcs[i], x.arcs[j] = x.arcs[j], x.arcs[i] }

func (x *structLit) close() *structLit {
	if x.optionals.isFull() {
		return x
	}

	newS := *x
	newS.closeStatus = isClosed
	return &newS
}

// lookup returns the node for the given label f, if present, or nil otherwise.
func (x *structLit) lookup(ctx *context, f label) arc {
	x, err := x.expandFields(ctx)
	if err != nil {
		return arc{}
	}
	// Lookup is done by selector or index references. Either this is done on
	// literal nodes or nodes obtained from references. In the later case,
	// noderef will have ensured that the ancestors were evaluated.
	for i, a := range x.arcs {
		if a.feature == f {
			a := x.iterAt(ctx, i)
			// TODO: adding more technical debt here. The evaluator should be
			// rewritten.
			if x.optionals != nil {
				name := ctx.labelStr(x.arcs[i].feature)
				arg := &stringLit{x.baseValue, name, nil}

				val, _ := x.optionals.constraint(ctx, arg)
				if val != nil {
					a.v = mkBin(ctx, x.Pos(), opUnify, a.v, val)
				}
			}
			return a
		}
	}
	return arc{}
}

func (x *structLit) iterAt(ctx *context, i int) arc {
	fmt.Println("structLit.iterAt - x.attrs", x.arcs[i].attrs)
	x, err := x.expandFields(ctx)
	if err != nil || i >= len(x.arcs) {
		return arc{}
	}
	a := x.arcs[i]
	a.cache = x.at(ctx, i) // TODO: return template & v for original?
	fmt.Println("structLit.iterAt - a.attrs", a.attrs)
	a.attrs = x.arcs[i].attrs
	return a
}

func (x *structLit) at(ctx *context, i int) evaluated {
	// if x.arcs[i].attrs != nil {
		fmt.Println("structLit.at", i, x.arcs[i].attrs, x.emit != nil)
	// }
	// TODO: limit visibility of definitions:
	// Approach:
	// - add package identifier to arc (label)
	// - assume ctx is unique for a package
	// - record package identifier in context
	// - if arc is a definition, check IsExported and verify the package if not.
	//
	// The same approach could be valid for looking up package-level identifiers.
	// - detect somehow aht root nodes are.
	//
	// Allow import of CUE files. These cannot have a package clause.

	var err *bottom

	// Lookup is done by selector or index references. Either this is done on
	// literal nodes or nodes obtained from references. In the later case,
	// noderef will have ensured that the ancestors were evaluated.
	if v := x.arcs[i].cache; v == nil {
		// fmt.Println("  V is NIL")

		// cycle detection

		popped := ctx.evalStack
		ctx.evalStack = append(ctx.evalStack, bottom{
			baseValue: x.base(),
			index:     ctx.index,
			code:      codeCycle,
			value:     x.arcs[i].v,
			format:    "cycle detected",
		})
		x.arcs[i].cache = &(ctx.evalStack[len(ctx.evalStack)-1])

		v := x.arcs[i].v.evalPartial(ctx)
		ctx.evalStack = popped

		var doc *docNode
		v, doc = x.applyTemplate(ctx, i, v)
		// only place to apply template?

		if (len(ctx.evalStack) > 0 && ctx.cycleErr) || cycleError(v) != nil {
			// Don't cache while we're in a evaluation cycle as it will cache
			// partial results. Each field involved in the cycle will have to
			// reevaluated the values from scratch. As the result will be
			// cached after one cycle, it will evaluate the cycle at most twice.
			x.arcs[i].cache = nil
			return v
		}

		// If there as a cycle error, we have by now evaluated a full cycle and
		// it is safe to cache the result.
		ctx.cycleErr = false

		v = updateCloseStatus(ctx, v)
		if st, ok := v.(*structLit); ok {
			v, err = st.expandFields(ctx)
			if err != nil {
				v = err
			}
		}
		x.arcs[i].cache = v


		if doc != nil {
			x.arcs[i].docs = &docNode{left: doc, right: x.arcs[i].docs}
		}
		if len(ctx.evalStack) == 0 {
			if err := ctx.processDelayedConstraints(); err != nil {
				x.arcs[i].cache = err
			}
		}

	} else if b := cycleError(v); b != nil {
		copy := *b
		return &copy
	}

	// fmt.Println("structLit.at - return")
	fmt.Printf("structLit.at %d - return %+v\n", i, x.arcs[i].cache)
	return x.arcs[i].cache
}

// expandFields merges in embedded and interpolated fields.
// Such fields are semantically equivalent to child values, and thus
// should not be evaluated until the other fields of a struct are
// fully evaluated.
func (x *structLit) expandFields(ctx *context) (st *structLit, err *bottom) {
	switch v := x.expanded.(type) {
	case nil:
	case *structLit:
		return v, nil
	default:
		return nil, x.expanded.(*bottom)
	}
	if len(x.comprehensions) == 0 {
		x.expanded = x
		return x, nil
	}

	x.expanded = x

	comprehensions := x.comprehensions

	var incomplete []compValue

	var n evaluated = &top{x.baseValue}
	if x.emit != nil {
		n = x.emit.evalPartial(ctx)
	}

	var checked evaluated = &top{x.baseValue}

	for _, x := range comprehensions {
		v := x.comp.evalPartial(ctx)
		if v, ok := v.(*bottom); ok {
			if isIncomplete(v) {
				incomplete = append(incomplete, x)
				continue
			}

			return nil, v
		}
		src := binSrc(x.comp.Pos(), opUnify, x.comp, v)
		_ = checked
		if x.checked {
			checked = binOp(ctx, src, opUnifyUnchecked, checked, v)
		} else {
			n = binOp(ctx, src, opUnifyUnchecked, n, v)
		}
	}
	if len(comprehensions) == len(incomplete) {
		return x, nil
	}

	switch n.(type) {
	case *bottom, *top:
	default:
		orig := x.comprehensions
		x.comprehensions = incomplete
		src := binSrc(x.Pos(), opUnify, x, n)
		n = binOp(ctx, src, opUnifyUnchecked, x, n)
		x.comprehensions = orig
	}

	switch checked.(type) {
	case *bottom, *top:
	default:
		orig := x.comprehensions
		x.comprehensions = incomplete
		src := binSrc(x.Pos(), opUnify, n, checked)
		n = binOp(ctx, src, opUnify, x, checked)
		x.comprehensions = orig
	}

	switch v := n.(type) {
	case *bottom:
		x.expanded = n
		return nil, v
	case *structLit:
		x.expanded = n
		return v, nil

	default:
		x.expanded = x
		return x, nil
	}
}

func (x *structLit) applyTemplate(ctx *context, i int, v evaluated) (e evaluated, doc *docNode) {
	if x.optionals == nil {
		return v, nil
	}

	name := ctx.labelStr(x.arcs[i].feature)
	arg := &stringLit{x.baseValue, name, nil}

	val, doc := x.optionals.constraint(ctx, arg)
	if val != nil {
		v = binOp(ctx, x, opUnify, v, val.evalPartial(ctx))
	}

	if x.closeStatus != 0 {
		v = updateCloseStatus(ctx, v)
	}
	return v, doc
}

// A label is a canonicalized feature name.
type label uint32

const hidden label = 0x01 // only set iff identifier starting with _

// An arc holds the label-value pair.
//
// A fully evaluated arc has either a node or a value. An unevaluated arc,
// however, may have both. In this case, the value must ultimately evaluate
// to a node, which will then be merged with the existing one.
type arc struct {
	feature    label
	optional   bool
	definition bool // field is a definition

	// TODO: add index to preserve approximate order within a struct and use
	// topological sort to compute new struct order when unifying. This could
	// also be achieved by not sorting labels on features and doing
	// a linear search in fields.

	v     value
	cache evaluated // also used as newValue during unification.
	attrs *attributes
	docs  *docNode
}

type docNode struct {
	n     *ast.Field
	left  *docNode
	right *docNode
}

func (d *docNode) appendDocs(docs []*ast.CommentGroup) []*ast.CommentGroup {
	if d == nil {
		return docs
	}
	docs = d.left.appendDocs(docs)
	if d.n != nil {
		docs = appendDocComments(docs, d.n)
		docs = appendDocComments(docs, d.n.Label)
	}
	docs = d.right.appendDocs(docs)
	return docs
}

func appendDocComments(docs []*ast.CommentGroup, n ast.Node) []*ast.CommentGroup {
	for _, c := range n.Comments() {
		if c.Doc {
			docs = append(docs, c)
		}
	}
	return docs
}

func mergeDocs(a, b *docNode) *docNode {
	if a == b || a == nil {
		return b
	}
	if b == nil {
		return a
	}
	// TODO: filter out duplicates?
	return &docNode{nil, a, b}
}

func (a *arc) val() evaluated {
	return a.cache
}

func (a *arc) setValue(v value) {
	a.v = v
	a.cache = nil
}

type closeIfStruct struct {
	value
}

func wrapFinalize(ctx *context, v value) value {
	if v.kind().isAnyOf(structKind | listKind) {
		switch x := v.(type) {
		case *top:
			return v
		case *structLit:
			v = updateCloseStatus(ctx, x)
		case *list:
			v = updateCloseStatus(ctx, x)
		case *disjunction:
			v = updateCloseStatus(ctx, x)
		case *closeIfStruct:
			return x
		}
		return &closeIfStruct{v}
	}
	return v
}

func updateCloseStatus(ctx *context, v evaluated) evaluated {
	switch x := v.(type) {
	case *structLit:
		if x.closeStatus.shouldClose() {
			x.closeStatus = isClosed
			x.optionals = x.optionals.close()
		}
		x.closeStatus |= shouldFinalize
		return x

	case *disjunction:
		for _, d := range x.values {
			d.val = wrapFinalize(ctx, d.val)
		}

	case *list:
		wrapFinalize(ctx, x.elem)
		if x.typ != nil {
			wrapFinalize(ctx, x.typ)
		}
	}
	return v
}

// insertValue is used during initialization but never during evaluation.
func (x *structLit) insertValue(ctx *context, f label, optional, isDef bool, value value, a *attributes, docs *docNode) {
	for i, p := range x.arcs {
		if f != p.feature {
			continue
		}
		x.arcs[i].optional = x.arcs[i].optional && optional
		x.arcs[i].docs = mergeDocs(x.arcs[i].docs, docs)
		x.arcs[i].v = mkBin(ctx, token.NoPos, opUnify, p.v, value)
		if isDef != p.definition {
			src := binSrc(token.NoPos, opUnify, p.v, value)
			x.arcs[i].v = ctx.mkErr(src,
				"field %q declared as definition and regular field",
				ctx.labelStr(f))
			isDef = false
		}
		x.arcs[i].definition = isDef
		// TODO: should we warn if there is a mixed mode of optional and non
		// optional fields at this point?
		return
	}
	x.arcs = append(x.arcs, arc{f, optional, isDef, value, nil, a, docs})
	sort.Stable(x)
}

// A nodeRef is a reference to a node.
type nodeRef struct {
	baseValue
	node  scope
	label label // for direct ancestor nodes
}

func (x *nodeRef) kind() kind {
	// TODO(REWORK): no context available
	// n := x.node.deref(nil)
	n := x.node
	return n.kind() | nonGround | referenceKind
}

type selectorExpr struct {
	baseValue
	x       value
	feature label
}

// TODO: could this be narrowed down?
func (x *selectorExpr) kind() kind {
	isRef := x.x.kind() & referenceKind
	return topKind | isRef
}

type indexExpr struct {
	baseValue
	x     value
	index value
}

// TODO: narrow this down when we have list types.
func (x *indexExpr) kind() kind { return topKind | referenceKind }

type sliceExpr struct {
	baseValue
	x  value
	lo value
	hi value
}

// TODO: narrow this down when we have list types.
func (x *sliceExpr) kind() kind { return topKind | referenceKind }

type callExpr struct {
	baseValue
	x    value
	args []value
}

func (x *callExpr) kind() kind {
	// TODO: could this be narrowed down?
	switch c := x.x.(type) {
	case *lambdaExpr:
		return c.returnKind() | nonGround
	case *builtin:
		switch len(x.args) {
		case len(c.Params):
			return c.Result
		case len(c.Params) - 1:
			if len(c.Params) == 0 || c.Result&boolKind == 0 {
				return bottomKind
			}
			return c.Params[0]
		}
	}
	return topKind | referenceKind
}

type customValidator struct {
	baseValue

	args []evaluated // any but the first value
	call *builtin    // function must return a bool
}

func (x *customValidator) kind() kind {
	if len(x.call.Params) == 0 {
		return bottomKind
	}
	return x.call.Params[0] | nonGround
}

type params struct {
	arcs []arc
}

func (x *params) add(f label, v value) {
	if v == nil {
		panic("nil node")
	}
	x.arcs = append(x.arcs, arc{feature: f, v: v})
}

func (x *params) iterAt(ctx *context, i int) (evaluated, value) {
	if i >= len(x.arcs) {
		return nil, nil
	}
	return x.at(ctx, i), x.arcs[i].v
}

// lookup returns the node for the given label f, if present, or nil otherwise.
func (x *params) at(ctx *context, i int) evaluated {
	// Lookup is done by selector or index references. Either this is done on
	// literal nodes or nodes obtained from references. In the later case,
	// noderef will have ensured that the ancestors were evaluated.
	if x.arcs[i].cache == nil {
		x.arcs[i].cache = x.arcs[i].v.evalPartial(ctx)
	}
	return x.arcs[i].cache
}

// lookup returns the node for the given label f, if present, or nil otherwise.
func (x *params) lookup(ctx *context, f label) arc {
	if f == 0 && len(x.arcs) == 1 {
		// A template binding.
		a := x.arcs[0]
		a.cache = x.at(ctx, 0)
		return a
	}
	// Lookup is done by selector or index references. Either this is done on
	// literal nodes or nodes obtained from references. In the later case,
	// noderef will have ensured that the ancestors were evaluated.
	for i, a := range x.arcs {
		if a.feature == f {
			a.cache = x.at(ctx, i)
			return a
		}
	}
	return arc{}
}

type lambdaExpr struct {
	baseValue
	*params
	value value
}

// TODO: could this be narrowed down?
func (x *lambdaExpr) kind() kind       { return lambdaKind }
func (x *lambdaExpr) returnKind() kind { return x.value.kind() }

// call calls and evaluates a  lambda expression. It is assumed that x may be
// destroyed, either because it is copied as a result of a reference or because
// it is invoked as a literal.
func (x *lambdaExpr) call(ctx *context, p source, args ...evaluated) value {
	// fully evaluated.
	if len(x.params.arcs) != len(args) {
		return ctx.mkErr(p, x, "number of arguments does not match (%d vs %d)",
			len(x.params.arcs), len(args))
	}

	// force parameter substitution. It is important that the result stands on
	// its own and does not depend on its input parameters.
	arcs := make(arcs, len(x.arcs))
	for i, a := range x.arcs {
		v := binOp(ctx, p, opUnify, a.v.evalPartial(ctx), args[i])
		if isBottom(v) {
			return v
		}
		arcs[i] = arc{feature: a.feature, v: v, cache: v}
	}
	lambda := &lambdaExpr{x.baseValue, &params{arcs}, nil}
	defer ctx.pushForwards(x, lambda).popForwards()
	obj := ctx.copy(x.value)
	return obj
}

// Operations

type unaryExpr struct {
	baseValue
	op op
	x  value
}

func (x *unaryExpr) kind() kind { return x.x.kind() }

func compileRegexp(ctx *context, v value) value {
	var err error
	switch x := v.(type) {
	case *stringLit:
		if x.re == nil {
			x.re, err = regexp.Compile(x.str)
			if err != nil {
				return ctx.mkErr(v, "could not compile regular expression %q: %v", x.str, err)
			}
		}
	case *bytesLit:
		if x.re == nil {
			x.re, err = regexp.Compile(string(x.b))
			if err != nil {
				return ctx.mkErr(v, "could not compile regular expression %q: %v", x.b, err)
			}
		}
	}
	return v
}

type binaryExpr struct {
	baseValue
	op    op
	left  value
	right value
}

func mkBin(ctx *context, pos token.Pos, op op, left, right value) value {
	if left == nil || right == nil {
		panic("operands may not be nil")
	}
	if op == opUnify {
		if left == right {
			return left
		}
		if _, ok := left.(*top); ok {
			return right
		}
		if _, ok := right.(*top); ok {
			return left
		}
		// TODO(perf): consider adding a subsumption filter.
		// if subsumes(ctx, left, right) {
		// 	return right
		// }
		// if subsumes(ctx, right, left) {
		// 	return left
		// }
	}
	bin := &binaryExpr{binSrc(pos, op, left, right), op, left, right}
	return updateBin(ctx, bin)
}

func updateBin(ctx *context, bin *binaryExpr) value {
	switch bin.op {
	case opMat, opNMat:
		bin.right = compileRegexp(ctx, bin.right)
		if isBottom(bin.right) {
			return bin.right
		}
	}
	return bin
}

func (x *binaryExpr) kind() kind {
	// TODO: cache results
	kind, _, _ := matchBinOpKind(x.op, x.left.kind(), x.right.kind())
	return kind | nonGround
}

// unification collects evaluated values that are not mutually exclusive
// but cannot be represented as a single value. It allows doing the bookkeeping
// on accumulating conjunctions, simplifying them along the way, until they do
// resolve into a single value.
type unification struct {
	baseValue
	values []evaluated
}

func (x *unification) kind() kind {
	k := topKind
	for _, v := range x.values {
		k &= v.kind()
	}
	return k | nonGround
}

type disjunction struct {
	baseValue

	values []dValue

	// errors is used to keep track of all errors that occurred in
	// a disjunction for better error reporting down the road.
	// TODO: consider storing the errors in values.
	errors []*bottom

	hasDefaults bool // also true if it had elminated defaults.

	// bind is the node that a successful disjunction will bind to. This
	// allows other arcs to point to this node before the disjunction is
	// completed. For early failure, this node can be set to the glb of all
	// disjunctions. Otherwise top will suffice.
	// bind node
}

type dValue struct {
	val    value
	marked bool
}

func (x *disjunction) kind() kind {
	k := kind(0)
	for _, v := range x.values {
		k |= v.val.kind()
	}
	if k != bottomKind {
		k |= nonGround
	}
	return k
}

func (x *disjunction) Pos() token.Pos { return x.values[0].val.Pos() }

// add add a value to the disjunction. It is assumed not to be a disjunction.
func (x *disjunction) add(ctx *context, v value, marked bool) {
	x.values = append(x.values, dValue{v, marked})
	if b, ok := v.(*bottom); ok {
		x.errors = append(x.errors, b)
	}
}

// normalize removes redundant element from unification.
// x must already have been evaluated.
func (x *disjunction) normalize(ctx *context, src source) mVal {
	leq := func(ctx *context, lt, gt dValue) bool {
		if isBottom(lt.val) {
			return true
		}
		s := subsumer{ctx: ctx}
		return (!lt.marked || gt.marked) && s.subsumes(gt.val, lt.val)
	}
	k := 0

	hasMarked := false
	var markedErr *bottom
outer:
	for i, v := range x.values {
		// TODO: this is pre-evaluation is quite aggressive. Verify whether
		// this does not trigger structural cycles (it does). If so, this can check for
		// bottom and the validation can be delayed to as late as picking
		// defaults. The drawback of this approach is that printed intermediate
		// results will not look great.
		if err := validate(ctx, v.val); err != nil {
			x.errors = append(x.errors, err)
			if v.marked {
				markedErr = err
			}
			continue
		}
		if v.marked {
			hasMarked = true
		}
		for j, w := range x.values {
			if i == j {
				continue
			}
			if leq(ctx, v, w) && (!leq(ctx, w, v) || j < i) {
				// strictly subsumed, or equal and and the equal element was
				// processed earlier.
				continue outer
			}
		}
		// If there was a three-way equality, an element w, where w == v could
		// already have been added.
		for j := 0; j < k; j++ {
			if leq(ctx, v, x.values[j]) {
				continue outer
			}
		}
		// TODO: do not modify value, but create a new disjunction.
		x.values[k] = v
		k++
	}
	if !hasMarked && markedErr != nil && (k > 1 || !x.values[0].val.kind().isGround()) {
		x.values[k] = dValue{&bottom{}, true}
		k++
	}

	switch k {
	case 0:
		// Empty disjunction. All elements must be errors.
		// Take the first error as an example.
		err := x.values[0].val
		if !isBottom(err) {
			// TODO: use format instead of debugStr.
			err = ctx.mkErr(src, ctx.str(err))
		}
		return mVal{x.computeError(ctx, src), false}
	case 1:
		v := x.values[0]
		return mVal{v.val.(evaluated), v.marked}
	}
	// TODO: do not modify value, but create a new disjunction.
	x.values = x.values[:k]
	return mVal{x, false}
}

func (x *disjunction) computeError(ctx *context, src source) evaluated {
	var errors []*bottom

	// Ensure every position is visited at least once.
	// This prevents discriminators fields from showing up too much. A special
	// "all errors" flag could be used to expand all errors.
	visited := map[token.Pos]bool{}

	for _, b := range x.errors {
		positions := b.Positions(ctx)
		if len(positions) == 0 {
			positions = append(positions, token.NoPos)
		}
		// Include the error if at least one of its positions wasn't covered
		// before.
		done := true
		for _, p := range positions {
			if !visited[p] {
				done = false
			}
			visited[p] = true
		}
		if !done {
			b := *b
			b.format = "empty disjunction: " + b.format
			errors = append(errors, &b)
		}
	}
	switch len(errors) {
	case 0:
		// Should never happen.
		return ctx.mkErr(src, errors, "empty disjunction")
	case 1:
		return ctx.mkErr(src, errors, "empty disjunction: %v", errors[0])
	default:
		return ctx.mkErr(src, errors, "empty disjunction: %v (and %d other errors)", errors[0], len(errors)-1)
	}
}

type listComprehension struct {
	baseValue
	clauses yielder
}

func (x *listComprehension) kind() kind {
	return listKind | nonGround | referenceKind
}

type structComprehension struct {
	baseValue
	clauses yielder
}

func (x *structComprehension) kind() kind {
	return structKind | nonGround | referenceKind
}

// TODO: rename to something better. No longer a comprehension.
// Generated field, perhaps.
type fieldComprehension struct {
	baseValue
	key   value
	val   value
	opt   bool
	def   bool
	doc   *docNode
	attrs *attributes
}

func (x *fieldComprehension) kind() kind {
	return structKind | nonGround
}

type yieldFunc func(v evaluated) *bottom

type yielder interface {
	value
	yield(*context, yieldFunc) *bottom
}

type yield struct {
	baseValue
	value value
}

func (x *yield) kind() kind { return topKind | referenceKind }

func (x *yield) yield(ctx *context, fn yieldFunc) *bottom {
	v := x.value.evalPartial(ctx)
	if err, ok := v.(*bottom); ok {
		return err
	}
	if err := fn(v); err != nil {
		return err
	}
	return nil
}

type guard struct { // rename to guard
	baseValue
	condition value
	value     yielder
}

func (x *guard) kind() kind { return topKind | referenceKind }

func (x *guard) yield(ctx *context, fn yieldFunc) *bottom {
	filter := ctx.manifest(x.condition)
	if err, ok := filter.(*bottom); ok {
		return err
	}
	if err := checkKind(ctx, filter, boolKind); err != nil {
		return err
	}
	if filter.(*boolLit).b {
		if err := x.value.yield(ctx, fn); err != nil {
			return err
		}
	}
	return nil
}

type feed struct {
	baseValue
	source value
	fn     *lambdaExpr
}

func (x *feed) kind() kind { return topKind | referenceKind }

func (x *feed) yield(ctx *context, yfn yieldFunc) (result *bottom) {
	if ctx.trace {
		defer uni(indent(ctx, "feed", x))
	}
	source := ctx.manifest(x.source)
	fn := x.fn // no need to evaluate eval

	switch src := source.(type) {
	case *structLit:
		var err *bottom
		src, err = src.expandFields(ctx)
		if err != nil {
			return err
		}
		for i, a := range src.arcs {
			key := &stringLit{
				x.baseValue,
				ctx.labelStr(a.feature),
				nil,
			}
			if a.definition || a.optional || a.feature&hidden != 0 {
				continue
			}
			val := src.at(ctx, i)
			v := fn.call(ctx, x, key, val)
			if err, ok := v.(*bottom); ok {
				return err
			}
			if err := v.(yielder).yield(ctx, yfn); err != nil {
				return err
			}
		}
		return nil

	case *list:
		for i := range src.elem.arcs {
			idx := newInt(x, 0).setInt(i)
			v := fn.call(ctx, x, idx, src.at(ctx, i))
			if err, ok := v.(*bottom); ok {
				return err
			}
			if err := v.(yielder).yield(ctx, yfn); err != nil {
				return err
			}
		}
		return nil

	default:
		if err, ok := source.(*bottom); ok {
			return err
		}
		if k := source.kind(); k&(structKind|listKind) == bottomKind {
			return ctx.mkErr(x, x.source, "feed source must be list or struct, found %s", k)
		}
		return ctx.mkErr(x, x.source, codeIncomplete, "incomplete feed source")
	}
}
