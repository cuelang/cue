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

package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/encoding"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/literal"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/encoding/protobuf"
	"cuelang.org/go/internal"
	"cuelang.org/go/internal/third_party/yaml"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func newImportCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "convert other data formats to CUE files",
		Long: `import converts other data formats, like JSON and YAML to CUE files

The following file formats are currently supported:

  Format       Extensions
	JSON       .json .jsonl .ndjson
	YAML       .yaml .yml
	protobuf   .proto

Files can either be specified explicitly, or inferred from the specified
packages. In either case, the file extension is replaced with .cue. It will
fail if the file already exists by default. The -f flag overrides this.

Examples:

  # Convert individual files:
  $ cue import foo.json bar.json  # create foo.yaml and bar.yaml

  # Convert all json files in the indicated directories:
  $ cue import ./... -type=json


The -path flag

By default the parsed files are included as emit values. This default can be
overridden by specifying a sequence of labels as you would in a CUE file.
An identifier or string label are interpreted as usual. A label expression is
evaluated within the context of the imported file. label expressions may also
refer to builtin packages, which will be implicitly imported.


Handling multiple documents or streams

To handle Multi-document files, such as concatenated JSON objects or
YAML files with document separators (---) the user must specify either the
-path, -list, or -files flag. The -path flag assign each element to a path
(identical paths are treated as usual); -list concatenates the entries, and
-files causes each entry to be written to a different file. The -files flag
may only be used if files are explicitly imported. The -list flag may be
used in combination with the -path flag, concatenating each entry to the
mapped location.


Examples:

  $ cat <<EOF > foo.yaml
  kind: Service
  name: booster
  EOF

  # include the parsed file as an emit value:
  $ cue import foo.yaml
  $ cat foo.cue
  {
      kind: Service
      name: booster
  }

  # include the parsed file at the root of the CUE file:
  $ cue import -f -l "" foo.yaml
  $ cat foo.cue
  kind: Service
  name: booster

  # include the import config at the mystuff path
  $ cue import -f -l mystuff foo.yaml
  $ cat foo.cue
  myStuff: {
      kind: Service
      name: booster
  }

  # append another object to the input file
  $ cat <<EOF >> foo.yaml
  ---
  kind: Deployment
  name: booster
  replicas: 1

  # base the path values on th input
  $ cue import -f -l '"\(strings.ToLower(kind))" "\(x.name)"' foo.yaml
  $ cat foo.cue
  service booster: {
      kind: "Service"
      name: "booster"
  }

  deployment booster: {
      kind:     "Deployment"
      name:     "booster
      replicas: 1
  }

  # base the path values on th input
  $ cue import -f -list -foo.yaml
  $ cat foo.cue
  [{
      kind: "Service"
      name: "booster"
  }, {
      kind:     "Deployment"
      name:     "booster
      replicas: 1
  }]

  # base the path values on th input
  $ cue import -f -list -l '"\(strings.ToLower(kind))"' foo.yaml
  $ cat foo.cue
  service: [{
      kind: "Service"
      name: "booster"
  }
  deployment: [{
      kind:     "Deployment"
      name:     "booster
      replicas: 1
  }]


Embedded data files

The --recursive or -R flag enables the parsing of fields that are string
representations of data formats themselves. A field that can be parsed is
replaced with a call encoding the data from a structured form that is placed
in a sibling field.

It is also possible to recursively hoist data formats:

Example:
  $ cat <<EOF > example.json
  "a": {
      "data": '{ "foo": 1, "bar": 2 }',
  }
  EOF

  $ cue import -R example.json
  $ cat example.cue
  import "encode/json"

  a: {
      data: json.Encode(_data),
      _data: {
          foo: 1
          bar: 2
      }
  }
`,
		RunE: mkRunE(c, runImport),
	}

	flagOut.Add(cmd)
	cmd.Flags().StringP(string(flagGlob), "n", "", "glob filter for file names")
	cmd.Flags().String(string(flagType), "", "only apply to files of this type")
	cmd.Flags().BoolP(string(flagForce), "f", false, "force overwriting existing files")
	cmd.Flags().Bool(string(flagDryrun), false, "only run simulation")

	cmd.Flags().StringP(string(flagPath), "l", "", "path to include root")
	cmd.Flags().Bool(string(flagList), false, "concatenate multiple objects into a list")
	cmd.Flags().Bool(string(flagFiles), false, "split multiple entries into different files")
	cmd.Flags().BoolP(string(flagRecursive), "R", false, "recursively parse string values")

	cmd.Flags().String("fix", "", "apply given fix")

	cmd.Flags().StringArrayP(string(flagProtoPath), "I", nil, "paths in which to search for imports")

	return cmd
}

const (
	flagFiles     flagName = "files"
	flagProtoPath flagName = "proto_path"
)

type importStreamFunc func(path string, r io.Reader) ([]ast.Expr, error)
type importFileFunc func(cmd *Command, path string, r io.Reader) (*ast.File, error)

type encodingInfo struct {
	fnStream importStreamFunc
	fnFile   importFileFunc
	typ      string
}

var (
	jsonEnc     = &encodingInfo{fnStream: handleJSON, typ: "json"}
	yamlEnc     = &encodingInfo{fnStream: handleYAML, typ: "yaml"}
	protodefEnc = &encodingInfo{fnFile: handleProtoDef, typ: "proto"}
)

func getExtInfo(ext string) *encodingInfo {
	enc := encoding.MapExtension(ext)
	if enc == nil {
		return nil
	}
	switch enc.Name() {
	case "json":
		return jsonEnc
	case "yaml":
		return yamlEnc
	case "protobuf":
		return protodefEnc
	}
	return nil
}

func runImport(cmd *Command, args []string) error {
	var group errgroup.Group

	pkgFlag := flagPackage.String(cmd)

	group.Go(func() (err error) {
		if len(args) > 0 && len(filepath.Ext(args[0])) > len(".") {
			for _, a := range args {
				group.Go(func() error { return handleFile(cmd, pkgFlag, a) })
			}
			return nil
		}

		done := map[string]bool{}

		inst := load.Instances(args, &load.Config{DataFiles: true})
		for _, pkg := range inst {
			pkgName := pkgFlag
			if pkgName == "" {
				pkgName = pkg.PkgName
			}
			if pkgName == "" && len(inst) > 1 {
				return fmt.Errorf("must specify package name with the -p flag")
			}
			dir := pkg.Dir
			if err := pkg.Err; err != nil {
				return err
			}
			if done[dir] {
				continue
			}
			done[dir] = true

			files, err := ioutil.ReadDir(dir)
			if err != nil {
				return err
			}
			for _, file := range files {
				ext := filepath.Ext(file.Name())
				typ := flagType.String(cmd)
				if enc := getExtInfo(ext); enc == nil || (typ != "" && typ != enc.typ) {
					continue
				}
				path := filepath.Join(dir, file.Name())
				group.Go(func() error { return handleFile(cmd, pkgName, path) })
			}
		}
		return nil
	})

	err := group.Wait()
	exitOnErr(cmd, err, true)
	return nil
}

func handleFile(cmd *Command, pkg, filename string) (err error) {
	re, err := regexp.Compile(flagGlob.String(cmd))
	if err != nil {
		return err
	}
	if !re.MatchString(filepath.Base(filename)) {
		return nil
	}
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer f.Close()

	ext := filepath.Ext(filename)
	handler := getExtInfo(ext)

	switch {
	case handler == nil:
		return fmt.Errorf("unsupported extension %q", ext)

	case handler.fnFile != nil:
		file, err := handler.fnFile(cmd, filename, f)
		if err != nil {
			return err
		}
		file.Filename = filename
		return processFile(cmd, file)

	case handler.fnStream != nil:
		objs, err := handler.fnStream(filename, f)
		if err != nil {
			return err
		}
		return processStream(cmd, pkg, filename, objs)

	default:
		panic("incorrect handler")
	}
}

func processFile(cmd *Command, file *ast.File) (err error) {
	name := file.Filename + ".cue"

	b, err := format.Node(file)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(name, b, 0644)
}

func processStream(cmd *Command, pkg, filename string, objs []ast.Expr) error {
	if flagFiles.Bool(cmd) {
		for i, f := range objs {
			err := combineExpressions(cmd, pkg, newName(filename, i), f)
			if err != nil {
				return err
			}
		}
		return nil
	} else if len(objs) > 1 {
		if !flagList.Bool(cmd) && flagPath.String(cmd) == "" && !flagFiles.Bool(cmd) {
			return fmt.Errorf("list, flag, or files flag needed to handle multiple objects in file %q", filename)
		}
	}
	return combineExpressions(cmd, pkg, newName(filename, 0), objs...)
}

// TODO: implement a more fine-grained approach.
var mutex sync.Mutex

func combineExpressions(cmd *Command, pkg, cueFile string, objs ...ast.Expr) error {
	mutex.Lock()
	defer mutex.Unlock()

	if out := flagOut.String(cmd); out != "" {
		cueFile = out
	}
	if cueFile != "-" {
		switch _, err := os.Stat(cueFile); {
		case os.IsNotExist(err):
		case err == nil:
			if !flagForce.Bool(cmd) {
				// TODO: mimic old behavior: write to stderr, but do not exit
				// with error code. Consider what is best to do here.
				stderr := cmd.Command.OutOrStderr()
				fmt.Fprintf(stderr, "skipping file %q: already exists\n", cueFile)
				return nil
			}
		default:
			return fmt.Errorf("error creating file: %v", err)
		}
	}

	f := &ast.File{}

	h := hoister{
		fields:   map[string]bool{},
		altNames: map[string]*ast.Ident{},
	}

	index := newIndex()
	for _, expr := range objs {
		if flagRecursive.Bool(cmd) {
			h.hoist(expr)
		}

		// Compute a path different from root.
		var pathElems []ast.Label

		switch {
		case flagPath.String(cmd) != "":
			inst, err := runtime.CompileExpr(expr)
			if err != nil {
				return err
			}

			labels, err := parsePath(flagPath.String(cmd))
			if err != nil {
				return err
			}
			for _, l := range labels {
				switch x := l.(type) {
				case *ast.Interpolation:
					v := inst.Eval(x)
					if v.Kind() == cue.BottomKind {
						return v.Err()
					}
					pathElems = append(pathElems, v.Syntax().(ast.Label))

				case *ast.Ident, *ast.BasicLit:
					pathElems = append(pathElems, x)

				case *ast.TemplateLabel:
					return fmt.Errorf("template labels not supported in path flag")
				}
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
				return fmt.Errorf("cannot map non-struct to object root")
			}
			f.Decls = append(f.Decls, obj.Elts...)
		} else {
			field := &ast.Field{Label: pathElems[0]}
			f.Decls = append(f.Decls, field)
			for _, e := range pathElems[1:] {
				newField := &ast.Field{Label: e}
				newVal := &ast.StructLit{Elts: []ast.Decl{newField}}
				field.Value = newVal
				field = newField
			}
			field.Value = expr
		}
	}

	if len(h.altNames) > 0 {
		imports := &ast.ImportDecl{}

		for _, enc := range encoding.All() {
			if ident, ok := h.altNames[enc.Name()]; ok {
				short := enc.Name()
				name := h.uniqueName(short, "", "")
				ident.Name = name
				if name == short {
					ident = nil
				}

				path := fmt.Sprintf(`"encoding/%s"`, short)
				imports.Specs = append(imports.Specs, &ast.ImportSpec{
					Name: ident,
					Path: &ast.BasicLit{Kind: token.STRING, Value: path},
				})
			}
		}
		f.Decls = append([]ast.Decl{imports}, f.Decls...)
	}

	if pkg != "" {
		p := &ast.Package{Name: ast.NewIdent(pkg)}
		f.Decls = append([]ast.Decl{p}, f.Decls...)
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

	b, err := format.Node(f, format.Simplify())
	if err != nil {
		return fmt.Errorf("error formatting file: %v", err)
	}

	if cueFile == "-" {
		_, err := cmd.OutOrStdout().Write(b)
		return err
	}
	return ioutil.WriteFile(cueFile, b, 0644)
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

func parsePath(exprs string) (p []ast.Label, err error) {
	f, err := parser.ParseFile("<path flag>", exprs+": _")
	if err != nil {
		return nil, fmt.Errorf("parser error in path %q: %v", exprs, err)
	}

	if len(f.Decls) != 1 {
		return nil, errors.New("path flag must be a space-separated sequence of labels")
	}

	for d := f.Decls[0]; ; {
		field, ok := d.(*ast.Field)
		if !ok {
			// This should never happen
			return nil, errors.New("%q not a sequence of labels")
		}

		p = append(p, field.Label)

		v, ok := field.Value.(*ast.StructLit)
		if !ok {
			break
		}

		if len(v.Elts) != 1 {
			// This should never happen
			return nil, errors.New("path value may not contain a struct")
		}

		d = v.Elts[0]
	}
	return p, nil
}

func newName(filename string, i int) string {
	ext := filepath.Ext(filename)
	filename = filename[:len(filename)-len(ext)]
	if i > 0 {
		filename += fmt.Sprintf("-%d", i)
	}
	filename += ".cue"
	return filename
}

func handleJSON(path string, r io.Reader) (objects []ast.Expr, err error) {
	d := json.NewDecoder(r)

	for {
		var raw json.RawMessage
		err := d.Decode(&raw)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("could not parse JSON: %v", err)
		}
		expr, err := parser.ParseExpr(path, []byte(raw))
		if err != nil {
			return nil, fmt.Errorf("invalid input: %v %q", err, raw)
		}
		objects = append(objects, expr)
	}
	return objects, nil
}

func handleYAML(path string, r io.Reader) (objects []ast.Expr, err error) {
	d, err := yaml.NewDecoder(path, r)
	if err != nil {
		return nil, err
	}
	for i := 0; ; i++ {
		expr, err := d.Decode()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		objects = append(objects, expr)
	}
	return objects, nil
}

func handleProtoDef(cmd *Command, path string, r io.Reader) (f *ast.File, err error) {
	return protobuf.Extract(path, r, &protobuf.Config{Paths: flagProtoPath.StringArray(cmd)})
}

type hoister struct {
	fields   map[string]bool
	altNames map[string]*ast.Ident
}

func (h *hoister) hoist(expr ast.Expr) {
	ast.Walk(expr, nil, func(n ast.Node) {
		name := ""
		switch x := n.(type) {
		case *ast.Field:
			name, _ = internal.LabelName(x.Label)
		case *ast.Alias:
			name = x.Ident.Name
		}
		if name != "" {
			h.fields[name] = true
		}
	})

	ast.Walk(expr, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.Comprehension:
			return false
		}
		return true

	}, func(n ast.Node) {
		obj, ok := n.(*ast.StructLit)
		if !ok {
			return
		}
		for i := 0; i < len(obj.Elts); i++ {
			f, ok := obj.Elts[i].(*ast.Field)
			if !ok {
				continue
			}

			name, ident := internal.LabelName(f.Label)
			if name == "" || !ident {
				continue
			}

			lit, ok := f.Value.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}

			str, err := literal.Unquote(lit.Value)
			if err != nil {
				continue
			}

			expr, enc := tryParse(str)
			if expr == nil {
				continue
			}

			if h.altNames[enc.typ] == nil {
				h.altNames[enc.typ] = &ast.Ident{Name: "_cue"} // set name later
			}

			// found a replacable string
			dataField := h.uniqueName(name, "_", "cue_")

			f.Value = &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   h.altNames[enc.typ],
					Sel: ast.NewIdent("Marshal"),
				},
				Args: []ast.Expr{
					ast.NewIdent(dataField),
				},
			}

			obj.Elts = append(obj.Elts, nil)
			copy(obj.Elts[i+1:], obj.Elts[i:])

			obj.Elts[i+1] = &ast.Alias{
				Ident: ast.NewIdent(dataField),
				Expr:  expr,
			}

			h.hoist(expr)
		}
	})
}

func tryParse(str string) (s ast.Expr, format *encodingInfo) {
	b := []byte(str)
	if json.Valid(b) {
		expr, err := parser.ParseExpr("", b)
		if err != nil {
			// TODO: report error
			return nil, nil
		}
		switch expr.(type) {
		case *ast.StructLit, *ast.ListLit:
		default:
			return nil, nil
		}
		return expr, jsonEnc
	}

	if expr, err := yaml.Unmarshal("", b); err == nil {
		switch expr.(type) {
		case *ast.StructLit, *ast.ListLit:
		default:
			return nil, nil
		}
		return expr, yamlEnc
	}

	return nil, nil
}

func (h *hoister) uniqueName(base, prefix, typ string) string {
	base = strings.Map(func(r rune) rune {
		if unicode.In(r, unicode.L, unicode.N) {
			return r
		}
		return '_'
	}, base)

	name := prefix + typ + base
	for {
		if !h.fields[name] {
			h.fields[name] = true
			return name
		}
		name = prefix + typ + base
		typ += "x"
	}
}
