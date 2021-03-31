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

package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal"
)

// This file contains logic for placing orphan files within a CUE namespace.

func (b *buildPlan) usePlacement() bool {
	return b.importing || b.perFile || b.useList || len(b.path) > 0
}

func (b *buildPlan) parsePlacementFlags() error {
	cmd := b.cmd
	b.perFile = flagFiles.Bool(cmd)
	b.useList = flagList.Bool(cmd)
	b.path = flagPath.StringArray(cmd)
	b.useContext = flagWithContext.Bool(cmd)

	if !b.importing && !b.perFile && !b.useList && len(b.path) == 0 {
		if b.useContext {
			return fmt.Errorf(
				"flag %q must be used with at least one of flag %q, %q, or %q",
				flagWithContext, flagPath, flagList, flagFiles,
			)
		}
	} else if b.schema != nil {
		return fmt.Errorf(
			"cannot combine -schema flag with flag %q, %q, or %q",
			flagPath, flagList, flagFiles,
		)
	}
	return nil
}

func (b *buildPlan) placeOrphans(i *build.Instance, a []decoderInfo) error {
	pkg := b.encConfig.PkgName
	if pkg == "" {
		pkg = i.PkgName
	} else if pkg != "" && i.PkgName != "" && i.PkgName != pkg && !flagForce.Bool(b.cmd) {
		return fmt.Errorf(
			"%q flag clashes with existing package name (%s vs %s)",
			flagPackage, pkg, i.PkgName,
		)
	}

	var files []*ast.File

	re, err := regexp.Compile(b.cfg.fileFilter)
	if err != nil {
		return err
	}

	for _, di := range a {
		if !i.User && !re.MatchString(filepath.Base(di.file.Filename)) {
			continue
		}

		d := di.dec

		var objs []*ast.File

		// Filter only need to filter files that can stream:
		for ; !d.Done(); d.Next() {
			if f := d.File(); f != nil {
				f.Filename = newName(d.Filename(), 0)
				objs = append(objs, f)
			}
		}
		if err := d.Err(); err != nil {
			return err
		}

		if b.perFile {
			for i, obj := range objs {
				f, err := placeOrphans(b.cmd, d.Filename(), pkg, obj)
				if err != nil {
					return err
				}
				f.Filename = newName(d.Filename(), i)
				files = append(files, f)
			}
			continue
		}
		// TODO: consider getting rid of this requirement. It is important that
		// import will catch conflicts ahead of time then, though, and report
		// this messages as a possible solution if there are conflicts.
		if b.importing && len(objs) > 1 && len(b.path) == 0 && !b.useList {
			return fmt.Errorf(
				"%s, %s, or %s flag needed to handle multiple objects in file %s",
				flagPath, flagList, flagFiles, shortFile(i.Root, di.file))
		}

		if !b.useList && len(b.path) == 0 && !b.useContext {
			for _, f := range objs {
				if pkg := b.encConfig.PkgName; pkg != "" {
					internal.SetPackage(f, pkg, false)
				}
				files = append(files, f)
			}
		} else {
			// TODO: handle imports correctly, i.e. for proto.
			f, err := placeOrphans(b.cmd, d.Filename(), pkg, objs...)
			if err != nil {
				return err
			}
			f.Filename = newName(d.Filename(), 0)
			files = append(files, f)
		}
	}

	b.imported = append(b.imported, files...)
	for _, f := range files {
		if err := i.AddSyntax(f); err != nil {
			return err
		}
	}
	return nil
}

func placeOrphans(cmd *Command, filename, pkg string, objs ...*ast.File) (*ast.File, error) {
	f := &ast.File{}

	index := newIndex()
	for i, file := range objs {
		if i == 0 {
			astutil.CopyMeta(f, file)
		}
		expr := internal.ToExpr(file)
		p, _, _ := internal.PackageInfo(file)

		var pathElems []ast.Label
		var pathTokens []token.Token

		switch {
		case len(flagPath.StringArray(cmd)) > 0:
			expr := expr
			if flagWithContext.Bool(cmd) {
				expr = ast.NewStruct(
					"data", expr,
					"filename", ast.NewString(filename),
					"index", ast.NewLit(token.INT, strconv.Itoa(i)),
					"recordCount", ast.NewLit(token.INT, strconv.Itoa(len(objs))),
				)
			}
			var f *ast.File
			if s, ok := expr.(*ast.StructLit); ok {
				f = &ast.File{Decls: s.Elts}
			} else {
				f = &ast.File{Decls: []ast.Decl{&ast.EmbedDecl{Expr: expr}}}
			}
			err := astutil.Sanitize(f)
			if err != nil {
				return nil, errors.Wrapf(err, token.NoPos,
					"invalid combination of input files")
			}
			inst, err := runtime.CompileFile(f)
			if err != nil {
				return nil, err
			}

			for _, str := range flagPath.StringArray(cmd) {
				l, err := parser.ParseExpr("--path", str)
				if err != nil {
					labels, tokens, err := parseFullPath(inst, str)
					if err != nil {
						return nil, fmt.Errorf(
							`labels must be expressions (-l foo -l 'strings.ToLower(bar)') or full paths (-l '"foo": "\(strings.ToLower(bar))":) : %v`, err)
					}
					pathElems = append(pathElems, labels...)
					pathTokens = append(pathTokens, tokens...)
					continue
				}

				str, err := inst.Eval(l).String()
				if err != nil {
					return nil, fmt.Errorf("unsupported label path type: %v", err)
				}
				pathElems = append(pathElems, ast.NewString(str))
				pathTokens = append(pathTokens, 0)
			}
		}

		if flagList.Bool(cmd) {
			idx := index
			for _, e := range pathElems {
				idx = idx.label(e)
			}
			if idx.field.Value == nil {
				idx.field.Value = &ast.ListLit{
					Lbrack: token.NoSpace.Pos(),
					Rbrack: token.NoSpace.Pos(),
				}
			}
			list := idx.field.Value.(*ast.ListLit)
			list.Elts = append(list.Elts, expr)
		} else if len(pathElems) == 0 {
			obj, ok := expr.(*ast.StructLit)
			if !ok {
				if _, ok := expr.(*ast.ListLit); ok {
					return nil, fmt.Errorf("expected struct as object root, did you mean to use the --list flag?")
				}
				return nil, fmt.Errorf("cannot map non-struct to object root")
			}
			f.Decls = append(f.Decls, obj.Elts...)
		} else {
			field := &ast.Field{Label: pathElems[0]}
			field.Token = pathTokens[0]
			f.Decls = append(f.Decls, field)
			if p != nil {
				astutil.CopyComments(field, p)
			}
			for i, e := range pathElems[1:] {
				newField := &ast.Field{Label: e}
				newVal := ast.NewStruct(newField)
				newField.Token = pathTokens[i+1]
				field.Value = newVal
				field = newField
			}
			field.Value = expr
		}
	}

	if pkg != "" {
		internal.SetPackage(f, pkg, false)
	}

	if flagList.Bool(cmd) {
		switch x := index.field.Value.(type) {
		case *ast.StructLit:
			f.Decls = append(f.Decls, x.Elts...)
		case *ast.ListLit:
			f.Decls = append(f.Decls, &ast.EmbedDecl{Expr: x})
		default:
			panic("unreachable")
		}
	}

	return f, astutil.Sanitize(f)
}

func parseFullPath(inst *cue.Instance, exprs string) (p []ast.Label, t []token.Token, err error) {
	f, err := parser.ParseFile("--path", exprs+"_")
	if err != nil {
		return nil, nil, fmt.Errorf("parser error in path %q: %v", exprs, err)
	}

	if len(f.Decls) != 1 {
		return nil, nil, errors.New("path flag must be a space-separated sequence of labels")
	}

	for d := f.Decls[0]; ; {
		field, ok := d.(*ast.Field)
		if !ok {
			// This should never happen
			return nil, nil, errors.New("%q not a sequence of labels")
		}

		t = append(t, field.Token)

		switch x := field.Label.(type) {
		case *ast.Ident, *ast.BasicLit:
			p = append(p, x)

		case *ast.TemplateLabel:
			return nil, nil, fmt.Errorf("template labels not supported in path flag")

		case ast.Expr:
			v := inst.Eval(x)
			if v.Kind() == cue.BottomKind {
				return nil, nil, v.Err()
			}
			p = append(p, v.Syntax().(ast.Label))

		}

		v, ok := field.Value.(*ast.StructLit)
		if !ok {
			break
		}

		if len(v.Elts) != 1 {
			return nil, nil, errors.New("path value may not contain a struct")
		}

		d = v.Elts[0]
	}
	return p, t, nil
}

type listIndex struct {
	index map[string]*listIndex
	field *ast.Field
}

func newIndex() *listIndex {
	return &listIndex{
		index: map[string]*listIndex{},
		field: &ast.Field{},
	}
}

func (x *listIndex) label(label ast.Label) *listIndex {
	key := internal.DebugStr(label)
	idx := x.index[key]
	if idx == nil {
		if x.field.Value == nil {
			x.field.Value = &ast.StructLit{}
		}
		obj := x.field.Value.(*ast.StructLit)
		newField := &ast.Field{Label: label}
		obj.Elts = append(obj.Elts, newField)
		idx = &listIndex{
			index: map[string]*listIndex{},
			field: newField,
		}
		x.index[key] = idx
	}
	return idx
}

func newName(filename string, i int) string {
	if filename == "-" {
		return filename
	}
	ext := filepath.Ext(filename)
	filename = filename[:len(filename)-len(ext)]
	if i > 0 {
		filename += fmt.Sprintf("-%d", i)
	}
	filename += ".cue"
	return filename
}
