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
	"reflect"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
)

var _ errors.Error = &nodeError{}

// A nodeError is an error associated with processing an AST node.
type nodeError struct {
	path []string // optional
	n    ast.Node

	errors.Message
}

func (n *nodeError) Error() string {
	return errors.String(n)
}

func nodeErrorf(n ast.Node, format string, args ...interface{}) *nodeError {
	return &nodeError{
		n:       n,
		Message: errors.NewMessage(format, args),
	}
}

func (e *nodeError) Position() token.Pos {
	return e.n.Pos()
}

func (e *nodeError) InputPositions() []token.Pos { return nil }

func (e *nodeError) Path() []string {
	return e.path
}

func (v Value) appendErr(err errors.Error, b *bottom) errors.Error {
	switch {
	case len(b.sub) > 0:
		for _, b := range b.sub {
			err = v.appendErr(err, b)
		}
		fallthrough
	case b.err != nil:
		err = errors.Append(err, b.err)
	default:
		err = errors.Append(err, &valueError{
			v:   v,
			err: b,
		})
	}
	return err
}

func (v Value) toErr(b *bottom) (err errors.Error) {
	return v.appendErr(nil, b)
}

var _ errors.Error = &valueError{}

// A valueError is returned as a result of evaluating a value.
type valueError struct {
	v   Value
	err *bottom
}

func (e *valueError) Error() string {
	return errors.String(e)
}

func (e *valueError) Position() token.Pos {
	return e.err.Pos()
}

func (e *valueError) InputPositions() []token.Pos {
	return e.err.Positions(e.v.ctx())
}

func (e *valueError) Msg() (string, []interface{}) {
	return e.err.Msg()
}

func (e *valueError) Path() (a []string) {
	if e.v.path == nil {
		return nil
	}
	a, _ = e.v.path.appendPath(a, e.v.idx)
	return a
}

type errCode int

const (
	codeNone errCode = iota
	codeFatal
	codeNotExist
	codeTypeError
	codeIncomplete
	codeUser
	codeCycle
)

func isIncomplete(v value) bool {
	if err, ok := v.(*bottom); ok {
		return err.code == codeIncomplete || err.code == codeCycle
	}
	return false
}

func isLiteralBottom(v value) bool {
	if err, ok := v.(*bottom); ok {
		return err.code == codeUser
	}
	return false
}

var errNotExists = &bottom{code: codeNotExist, format: "undefined value"}

func exists(v value) bool {
	if err, ok := v.(*bottom); ok {
		return err.code != codeNotExist
	}
	return true
}

// bottom is the bottom of the value lattice. It is subsumed by all values.
type bottom struct {
	baseValue

	index     *index
	code      errCode
	exprDepth int
	pos       source
	format    string
	args      []interface{}

	err     errors.Error // pass-through from higher-level API
	sub     []*bottom    // sub errors
	value   value
	wrapped *bottom
}

func (x *bottom) kind() kind { return bottomKind }

func (x *bottom) Positions(ctx *context) []token.Pos {
	var a []token.Pos
	if x.index != nil { // TODO: remove check?
		a = appendPositions(ctx, nil, x.pos)
	}
	if w := x.wrapped; w != nil {
		a = append(a, w.Positions(ctx)...)
	}
	for _, sub := range x.sub {
		a = append(a, sub.Positions(ctx)...)
	}
	return a
}

func appendPositions(ctx *context, pos []token.Pos, src source) []token.Pos {
	if len(pos) > 15 {
		return pos
	}
	if src != nil {
		if p := src.Pos(); p != token.NoPos {
			pos = append(pos, src.Pos())
		}
		if c := src.computed(); c != nil {
			pos = appendPositions(ctx, pos, c.x)
			pos = appendPositions(ctx, pos, c.y)
		}
		switch x := src.(type) {
		case evaluated:
		case value:
			pos = appendPositions(ctx, pos, x.evalPartial(ctx))
		}
	}
	return pos
}

func (x *bottom) Msg() (format string, args []interface{}) {
	ctx := x.index.newContext()
	// We need to copy to avoid races.
	args = make([]interface{}, len(x.args))
	copy(args, x.args)
	preEvalArgs(ctx, args)
	return x.format, x.args
}

func (x *bottom) msg() string {
	return fmt.Sprint(x)
}

func (x *bottom) Format(s fmt.State, verb rune) {
	msg, args := x.Msg()
	fmt.Fprintf(s, msg, args...)
}

func cycleError(v evaluated) *bottom {
	if err, ok := v.(*bottom); ok && err.code == codeCycle {
		return err
	}
	return nil
}

func (c *context) mkIncompatible(src source, op op, a, b evaluated) evaluated {
	if err := firstBottom(a, b); err != nil {
		return err
	}
	e := mkBin(c, src.Pos(), op, a, b)
	return c.mkErr(e, "invalid operation %s %s %s (mismatched types %s and %s)",
		c.str(a), op, c.str(b), a.kind(), b.kind())
}

func (idx *index) mkErr(src source, args ...interface{}) *bottom {
	e := &bottom{baseValue: src.base(), index: idx, pos: src}

	if v, ok := src.(value); ok {
		e.value = v
	}
outer:
	for i, a := range args {
		switch x := a.(type) {
		case errCode:
			e.code = x
		case *bottom:
			e.wrapped = x
		case []*bottom:
			e.sub = x
		case errors.Error:
			e.err = x
		case value:
		case string:
			e.format = x
			e.args = args[i+1:]
			// Do not expand message so that errors can be localized.
			for i, a := range e.args {
				e.args[i] = fixArg(idx, a)
			}
			break outer
		}
	}
	if e.code == codeNone && e.wrapped != nil {
		e.code = e.wrapped.code
	}
	return e
}

func fixArg(idx *index, x interface{}) interface{} {
	switch x.(type) {
	case uint, int, string:
		return x
	case value:
		return x
	}
	t := reflect.TypeOf(x)
	// Store all non-ptr types as is, as they cannot change.
	if k := t.Kind(); k == reflect.String || k <= reflect.Complex128 {
		return x
	}
	return fmt.Sprint(x)
}

// preEvalArgs is used to expand value arguments just before printing.
func preEvalArgs(ctx *context, args []interface{}) {
	for i, a := range args {
		switch v := a.(type) {
		case *bottom:
			args[i] = v.msg()
		case value:
			// TODO: convert to Go values so that localization frameworks
			// can format values accordingly.
			args[i] = ctx.str(v)
		}
	}
}

func isBottom(n value) bool {
	return n.kind() == bottomKind
}

func firstBottom(v ...value) *bottom {
	for _, b := range v {
		if isBottom(b) {
			return b.(*bottom)
		}
	}
	return nil
}
