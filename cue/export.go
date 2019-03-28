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
	"math/rand"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/token"
)

type exportMode int

const (
	exportEval exportMode = 1 << iota
	exportAttrs
	exportRaw exportMode = 0
)

func export(ctx *context, v value, m exportMode) ast.Expr {
	e := exporter{ctx, m, nil}
	return e.expr(v)
}

type exporter struct {
	ctx   *context
	mode  exportMode
	stack []remap
}

type remap struct {
	key  scope // structLit or params
	from label
	to   *ast.Ident
	syn  *ast.StructLit
}

func (p *exporter) unique(s string) string {
	s = strings.ToUpper(s)
	lab := s
	for {
		if _, ok := p.ctx.labelMap[lab]; !ok {
			p.ctx.label(lab, true)
			break
		}
		lab = s + fmt.Sprintf("%0.6x", rand.Intn(1<<24))
	}
	return lab
}

func (p *exporter) label(f label) ast.Label {
	orig := p.ctx.labelStr(f)
	str := strconv.Quote(orig)
	if len(orig)+2 < len(str) {
		return &ast.BasicLit{Value: str}
	}
	for i, r := range orig {
		if unicode.IsLetter(r) || r == '_' {
			continue
		}
		if i > 0 && unicode.IsDigit(r) {
			continue
		}
		return &ast.BasicLit{Value: str}
	}
	return &ast.Ident{Name: orig}
}

func (p *exporter) identifier(f label) *ast.Ident {
	str := p.ctx.labelStr(f)
	return &ast.Ident{Name: str}
}

func (p *exporter) ident(str string) *ast.Ident {
	return &ast.Ident{Name: str}
}

func (p *exporter) clause(v value) (n ast.Clause, next yielder) {
	switch x := v.(type) {
	case *feed:
		feed := &ast.ForClause{
			Value:  p.identifier(x.fn.params.arcs[1].feature),
			Source: p.expr(x.source),
		}
		key := x.fn.params.arcs[0]
		if p.ctx.labelStr(key.feature) != "_" {
			feed.Key = p.identifier(key.feature)
		}
		return feed, x.fn.value.(yielder)

	case *guard:
		return &ast.IfClause{Condition: p.expr(x.condition)}, x.value
	}
	panic(fmt.Sprintf("unsupported clause type %T", v))
}

func (p *exporter) expr(v value) ast.Expr {
	if p.mode == exportEval {
		x := p.ctx.manifest(v)
		if isIncomplete(x) {
			p = &exporter{p.ctx, exportRaw, p.stack}
			return p.expr(v)
		}
		v = x
	}

	old := p.stack
	defer func() { p.stack = old }()

	// TODO: also add position information.
	switch x := v.(type) {
	case *builtin:
		return &ast.Ident{Name: x.Name}

	case *nodeRef:
		return nil

	case *selectorExpr:
		n := p.expr(x.x)
		if n != nil {
			return &ast.SelectorExpr{X: n, Sel: p.identifier(x.feature)}
		}
		ident := p.identifier(x.feature)
		node, ok := x.x.(*nodeRef)
		if !ok {
			// TODO: should not happen: report error
			return ident
		}
		conflict := false
		for i := len(p.stack) - 1; i >= 0; i-- {
			e := &p.stack[i]
			if e.from != x.feature {
				continue
			}
			if e.key != node.node {
				conflict = true
				continue
			}
			if conflict {
				ident = e.to
				if e.to == nil {
					name := p.unique(p.ctx.labelStr(x.feature))
					e.syn.Elts = append(e.syn.Elts, &ast.Alias{
						Ident: p.ident(name),
						Expr:  p.identifier(x.feature),
					})
					ident = p.ident(name)
					e.to = ident
				}
			}
			return ident
		}
		// TODO: should not happen: report error
		return ident

	case *indexExpr:
		return &ast.IndexExpr{X: p.expr(x.x), Index: p.expr(x.index)}

	case *sliceExpr:
		return &ast.SliceExpr{
			X:    p.expr(x.x),
			Low:  p.expr(x.lo),
			High: p.expr(x.hi),
		}

	case *callExpr:
		call := &ast.CallExpr{Fun: p.expr(x.x)}
		for _, a := range x.args {
			call.Args = append(call.Args, p.expr(a))
		}
		return call

	case *unaryExpr:
		return &ast.UnaryExpr{Op: opMap[x.op], X: p.expr(x.x)}

	case *binaryExpr:
		return &ast.BinaryExpr{
			X:  p.expr(x.left),
			Op: opMap[x.op], Y: p.expr(x.right),
		}

	case *bound:
		return &ast.UnaryExpr{Op: opMap[x.op], X: p.expr(x.value)}

	case *unification:
		if len(x.values) == 1 {
			return p.expr(x.values[0])
		}
		bin := p.expr(x.values[0])
		for _, v := range x.values[1:] {
			bin = &ast.BinaryExpr{X: bin, Op: token.AND, Y: p.expr(v)}
		}
		return bin

	case *disjunction:
		if len(x.values) == 1 {
			return p.expr(x.values[0].val)
		}
		expr := func(v dValue) ast.Expr {
			e := p.expr(v.val)
			if v.marked {
				e = &ast.UnaryExpr{Op: token.MUL, X: e}
			}
			return e
		}
		bin := expr(x.values[0])
		for _, v := range x.values[1:] {
			bin = &ast.BinaryExpr{X: bin, Op: token.OR, Y: expr(v)}
		}
		return bin

	case *structLit:
		obj := &ast.StructLit{}
		if p.mode == exportEval {
			for _, a := range x.arcs {
				p.stack = append(p.stack, remap{
					key:  x,
					from: a.feature,
					to:   nil,
					syn:  obj,
				})
			}
			x = x.expandFields(p.ctx)
		}
		if x.emit != nil {
			obj.Elts = append(obj.Elts, &ast.EmitDecl{Expr: p.expr(x.emit)})
		}
		if p.mode != exportEval && x.template != nil {
			l, ok := x.template.evalPartial(p.ctx).(*lambdaExpr)
			if ok {
				obj.Elts = append(obj.Elts, &ast.Field{
					Label: &ast.TemplateLabel{
						Ident: p.identifier(l.params.arcs[0].feature),
					},
					Value: p.expr(l.value),
				})
			} // TODO: else record error
		}
		for i, a := range x.arcs {
			f := &ast.Field{
				Label: p.label(a.feature),
			}
			if p.mode != exportEval {
				f.Value = p.expr(a.v)
			} else if v := p.ctx.manifest(x.at(p.ctx, i)); isIncomplete(v) {
				p := &exporter{p.ctx, exportRaw, p.stack}
				f.Value = p.expr(a.v)
			} else {
				f.Value = p.expr(v)
			}
			if a.attrs != nil { // TODO: && p.mode&exportAttrs != 0 {
				for _, at := range a.attrs.attr {
					f.Attrs = append(f.Attrs, &ast.Attribute{Text: at.text})
				}
			}
			obj.Elts = append(obj.Elts, f)
		}

		for _, c := range x.comprehensions {
			var clauses []ast.Clause
			next := c.clauses
			for {
				if yield, ok := next.(*yield); ok {
					l := p.expr(yield.key)
					label, ok := l.(ast.Label)
					if !ok {
						// TODO: add an invalid field instead?
						continue
					}
					f := &ast.Field{
						Label: label,
						Value: p.expr(yield.value),
					}
					var decl ast.Decl = f
					if len(clauses) > 0 {
						decl = &ast.ComprehensionDecl{Field: f, Clauses: clauses}
					}
					obj.Elts = append(obj.Elts, decl)
					break
				}

				var y ast.Clause
				y, next = p.clause(next)
				clauses = append(clauses, y)
			}
		}
		return obj

	case *fieldComprehension:
		panic("should be handled in structLit")

	case *listComprehension:
		var clauses []ast.Clause
		for y, next := p.clause(x.clauses); ; y, next = p.clause(next) {
			clauses = append(clauses, y)
			if yield, ok := next.(*yield); ok {
				return &ast.ListComprehension{
					Expr:    p.expr(yield.value),
					Clauses: clauses,
				}
			}
		}

	case *nullLit:
		return p.ident("null")

	case *boolLit:
		return p.ident(fmt.Sprint(x.b))

	case *stringLit:
		return &ast.BasicLit{
			Kind:  token.STRING,
			Value: quote(x.str, '"'),
		}

	case *bytesLit:
		return &ast.BasicLit{
			Kind:  token.STRING,
			Value: quote(string(x.b), '\''),
		}

	case *numLit:
		if x.k&intKind != 0 {
			return &ast.BasicLit{
				Kind:  token.INT,
				Value: x.v.Text('f'),
			}
		}
		return &ast.BasicLit{
			Kind:  token.FLOAT,
			Value: x.v.Text('g'),
		}

	case *durationLit:
		panic("unimplemented")

	case *interpolation:
		t := &ast.Interpolation{}
		multiline := false
		// TODO: mark formatting in interpolation itself.
		for i := 0; i < len(x.parts); i += 2 {
			str := x.parts[i].(*stringLit).str
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
		for i, e := range x.parts {
			if i%2 == 1 {
				t.Elts = append(t.Elts, p.expr(e))
			} else {
				buf := []byte(prefix)
				if i == len(x.parts)-1 {
					suffix = quote
				}
				str := e.(*stringLit).str
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

	case *list:
		list := &ast.ListLit{}
		var expr ast.Expr = list
		for _, e := range x.a {
			list.Elts = append(list.Elts, p.expr(e))
		}
		max := maxNum(x.len)
		num, ok := max.(*numLit)
		if !ok {
			min := minNum(x.len)
			num, _ = min.(*numLit)
		}
		ln := 0
		if num != nil {
			x, _ := num.v.Int64()
			ln = int(x)
		}
		open := false
		switch max.(type) {
		case *top, *basicType:
			open = true
		}
		if !ok || ln > len(x.a) {
			list.Type = p.expr(x.typ)
			if !open && !isTop(x.typ) {
				expr = &ast.BinaryExpr{
					X: &ast.BinaryExpr{
						X:  p.expr(x.len),
						Op: token.MUL,
						Y: &ast.ListLit{Elts: []ast.Expr{
							p.expr(x.typ),
						}},
					},
					Op: token.AND,
					Y:  list,
				}

			}
		}
		return expr

	case *bottom:
		err := &ast.BottomLit{}
		comment := &ast.Comment{Text: "// " + x.msg}
		err.AddComment(&ast.CommentGroup{
			Line:     true,
			Position: 1,
			List:     []*ast.Comment{comment},
		})
		return err

	case *top:
		return p.ident("_")

	case *basicType:
		return p.ident(x.k.String())

	case *lambdaExpr:
		return p.ident("TODO: LAMBDA")

	default:
		panic(fmt.Sprintf("unimplemented type %T", x))
	}
}

// quote quotes the given string.
func quote(str string, quote byte) string {
	if strings.IndexByte(str, '\n') < 0 {
		buf := []byte{quote}
		buf = appendEscaped(buf, str, quote, true)
		buf = append(buf, quote)
		return string(buf)
	}
	buf := []byte{quote, quote, quote}
	buf = append(buf, multiSep...)
	buf = appendEscapeMulti(buf, str, quote)
	buf = append(buf, quote, quote, quote)
	return string(buf)
}

// TODO: consider the best indent strategy.
const multiSep = "\n        "

func appendEscapeMulti(buf []byte, str string, quote byte) []byte {
	// TODO(perf)
	a := strings.Split(str, "\n")
	for _, s := range a {
		buf = appendEscaped(buf, s, quote, true)
		buf = append(buf, multiSep...)
	}
	return buf
}

const lowerhex = "0123456789abcdef"

func appendEscaped(buf []byte, s string, quote byte, graphicOnly bool) []byte {
	for width := 0; len(s) > 0; s = s[width:] {
		r := rune(s[0])
		width = 1
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRuneInString(s)
		}
		if width == 1 && r == utf8.RuneError {
			buf = append(buf, `\x`...)
			buf = append(buf, lowerhex[s[0]>>4])
			buf = append(buf, lowerhex[s[0]&0xF])
			continue
		}
		buf = appendEscapedRune(buf, r, quote, graphicOnly)
	}
	return buf
}

func appendEscapedRune(buf []byte, r rune, quote byte, graphicOnly bool) []byte {
	var runeTmp [utf8.UTFMax]byte
	if r == rune(quote) || r == '\\' { // always backslashed
		buf = append(buf, '\\')
		buf = append(buf, byte(r))
		return buf
	}
	// TODO(perf): IsGraphic calls IsPrint.
	if strconv.IsPrint(r) || graphicOnly && strconv.IsGraphic(r) {
		n := utf8.EncodeRune(runeTmp[:], r)
		buf = append(buf, runeTmp[:n]...)
		return buf
	}
	switch r {
	case '\a':
		buf = append(buf, `\a`...)
	case '\b':
		buf = append(buf, `\b`...)
	case '\f':
		buf = append(buf, `\f`...)
	case '\n':
		buf = append(buf, `\n`...)
	case '\r':
		buf = append(buf, `\r`...)
	case '\t':
		buf = append(buf, `\t`...)
	case '\v':
		buf = append(buf, `\v`...)
	default:
		switch {
		case r < ' ':
			// Invalid for strings, only bytes.
			buf = append(buf, `\x`...)
			buf = append(buf, lowerhex[byte(r)>>4])
			buf = append(buf, lowerhex[byte(r)&0xF])
		case r > utf8.MaxRune:
			r = 0xFFFD
			fallthrough
		case r < 0x10000:
			buf = append(buf, `\u`...)
			for s := 12; s >= 0; s -= 4 {
				buf = append(buf, lowerhex[r>>uint(s)&0xF])
			}
		default:
			buf = append(buf, `\U`...)
			for s := 28; s >= 0; s -= 4 {
				buf = append(buf, lowerhex[r>>uint(s)&0xF])
			}
		}
	}
	return buf
}
