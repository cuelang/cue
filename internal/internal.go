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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/apd/v2"
	"golang.org/x/xerrors"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
)

// A Decimal is an arbitrary-precision binary-coded decimal number.
//
// Right now Decimal is aliased to apd.Decimal. This may change in the future.
type Decimal = apd.Decimal

// DebugStr prints a syntax node.
var DebugStr func(x interface{}) string

// ErrIncomplete can be used by builtins to signal the evaluation was
// incomplete.
var ErrIncomplete = errors.New("incomplete value")

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

// UnifyBuiltin returns the given Value unified with the given builtin template.
var UnifyBuiltin func(v interface{}, kind string) interface{}

// GetRuntime reports the runtime for an Instance or Value.
var GetRuntime func(instance interface{}) interface{}

// MakeInstance makes a new instance from a value.
var MakeInstance func(value interface{}) (instance interface{})

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

// NewComment creates a new CommentGroup from the given text.
// Each line is prefixed with "//" and the last newline is removed.
// Useful for ASTs generated by code other than the CUE parser.
func NewComment(isDoc bool, s string) *ast.CommentGroup {
	if s == "" {
		return nil
	}
	cg := &ast.CommentGroup{Doc: isDoc}
	if !isDoc {
		cg.Line = true
		cg.Position = 10
	}
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		cg.List = append(cg.List, &ast.Comment{Text: "// " + scanner.Text()})
	}
	if last := len(cg.List) - 1; cg.List[last].Text == "// " {
		cg.List = cg.List[:last]
	}
	return cg
}

func NewAttr(name, str string) *ast.Attribute {
	buf := &strings.Builder{}
	buf.WriteByte('@')
	buf.WriteString(name)
	buf.WriteByte('(')
	fmt.Fprintf(buf, str)
	buf.WriteByte(')')

	return &ast.Attribute{Text: buf.String()}
}

// ToExpr converts a node to an expression. If it is a file, it will return
// it as a struct. If is an expression, it will return it as is. Otherwise
// it panics.
func ToExpr(n ast.Node) ast.Expr {
	switch x := n.(type) {
	case nil:
		return nil

	case ast.Expr:
		return x

	case *ast.File:
		start := 0
	outer:
		for i, d := range x.Decls {
			switch d.(type) {
			case *ast.Package, *ast.ImportDecl:
				start = i + 1
			case *ast.CommentGroup, *ast.Attribute:
			default:
				break outer
			}
		}
		return &ast.StructLit{Elts: x.Decls[start:]}

	default:
		panic(fmt.Sprintf("Unsupported node type %T", x))
	}
}

// ToFile converts an expression to a file.
//
// Adjusts the spacing of x when needed.
func ToFile(n ast.Node) *ast.File {
	switch x := n.(type) {
	case nil:
		return nil
	case *ast.StructLit:
		return &ast.File{Decls: x.Elts}
	case ast.Expr:
		ast.SetRelPos(x, token.NoSpace)
		return &ast.File{Decls: []ast.Decl{&ast.EmbedDecl{Expr: x}}}
	case *ast.File:
		return x
	default:
		panic(fmt.Sprintf("Unsupported node type %T", x))
	}
}

// IsEllipsis reports whether the declaration can be represented as an ellipsis.
func IsEllipsis(x ast.Decl) bool {
	// ...
	if _, ok := x.(*ast.Ellipsis); ok {
		return true
	}

	// [string]: _ or [_]: _
	f, ok := x.(*ast.Field)
	if !ok {
		return false
	}
	v, ok := f.Value.(*ast.Ident)
	if !ok || v.Name != "_" {
		return false
	}
	l, ok := f.Label.(*ast.ListLit)
	if !ok || len(l.Elts) != 1 {
		return false
	}
	i, ok := l.Elts[0].(*ast.Ident)
	if !ok {
		return false
	}
	return i.Name == "string" || i.Name == "_"
}

// GenPath reports the directory in which to store generated files.
func GenPath(root string) string {
	info, err := os.Stat(filepath.Join(root, "cue.mod"))
	if os.IsNotExist(err) || !info.IsDir() {
		// Try legacy pkgDir mode
		pkgDir := filepath.Join(root, "pkg")
		if err == nil && !info.IsDir() {
			return pkgDir
		}
		if info, err := os.Stat(pkgDir); err == nil && info.IsDir() {
			return pkgDir
		}
	}
	return filepath.Join(root, "cue.mod", "gen")
}

var ErrInexact = errors.New("inexact subsumption")

func DecorateError(info error, err errors.Error) errors.Error {
	return &decorated{cueError: err, info: info}
}

type cueError = errors.Error

type decorated struct {
	cueError

	info error
}

func (e *decorated) Is(err error) bool {
	return xerrors.Is(e.info, err) || xerrors.Is(e.cueError, err)
}

// MaxDepth indicates the maximum evaluation depth. This is there to break
// cycles in the absence of cycle detection.
//
// It is registered in a central place to make it easy to find all spots where
// cycles are broken in this brute-force manner.
//
// TODO(eval): have cycle detection.
const MaxDepth = 20
