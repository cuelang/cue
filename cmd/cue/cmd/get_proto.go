// Copyright 2019 The CUE Authors
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

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"github.com/emicklei/proto"
	"github.com/spf13/cobra"
)

var getProtoCmd = &cobra.Command{
	// TODO: this command is still experimental, don't show it in
	// the documentation just yet.
	Hidden: true,

	Use:   "proto",
	Short: "convert proto definitions to CUE",
	Long: `
`,
	RunE: runGetProto,
}

func init() {
	getCmd.AddCommand(getProtoCmd)
}

func runGetProto(cmd *cobra.Command, args []string) (err error) {
	// Determine files to convert.

	r, err := os.Open("cmd/gateway.proto")
	// r, err := os.Open("test.proto")
	if err != nil {
		return err
	}
	defer r.Close()

	parser := proto.NewParser(r)
	parser.Filename("cmd/gateway.proto")
	p, err := parser.Parse()
	if err != nil {
		return err
	}

	defer func() {
		switch x := recover().(type) {
		case string:
			// determine whether it is our own.
			err = errors.New(x)
		case error:
			err = x
		}
	}()

	// pretty.Println(p)
	c := protoConverter{}
	for _, e := range p.Elements {
		c.topElement(e)
	}
	file := &ast.File{Decls: c.decl}
	if c.pkg != nil {
		file.Name = c.pkg
	}

	w, err := os.Create("cmd/gateway_proto_gen.cue")
	if err != nil {
		return err
	}
	defer w.Close()
	return format.Node(w, file)
}

// A protoConverter converts a proto definition to CUE. Proto files map to
// CUE files one to one.
type protoConverter struct {
	path []string // length 0 means top-level

	proto3 bool

	// w bytes.Buffer

	extraComments []*proto.Comment

	fileComment *ast.CommentGroup
	pkgComment  *ast.CommentGroup

	pkg  *ast.Ident
	decl []ast.Decl
}

func (p *protoConverter) stringLit(s string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(s)}
}

func (p *protoConverter) ref() *ast.Ident {
	return ast.NewIdent(strings.Join(p.path, "_"))
}

func (p *protoConverter) subref(name string) *ast.Ident {
	return ast.NewIdent(strings.Join(append(p.path, name), "_"))
}

func (p *protoConverter) addTag(f *ast.Field, body string) {
	tag := "@protobuf(" + body + ")"
	f.Attrs = append(f.Attrs, &ast.Attribute{Text: tag})
}

func (p *protoConverter) topElement(v proto.Visitee) {
	switch x := v.(type) {
	case *proto.Syntax:
	case *proto.Comment:
		p.extraComments = append(p.extraComments, x)

	case *proto.Enum:
		// p.enum()

	case *proto.Package:

	case *proto.Option:
		if x.Name == "go_package" {
			str, err := strconv.Unquote(x.Constant.SourceRepresentation())
			if err != nil {
				panic(err)
			}
			name := ast.NewIdent(path.Base(str))
			// name.AddComment(comment(x.Comment, true))
			// name.AddComment(comment(x.InlineComment, false))
			p.pkg = name
		}

	case *proto.Message:
		p.message(x)

	default:
		panic(fmt.Sprintf("unsupported type %T", x))
	}
}

func (p *protoConverter) message(v *proto.Message) ast.Decl {
	defer func(saved []string) { p.path = saved }(p.path)
	p.path = append(p.path, v.Name)

	// TODO: handle IsExtend/ proto2

	s := &ast.StructLit{
		// TOOD: set proto file position.

	}
	ref := p.ref()
	if v.Comment == nil {
		ref.NamePos = newSection
	}
	f := &ast.Field{Label: ref, Value: s}
	addComments(f, 1, v.Comment, nil)

	// In CUE a message is always defined at the top level.
	p.decl = append(p.decl, f)

	for i, e := range v.Elements {
		p.messageField(s, i, e)
	}

	// The returned alias is ignored at the top-level and inserted in
	// an enclosing Struct for easy referencing.
	alias := &ast.Alias{Ident: ast.NewIdent(v.Name), Expr: p.ref()}
	alias.Ident.NamePos = newSection
	return alias
}

func (p *protoConverter) messageField(s *ast.StructLit, i int, v proto.Visitee) {
	switch x := v.(type) {
	case *proto.Comment:
		// p.extraComments = append(p.extraComments, x)

	case *proto.NormalField:
		f := &ast.Field{}
		addComments(f, i, x.Comment, x.InlineComment)

		name := labelName(x.Name)
		f.Label = ast.NewIdent(name)
		typ := protoToCUE(x.Type)
		f.Value = ast.NewIdent(typ)
		s.Elts = append(s.Elts, f)

		o := optionParser{message: s, field: f}

		// body of @protobuf tag: sequence[,type][,name=<name>][,...]
		o.tags += fmt.Sprint(x.Sequence)
		if x.Type != typ {
			o.tags += ",type=" + x.Type
		}
		if x.Name != name {
			o.tags += ",name=" + x.Name
		}
		o.parse(x.Options)
		p.addTag(f, o.tags)

		if !o.required {
			f.Optional = token.Pos(token.NoSpace)
		}

		if x.Repeated {
			f.Value = &ast.ListLit{
				Ellipsis: token.Pos(token.NoSpace),
				Type:     f.Value,
			}
		}

	case *proto.MapField:
		f := &ast.Field{}

		// All keys are converted to strings.
		// TODO: support integer keys.
		f.Label = &ast.TemplateLabel{Ident: ast.NewIdent("_")}
		f.Value = ast.NewIdent(protoToCUE(x.Type))

		name := labelName(x.Name)
		f = &ast.Field{
			Label: ast.NewIdent(name),
			Value: &ast.StructLit{Elts: []ast.Decl{f}},
		}
		addComments(f, i, x.Comment, x.InlineComment)

		o := optionParser{message: s, field: f}
		o.tags = fmt.Sprintf("%d,type=map<%s,%s>", x.Sequence, x.KeyType, x.Type)
		if x.Name != name {
			o.tags += "," + x.Name
		}
		o.parse(x.Options)
		p.addTag(f, o.tags)
		s.Elts = append(s.Elts, f)

	case *proto.Enum:
		s.Elts = append(s.Elts, p.enum(s, x))

	case *proto.Message:
		s.Elts = append(s.Elts, p.message(x))

	case *proto.Oneof:
		p.oneOf(x)

	default:
		panic("unsupported type")
	}
}

// enum converts a proto enum definition to CUE.
//
// An enum will generate two top-level definitions:
//
//    Enum:
//      "Value1" |
//      "Value2" |
//      "Value3"
//
// and
//
//    Enum_value: {
//        "Value1": 0
//        "Value2": 1
//    }
//
// Enums are always defined at the top level. The name of a nested enum
// will be prefixed with the name of its parent and an underscore.
func (p *protoConverter) enum(s *ast.StructLit, x *proto.Enum) ast.Decl {
	if len(x.Elements) == 0 {
		panic("empty enum")
	}

	name := p.subref(x.Name)

	if len(p.path) == 0 {
		defer func() { p.path = p.path[:0] }()
		p.path = append(p.path, x.Name)
	}

	// Top-level enum entry.
	enum := &ast.Field{Label: name}
	addComments(enum, 1, x.Comment, nil)

	// Top-level enum values entry.
	valueName := ast.NewIdent(name.Name + "_value")
	valueName.NamePos = newSection
	valueMap := &ast.StructLit{}
	d := &ast.Field{Label: valueName, Value: valueMap}
	// addComments(valueMap, 1, x.Comment, nil)

	p.decl = append(p.decl, enum, d)

	// The line comments for an enum field need to attach after the '|', which
	// is only known at the next iteration.
	var lastComment *proto.Comment
	for i, v := range x.Elements {
		switch y := v.(type) {
		case *proto.EnumField:
			// Add enum value to map
			f := &ast.Field{
				Label: p.stringLit(y.Name),
				Value: &ast.BasicLit{Value: strconv.Itoa(y.Integer)},
			}
			valueMap.Elts = append(valueMap.Elts, f)

			// add to enum disjunction
			value := p.stringLit(y.Name)

			var e ast.Expr = value
			// Make the first value the default value.
			if i == 0 {
				e = &ast.UnaryExpr{OpPos: newline, Op: token.MUL, X: value}
			} else {
				value.ValuePos = newline
			}
			addComments(e, i, y.Comment, nil)
			if enum.Value != nil {
				e = &ast.BinaryExpr{X: enum.Value, Op: token.OR, Y: e}
				if cg := comment(lastComment, false); cg != nil {
					cg.Position = 2
					e.AddComment(cg)
				}
			}
			enum.Value = e

			if y.Comment != nil {
				lastComment = nil
				addComments(f, 0, nil, y.InlineComment)
			} else {
				lastComment = y.InlineComment
			}

			// a := fmt.Sprintf("@protobuf(enum,name=%s)", y.Name)
			// f.Attrs = append(f.Attrs, &ast.Attribute{Text: a})
		}
	}
	addComments(enum.Value, 1, nil, lastComment)

	// The returned alias is ignored at the top-level and inserted in
	// an enclosing Struct for easy referencing.
	// Make the enum available for reference within the object as an alias.
	alias := &ast.Alias{Ident: ast.NewIdent(x.Name), Expr: name}
	alias.Ident.NamePos = newSection
	return alias
}

func (p *protoConverter) oneOf(x *proto.Oneof) {
	f := &ast.Field{
		Label: p.ref(),
	}
	f.AddComment(comment(x.Comment, true))

	p.decl = append(p.decl, f)

	for _, v := range x.Elements {
		s := &ast.StructLit{}
		p.messageField(s, 1, v)
		var e ast.Expr = s
		if f.Value != nil {
			e = &ast.BinaryExpr{}
		}
		f.Value = e
	}
}

type optionParser struct {
	message  *ast.StructLit
	field    *ast.Field
	required bool
	tags     string
}

func (p *optionParser) parse(options []*proto.Option) {

	// TODO: handle options
	// - translate options to tags
	// - interpret CUE options.
	for _, o := range options {
		switch o.Name {
		case "(cue).req":
			p.required = true
			// TODO: Dropping comments. Maybe add a dummy tag?

		case "cue", "(cue)":
			// TODO: set filename and base offset.
			fset := token.NewFileSet()
			expr, err := parser.ParseExpr(fset, "", o.Constant.Source)
			if err != nil {
				panic(err)
			}
			// Any further checks will be done at the end.
			constraint := &ast.Field{Label: p.field.Label, Value: expr}
			addComments(constraint, 1, o.Comment, o.InlineComment)
			p.message.Elts = append(p.message.Elts, constraint)
			if !p.required {
				constraint.Optional = token.Pos(token.NoSpace)
			}

		default:
			// TODO: dropping comments. Maybe add dummy tag?

			// TODO: should CUE support nested attributes?
			source := o.Constant.SourceRepresentation()
			p.tags += "," + quote("option("+o.Name+","+source+")")
		}
	}
}

func labelName(s string) string {
	split := strings.Split(s, "_")
	for i := 1; i < len(split); i++ {
		split[i] = strings.Title(split[i])
	}
	return strings.Join(split, "")
}

func protoToCUE(typ string) string {
	if t, ok := scalars[typ]; ok {
		return t
	}
	return typ
}

var scalars = map[string]string{
	// Differing
	"sint32":   "int32",
	"sint64":   "int64",
	"fixed32":  "uint32",
	"fixed64":  "uint64",
	"sfixed32": "int32",
	"sfixed64": "int64",

	// Identical to CUE
	"int32":  "int32",
	"int64":  "int64",
	"uint32": "uint32",
	"uint64": "uint64",

	"double": "number", // float64?
	"float":  "number", // float32?

	"bool":   "bool",
	"string": "string",
	"bytes":  "bytes",

	"google.protobuf.Any": "_",
}

var (
	newline    = token.Pos(token.Newline)
	newSection = token.Pos(token.NewSection)
)

func addComments(f ast.Node, i int, doc, inline *proto.Comment) bool {
	cg := comment(doc, true)
	if cg != nil && len(cg.List) > 0 && i > 0 {
		cg.List[0].Slash = newSection
	}
	f.AddComment(cg)
	f.AddComment(comment(inline, false))
	return doc != nil
}

func comment(c *proto.Comment, doc bool) *ast.CommentGroup {
	if c == nil || len(c.Lines) == 0 {
		return nil
	}
	cg := &ast.CommentGroup{}
	if doc {
		cg.Doc = true
	} else {
		cg.Line = true
		cg.Position = 10
	}
	for _, s := range c.Lines {
		cg.List = append(cg.List, &ast.Comment{Text: "// " + s})
	}
	return cg
}

func quote(s string) string {
	if !strings.ContainsAny(s, `"\`) {
		return s
	}
	esc := `\#`
	for strings.Contains(s, esc) {
		esc += "#"
	}
	return esc[1:] + `"` + s + `"` + esc[1:]
}
