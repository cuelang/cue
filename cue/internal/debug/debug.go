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

// Package debug prints a given ADT node.
//
// Note that the result is not valid CUE, but instead prints the internals
// of an ADT node in human-readable form. It uses a simple indentation algorithm
// for improved readability and diffing.
//
package debug

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/internal/adt"
	"cuelang.org/go/internal"
)

const (
	openTuple  = "\u3008"
	closeTuple = "\u3009"
)

type Config struct {
	Cwd string
}

func WriteNode(w io.Writer, i adt.StringIndexer, n adt.Node, config *Config) {
	if config == nil {
		config = &Config{}
	}
	p := printer{Writer: w, index: i, cfg: config}
	p.node(n)
}

func NodeString(i adt.StringIndexer, n adt.Node, config *Config) string {
	if config == nil {
		config = &Config{}
	}
	b := &strings.Builder{}
	p := printer{Writer: b, index: i, cfg: config}
	p.node(n)
	return b.String()
}

type printer struct {
	io.Writer
	index  adt.StringIndexer
	indent string
	cfg    *Config
}

func (w *printer) string(s string) {
	s = strings.Replace(s, "\n", "\n"+w.indent, -1)
	_, _ = io.WriteString(w, s)
}

func (w *printer) label(f adt.Feature) {
	w.string(w.labelString(f))
}

// TODO: fold into label once :: is no longer supported.
func (w *printer) labelString(f adt.Feature) string {
	return f.ToString(w.index)
}

func (w *printer) node(n adt.Node) {
	switch x := n.(type) {
	case *adt.Vertex:
		var kind adt.Kind
		if x.Value != nil {
			kind = x.Value.Kind()
		}

		fmt.Fprintf(w, "(%s){", kind)

		if x.Value != nil && kind&^(adt.StructKind|adt.ListKind) != 0 {
			w.string(" ")
			w.node(x.Value)
			w.string(" }")
			return
		}

		saved := w.indent
		w.indent += "  "

		if b, ok := x.Value.(*adt.Bottom); ok {
			saved := w.indent
			w.indent += "// "
			w.string("\n")
			w.string(strings.TrimSpace(errors.Details(b.Err, &errors.Config{
				Cwd:     w.cfg.Cwd,
				ToSlash: true,
			})))
			w.indent = saved
		}

		if len(x.Arcs) > 0 {
			for _, a := range x.Arcs {
				w.string("\n")
				w.label(a.Label)
				w.string(": ")
				w.node(a)
			}
		}

		if x.Value == nil {
			w.indent += "// "
			w.string("// ")
			for i, c := range x.Conjuncts {
				if i > 0 {
					w.string(" & ")
				}
				w.node(c.Expr()) // TODO: also include env?
			}
		}

		w.indent = saved
		w.string("\n")
		w.string("}")

	case *adt.StructMarker:
		w.string("struct")

	case *adt.ListMarker:
		w.string("list")

	case *adt.StructLit:
		if len(x.Decls) == 0 {
			w.string("{}")
			break
		}
		w.string("{")
		w.indent += "  "
		for _, d := range x.Decls {
			w.string("\n")
			w.node(d)
		}
		w.indent = w.indent[:len(w.indent)-2]
		w.string("\n}")

	case *adt.ListLit:
		if len(x.Elems) == 0 {
			w.string("[]")
			break
		}
		w.string("[")
		w.indent += "  "
		for _, d := range x.Elems {
			w.string("\n")
			w.node(d)
			w.string(",")
		}
		w.indent = w.indent[:len(w.indent)-2]
		w.string("\n]")

	case *adt.Field:
		s := w.labelString(x.Label)
		w.string(s)
		w.string(":")
		if x.Label.IsDef() && !internal.IsDef(s) {
			w.string(":")
		}
		w.string(" ")
		w.node(x.Value)

	case *adt.OptionalField:
		s := w.labelString(x.Label)
		w.string(s)
		w.string("?:")
		if x.Label.IsDef() && !internal.IsDef(s) {
			w.string(":")
		}
		w.string(" ")
		w.node(x.Value)

	case *adt.BulkOptionalField:
		w.string("[")
		w.node(x.Filter)
		w.string("]: ")
		w.node(x.Value)

	case *adt.DynamicField:
		w.node(x.Key)
		if x.IsOptional() {
			w.string("?")
		}
		w.string(": ")
		w.node(x.Value)

	case *adt.Ellipsis:
		w.string("...")
		if x.Value != nil {
			w.node(x.Value)
		}

	case *adt.Bottom:
		w.string(`_|_`)

	case *adt.Null:
		w.string("null")

	case *adt.Bool:
		fmt.Fprint(w, x.B)

	case *adt.Num:
		fmt.Fprint(w, &x.X)

	case *adt.String:
		w.string(strconv.Quote(x.Str))

	case *adt.Bytes:
		b := []byte(strconv.Quote(string(x.B)))
		b[0] = '\''
		b[len(b)-1] = '\''
		w.string(string(b))

	case *adt.Top:
		w.string("_")

	case *adt.BasicType:
		fmt.Fprint(w, x.K)

	case *adt.BoundExpr:
		fmt.Fprint(w, x.Op)
		w.node(x.Expr)

	case *adt.BoundValue:
		fmt.Fprint(w, x.Op)
		w.node(x.Value)

	case *adt.FieldReference:
		w.string(openTuple)
		w.string(strconv.Itoa(int(x.UpCount)))
		w.string(";")
		w.label(x.Label)
		w.string(closeTuple)

	case *adt.LabelReference:
		w.string(openTuple)
		w.string(strconv.Itoa(int(x.UpCount)))
		w.string(";-")
		w.string(closeTuple)

	case *adt.DynamicReference:
		w.string(openTuple)
		w.string(strconv.Itoa(int(x.UpCount)))
		w.string(";(")
		w.node(x.Label)
		w.string(")")
		w.string(closeTuple)

	case *adt.ImportReference:
		w.string(openTuple + "import;")
		w.label(x.ImportPath)
		w.string(closeTuple)

	case *adt.LetReference:
		w.string(openTuple)
		w.string(strconv.Itoa(int(x.UpCount)))
		w.string(";let ")
		w.label(x.Label)
		w.string(closeTuple)

	case *adt.SelectorExpr:
		w.node(x.X)
		w.string(".")
		w.label(x.Sel)

	case *adt.IndexExpr:
		w.node(x.X)
		w.string("[")
		w.node(x.Index)
		w.string("]")

	case *adt.SliceExpr:
		w.node(x.X)
		w.string("[")
		if x.Lo != nil {
			w.node(x.Lo)
		}
		w.string(":")
		if x.Hi != nil {
			w.node(x.Hi)
		}
		if x.Stride != nil {
			w.string(":")
			w.node(x.Stride)
		}
		w.string("]")

	case *adt.Interpolation:
		w.string(`"`)
		for i := 0; i < len(x.Parts); i += 2 {
			if s, ok := x.Parts[i].(*adt.String); ok {
				w.string(s.Str)
			} else {
				w.string("<bad string>")
			}
			if i+1 < len(x.Parts) {
				w.string(`\(`)
				w.node(x.Parts[i+1])
				w.string(`)`)
			}
		}
		w.string(`"`)

	case *adt.UnaryExpr:
		fmt.Fprint(w, x.Op)
		w.node(x.X)

	case *adt.BinaryExpr:
		w.string("(")
		w.node(x.X)
		fmt.Fprint(w, " ", x.Op, " ")
		w.node(x.Y)
		w.string(")")

	case *adt.CallExpr:
		w.node(x.Fun)
		w.string("(")
		for i, a := range x.Args {
			if i > 0 {
				w.string(", ")
			}
			w.node(a)
		}
		w.string(")")

	case *adt.BuiltinValidator:
		w.node(x.Fun)
		w.string("(")
		for i, a := range x.Args {
			if i > 0 {
				w.string(", ")
			}
			w.node(a)
		}
		w.string(")")

	case *adt.Disjunction:
		w.string("(")
		for i, a := range x.Values {
			if i > 0 {
				w.string("|")
			}
			// Disjunct
			if a.Default {
				w.string("*")
			}
			w.node(a.Val)
		}
		w.string(")")

	case *adt.Conjunction:
		w.string("&(")
		for i, c := range x.Values {
			if i > 0 {
				w.string(", ")
			}
			w.node(c)
		}
		w.string(")")

	case *adt.ForClause:
		w.string("for ")
		w.label(x.Key)
		w.string(", ")
		w.label(x.Value)
		w.string(" in ")
		w.node(x.Src)
		w.string(" ")
		w.node(x.Dst)

	case *adt.IfClause:
		w.string("if ")
		w.node(x.Condition)
		w.string(" ")
		w.node(x.Dst)

	case *adt.LetClause:
		w.string("let ")
		w.label(x.Label)
		w.string(" = ")
		w.node(x.Expr)
		w.string(" ")
		w.node(x.Dst)

	case *adt.ValueClause:
		w.node(x.StructLit)

	default:
		panic(fmt.Sprintf("unknown type %T", x))
	}
}
