// Copyright 2020 CUE Authors
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

package adt

import (
	"fmt"
	"regexp"

	"github.com/cockroachdb/apd/v2"
	"golang.org/x/text/runes"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/token"
)

// A Unifier implements a strategy for CUE's unification operation. It must
// handle the following aspects of CUE evaluation:
//
//    - Structural and reference cycles
//    - Non-monotic validation
//    - Fixed-point computation of comprehension
//
type Unifier interface {
	// Unify fully unifies all values of a Vertex to completion and stores
	// the result in the Vertex. If Unify was called on v before it returns
	// the cached results.
	Unify(c *OpContext, v *Vertex) // error or bool?

	// Evaluate returns the evaluated value associated with v. It may return a
	// partial result. That is, if v was not yet unified, it may return a
	// concrete value that must be the result assuming the configuration has no
	// errors.
	//
	// This semantics allows CUE to break reference cycles in a straightforward
	// manner.
	//
	// Vertex v must still be evaluated at some point to catch the underlying
	// error.
	//
	Evaluate(c *OpContext, v *Vertex) Value
}

// Runtime defines an interface for low-level representation conversion and
// lookup.
type Runtime interface {
	// StringIndexer allows for converting string labels to and from a
	// canonical numeric representation.
	StringIndexer
}

// An OpContext associates a Runtime and Unifier to allow evaluating the types
// defined in this package. It tracks errors provides convenience methods for
// evaluating values.
type OpContext struct {
	Runtime
	unifier Unifier

	// Format computes a string representation of a Node.
	format func(Node) string

	e            *Environment
	src          ast.Node
	errs         *Bottom
	isIncomplete bool

	tentative int // set during comprehension evaluation
}

// If IsTentative is set, evaluation of an arc should not finalize
// to non-concrete values.
func (c *OpContext) IsTentative() bool {
	return c.tentative > 0
}

func (c *OpContext) Pos() token.Pos {
	if c.src == nil {
		return token.NoPos
	}
	return c.src.Pos()
}

func (c *OpContext) Source() ast.Node {
	return c.src
}

// NewContext creates an operation context.
func NewContext(r Runtime, u Unifier, v *Vertex) *OpContext {
	env := &Environment{Up: nil, Vertex: v}
	return &OpContext{
		Runtime: r,
		unifier: u,
		e:       env,
	}
}

func (c *OpContext) pos() token.Pos {
	if c.src == nil {
		return token.NoPos
	}
	return c.src.Pos()
}

func (c *OpContext) spawn(node *Vertex) *OpContext {
	sub := *c
	node.Parent = c.e.Vertex
	sub.e = &Environment{Up: c.e, Vertex: node}
	if c.e != nil {
		sub.e.CloseID = c.e.CloseID
	}
	return &sub
}

func (c *OpContext) Env(upCount int32) *Environment {
	e := c.e
	for ; upCount > 0; upCount-- {
		e = e.Up
	}
	return e
}

func (c *OpContext) relNode(upCount int32) *Vertex {
	e := c.e
	for ; upCount > 0; upCount-- {
		e = e.Up
	}
	return e.Vertex
}

func (c *OpContext) relLabel(upCount int32) Feature {
	// locate current label.
	e := c.e
	for upCount--; upCount > 0; upCount-- {
		e = e.Up
	}
	return e.Vertex.Label
}

func (c *OpContext) concreteIsPossible(x Expr) bool {
	if v, ok := x.(Value); ok {
		if v.Concreteness() > Concrete {
			c.errf("value can never become concrete")
			return false
		}
	}
	return true
}

// HasErr reports whether any error was reported, including whether value
// was incomplete.
func (c *OpContext) HasErr() bool {
	return c.errs != nil //|| c.isIncomplete
}

func (c *OpContext) Err() *Bottom {
	b := c.errs
	c.errs = nil
	return b
}

func (c *OpContext) addErrf(code ErrorCode, pos token.Pos, msg string, args ...interface{}) {
	for i, a := range args {
		switch x := a.(type) {
		case Node:
			args[i] = c.Str(x)
		case ast.Node:
			b, _ := format.Node(x)
			args[i] = string(b)
		case Feature:
			args[i] = x.ToString(c.Runtime)
		}
	}

	err := errors.Newf(pos, msg, args...)
	c.addErr(code, err)
}

func (c *OpContext) addErr(code ErrorCode, err errors.Error) {
	c.errs = CombineErrors(c.src, c.errs, &Bottom{Code: code, Err: err})
}

func (c *OpContext) AddBottom(b *Bottom) {
	c.errs = CombineErrors(c.src, c.errs, b)
}

func (c *OpContext) AddErr(err errors.Error) {
	if err != nil {
		c.errs = CombineErrors(c.src, c.errs, &Bottom{Err: err})
	}
}

// NewErrf creates a *Bottom value and returns it. The returned uses the
// current source as the point of origin of the error.
func (c *OpContext) NewErrf(format string, args ...interface{}) *Bottom {
	err := errors.Newf(c.pos(), format, args...)
	return &Bottom{Src: c.src, Err: err, Code: EvalError}
}

// errf records an error in OpContext. It requires less arguments than addErrf.
func (c *OpContext) errf(format string, args ...interface{}) {
	c.AddErr(errors.Newf(c.pos(), format, args...))
}

func (c *OpContext) validate(v Value) *Bottom {
	switch x := v.(type) {
	case *Bottom:
		return x
	case *Vertex:
		v := c.unifier.Evaluate(c, x)
		if b, ok := v.(*Bottom); ok {
			return b
		}
	}
	return nil
}

// Resolve finds a node in the tree.
//
// Should only be used to insert Conjuncts. TODO: perhaps only return Conjuncts
// and error.
func (c *OpContext) Resolve(env *Environment, r Resolver) (*Vertex, *Bottom) {
	savedEnv := c.e
	savedErr := c.errs
	savedSrc := c.src
	c.errs = nil
	c.src = r.Source()
	c.e = env

	arc := r.resolve(c)
	err := c.Err()
	// TODO: check for cycle errors?

	c.errs = savedErr
	c.src = savedSrc
	c.e = savedEnv
	if err != nil {
		return nil, err
	}

	return arc, err
}

// Validate calls validates value for the given validator.
func (c *OpContext) Validate(check Validator, value Value) *Bottom {
	return check.validate(c, value)
}

// Yield evaluates a Yielder and calls f for each result.
func (c *OpContext) Yield(env *Environment, y Yielder, f YieldFunc) *Bottom {
	savedEnv := c.e
	savedErr := c.errs
	savedSrc := c.src
	c.errs = nil
	c.src = y.Source()
	c.e = env

	c.tentative++

	y.yield(c, f)

	c.tentative--

	err := c.Err()
	if c.isIncomplete {
		// TODO: remove incomplete flag
		err = CombineErrors(nil, err, &Bottom{Code: IncompleteError})
	}
	c.isIncomplete = false

	c.errs = savedErr
	c.src = savedSrc
	c.e = savedEnv

	// return complete
	return err
}

// Evaluate evaluates an expression within the given environment and indicates
// whether the result is complete. It will always return a non-nil result.
func (c *OpContext) Evaluate(env *Environment, x Expr) (result Value, complete bool) {
	savedEnv := c.e
	savedSrc := c.src
	savedErr := c.errs
	c.e = env
	c.errs = nil
	c.src = x.Source()
	c.isIncomplete = false

	val := c.eval(x)

	complete = true

	if err, _ := val.(*Bottom); err != nil && err.IsIncomplete() {
		complete = false
	}
	if val == nil {
		complete = false
		// TODO ENSURE THIS DOESN"T HAPPEN>
		val = &Bottom{
			Code: IncompleteError,
			Err:  errors.Newf(token.NoPos, "UNANTICIPATED ERROR"),
		}

	}

	c.e = savedEnv
	c.src = savedSrc
	c.errs = savedErr

	if !complete || val == nil {
		return val, false
	}

	return val, true
}

// eval evaluates expression v within the current environment. The result may
// be nil if the result is incomplete. eval leaves errors untouched to that
// they can be collected by the caller.
func (c *OpContext) eval(v Expr) (result Value) {
	savedSrc := c.src
	c.src = v.Source()

	defer func() {
		if c.errs = CombineErrors(c.src, result, c.errs); c.errs != nil {
			result = c.errs
		}
		if result == nil {
			c.isIncomplete = true // Used by Yield
		}
		c.src = savedSrc
	}()

	switch x := v.(type) {
	case Value:
		return x

	case Evaluator:
		v := x.evaluate(c)
		return v

	case Resolver:
		arc := x.resolve(c)
		if c.HasErr() || isIncomplete(arc) {
			return nil
		}

		v := c.unifier.Evaluate(c, arc)
		return v

	default:
		// return nil
		c.errf("unexpected Expr type %T", v)
	}
	return nil
}

func (c *OpContext) lookup(x *Vertex, pos token.Pos, l Feature) *Vertex {
	if l == InvalidLabel || x == nil {
		// TODO??
		return &Vertex{}
	}

	var kind Kind
	if x.Value != nil {
		kind = x.Value.Kind()
	}

	switch kind {
	case StructKind:
		if l.Typ() == IntLabel {
			c.addErrf(0, pos, "invalid struct selector %s (type int)", l)
		}

	case ListKind:
		switch {
		case l.Typ() == IntLabel:
			switch {
			case l.Index() < 0:
				c.addErrf(0, pos, "invalid list index %s (index must be non-negative)", l)
				return nil
			case l.Index() > len(x.Arcs):
				c.addErrf(0, pos, "invalid list index %s (out of bounds)", l)
				return nil
			}

		case l.IsDef():

		default:
			c.addErrf(0, pos, "invalid list index %s (type string)", l)
			return nil
		}

	default:
		// if !l.IsDef() {
		// 	c.addErrf(0, nil, "invalid selector %s (must be definition for non-structs)", l)
		// }
	}

	a := x.lookup(l)
	if a == nil {
		code := IncompleteError
		if x.Closed != nil && !x.Closed.Accept(c, l) {
			code = 0
		}
		c.addErrf(code, pos, "undefined field %s", l.ToString(c.Runtime))
	}
	// c.unifier.Finalize(c, a)
	return a
}

func (c *OpContext) Label(x Value) Feature {
	return labelFromValue(c, x)
}

func (c *OpContext) typeError(v Value, k Kind) {
	if isError(v) {
		return
	}
	c.errf("type error")
}

var emptyNode = &Vertex{}

func pos(x Node) token.Pos {
	if x.Source() == nil {
		return token.NoPos
	}
	return x.Source().Pos()
}

func (c *OpContext) node(x Expr) *Vertex {
	v := c.eval(x)
	if isError(v) {
		// TODO: Incomplete error
		return emptyNode
	}
	node, ok := v.(*Vertex)
	if !ok {
		c.addErrf(0, pos(x), "invalid operand %s (expect list or struct, found %s)", x.Source(), v.Kind())
		return emptyNode
	}
	return node
}

func (c *OpContext) list(v Value) *Vertex {
	x, ok := v.(*Vertex)
	if !ok || !x.IsList() {
		c.typeError(v, ListKind)
		return emptyNode
	}
	return x
}

func (c *OpContext) scalar(v Value) Value {
	switch v.(type) {
	case *Null, *Bool, *Num, *String, *Bytes:
	default:
		c.typeError(v, ScalarKinds)
	}
	return v
}

var zero = &Num{K: NumKind}

func (c *OpContext) num(v Value) *Num {
	if isError(v) {
		return zero
	}
	x, ok := v.(*Num)
	if !ok {
		c.typeError(v, NumKind)
		return zero
	}
	return x
}

func (c *OpContext) Int64(v Value) int64 {
	if isError(v) {
		return 0
	}
	x, ok := v.(*Num)
	if !ok {
		c.typeError(v, IntKind)
		return 0
	}
	i, err := x.X.Int64()
	if err != nil {
		c.errf("number is not an int64: %v", err)
		return 0
	}
	return i
}

func (c *OpContext) uint64(v Value) uint64 {
	if isError(v) {
		return 0
	}
	x, ok := v.(*Num)
	if !ok {
		c.typeError(v, IntKind)
		return 0
	}
	if x.X.Negative {
		// TODO: improve message
		c.errf("cannot convert negative number to uint64")
		return 0
	}
	if !x.X.Coeff.IsUint64() {
		// TODO: improve message
		c.errf("cannot convert number %s to uint64", x.X)
		return 0
	}
	return x.X.Coeff.Uint64()
}

func (c *OpContext) BoolValue(v Value) bool {
	if isError(v) {
		return false
	}
	x, ok := v.(*Bool)
	if !ok {
		c.typeError(v, BoolKind)
		return false
	}
	return x.B
}

func (c *OpContext) StringValue(v Value) string {
	if isError(v) {
		return ""
	}
	switch x := v.(type) {
	case *String:
		return x.Str

	case *Bytes:
		return string(runes.ReplaceIllFormed().Bytes(x.B))

	case *Num:
		return x.X.String()

	default:
		if x.Concreteness() > Concrete {
			c.addErrf(IncompleteError, pos(v), "cannot convert incomplete value to string")
		} else {
			c.typeError(v, StringKind)
		}
	}
	return ""
}

func (c *OpContext) bytesValue(v Value) []byte {
	if isError(v) {
		return nil
	}
	x, ok := v.(*Bytes)
	if !ok {
		c.typeError(v, BytesKind)
		return nil
	}
	return x.B
}

var matchNone = regexp.MustCompile("^$")

func (c *OpContext) regexp(v Value) *regexp.Regexp {
	if isError(v) {
		return matchNone
	}
	switch x := v.(type) {
	case *String:
		if x.RE != nil {
			return x.RE
		}
		// TODO: synchronization
		p, err := regexp.Compile(x.Str)
		if err != nil {
			// FatalError? How to cache error
			c.errf("invalid regexp: %s", err)
			x.RE = matchNone
		} else {
			x.RE = p
		}
		return x.RE

	case *Bytes:
		if x.RE != nil {
			return x.RE
		}
		// TODO: synchronization
		p, err := regexp.Compile(string(x.B))
		if err != nil {
			c.errf("invalid regexp: %s", err)
			x.RE = matchNone
		} else {
			x.RE = p
		}
		return x.RE

	default:
		c.typeError(v, StringKind|BytesKind)
		return matchNone
	}
}

func (c *OpContext) newNum(d *apd.Decimal, k Kind, sources ...Node) Value {
	if c.HasErr() {
		return c.Err()
	}
	return &Num{Src: c.src, X: *d, K: k}
}

func (c *OpContext) newInt64(n int64, sources ...Node) Value {
	if c.HasErr() {
		return c.Err()
	}
	d := apd.New(n, 0)
	return &Num{Src: c.src, X: *d, K: IntKind}
}

func (c *OpContext) newString(s string) Value {
	if c.HasErr() {
		return c.Err()
	}
	return &String{Src: c.src, Str: s}
}

func (c *OpContext) newBytes(b []byte) Value {
	if c.HasErr() {
		return c.Err()
	}
	return &Bytes{Src: c.src, B: b}
}

func (c *OpContext) newBool(b bool) Value {
	if c.HasErr() {
		return c.Err()
	}
	return &Bool{Src: c.src, B: b}
}

func (c *OpContext) newList(src ast.Node, parent *Vertex, accepted Acceptor) *Vertex {
	return &Vertex{Parent: parent, Value: &ListMarker{}, Closed: accepted}
}

// Str reports a debug string of x.
func (c *OpContext) Str(x Node) string {
	if c.format == nil {
		return fmt.Sprintf("%T", x)
	}
	return c.format(x)
}
