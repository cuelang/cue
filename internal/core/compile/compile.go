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

package compile

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/runtime"
	"golang.org/x/xerrors"
)

// Config configures a compilation.
type Config struct {
}

// Files compiles the given files as a single instance. It disregards
// the package names and it is the responsibility of the user to verify that
// the packages names are consistent.
//
// Files may return a completed parse even if it has errors.
func Files(cfg *Config, r *runtime.Runtime, files ...*ast.File) (*adt.Vertex, errors.Error) {
	c := &compiler{index: r}

	v := c.compileFiles(files)

	if c.errs != nil {
		return v, c.errs
	}
	return v, nil
}

type compiler struct {
	index adt.StringIndexer

	stack      []frame
	inSelector int

	fileScope map[adt.Feature]bool

	num literal.NumInfo

	errs errors.Error
}

func (c *compiler) reset() {
	c.fileScope = nil
	c.stack = c.stack[:0]
	c.errs = nil
}

func (c *compiler) errf(n ast.Node, format string, args ...interface{}) *adt.Bottom {
	err := &compilerError{
		n:       n,
		path:    c.path(),
		Message: errors.NewMessage(format, args),
	}
	c.errs = errors.Append(c.errs, err)
	return &adt.Bottom{}
}

func (c *compiler) path() []string {
	a := []string{}
	for _, f := range c.stack {
		if f.label != nil {
			a = append(a, f.label.labelString())
		}
	}
	return a
}

type frame struct {
	label labeler  // path name leading to this frame.
	scope ast.Node // *ast.File or *ast.Struct
	field *ast.Field
	// scope   map[ast.Node]bool
	upCount int32 // 1 for field, 0 for embedding.

	aliases map[string]aliasEntry
}

type aliasEntry struct {
	expr   adt.Expr
	source ast.Node
	used   bool
}

func (c *compiler) insertAlias(id *ast.Ident, a aliasEntry) *adt.Bottom {
	k := len(c.stack) - 1
	m := c.stack[k].aliases
	if m == nil {
		m = map[string]aliasEntry{}
		c.stack[k].aliases = m
	}

	if id == nil || !ast.IsValidIdent(id.Name) {
		return c.errf(a.source, "invalid identifier name")
	}

	if e, ok := m[id.Name]; ok {
		return c.errf(a.source,
			"alias %q already declared; previous declaration at %s",
			id.Name, e.source.Pos())
	}

	m[id.Name] = a
	return nil
}

// lookupAlias looks up an alias with the given name at the k'th stack position.
func (c compiler) lookupAlias(k int, id *ast.Ident) aliasEntry {
	m := c.stack[k].aliases
	name := id.Name
	entry, ok := m[name]

	if !ok {
		err := c.errf(id, "could not find LetClause associated with identifier %q", name)
		return aliasEntry{expr: err}
	}

	entry.used = true
	m[name] = entry
	return entry
}

func (c *compiler) pushScope(n labeler, upCount int32, id ast.Node) *frame {
	c.stack = append(c.stack, frame{
		label:   n,
		scope:   id,
		upCount: upCount,
	})
	return &c.stack[len(c.stack)-1]
}

func (c *compiler) popScope() {
	k := len(c.stack) - 1
	f := c.stack[k]
	for k, v := range f.aliases {
		if !v.used {
			c.errf(v.source, "unreferenced alias or let clause %s", k)
		}
	}
	c.stack = c.stack[:k]
}

// entry points // USE CONFIG
func (c *compiler) compileFiles(a []*ast.File) *adt.Vertex { // Or value?
	c.fileScope = map[adt.Feature]bool{}

	// Populate file scope to handle unresolved references. Note that we do
	// not allow aliases to be resolved across file boundaries.
	for _, f := range a {
		for _, d := range f.Decls {
			if f, ok := d.(*ast.Field); ok {
				if id, ok := f.Label.(*ast.Ident); ok {
					c.fileScope[c.label(id)] = true
				}
			}
		}
	}

	// TODO: set doc.
	res := &adt.Vertex{}

	// env := &adt.Environment{Vertex: nil} // runtime: c.runtime

	for _, file := range a {
		c.pushScope(nil, 0, file) // File scope
		v := &adt.StructLit{Src: file}
		c.addDecls(v, file.Decls)
		res.Conjuncts = append(res.Conjuncts, adt.MakeConjunct(nil, v))
		c.popScope()
	}

	return res
}

// resolve assumes that all existing resolutions are legal. Validation should
// be done in a separate step if required.
//
// TODO: collect validation pass to verify that all resolutions are
// legal?
func (c *compiler) resolve(n *ast.Ident) adt.Expr {
	// X in import "path/X"
	// X in import X "path"
	if imp, ok := n.Node.(*ast.ImportSpec); ok {
		return &adt.ImportReference{
			Src:        n,
			ImportPath: c.label(imp.Path),
			Label:      c.label(n),
		}
	}

	label := c.label(n)

	// Unresolved field.
	if n.Node == nil {
		upCount := int32(0)
		for _, c := range c.stack {
			upCount += c.upCount
		}
		if c.fileScope[label] {
			return &adt.FieldReference{
				Src:     n,
				UpCount: upCount,
				Label:   label,
			}
		}

		if p := predeclared(n); p != nil {
			return p
		}

		return c.errf(n, "reference %q not found", n.Name)
	}

	//   X in [X=x]: y  Scope: Field  Node: Expr (x)
	//   X in X=[x]: y  Scope: Field  Node: Field
	if f, ok := n.Scope.(*ast.Field); ok {
		upCount := int32(0)

		k := len(c.stack) - 1
		for ; k >= 0; k-- {
			if c.stack[k].field == f {
				break
			}
			upCount += c.stack[k].upCount
		}

		label := &adt.LabelReference{
			Src:     n,
			UpCount: upCount,
		}

		if f, ok := n.Node.(*ast.Field); ok {
			_ = c.lookupAlias(k, f.Label.(*ast.Alias).Ident) // mark as used
			return &adt.DynamicReference{
				Src:     n,
				UpCount: upCount,
				Label:   label,
			}
		}
		return label
	}

	upCount := int32(0)

	k := len(c.stack) - 1
	for ; k >= 0; k-- {
		if c.stack[k].scope == n.Scope {
			break
		}
		upCount += c.stack[k].upCount
	}
	if k < 0 {
		// This is a programmatic error and should never happen if the users
		// just builds with the cue command or if astutil.Resolve is used
		// correctly.
		c.errf(n, "reference %q set to unknown node in AST; "+
			"this can result from incorrect API usage or a compiler bug",
			n.Name)
	}

	switch n.Node.(type) {
	// Local expressions
	case *ast.LetClause, *ast.Alias:
		entry := c.lookupAlias(k, n)

		return &adt.LetReference{
			Src:     n,
			UpCount: upCount,
			Label:   label,
			X:       entry.expr,
		}
	}

	if n.Scope == nil {
		// Package.
		// Should have been handled above.
		panic("unreachable") // Or direct ancestor node?
	}

	// X=x: y
	// X=(x): y
	// X="\(x)": y
	if f, ok := n.Node.(*ast.Field); ok {
		a, ok := f.Label.(*ast.Alias)
		if !ok {
			return c.errf(n, "illegal reference %s", n.Name)
		}
		aliasInfo := c.lookupAlias(k, a.Ident) // marks alias as used.
		lab, ok := a.Expr.(ast.Label)
		if !ok {
			return c.errf(a.Expr, "invalid label expression")
		}
		name, _, err := ast.LabelName(lab)
		switch {
		case xerrors.Is(err, ast.ErrIsExpression):
			if aliasInfo.expr == nil {
				panic("unreachable")
			}
			return &adt.DynamicReference{
				Src:     n,
				UpCount: upCount,
				Label:   aliasInfo.expr,
			}

		case err != nil:
			return c.errf(n, "invalid label: %v", err)

		case name != "":
			label = c.label(lab)

		default:
			return c.errf(n, "unsupported field alias %q", name)
		}
	}

	return &adt.FieldReference{
		Src:     n,
		UpCount: upCount,
		Label:   label,
	}
}

func (c *compiler) addDecls(st *adt.StructLit, a []ast.Decl) {
	for _, d := range a {
		if x := c.decl(d); x != nil {
			st.Decls = append(st.Decls, x)
		}
	}
}

func (c *compiler) decl(d ast.Decl) adt.Decl {
	switch x := d.(type) {
	case *ast.BadDecl:
		return c.errf(d, "")

	case *ast.Field:
		lab := x.Label
		if a, ok := lab.(*ast.Alias); ok {
			if lab, ok = a.Expr.(ast.Label); !ok {
				return c.errf(a, "alias expression is not a valid label")
			}

			e := aliasEntry{source: a}

			switch lab.(type) {
			case *ast.Ident, *ast.BasicLit:
				// Even though we won't need the alias, we still register it
				// for duplicate and failed reference detection.
			default:
				e.expr = c.expr(a.Expr)
			}

			if err := c.insertAlias(a.Ident, e); err != nil {
				return err
			}
		}

		value := c.labeledExpr(x, (*fieldLabel)(x), x.Value)

		switch l := lab.(type) {
		case *ast.Ident, *ast.BasicLit:
			label := c.label(lab)

			// TODO(legacy): remove: old-school definitions
			if x.Token == token.ISA && !label.IsDef() {
				name, isIdent, err := ast.LabelName(lab)
				if err == nil && isIdent {
					idx := c.index.StringToIndex(name)
					label, _ = adt.MakeLabel(x.Pos(), idx, adt.DefinitionLabel)
				}
			}

			if x.Optional == token.NoPos {
				return &adt.Field{
					Src:   x,
					Label: label,
					Value: value,
				}
			} else {
				return &adt.OptionalField{
					Src:   x,
					Label: label,
					Value: value,
				}
			}

		case *ast.ListLit:
			if len(l.Elts) != 1 {
				// error
				return c.errf(x, "list label must have one element")
			}
			elem := l.Elts[0]
			// TODO: record alias for error handling? In principle it is okay
			// to have duplicates, but we do want it to be used.
			if a, ok := elem.(*ast.Alias); ok {
				elem = a.Expr
			}

			return &adt.BulkOptionalField{
				Src:    x,
				Filter: c.expr(elem),
				Value:  value,
			}

		case *ast.Interpolation: // *ast.ParenExpr,
			if x.Token == token.ISA {
				c.errf(x, "definitions not supported for interpolations")
			}
			return &adt.DynamicField{
				Src:   x,
				Key:   c.expr(l),
				Value: value,
			}
		}

	// An alias reference will have an expression that is looked up in the
	// environment cash.
	case *ast.LetClause:
		// Cache the parsed expression. Creating a unique expression for each
		// reference allows the computation to be shared given that we don't
		// have fields for expressions. This, in turn, prevents exponential
		// blowup. in x2: x1+x1, x3: x2+x2, ... patterns.

		expr := c.labeledExpr(nil, (*letScope)(x), x.Expr)

		a := aliasEntry{source: x, expr: expr}

		if err := c.insertAlias(x.Ident, a); err != nil {
			return err
		}

	case *ast.Alias:

		expr := c.labeledExpr(nil, (*deprecatedAliasScope)(x), x.Expr)

		// TODO(legacy): deprecated, remove this use of Alias
		a := aliasEntry{source: x, expr: expr}

		if err := c.insertAlias(x.Ident, a); err != nil {
			return err
		}

	case *ast.CommentGroup:
		// Nothing to do for a free-floating comment group.

	case *ast.Attribute:
		// Nothing to do for now for an attribute declaration.

	case *ast.Ellipsis:
		return &adt.Ellipsis{
			Src:   x,
			Value: c.expr(x.Type),
		}

	case *ast.Comprehension:
		return c.comprehension(x)

	case *ast.EmbedDecl: // Deprecated
		return c.embed(x.Expr)

	case ast.Expr:
		return c.embed(x)
	}
	return nil
}

func (c *compiler) elem(n ast.Expr) adt.Elem {
	switch x := n.(type) {
	case *ast.Ellipsis:
		return &adt.Ellipsis{
			Src:   x,
			Value: c.expr(x.Type),
		}

	case *ast.Comprehension:
		return c.comprehension(x)

	case ast.Expr:
		return c.expr(x)
	}
	return nil
}

func (c *compiler) comprehension(x *ast.Comprehension) adt.Elem {
	var cur adt.Yielder
	var first adt.Elem
	var prev, next *adt.Yielder
	for _, v := range x.Clauses {
		switch x := v.(type) {
		case *ast.ForClause:
			var key adt.Feature
			if x.Key != nil {
				key = c.label(x.Key)
			}
			y := &adt.ForClause{
				Syntax: x,
				Key:    key,
				Value:  c.label(x.Value),
				Src:    c.expr(x.Source),
			}
			cur = y
			c.pushScope((*forScope)(x), 1, v)
			defer c.popScope()
			next = &y.Dst

		case *ast.IfClause:
			y := &adt.IfClause{
				Src:       x,
				Condition: c.expr(x.Condition),
			}
			cur = y
			next = &y.Dst

		case *ast.LetClause:
			y := &adt.LetClause{
				Src:   x,
				Label: c.label(x.Ident),
				Expr:  c.expr(x.Expr),
			}
			cur = y
			c.pushScope((*letScope)(x), 1, v)
			defer c.popScope()
			next = &y.Dst
		}

		if prev != nil {
			*prev = cur
		} else {
			var ok bool
			if first, ok = cur.(adt.Elem); !ok {
				return c.errf(x,
					"first comprehension clause must be 'if' or 'for'")
			}
		}
		prev = next
	}

	if y, ok := x.Value.(*ast.StructLit); !ok {
		return c.errf(x.Value,
			"comprehension value must be struct, found %T", y)
	}

	y := c.expr(x.Value)

	st, ok := y.(*adt.StructLit)
	if !ok {
		// Error must have been generated.
		return y
	}

	if prev != nil {
		*prev = &adt.ValueClause{StructLit: st}
	} else {
		return c.errf(x, "comprehension value without clauses")
	}

	return first
}

func (c *compiler) embed(expr ast.Expr) adt.Expr {
	switch n := expr.(type) {
	case *ast.StructLit:
		c.pushScope(nil, 1, n)
		v := &adt.StructLit{Src: n}
		c.addDecls(v, n.Elts)
		c.popScope()
		return v
	}
	return c.expr(expr)
}

func (c *compiler) labeledExpr(f *ast.Field, lab labeler, expr ast.Expr) adt.Expr {
	k := len(c.stack) - 1
	if c.stack[k].field != nil {
		panic("expected nil field")
	}
	c.stack[k].label = lab
	c.stack[k].field = f
	value := c.expr(expr)
	c.stack[k].label = nil
	c.stack[k].field = nil
	return value
}

func (c *compiler) expr(expr ast.Expr) adt.Expr {
	switch n := expr.(type) {
	case nil:
		return nil
	case *ast.Ident:
		return c.resolve(n)

	case *ast.StructLit:
		c.pushScope(nil, 1, n)
		v := &adt.StructLit{Src: n}
		c.addDecls(v, n.Elts)
		c.popScope()
		return v

	case *ast.ListLit:
		v := &adt.ListLit{Src: n}
		elts, ellipsis := internal.ListEllipsis(n)
		for _, d := range elts {
			elem := c.elem(d)

			switch x := elem.(type) {
			case nil:
			case adt.Elem:
				v.Elems = append(v.Elems, x)
			default:
				c.errf(d, "type %T not allowed in ListLit", d)
			}
		}
		if ellipsis != nil {
			d := &adt.Ellipsis{
				Src:   ellipsis,
				Value: c.expr(ellipsis.Type),
			}
			v.Elems = append(v.Elems, d)
		}
		return v

	case *ast.SelectorExpr:
		c.inSelector++
		ret := &adt.SelectorExpr{
			Src: n,
			X:   c.expr(n.X),
			Sel: c.label(n.Sel)}
		c.inSelector--
		return ret

	case *ast.IndexExpr:
		return &adt.IndexExpr{
			Src:   n,
			X:     c.expr(n.X),
			Index: c.expr(n.Index),
		}

	case *ast.SliceExpr:
		slice := &adt.SliceExpr{Src: n, X: c.expr(n.X)}
		if n.Low != nil {
			slice.Lo = c.expr(n.Low)
		}
		if n.High != nil {
			slice.Hi = c.expr(n.High)
		}
		return slice

	case *ast.BottomLit:
		return &adt.Bottom{Src: n}

	case *ast.BadExpr:
		return c.errf(n, "invalidExpression")

	case *ast.BasicLit:
		return c.parse(n)

	case *ast.Interpolation:
		if len(n.Elts) == 0 {
			return c.errf(n, "invalid interpolation")
		}
		first, ok1 := n.Elts[0].(*ast.BasicLit)
		last, ok2 := n.Elts[len(n.Elts)-1].(*ast.BasicLit)
		if !ok1 || !ok2 {
			return c.errf(n, "invalid interpolation")
		}
		if len(n.Elts) == 1 {
			return c.expr(n.Elts[0])
		}
		lit := &adt.Interpolation{Src: n, K: adt.StringKind}
		info, prefixLen, _, err := literal.ParseQuotes(first.Value, last.Value)
		if err != nil {
			return c.errf(n, "invalid interpolation: %v", err)
		}
		prefix := ""
		for i := 0; i < len(n.Elts); i += 2 {
			l, ok := n.Elts[i].(*ast.BasicLit)
			if !ok {
				return c.errf(n, "invalid interpolation")
			}
			s := l.Value
			if !strings.HasPrefix(s, prefix) {
				return c.errf(l, "invalid interpolation: unmatched ')'")
			}
			s = l.Value[prefixLen:]
			x := parseString(c, l, info, s)
			lit.Parts = append(lit.Parts, x)
			if i+1 < len(n.Elts) {
				lit.Parts = append(lit.Parts, c.expr(n.Elts[i+1]))
			}
			prefix = ")"
			prefixLen = 1
		}
		return lit

	case *ast.ParenExpr:
		return c.expr(n.X)

	case *ast.CallExpr:
		call := &adt.CallExpr{Src: n, Fun: c.expr(n.Fun)}
		for _, a := range n.Args {
			call.Args = append(call.Args, c.expr(a))
		}
		return call

	case *ast.UnaryExpr:
		switch n.Op {
		case token.NOT, token.ADD, token.SUB:
			return &adt.UnaryExpr{
				Src: n,
				Op:  adt.OpFromToken(n.Op),
				X:   c.expr(n.X),
			}
		case token.GEQ, token.GTR, token.LSS, token.LEQ,
			token.NEQ, token.MAT, token.NMAT:
			return &adt.BoundExpr{
				Src:  n,
				Op:   adt.OpFromToken(n.Op),
				Expr: c.expr(n.X),
			}

		case token.MUL:
			return c.errf(n, "preference mark not allowed at this position")
		default:
			return c.errf(n, "unsupported unary operator %q", n.Op)
		}

	case *ast.BinaryExpr:
		switch n.Op {
		case token.OR:
			d := &adt.DisjunctionExpr{Src: n}
			c.addDisjunctionElem(d, n.X, false)
			c.addDisjunctionElem(d, n.Y, false)
			return d

		default:
			// return updateBin(c,
			return &adt.BinaryExpr{
				Src: n,
				Op:  adt.OpFromToken(n.Op), // op
				X:   c.expr(n.X),           // left
				Y:   c.expr(n.Y),           // right
			} // )
		}

	default:
		panic(fmt.Sprintf("unknown expression type %T", n))
		// return c.errf(n, "unknown expression type %T", n)
	}
}

func (c *compiler) addDisjunctionElem(d *adt.DisjunctionExpr, n ast.Expr, mark bool) {
	switch x := n.(type) {
	case *ast.BinaryExpr:
		if x.Op == token.OR {
			c.addDisjunctionElem(d, x.X, mark)
			c.addDisjunctionElem(d, x.Y, mark)
			return
		}
	case *ast.UnaryExpr:
		if x.Op == token.MUL {
			d.HasDefaults = true
			c.addDisjunctionElem(d, x.X, true)
			return
		}
	}
	d.Values = append(d.Values, adt.Disjunct{Val: c.expr(n), Default: mark})
}

func (c *compiler) parse(l *ast.BasicLit) (n adt.Expr) {
	s := l.Value
	if s == "" {
		return c.errf(l, "invalid literal %q", s)
	}
	switch l.Kind {
	case token.STRING:
		info, nStart, _, err := literal.ParseQuotes(s, s)
		if err != nil {
			return c.errf(l, err.Error())
		}
		s := s[nStart:]
		return parseString(c, l, info, s)

	case token.FLOAT, token.INT:
		err := literal.ParseNum(s, &c.num)
		if err != nil {
			return c.errf(l, "parse error: %v", err)
		}
		kind := adt.FloatKind
		if c.num.IsInt() {
			kind = adt.IntKind
		}
		n := &adt.Num{Src: l, K: kind}
		if err = c.num.Decimal(&n.X); err != nil {
			return c.errf(l, "error converting number to decimal: %v", err)
		}
		return n

	case token.TRUE:
		return &adt.Bool{Src: l, B: true}

	case token.FALSE:
		return &adt.Bool{Src: l, B: false}

	case token.NULL:
		return &adt.Null{Src: l}

	default:
		return c.errf(l, "unknown literal type")
	}
}

// parseString decodes a string without the starting and ending quotes.
func parseString(c *compiler, node ast.Expr, q literal.QuoteInfo, s string) (n adt.Expr) {
	str, err := q.Unquote(s)
	if err != nil {
		return c.errf(node, "invalid string: %v", err)
	}
	if q.IsDouble() {
		return &adt.String{Src: node, Str: str, RE: nil}
	}
	return &adt.Bytes{Src: node, B: []byte(str), RE: nil}
}
