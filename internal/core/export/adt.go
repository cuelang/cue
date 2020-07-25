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

package export

import (
	"fmt"
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal/core/adt"
)

func (e *exporter) ident(x adt.Feature) *ast.Ident {
	s := e.ctx.IndexToString(int64(x.Index()))
	if !ast.IsValidIdent(s) {
		panic(s + " is not a valid identifier")
	}
	return ast.NewIdent(s)
}

func (e *exporter) adt(expr adt.Expr, conjuncts []adt.Conjunct) ast.Expr {
	switch x := expr.(type) {
	case adt.Value:
		return e.expr(x)

	case *adt.ListLit:
		a := []ast.Expr{}
		for _, x := range x.Elems {
			a = append(a, e.elem(x))
		}
		return ast.NewList(a...)

	case *adt.StructLit:
		// TODO: should we use pushFrame here?
		// _, saved := e.pushFrame([]adt.Conjunct{adt.MakeConjunct(nil, x)})
		// defer e.popFrame(saved)
		// s := e.frame(0).scope

		s := &ast.StructLit{}

		for _, d := range x.Decls {
			s.Elts = append(s.Elts, e.decl(d))
		}
		return s

	case *adt.FieldReference:
		f := e.frame(x.UpCount)
		ident := e.ident(x.Label)
		entry := f.fields[x.Label]
		entry.references = append(entry.references, ident)
		return ident

	case *adt.LabelReference:
		// get potential label from Source. Otherwise use X.
		f := e.frame(x.UpCount)
		var ident *ast.Ident
		if f.field == nil {
			// This can happen when the LabelReference is evaluated outside of
			// normal evaluation, that is, if a pattern constraint or
			// additional constraint is evaluated by itself.
			return ast.NewIdent("string")
		}
		list, ok := f.field.Label.(*ast.ListLit)
		if !ok || len(list.Elts) != 1 {
			panic("label reference to non-pattern constraint field or invalid list")
		}
		if a, ok := list.Elts[0].(*ast.Alias); ok {
			ident = ast.NewIdent(a.Ident.Name)
		} else {
			ident = ast.NewIdent("X" + strconv.Itoa(e.unique))
			e.unique++
			list.Elts[0] = &ast.Alias{
				Ident: ast.NewIdent(ident.Name),
				Expr:  list.Elts[0],
			}
		}
		ident.Scope = f.field
		ident.Node = f.labelExpr
		return ident

	case *adt.DynamicReference:
		// get potential label from Source. Otherwise use X.
		ident := ast.NewIdent("X")
		f := e.frame(x.UpCount)
		ident.Scope = f.field
		ident.Node = f.field
		return ident

	case *adt.ImportReference:
		importPath := x.ImportPath.StringValue(e.index)
		spec := ast.NewImport(nil, importPath)

		info, _ := astutil.ParseImportSpec(spec)
		name := info.PkgName
		if x.Label != 0 {
			name = x.Label.StringValue(e.index)
			if name != info.PkgName {
				spec.Name = ast.NewIdent(name)
			}
		}
		ident := ast.NewIdent(name)
		ident.Node = spec
		return ident

	case *adt.LetReference:
		// TODO:
		// - rename if necessary
		// - look in to reusing the mechanism of the old evaluator
		//
		// Either way, we need a better mechanism. References may go out of
		// scope. In case of aliases this means they may need to be reproduced
		// locally. Most of these issues can be avoided by either fully
		// expanding a configuration (export) or not at all (def).
		//
		i := len(e.stack) - 1 - int(x.UpCount) - 1
		if i < 0 {
			i = 0
		}
		f := &(e.stack[i])
		let := f.let[x.X]
		if let == nil {
			if f.let == nil {
				f.let = map[adt.Expr]*ast.LetClause{}
			}
			let = &ast.LetClause{
				Ident: e.ident(x.Label),
				Expr:  e.expr(x.X),
			}
			f.let[x.X] = let
			f.scope.Elts = append(f.scope.Elts, let)
		}
		ident := e.ident(x.Label)
		ident.Node = let
		ident.Scope = f.scope
		return ident

	case *adt.SelectorExpr:
		return &ast.SelectorExpr{
			X:   e.expr(x.X),
			Sel: e.ident(x.Sel),
		}

	case *adt.IndexExpr:
		return &ast.IndexExpr{
			X:     e.expr(x.X),
			Index: e.expr(x.Index),
		}

	case *adt.SliceExpr:
		var lo, hi ast.Expr
		if x.Lo != nil {
			lo = e.expr(x.Lo)
		}
		if x.Hi != nil {
			hi = e.expr(x.Hi)
		}
		// TODO: Stride not yet? implemented.
		// if x.Stride != nil {
		// 	stride = e.expr(x.Stride)
		// }
		return &ast.SliceExpr{X: e.expr(x.X), Low: lo, High: hi}

	case *adt.Interpolation:
		t := &ast.Interpolation{}
		multiline := false
		// TODO: mark formatting in interpolation itself.
		for i := 0; i < len(x.Parts); i += 2 {
			str := x.Parts[i].(*adt.String).Str
			if strings.IndexByte(str, '\n') >= 0 {
				multiline = true
				break
			}
		}
		quote := `"`
		if multiline {
			quote = `"""`
		}
		prefix := quote
		suffix := `\(`
		for i, elem := range x.Parts {
			if i%2 == 1 {
				t.Elts = append(t.Elts, e.expr(elem))
			} else {
				buf := []byte(prefix)
				if i == len(x.Parts)-1 {
					suffix = quote
				}
				str := elem.(*adt.String).Str
				if multiline {
					buf = appendEscapeMulti(buf, str, '"')
				} else {
					buf = appendEscaped(buf, str, '"', true)
				}
				buf = append(buf, suffix...)
				t.Elts = append(t.Elts, &ast.BasicLit{
					Kind:  token.STRING,
					Value: string(buf),
				})
			}
			prefix = ")"
		}
		return t

	case *adt.BoundExpr:
		return &ast.UnaryExpr{
			Op: x.Op.Token(),
			X:  e.expr(x.Expr),
		}

	case *adt.UnaryExpr:
		return &ast.UnaryExpr{
			Op: x.Op.Token(),
			X:  e.expr(x.X),
		}

	case *adt.BinaryExpr:
		return &ast.BinaryExpr{
			Op: x.Op.Token(),
			X:  e.expr(x.X),
			Y:  e.expr(x.Y),
		}

	case *adt.CallExpr:
		a := []ast.Expr{}
		for _, arg := range x.Args {
			v := e.expr(arg)
			if v == nil {
				e.expr(arg)
				panic("")
			}
			a = append(a, v)
		}
		fun := e.expr(x.Fun)
		return &ast.CallExpr{Fun: fun, Args: a}

	case *adt.DisjunctionExpr:
		a := []ast.Expr{}
		for _, d := range x.Values {
			v := e.expr(d.Val)
			if d.Default {
				v = &ast.UnaryExpr{Op: token.MUL, X: v}
			}
			a = append(a, v)
		}
		return ast.NewBinExpr(token.OR, a...)

	default:
		panic(fmt.Sprintf("unknown field %T", x))
	}
}

func (e *exporter) decl(d adt.Decl) ast.Decl {
	switch x := d.(type) {
	case adt.Elem:
		return e.elem(x)

	case *adt.Field:
		e.setDocs(x)
		f := &ast.Field{
			Label: e.stringLabel(x.Label),
			Value: e.expr(x.Value),
		}
		e.addField(x.Label, f.Value)
		// extractDocs(nil)
		return f

	case *adt.OptionalField:
		e.setDocs(x)
		f := &ast.Field{
			Label:    e.stringLabel(x.Label),
			Optional: token.NoSpace.Pos(),
			Value:    e.expr(x.Value),
		}
		e.addField(x.Label, f.Value)
		// extractDocs(nil)
		return f

	case *adt.BulkOptionalField:
		e.setDocs(x)
		// set bulk in frame.
		frame := e.frame(0)

		expr := e.expr(x.Filter)
		frame.labelExpr = expr // see astutil.Resolve.

		if x.Label != 0 {
			expr = &ast.Alias{Ident: e.ident(x.Label), Expr: expr}
		}
		f := &ast.Field{
			Label: ast.NewList(expr),
		}

		frame.field = f

		f.Value = e.expr(x.Value)

		return f

	case *adt.DynamicField:
		e.setDocs(x)
		key := e.expr(x.Key)
		if _, ok := key.(*ast.Interpolation); !ok {
			key = &ast.ParenExpr{X: key}
		}
		f := &ast.Field{
			Label: key.(ast.Label),
		}

		frame := e.frame(0)
		frame.field = f
		frame.labelExpr = key
		// extractDocs(nil)

		f.Value = e.expr(x.Value)

		return f

	default:
		panic(fmt.Sprintf("unknown field %T", x))
	}
}

func (e *exporter) elem(d adt.Elem) ast.Expr {

	switch x := d.(type) {
	case adt.Expr:
		return e.expr(x)

	case *adt.Ellipsis:
		t := &ast.Ellipsis{}
		if x.Value != nil {
			t.Type = e.expr(x.Value)
		}
		return t

	case adt.Yielder:
		return e.comprehension(x)

	default:
		panic(fmt.Sprintf("unknown field %T", x))
	}
}

func (e *exporter) comprehension(y adt.Yielder) ast.Expr {
	c := &ast.Comprehension{}

	for {
		switch x := y.(type) {
		case *adt.ForClause:
			value := e.ident(x.Value)
			clause := &ast.ForClause{
				Value:  value,
				Source: e.expr(x.Src),
			}
			c.Clauses = append(c.Clauses, clause)

			_, saved := e.pushFrame(nil)
			defer e.popFrame(saved)

			if x.Key != 0 {
				key := e.ident(x.Key)
				clause.Key = key
				e.addField(x.Key, clause)
			}
			e.addField(x.Value, clause)

			y = x.Dst

		case *adt.IfClause:
			clause := &ast.IfClause{Condition: e.expr(x.Condition)}
			c.Clauses = append(c.Clauses, clause)
			y = x.Dst

		case *adt.LetClause:
			clause := &ast.LetClause{Expr: e.expr(x.Expr)}
			c.Clauses = append(c.Clauses, clause)

			_, saved := e.pushFrame(nil)
			defer e.popFrame(saved)

			e.addField(x.Label, clause)

			y = x.Dst

		case *adt.ValueClause:
			c.Value = e.expr(x.StructLit)
			return c

		default:
			panic(fmt.Sprintf("unknown field %T", x))
		}
	}

}
