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

// Package internal exposes some cue internals to other packages.
//
// A better name for this package would be technicaldebt.
package internal // import "cuelang.org/go/internal"

// TODO: refactor packages as to make this package unnecessary.

import (
	"strconv"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/token"
	"github.com/cockroachdb/apd/v2"
)

// A Decimal is an arbitrary-precision binary-coded decimal number.
//
// Right now Decimal is aliased to apd.Decimal. This may change in the future.
type Decimal = apd.Decimal

// DebugStr prints a syntax node.
var DebugStr func(x interface{}) string

// EvalExpr evaluates an expression within an existing struct value.
// Identifiers only resolve to values defined within the struct.
//
// Expressions may refer to builtin packages if they can be uniquely identified
//
// Both value and result are of type cue.Value, but are an interface to prevent
// cyclic dependencies.
//
// TODO: extract interface
var EvalExpr func(value, expr interface{}) (result interface{})

// FromGoValue converts an arbitrary Go value to the corresponding CUE value.
// instance must be of type *cue.Instance.
// The returned value is a cue.Value, which the caller must cast to.
var FromGoValue func(instance, x interface{}, allowDefault bool) interface{}

// FromGoType converts an arbitrary Go type to the corresponding CUE value.
// instance must be of type *cue.Instance.
// The returned value is a cue.Value, which the caller must cast to.
var FromGoType func(instance, x interface{}) interface{}

// DropOptional is a blanket override of handling optional values during
// compilation. TODO: should we make this a build option?
var DropOptional bool

// UnifyBuiltin returns the given Value unified with the given builtin template.
var UnifyBuiltin func(v interface{}, kind string) interface{}

// GetRuntime reports the runtime for an Instance.
var GetRuntime func(instance interface{}) interface{}

// CheckAndForkRuntime checks that value is created using runtime, panicking
// if it does not, and returns a forked runtime that will discard additional
// keys.
var CheckAndForkRuntime func(runtime, value interface{}) interface{}

// BaseContext is used as CUEs default context for arbitrary-precision decimals
var BaseContext = apd.BaseContext.WithPrecision(24)

// ListEllipsis reports the list type and remaining elements of a list. If we
// ever relax the usage of ellipsis, this function will likely change. Using
// this function will ensure keeping correct behavior or causing a compiler
// failure.
func ListEllipsis(n *ast.ListLit) (elts []ast.Expr, e *ast.Ellipsis) {
	elts = n.Elts
	if n := len(elts); n > 0 {
		var ok bool
		if e, ok = elts[n-1].(*ast.Ellipsis); ok {
			elts = elts[:n-1]
		}
	}
	return elts, e
}

func PackageInfo(f *ast.File) (p *ast.Package, name string, tok token.Pos) {
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.CommentGroup:
		case *ast.Package:
			if x.Name == nil {
				break
			}
			return x, x.Name.Name, x.Name.Pos()
		}
	}
	return nil, "", f.Pos()
}

// LabelName reports the name of a label, if known, and whether it is valid.
func LabelName(l ast.Label) (name string, ok bool) {
	switch n := l.(type) {
	case *ast.Ident:
		str, err := ast.ParseIdent(n)
		if err != nil {
			return "", false
		}
		return str, true

	case *ast.BasicLit:
		switch n.Kind {
		case token.STRING:
			// Use strconv to only allow double-quoted, single-line strings.
			if str, err := strconv.Unquote(n.Value); err == nil {
				return str, true
			}

		case token.NULL, token.TRUE, token.FALSE:
			return n.Value, true

			// TODO: allow numbers to be fields?
		}

	case *ast.TemplateLabel:
		return n.Ident.Name, false

	}
	// This includes interpolation.
	return "", false
}
