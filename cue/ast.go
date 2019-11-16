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
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal"
	"golang.org/x/xerrors"
)

// insertFile inserts the given file at the root of the instance.
//
// The contents will be merged (unified) with any pre-existing value. In this
// case an error may be reported, but only if the merge failed at the top-level.
// Other errors will be recorded at the respective values in the tree.
//
// There should be no unresolved identifiers in file, meaning the Node field
// of all identifiers should be set to a non-nil value.
func (inst *Instance) insertFile(f *ast.File) errors.Error {
	// TODO: insert by converting to value first so that the trim command can
	// also remove top-level fields.
	// First process single file.
	v := newVisitor(inst.index, inst.inst, inst.rootStruct, inst.scope)
	v.astState.astMap[f] = inst.rootStruct
	// TODO: fix cmd/import to resolve references in the AST before
	// inserting. For now, we accept errors that did not make it up to the tree.
	result := v.walk(f)
	if isBottom(result) {
		val := newValueRoot(v.ctx(), result)
		v.errors = errors.Append(v.errors, val.toErr(result.(*bottom)))
	}
	return v.errors
}

type astVisitor struct {
	*astState
	object *structLit

	parent *astVisitor
	sel    string // label or index; may be '*'
	// For single line fields, the doc comment is applied to the inner-most
	// field value.
	//
	//   // This comment is for bar.
	//   foo bar: value
	//
	doc *docNode

	inSelector int
}

func (v *astVisitor) ctx() *context {
	return v.astState.ctx
}

type astState struct {
	ctx *context
	*index
	inst *build.Instance

	litParser   *litParser
	resolveRoot *structLit

	// make unique per level to avoid reuse of structs being an issue.
	astMap map[ast.Node]scope

	errors errors.Error
}

func (s *astState) mapScope(n ast.Node) (m scope) {
	if m = s.astMap[n]; m == nil {
		m = newStruct(newNode(n))
		s.astMap[n] = m
	}
	return m
}

func (s *astState) setScope(n ast.Node, v scope) {
	if m, ok := s.astMap[n]; ok && m != v {
		panic("already defined")
	}
	s.astMap[n] = v
}

func newVisitor(idx *index, inst *build.Instance, obj, resolveRoot *structLit) *astVisitor {
	ctx := idx.newContext()
	return newVisitorCtx(ctx, inst, obj, resolveRoot)
}

func newVisitorCtx(ctx *context, inst *build.Instance, obj, resolveRoot *structLit) *astVisitor {
	v := &astVisitor{
		object: obj,
	}
	v.astState = &astState{
		ctx:         ctx,
		index:       ctx.index,
		inst:        inst,
		litParser:   &litParser{ctx: ctx},
		resolveRoot: resolveRoot,
		astMap:      map[ast.Node]scope{},
	}
	return v
}

func (v *astVisitor) errf(n ast.Node, format string, args ...interface{}) evaluated {
	v.astState.errors = errors.Append(v.astState.errors, &nodeError{
		path:    v.appendPath(nil),
		n:       n,
		Message: errors.NewMessage(format, args),
	})
	arguments := append([]interface{}{format}, args...)
	return v.mkErr(newNode(n), arguments...)
}

func (v *astVisitor) appendPath(a []string) []string {
	if v.parent != nil {
		a = v.parent.appendPath(a)
	}
	if v.sel != "" {
		a = append(a, v.sel)
	}
	return a
}

func (v *astVisitor) resolve(n *ast.Ident) value {
	ctx := v.ctx()
	name := v.ident(n)
	label := v.label(name, true)
	if r := v.resolveRoot; r != nil {
		for _, a := range r.arcs {
			if a.feature == label {
				return &selectorExpr{newExpr(n),
					&nodeRef{baseValue: newExpr(n), node: r, label: label}, label}
			}
		}
		if v.inSelector > 0 {
			if p := getBuiltinShorthandPkg(ctx, name); p != nil {
				return &nodeRef{newExpr(n), p, label}
			}
		}
	}
	return nil
}

func (v *astVisitor) loadImport(imp *ast.ImportSpec) evaluated {
	ctx := v.ctx()
	path, err := literal.Unquote(imp.Path.Value)
	if err != nil {
		return v.errf(imp, "illformed import spec")
	}
	// TODO: allow builtin *and* imported package. The result is a unified
	// struct.
	if p := getBuiltinPkg(ctx, path); p != nil {
		return p
	}
	bimp := v.inst.LookupImport(path)
	if bimp == nil {
		return v.errf(imp, "package %q not found", path)
	}
	impInst := v.index.loadInstance(bimp)
	return impInst.rootValue.evalPartial(ctx)
}

func (v *astVisitor) ident(n *ast.Ident) string {
	str, err := ast.ParseIdent(n)
	if err != nil {
		v.errf(n, "invalid literal: %v", err)
		return n.Name
	}
	return str
}

// We probably don't need to call Walk.s
func (v *astVisitor) walk(astNode ast.Node) (ret value) {
	switch n := astNode.(type) {
	case *ast.File:
		obj := v.object
		v1 := &astVisitor{
			astState: v.astState,
			object:   obj,
		}
		for _, e := range n.Decls {
			switch x := e.(type) {
			case *ast.EmbedDecl:
				if v1.object.emit == nil {
					v1.object.emit = v1.walk(x.Expr)
				} else {
					v1.object.emit = mkBin(v.ctx(), token.NoPos, opUnify, v1.object.emit, v1.walk(x.Expr))
				}
			default:
				v1.walk(e)
			}
		}
		ret = obj

	case *ast.Package:
		// NOTE: Do NOT walk the identifier of the package here, as it is not
		// supposed to resolve to anything.

	case *ast.ImportDecl:
		for _, s := range n.Specs {
			v.walk(s)
		}

	case *ast.ImportSpec:
		val := v.loadImport(n)
		if !isBottom(val) {
			v.setScope(n, val.(*structLit))
		}

	case *ast.StructLit:
		obj := v.mapScope(n).(*structLit)
		v1 := &astVisitor{
			astState: v.astState,
			object:   obj,
			parent:   v,
		}
		passDoc := len(n.Elts) == 1 && !n.Lbrace.IsValid() && v.doc != nil
		if passDoc {
			v1.doc = v.doc
		}
		ret = obj
		for i, e := range n.Elts {
			switch x := e.(type) {
			case *ast.Ellipsis:
				if i != len(n.Elts)-1 {
					return v1.walk(x.Type) // Generate an error
				}
				f := v.ctx().label("_", true)
				sig := &params{}
				sig.add(f, &basicType{newNode(x), stringKind})
				template := &lambdaExpr{newNode(x), sig, &top{newNode(x)}}
				v1.object.addTemplate(v.ctx(), x.Pos(), nil, template)

			case *ast.EmbedDecl:
				old := v.ctx().inDefinition
				v.ctx().inDefinition = 0
				e := v1.walk(x.Expr)
				v.ctx().inDefinition = old
				if isBottom(e) {
					return e
				}
				if e.kind()&structKind == 0 {
					return v1.errf(x, "can only embed structs (found %v)", e.kind())
				}
				ret = mkBin(v1.ctx(), x.Pos(), opUnifyUnchecked, ret, e)
				// TODO: preserve order of embedded fields. We cannot split into
				// separate unifications here, as recursive references point to
				// obj and would have to be dereferenced and copied.
				// Solving this is best done with a generic topological sort
				// mechanism.

			case *ast.Field, *ast.Alias:
				v1.walk(e)

			case *ast.Comprehension:
				v1.walk(x)
			}
		}
		if v.ctx().inDefinition > 0 && !obj.optionals.isFull() {
			// For embeddings this is handled in binOp, in which case the
			// isClosed bit is cleared if a template is introduced.
			obj.closeStatus = toClose
		}
		if passDoc {
			v.doc = v1.doc // signal usage of document back to parent.
		}

	case *ast.ListLit:
		v1 := &astVisitor{
			astState: v.astState,
			object:   v.object,
			parent:   v,
		}

		elts, ellipsis := internal.ListEllipsis(n)

		arcs := []arc{}
		for i, e := range elts {
			elem := v1.walk(e)
			if elem == nil {
				// TODO: it would be consistent to allow aliasing in lists
				// as well, with a similar meaning as alias declarations in
				// structs.
				return v.errf(n, "alias not allowed in list")
			}
			v1.sel = strconv.Itoa(i)
			arcs = append(arcs, arc{feature: label(i), v: elem})
		}
		s := &structLit{baseValue: newExpr(n), arcs: arcs}
		list := &list{baseValue: newExpr(n), elem: s}
		list.initLit()
		if ellipsis != nil {
			list.len = newBound(v.ctx(), list.baseValue, opGeq, intKind, list.len)
			if ellipsis.Type != nil {
				list.typ = v1.walk(ellipsis.Type)
			}
		}
		ret = list

	case *ast.Ellipsis:
		return v.errf(n, "ellipsis (...) only allowed at end of list or struct")

	case *ast.Comprehension:
		yielder := &yield{baseValue: newExpr(n.Value)}
		sc := &structComprehension{
			newNode(n),
			wrapClauses(v, yielder, n.Clauses),
		}
		// we don't support key for lists (yet?)
		switch n.Value.(type) {
		case *ast.StructLit:
		default:
			// Caught by parser, usually.
			v.errf(n, "comprehension must be struct")
		}
		yielder.value = v.walk(n.Value)
		v.object.comprehensions = append(v.object.comprehensions, compValue{comp: sc})

	case *ast.Field:
		opt := n.Optional != token.NoPos
		isDef := n.Token == token.ISA
		if isDef {
			ctx := v.ctx()
			ctx.inDefinition++
			defer func() { ctx.inDefinition-- }()
		}
		attrs, err := createAttrs(v.ctx(), newNode(n), n.Attrs)
		if err != nil {
			return v.errf(n, err.format, err.args)
		}
		var leftOverDoc *docNode
		for _, c := range n.Comments() {
			if c.Position == 0 {
				leftOverDoc = v.doc
				v.doc = &docNode{n: n}
				break
			}
		}

		lab := n.Label
		if a, ok := lab.(*ast.Alias); ok {
			if lab, ok = a.Expr.(ast.Label); !ok {
				return v.errf(n, "alias expression is not a valid label")
			}
		}

		switch x := lab.(type) {
		case *ast.Interpolation:
			v.sel = "?"
			// Must be struct comprehension.
			fc := &fieldComprehension{
				baseValue: newDecl(n),
				key:       v.walk(x),
				val:       v.walk(n.Value),
				opt:       opt,
				def:       isDef,
				doc:       leftOverDoc,
				attrs:     attrs,
			}
			v.object.comprehensions = append(v.object.comprehensions, compValue{comp: fc})

		case *ast.ListLit:
			if len(x.Elts) != 1 {
				return v.errf(x, "optional label expression must have exactly one element; found %d", len(x.Elts))
			}
			var f label
			expr := x.Elts[0]
			a, ok := expr.(*ast.Alias)
			if ok {
				expr = a.Expr
				f = v.label(v.ident(a.Ident), true)
			} else {
				f = v.label("_", true)
			}

			// Parse the key filter or a bulk-optional field. The special value
			// of nil to mean "all fields".
			var key value
			if i, ok := expr.(*ast.Ident); !ok || (i.Name != "string" && i.Name != "_") {
				key = v.walk(expr)
			}
			v.sel = "*"

			sig := &params{}
			sig.add(f, &basicType{newNode(lab), stringKind})
			template := &lambdaExpr{newNode(n), sig, nil}

			v.setScope(n, template)
			template.value = v.walk(n.Value)

			v.object.addTemplate(v.ctx(), token.NoPos, key, template)

		case *ast.TemplateLabel:
			if isDef {
				v.errf(x, "map element type cannot be a definition")
			}
			v.sel = "*"
			f := v.label(v.ident(x.Ident), true)

			sig := &params{}
			sig.add(f, &basicType{newNode(lab), stringKind})
			template := &lambdaExpr{newNode(n), sig, nil}

			v.setScope(n, template)
			template.value = v.walk(n.Value)

			v.object.addTemplate(v.ctx(), token.NoPos, nil, template)

		case *ast.BasicLit, *ast.Ident:
			if internal.DropOptional && opt {
				break
			}
			v.sel, _, _ = ast.LabelName(x)
			if v.sel == "_" {
				if _, ok := x.(*ast.BasicLit); ok {
					v.sel = "*"
				}
			}
			f, ok := v.nodeLabel(x)
			if !ok {
				return v.errf(lab, "invalid field name: %v", lab)
			}
			if f != 0 {
				val := v.walk(n.Value)
				v.object.insertValue(v.ctx(), f, opt, isDef, val, attrs, v.doc)
				v.doc = leftOverDoc
			}

		default:
			panic("cue: unknown label type")
		}

	case *ast.Alias:
		// parsed verbatim at reference.

	case *ast.ListComprehension:
		yielder := &yield{baseValue: newExpr(n.Expr)}
		lc := &listComprehension{
			newExpr(n),
			wrapClauses(v, yielder, n.Clauses),
		}
		// we don't support key for lists (yet?)
		yielder.value = v.walk(n.Expr)
		return lc

	// Expressions
	case *ast.Ident:
		name := v.ident(n)

		if name == "_" {
			ret = &top{newNode(n)}
			break
		}

		if n.Node == nil {
			if ret = v.resolve(n); ret != nil {
				break
			}

			switch name {
			case "_":
				return &top{newExpr(n)}
			case "string":
				return &basicType{newExpr(n), stringKind}
			case "bytes":
				return &basicType{newExpr(n), bytesKind}
			case "bool":
				return &basicType{newExpr(n), boolKind}
			case "int":
				return &basicType{newExpr(n), intKind}
			case "float":
				return &basicType{newExpr(n), floatKind}
			case "number":
				return &basicType{newExpr(n), numKind}
			case "duration":
				return &basicType{newExpr(n), durationKind}

			case "len":
				return lenBuiltin
			case "close":
				return closeBuiltin
			case "and":
				return andBuiltin
			case "or":
				return orBuiltin
			}
			if r, ok := predefinedRanges[name]; ok {
				return r
			}

			ret = v.errf(n, "reference %q not found", name)
			break
		}

		// Type of reference      Scope          Node
		// Alias declaration      File/Struct    Alias
		// Illegal Reference      File/Struct
		// Fields
		//    Label               File/Struct    ParenExpr, Ident, BasicLit
		//    Value               File/Struct    Field
		// Template               Field          Template
		// Fields inside lambda
		//    Label               Field          Expr
		//    Value               Field          Field
		// Pkg                    nil            ImportSpec

		if x, ok := n.Node.(*ast.Alias); ok {
			old := v.ctx().inDefinition
			v.ctx().inDefinition = 0
			ret = v.walk(x.Expr)
			v.ctx().inDefinition = old
			break
		}

		f := v.label(name, true)
		if n.Scope == nil {
			// Package or direct ancestor node.
			n2 := v.mapScope(n.Node)
			ref := &nodeRef{baseValue: newExpr(n), node: n2, label: f}
			ret = ref
			break
		}

		n2 := v.mapScope(n.Scope)
		ret = &nodeRef{baseValue: newExpr(n), node: n2}

		// Allow different names to refer to the same field in unification. We
		// do this by anonymizing the the reference. This then has to be
		// resolved again when refering to lambdas.
		l, lambda := n2.(*lambdaExpr)
		if lambda && len(l.params.arcs) == 1 {
			f = 0
		}

		if field, ok := n.Node.(*ast.Field); ok {
			if lambda {
				// inside bulk optional.
				ret = v.errf(n, "referencing field (%q) within lambda not yet unsupported", name)
				break
			}
			name, _, err := ast.LabelName(field.Label)
			switch {
			case xerrors.Is(err, ast.ErrIsExpression):
				a := field.Label.(*ast.Alias)
				ret = &indexExpr{newExpr(n), ret, v.walk(a.Expr)}

			case err != nil:
				ret = v.errf(n, "invalid label: %v", err)

			case name != "":
				f = v.label(name, true)
				ret = &selectorExpr{newExpr(n), ret, f}

			default:
				// TODO: support dynamically computed label lookup.
				// Should that also support lookup of definitions?
				ret = v.errf(n, "unsupported field alias %q", name)
			}
			break
		}

		ret = &selectorExpr{newExpr(n), ret, f}

	case *ast.BottomLit:
		// TODO: record inline comment.
		ret = &bottom{baseValue: newExpr(n), code: codeUser, format: "from source"}

	case *ast.BadDecl:
		// nothing to do

	case *ast.BadExpr:
		ret = v.errf(n, "invalid expression")

	case *ast.BasicLit:
		ret = v.litParser.parse(n)

	case *ast.Interpolation:
		if len(n.Elts) == 0 {
			return v.errf(n, "invalid interpolation")
		}
		first, ok1 := n.Elts[0].(*ast.BasicLit)
		last, ok2 := n.Elts[len(n.Elts)-1].(*ast.BasicLit)
		if !ok1 || !ok2 {
			return v.errf(n, "invalid interpolation")
		}
		if len(n.Elts) == 1 {
			ret = v.walk(n.Elts[0])
			break
		}
		lit := &interpolation{baseValue: newExpr(n), k: stringKind}
		ret = lit
		info, prefixLen, _, err := literal.ParseQuotes(first.Value, last.Value)
		if err != nil {
			return v.errf(n, "invalid interpolation: %v", err)
		}
		prefix := ""
		for i := 0; i < len(n.Elts); i += 2 {
			l, ok := n.Elts[i].(*ast.BasicLit)
			if !ok {
				return v.errf(n, "invalid interpolation")
			}
			s := l.Value
			if !strings.HasPrefix(s, prefix) {
				return v.errf(l, "invalid interpolation: unmatched ')'")
			}
			s = l.Value[prefixLen:]
			x := parseString(v.ctx(), l, info, s)
			lit.parts = append(lit.parts, x)
			if i+1 < len(n.Elts) {
				lit.parts = append(lit.parts, v.walk(n.Elts[i+1]))
			}
			prefix = ")"
			prefixLen = 1
		}

	case *ast.ParenExpr:
		ret = v.walk(n.X)

	case *ast.SelectorExpr:
		v.inSelector++
		ret = &selectorExpr{
			newExpr(n),
			v.walk(n.X),
			v.label(v.ident(n.Sel), true),
		}
		v.inSelector--

	case *ast.IndexExpr:
		ret = &indexExpr{newExpr(n), v.walk(n.X), v.walk(n.Index)}

	case *ast.SliceExpr:
		slice := &sliceExpr{baseValue: newExpr(n), x: v.walk(n.X)}
		if n.Low != nil {
			slice.lo = v.walk(n.Low)
		}
		if n.High != nil {
			slice.hi = v.walk(n.High)
		}
		ret = slice

	case *ast.CallExpr:
		call := &callExpr{baseValue: newExpr(n), x: v.walk(n.Fun)}
		for _, a := range n.Args {
			call.args = append(call.args, v.walk(a))
		}
		ret = call

	case *ast.UnaryExpr:
		switch n.Op {
		case token.NOT, token.ADD, token.SUB:
			ret = &unaryExpr{
				newExpr(n),
				tokenMap[n.Op],
				v.walk(n.X),
			}
		case token.GEQ, token.GTR, token.LSS, token.LEQ,
			token.NEQ, token.MAT, token.NMAT:
			ret = newBound(
				v.ctx(),
				newExpr(n),
				tokenMap[n.Op],
				topKind|nonGround,
				v.walk(n.X),
			)

		case token.MUL:
			return v.errf(n, "preference mark not allowed at this position")
		default:
			return v.errf(n, "unsupported unary operator %q", n.Op)
		}

	case *ast.BinaryExpr:
		switch n.Op {
		case token.OR:
			d := &disjunction{baseValue: newExpr(n)}
			v.addDisjunctionElem(d, n.X, false)
			v.addDisjunctionElem(d, n.Y, false)
			ret = d

		default:
			ret = updateBin(v.ctx(), &binaryExpr{
				newExpr(n),
				tokenMap[n.Op], // op
				v.walk(n.X),    // left
				v.walk(n.Y),    // right
			})
		}

	case *ast.CommentGroup:
		// Nothing to do for a free-floating comment group.

	// nothing to do
	// case *syntax.EmbedDecl:
	default:
		// TODO: unhandled node.
		// value = ctx.mkErr(n, "unknown node type %T", n)
		panic(fmt.Sprintf("unimplemented %T", n))

	}
	return ret
}

func (v *astVisitor) addDisjunctionElem(d *disjunction, n ast.Node, mark bool) {
	switch x := n.(type) {
	case *ast.BinaryExpr:
		if x.Op == token.OR {
			v.addDisjunctionElem(d, x.X, mark)
			v.addDisjunctionElem(d, x.Y, mark)
			return
		}
	case *ast.UnaryExpr:
		if x.Op == token.MUL {
			mark = true
			n = x.X
		}
		d.hasDefaults = true
	}
	d.values = append(d.values, dValue{v.walk(n), mark})
}

func wrapClauses(v *astVisitor, y yielder, clauses []ast.Clause) yielder {
	for _, c := range clauses {
		if n, ok := c.(*ast.ForClause); ok {
			params := &params{}
			fn := &lambdaExpr{newExpr(n.Source), params, nil}
			v.setScope(n, fn)
		}
	}
	for i := len(clauses) - 1; i >= 0; i-- {
		switch n := clauses[i].(type) {
		case *ast.ForClause:
			fn := v.mapScope(n).(*lambdaExpr)
			fn.value = y

			key := "_"
			if n.Key != nil {
				key = v.ident(n.Key)
			}
			f := v.label(key, true)
			fn.add(f, &basicType{newExpr(n.Key), stringKind | intKind})

			f = v.label(v.ident(n.Value), true)
			fn.add(f, &top{})

			y = &feed{newExpr(n.Source), v.walk(n.Source), fn}

		case *ast.IfClause:
			y = &guard{newExpr(n.Condition), v.walk(n.Condition), y}
		}
	}
	return y
}
