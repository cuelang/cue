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
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/tools/go/loader"
)

// extractGoCmd represents the go command
var extractGoCmd = &cobra.Command{
	Use:   "go",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		extract(args)
	},
}

func init() {
	extractCmd.AddCommand(extractGoCmd)

	exclude = extractGoCmd.Flags().StringP("exclude", "e", "", "comma-separated list of regexps of entries")

	stripStr = extractGoCmd.Flags().Bool("stripstr", false, "Remove String suffix from functions")
}

var (
	exclude  *string
	genLine  string
	stripStr *bool
)

const copyright = `// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
`

var exclusions []*regexp.Regexp

func initExclusions() {
	for _, re := range strings.Split(*exclude, ",") {
		if re != "" {
			exclusions = append(exclusions, regexp.MustCompile(re))
		}
	}
}

func filter(name string) bool {
	if !ast.IsExported(name) {
		return true
	}
	for _, ex := range exclusions {
		if ex.MatchString(name) {
			return true
		}
	}
	return false
}

func pkgName() string {
	pkg, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Base(pkg)
}

type extractor struct {
	prog *loader.Program
	pkg  *loader.PackageInfo
	cmap ast.CommentMap

	orig     map[types.Type]*ast.StructType
	consts   map[string][]string
	pkgNames map[string]string
	indent   int
}

func extract(args []string) {
	cfg := loader.Config{
		ParserMode: parser.ParseComments,
	}
	cfg.FromArgs(args, false)

	e := extractor{
		orig:   map[types.Type]*ast.StructType{},
		consts: map[string][]string{},
	}
	var err error
	e.prog, err = cfg.Load()
	if err != nil {
		log.Fatal(err)
	}

	lastPkg := ""
	var w *bytes.Buffer

	initExclusions()

	for _, p := range e.prog.InitialPackages() {
		for _, f := range p.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.StructType:
					e.orig[p.TypeOf(x)] = x
				}
				return true
			})
		}
	}

	for _, p := range e.prog.InitialPackages() {
		e.pkg = p

		for _, f := range p.Files {
			for _, d := range f.Decls {
				switch x := d.(type) {
				case *ast.GenDecl:
					e.recordConsts(w, x)
				}
			}
		}

		for _, f := range p.Files {
			e.cmap = ast.NewCommentMap(e.prog.Fset, f, f.Comments)

			e.pkgNames = map[string]string{}
			for _, spec := range f.Imports {
				key, _ := strconv.Unquote(spec.Path.Value)
				if spec.Name != nil {
					e.pkgNames[key] = spec.Name.Name
				} else {
					logf("KEY %s %v", key, path.Base(key))
					e.pkgNames[key] = path.Base(key)
				}
			}

			// e.cmap = ast.NewCommentMap(e.prog.Fset, f, f.Comments)
			if lastPkg != p.Pkg.Name() {
				if w != nil {
					b, err := format.Source(w.Bytes())
					if err != nil {
						log.Fatal(err)
					}
					err = ioutil.WriteFile(lastPkg+".go", b, 0644)
					if err != nil {
						log.Fatal(err)
					}
				}
				lastPkg = p.Pkg.Name()
				w = &bytes.Buffer{}
				fmt.Fprintln(w, copyright)
				fmt.Fprintln(w, genLine)
				fmt.Fprintln(w)
				fmt.Fprintf(w, "package %s\n", pkgName())
				fmt.Fprintln(w)
				fmt.Fprintf(w, "import %q", p.Pkg.Path())
				fmt.Fprintln(w)
			}

			for _, d := range f.Decls {
				switch x := d.(type) {
				case *ast.GenDecl:
					e.reportDecl(w, x)
				}
			}
		}
	}
}

func (e *extractor) recordConsts(w io.Writer, x *ast.GenDecl) {
	if x.Tok != token.CONST {
		return
	}
	for _, s := range x.Specs {
		if v, ok := s.(*ast.ValueSpec); ok {
			if t := v.Type; t != nil {
				if x, ok := t.(*ast.Ident); ok {
					for _, name := range v.Names {
						e.consts[x.Name] = append(e.consts[x.Name], name.Name)
					}
					logf("FOO %v %p %v", t, v.Type, e.consts[x.Name])
				}
			}
		}
	}
}

var trace bool = true

func logf(format string, args ...interface{}) {
	if trace {
		log.Printf(format+"\n", args...)
	}
}

func dolog(args ...interface{}) {
	if trace {
		log.Print(args...)
	}
}

func (e *extractor) reportDecl(w io.Writer, x *ast.GenDecl) {
	switch x.Tok {
	case token.TYPE:
		for _, s := range x.Specs {
			if v, ok := s.(*ast.TypeSpec); ok && !filter(v.Name.Name) {
				e.printDoc(x.Doc)
				fmt.Print(v.Name.Name, ": ")
				if enums := e.consts[v.Name.Name]; len(enums) > 0 {
					logf("FOOF %p %v", e.pkg.TypeOf(v.Type), enums)
					e.indent++
					for i, v := range enums {
						if i > 0 {
							fmt.Print(" |")
						}
						e.newLine()
						fmt.Print(v)
					}
					e.indent--
					e.newLine()
					return
				} else {
					e.printType(e.pkg.TypeOf(v.Type), true)
				}
				e.newLine()
			}
		}

	case token.CONST:
		for _, s := range x.Specs {
			// TODO: determine type name and filter.
			if v, ok := s.(*ast.ValueSpec); ok {
				for i, name := range v.Names {
					if ast.IsExported(name.Name) {
						e.printDoc(v.Doc)
						fmt.Print(name.Name, ": ")
						fmt.Println(e.pkg.Types[v.Values[i]].Value)
						e.newLine()
					}
				}
			}
		}
	}
}

func (e *extractor) printDoc(doc *ast.CommentGroup) {
	if doc == nil {
		e.newLine()
		return
	}
	for _, c := range doc.List {
		fmt.Print(c.Text)
		e.newLine()
	}
}

func (e *extractor) newLine() {
	fmt.Println()
	fmt.Print(strings.Repeat("    ", e.indent))
}

func (e *extractor) printType(expr types.Type, top bool) {
	switch x := expr.(type) {
	case *types.Named:
		if pkg := x.Obj().Pkg(); pkg != nil {
			if name := e.pkgNames[pkg.Path()]; name != "" {
				fmt.Print(name, ".")
			}
		}
		fmt.Print(x.Obj().Name())

	case *types.Pointer:
		fmt.Printf("null | ")
		e.printType(x.Elem(), false)

	case *types.Struct:
		for i := 0; i < x.NumFields(); i++ {
			f := x.Field(i)
			if f.Anonymous() && e.isInline(x.Tag(i)) {
				typ := f.Type()
				if named, ok := typ.(*types.Named); ok {
					fmt.Printf("%s & ", named.Obj().Name())
				}
			}
		}
		fmt.Print("{")
		e.indent++
		e.printFields(x)
		e.indent--
		e.newLine()
		fmt.Print("}")
		fmt.Println()

	case *types.Slice:
		fmt.Print("int*[")
		e.printType(x.Elem(), false)
		fmt.Print("]")

	case *types.Array:
		fmt.Printf("%d*[", x.Len())
		e.printType(x.Elem(), false)
		fmt.Print("]")

	case *types.Map:
		if b, ok := x.Key().Underlying().(*types.Basic); !ok || b.Kind() != types.String {
			log.Fatalf("unsupported map key type %T", x.Key())
		}
		fmt.Printf("{ <_>: ")
		e.printType(x.Elem(), false)
		fmt.Printf(" }")

	default:
		fmt.Print(x.String())
	}
}

func (e *extractor) printFields(x *types.Struct) {
	s := e.orig[x]
	for i := 0; i < x.NumFields(); i++ {
		f := x.Field(i)
		if f.Anonymous() && e.isInline(x.Tag(i)) {
			typ := f.Type()
			if _, ok := typ.(*types.Named); !ok {
				switch x := typ.(type) {
				case *types.Struct:
					e.printFields(x)
				default:
					panic(fmt.Sprintf("unimplemented embedding for type %T", x))
				}
			}
			continue
		}
		e.newLine()
		if i > 0 {
			e.newLine()
		}
		if s != nil && s.Fields != nil {
			e.printDoc(s.Fields.List[i].Doc)
		}
		fmt.Print(getName(f.Name(), x.Tag(i)), ": ")
		e.printType(f.Type(), false)
	}
}

func (e *extractor) isInline(tag string) bool {
	if t := reflect.StructTag(tag).Get("json"); t != "" {
		for _, str := range strings.Split(t, ",")[1:] {
			if str == "inline" {
				return true
			}
		}
	}
	return false
}

func getName(name string, tag string) string {
	tags := reflect.StructTag(tag)
	for _, s := range []string{"cue", "json"} {
		if tag, ok := tags.Lookup(s); ok {
			if p := strings.Index(tag, ","); p >= 0 {
				tag = tag[:p]
			}
			if tag != "" {
				return tag
			}
		}
	}
	if tag, ok := tags.Lookup("protobuf"); ok {
		for _, str := range strings.Split(tag, ",") {
			if strings.HasPrefix("name=", str) {
				return str[len("name="):]
			}
		}
	}
	return name
}
