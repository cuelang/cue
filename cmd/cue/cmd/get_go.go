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
	"go/token"
	"go/types"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	cueast "cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	cuetoken "cuelang.org/go/cue/token"
	"cuelang.org/go/internal"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
)

// TODO:
// Document:
// - Use ast package.
// - how to deal with "oneOf" or sum types?
// - generate cue files for cue field tags?
// - cue go get or cue get go
// - include generation report in doc_gen.cue or report.txt.
//   Possible enums:
//   package foo
//   Type: enumType

func newGoCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go [packages]",
		Short: "add Go dependencies to the current module",
		Long: `go converts Go types into CUE definitions

The command "cue get go" is like "go get", but converts the retrieved Go
packages to CUE. The retrieved packages are put in the CUE module's pkg
directory at the import path of the corresponding Go package. The converted
definitions are available to any CUE file within the CUE module by using
this import path.

The Go type definitions are converted to CUE based on how they would be
interpreted by Go's encoding/json package. Definitions for a Go file foo.go
are written to a CUE file named foo_go_gen.cue.

It is safe for users to add additional files to the generated directories,
as long as their name does not end with _gen.*.


Rules of Converting Go types to CUE

Go structs are converted to cue structs adhering to the following conventions:

	- field names are translated based on the definition of a "json" or "yaml"
	  tag, in that order.

	- embedded structs marked with a json inline tag unify with struct
	  definition. For instance, the Go struct

	    struct MyStruct {
			Common  ` + "json:\",inline\"" + `
			Field string
		 }

	  translates to the CUE struct

		 MyStruct: Common & {
			 Field: string
		 }

	- a type that implements MarshalJSON, UnmarshalJSON, MarshalYAML, or
	  UnmarshalYAML is translated to top (_) to indicate it may be any
	  value. For some Go core types for which the implementation of these
	  methods is known, like time.Time, the type may be more specific.

	- a type implementing MarshalText or UnmarshalText is represented as
	  the CUE type string

	- slices and arrays convert to CUE lists, except when the element type is
	  byte, in which case it translates to the CUE bytes type.
	  In the case of arrays, the length of the CUE value is constrained
	  accordingly, when possible.

	- Maps translate to a CUE struct, where all elements are constrained to
	  be of Go map element type. Like for JSON, maps may only have string keys.

	- Pointers translate to a sum type with the default value of null and
	  the Go type as an alternative value.

	- Field tags are translated to CUE's field attributes. In some cases,
	  the contents are rewritten to reflect the corresponding types in CUE.
	  The @go attribute is added if the field name or type definition differs
	  between the generated CUE and the original Go.


Native CUE Constraints

Native CUE constraints may be defined in separate cue files alongside the
generated files either in the original Go directory or in the generated
directory. These files can impose additional constraints on types and values
that are not otherwise expressible in Go. The package name for these CUE files
must be the same as that of the Go package.

For instance, for the type

	package foo

    type IP4String string

defined in the Go package, one could add a cue file foo.cue with the following
contents to allow IP4String to assume only valid IP4 addresses:

	package foo

	// IP4String defines a valid IP4 address.
	IP4String: =~#"^\#(byte)\.\#(byte)\.\#(byte)\.\#(byte)$"#

	// byte defines string allowing integer values of 0-255.
	byte = #"([01]?\d?\d|2[0-4]\d|25[0-5])"#


The "cue get go" command copies any cue files in the original Go package
directory that has a package clause with the same name as the Go package to the
destination directory, replacing its .cue ending with _gen.cue.

Alternatively, the additional native constraints can be added to the generated
package, as long as the file name does not end with _gen.cue.
Running cue get go again to regenerate the package will never overwrite any
files not ending with _gen.*.


Constants and Enums

Go does not have an enum or sum type. Conventionally, a type that is supposed
to be an enum is followed by a const block with the allowed values for that
type. However, as that is only a guideline and not a hard rule, these cases
cannot be translated to CUE disjunctions automatically.

Constant values, however, are generated in a way that makes it easy to convert
a type to a proper enum using native CUE constraints. For instance, the Go type

	package foo

	type Switch int

	const (
		Off Switch = iota
		On
	)

translates into the following CUE definitions:

	package foo

	Switch: int // enumSwitch

	enumSwitch: Off | On

	Off: 0
	On:  1

This definition allows any integer value for Switch, while the enumSwitch value
defines all defined constants for Switch and thus all valid values if Switch
were to be interpreted as an enum type. To turn Switch into an enum,
include the following constraint in, say, enum.cue, in either the original
source directory or the generated directory:

	package foo

	// limit the valid values for Switch to those existing as constants with
	// the same type.
	Switch: enumSwitch

This tells CUE that only the values enumerated by enumSwitch are valid
values for Switch. Note that there are now two definitions of Switch.
CUE handles this in the usual way by unifying the two definitions, in which case
the more restrictive enum interpretation of Switch remains.
`,
		// - TODO: interpret cuego's struct tags and annotations.

		RunE: mkRunE(c, extract),
	}

	cmd.Flags().StringP(string(flagExclude), "e", "",
		"comma-separated list of regexps of entries")

	return cmd
}

const (
	flagExclude flagName = "exclude"
)

var cueTestRoot string // the CUE module root for test purposes.

func (e *extractor) initExclusions(str string) {
	e.exclude = str
	for _, re := range strings.Split(str, ",") {
		if re != "" {
			e.exclusions = append(e.exclusions, regexp.MustCompile(re))
		}
	}
}

func (e *extractor) filter(name string) bool {
	if !ast.IsExported(name) {
		return true
	}
	for _, ex := range e.exclusions {
		if ex.MatchString(name) {
			return true
		}
	}
	return false
}

type extractor struct {
	cmd *Command

	stderr io.Writer
	pkgs   []*packages.Package
	done   map[string]bool

	// per package
	orig     map[types.Type]*ast.StructType
	usedPkgs map[string]bool

	// per file
	w          *bytes.Buffer
	cmap       ast.CommentMap
	pkg        *packages.Package
	consts     map[string][]string
	pkgNames   map[string]pkgInfo
	usedInFile map[string]bool
	indent     int

	exclusions []*regexp.Regexp
	exclude    string
}

type pkgInfo struct {
	id   string
	name string
}

func (e *extractor) logf(format string, args ...interface{}) {
	if flagVerbose.Bool(e.cmd) {
		fmt.Fprintf(e.stderr, format+"\n", args...)
	}
}

func (e *extractor) usedPkg(pkg string) {
	e.usedPkgs[pkg] = true
	e.usedInFile[pkg] = true
}

func initInterfaces() error {
	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax,
	}
	p, err := packages.Load(cfg, "cuelang.org/go/cmd/cue/cmd/interfaces")
	if err != nil {
		return err
	}

	for e, tt := range p[0].TypesInfo.Types {
		if n, ok := tt.Type.(*types.Named); ok && n.String() == "error" {
			continue
		}
		if tt.Type.Underlying().String() == "interface{}" {
			continue
		}

		switch tt.Type.Underlying().(type) {
		case *types.Interface:
			file := p[0].Fset.Position(e.Pos()).Filename
			switch filepath.Base(file) {
			case "top.go":
				toTop = append(toTop, tt.Type)
			case "text.go":
				toString = append(toString, tt.Type)
			}
		}
	}
	return nil
}

var (
	toTop    []types.Type
	toString []types.Type
)

// TODO:
// - consider not including types with any dropped fields.

func extract(cmd *Command, args []string) error {
	// determine module root:
	binst := loadFromArgs(cmd, []string{"."}, nil)[0]

	if err := initInterfaces(); err != nil {
		return err
	}

	// TODO: require explicitly set root.
	root := binst.Root

	// Override root in testing mode.
	if cueTestRoot != "" {
		root = cueTestRoot
	}

	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax,
	}
	pkgs, err := packages.Load(cfg, args...)
	if err != nil {
		return err
	}

	e := extractor{
		cmd:    cmd,
		stderr: cmd.Stderr(),
		pkgs:   pkgs,
		orig:   map[types.Type]*ast.StructType{},
	}

	e.initExclusions(flagExclude.String(cmd))

	e.done = map[string]bool{}

	for _, p := range pkgs {
		e.done[p.PkgPath] = true
	}

	for _, p := range pkgs {
		if err := e.extractPkg(root, p); err != nil {
			return err
		}
	}
	return nil
}

func (e *extractor) recordTypeInfo(p *packages.Package) {
	for _, f := range p.Syntax {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.StructType:
				e.orig[p.TypesInfo.TypeOf(x)] = x
			}
			return true
		})
	}
}

func (e *extractor) extractPkg(root string, p *packages.Package) error {
	e.pkg = p
	e.logf("--- Package %s", p.PkgPath)

	e.recordTypeInfo(p)

	e.consts = map[string][]string{}

	for _, f := range p.Syntax {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				e.recordConsts(x)
			}
		}
	}

	pkg := p.PkgPath
	dir := filepath.Join(load.GenPath(root), filepath.FromSlash(pkg))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	e.usedPkgs = map[string]bool{}

	args := pkg
	if e.exclude != "" {
		args += " --exclude=" + e.exclude
	}

	for i, f := range p.Syntax {
		e.w = &bytes.Buffer{}

		e.cmap = ast.NewCommentMap(p.Fset, f, f.Comments)

		e.pkgNames = map[string]pkgInfo{}
		e.usedInFile = map[string]bool{}

		for _, spec := range f.Imports {
			pkgPath, _ := strconv.Unquote(spec.Path.Value)
			pkg := p.Imports[pkgPath]

			info := pkgInfo{id: pkgPath, name: pkg.Name}
			if path.Base(pkgPath) != pkg.Name {
				info.id += ":" + pkg.Name
			}

			if spec.Name != nil {
				info.name = spec.Name.Name
			}

			e.pkgNames[pkgPath] = info
		}

		hasEntries := false
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				if e.reportDecl(e.w, x) {
					hasEntries = true
				}
			}
		}

		if !hasEntries && f.Doc == nil {
			continue
		}

		pkgs := []string{}
		for k := range e.usedInFile {
			pkgs = append(pkgs, k)
		}
		sort.Strings(pkgs)

		w := &bytes.Buffer{}

		fmt.Fprintln(w, "// Code generated by cue get go. DO NOT EDIT.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "//cue:generate cue get go", args)
		fmt.Fprintln(w)
		if f.Doc != nil {
			writeDoc(w, f.Doc)
		}
		fmt.Fprintf(w, "package %s\n", p.Name)
		fmt.Fprintln(w)
		if len(pkgs) > 0 {
			fmt.Fprintln(w, "import (")
			for _, s := range pkgs {
				info := e.pkgNames[s]
				if p.Imports[s].Name == info.name {
					fmt.Fprintf(w, "%q\n", info.id)
				} else {
					fmt.Fprintf(w, "%v %q\n", info.name, info.id)
				}
			}
			fmt.Fprintln(w, ")")
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
		_, _ = io.Copy(w, e.w)

		file := filepath.Base(p.CompiledGoFiles[i])

		file = strings.Replace(file, ".go", "_go", 1)
		file += "_gen.cue"
		b, err := format.Source(w.Bytes())
		if err != nil {
			_ = ioutil.WriteFile(filepath.Join(dir, file), w.Bytes(), 0644)
			stderr := e.cmd.Stderr()
			fmt.Fprintln(stderr, w.String())
			fmt.Fprintln(stderr, dir, file)
			return err
		}
		err = ioutil.WriteFile(filepath.Join(dir, file), b, 0644)
		if err != nil {
			return err
		}
	}

	for _, o := range p.CompiledGoFiles {
		root := filepath.Dir(o)
		err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
			if fi.IsDir() && path != root {
				return filepath.SkipDir
			}
			if filepath.Ext(path) != ".cue" {
				return nil
			}
			f, err := parser.ParseFile(path, nil)
			if err != nil {
				return err
			}

			if _, pkg, _ := internal.PackageInfo(f); pkg != "" && pkg == p.Name {
				file := filepath.Base(path)
				file = file[:len(file)-len(".cue")]
				file += "_gen.cue"

				w := &bytes.Buffer{}
				fmt.Fprintln(w, "// Code generated by cue get go. DO NOT EDIT.")
				fmt.Fprintln(w)
				fmt.Fprintln(w, "//cue:generate cue get go", args)
				fmt.Fprintln(w)

				b, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				w.Write(b)

				dst := filepath.Join(dir, file)
				if err := ioutil.WriteFile(dst, w.Bytes(), 0644); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	for path := range e.usedPkgs {
		if !e.done[path] {
			e.done[path] = true
			p := p.Imports[path]
			if err := e.extractPkg(root, p); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *extractor) recordConsts(x *ast.GenDecl) {
	if x.Tok != token.CONST {
		return
	}
	for _, s := range x.Specs {
		v, ok := s.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, n := range v.Names {
			typ := e.pkg.TypesInfo.TypeOf(n).String()
			e.consts[typ] = append(e.consts[typ], n.Name)
		}
	}
}

func (e *extractor) reportDecl(w io.Writer, x *ast.GenDecl) (added bool) {
	switch x.Tok {
	case token.TYPE:
		for _, s := range x.Specs {
			v, ok := s.(*ast.TypeSpec)
			if !ok || e.filter(v.Name.Name) {
				continue
			}

			typ := e.pkg.TypesInfo.TypeOf(v.Name)
			enums := e.consts[typ.String()]
			name := v.Name.Name
			switch tn, ok := e.pkg.TypesInfo.Defs[v.Name].(*types.TypeName); {
			case ok:
				if altType := e.altType(tn.Type()); altType != "" {
					// TODO: add the underlying tag as a Go tag once we have
					// proper string escaping for CUE.
					e.printDoc(x.Doc, true)
					fmt.Fprintf(e.w, "%s :: %s", name, altType)
					added = true
					break
				}
				fallthrough

			default:
				if !supportedType(nil, typ) {
					e.logf("    Dropped declaration %v of unsupported type %v", name, typ)
					continue
				}
				added = true

				if s := e.altType(types.NewPointer(typ)); s != "" {
					e.printDoc(x.Doc, true)
					fmt.Fprint(e.w, name, " :: ", s)
					break
				}
				// TODO: only print original type if value is not marked as enum.
				underlying := e.pkg.TypesInfo.TypeOf(v.Type)
				e.printField(name, cuetoken.ISA, underlying, x.Doc, true)
			}

			e.indent++
			if len(enums) > 0 {
				fmt.Fprintf(e.w, " // enum%s", name)

				e.newLine()
				e.newLine()
				fmt.Fprintf(e.w, "enum%s::\n%v", name, enums[0])
				for _, v := range enums[1:] {
					fmt.Fprint(e.w, " |")
					e.newLine()
					fmt.Fprint(e.w, v)
				}
			}
			e.indent--
			e.newLine()
			e.newLine()
		}

	case token.CONST:
		// TODO: copy over comments for constant blocks.

		for _, s := range x.Specs {
			// TODO: determine type name and filter.
			v, ok := s.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, name := range v.Names {
				if !ast.IsExported(name.Name) {
					continue
				}
				added = true

				e.printDoc(v.Doc, true)
				fmt.Fprint(e.w, name.Name, " :: ")

				typ := e.pkg.TypesInfo.TypeOf(name)
				if s := typ.String(); !strings.Contains(s, "untyped") {
					switch s {
					case "byte", "string", "error":
					default:
						e.printType(typ)
						fmt.Fprint(e.w, " & ")
					}
				}

				val := ""
				comment := ""
				if i < len(v.Values) {
					if lit, ok := v.Values[i].(*ast.BasicLit); ok {
						val = lit.Value
					}
				}

			outer:
				switch {
				case len(val) <= 1:
				case val[0] == '\'':
					comment = " // " + val
					val = ""

				case strings.HasPrefix(val, "0"):
					for _, c := range val[1:] {
						if c < '0' || '9' < c {
							val = ""
							break outer
						}
					}
					val = "0o" + val[1:]
				}

				if val == "" {
					c := e.pkg.TypesInfo.Defs[v.Names[i]].(*types.Const)
					val = c.Val().String()
				}

				fmt.Fprint(e.w, val, comment)
				e.newLine()
			}
		}
		e.newLine()
	}
	return added
}

func shortTypeName(t types.Type) string {
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return t.String()
}

func (e *extractor) altType(typ types.Type) string {
	ptr := types.NewPointer(typ)
	for _, x := range toTop {
		i := x.Underlying().(*types.Interface)
		if types.Implements(typ, i) || types.Implements(ptr, i) {
			t := shortTypeName(typ)
			e.logf("    %v implements %s; setting type to _", t, x)
			return "_"
		}
	}
	for _, x := range toString {
		i := x.Underlying().(*types.Interface)
		if types.Implements(typ, i) || types.Implements(ptr, i) {
			t := shortTypeName(typ)
			e.logf("    %v implements %s; setting type to string", t, x)
			return "string"
		}
	}
	return ""
}

func writeDoc(w io.Writer, g *ast.CommentGroup) {
	if g == nil {
		return
	}

	for _, comment := range g.List {
		c := comment.Text

		// Remove comment markers.
		// The parser has given us exactly the comment text.
		switch c[1] {
		case '/':
			//-style comment (no newline at the end)
			_, _ = fmt.Fprintf(w, "//%s\n", c[2:])

		case '*':
			/*-style comment */
			c = c[2 : len(c)-2]
			if len(c) > 0 && c[0] == '\n' {
				c = c[1:]
			}

			lines := strings.Split(c, "\n")

			// Find common space prefix
			i := 0
			line := lines[0]
			for ; i < len(line); i++ {
				if c := line[i]; c != ' ' && c != '\t' {
					break
				}
			}

			for _, l := range lines {
				for j := 0; j < i && j < len(l); j++ {
					if line[j] != l[j] {
						i = j
						break
					}
				}
			}

			// Strip last line if empty.
			if n := len(lines); n > 1 && len(lines[n-1]) < i {
				lines = lines[:n-1]
			}

			// Print lines.
			for _, l := range lines {
				if i >= len(l) {
					_, _ = io.WriteString(w, "//\n")
					continue
				}
				_, _ = fmt.Fprintf(w, "// %s\n", l[i:])
			}
		}
	}
}

func (e *extractor) printDoc(doc *ast.CommentGroup, newline bool) {
	if doc == nil {
		return
	}
	if newline {
		e.newLine()
	}
	writeDoc(e.w, doc)
}

func (e *extractor) newLine() {
	fmt.Fprintln(e.w)
	fmt.Fprint(e.w, strings.Repeat("    ", e.indent))
}

func supportedType(stack []types.Type, t types.Type) (ok bool) {
	// handle recursive types
	for _, t0 := range stack {
		if t0 == t {
			return true
		}
	}
	stack = append(stack, t)

	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()

		// Redirect or drop Go standard library types.
		if obj.Pkg() == nil {
			// error interface
			return true
		}
		switch obj.Pkg().Path() {
		case "time":
			switch named.Obj().Name() {
			case "Time", "Duration", "Location", "Month", "Weekday":
				return true
			}
			return false
		case "math/big":
			switch named.Obj().Name() {
			case "Int", "Float":
				return true
			}
			// case "net":
			// 	// TODO: IP, Host, SRV, etc.
			// case "url":
			// 	// TODO: URL and Values
		}
	}

	t = t.Underlying()
	switch x := t.(type) {
	case *types.Basic:
		return x.String() != "invalid type"
	case *types.Named:
		return true
	case *types.Pointer:
		return supportedType(stack, x.Elem())
	case *types.Slice:
		return supportedType(stack, x.Elem())
	case *types.Array:
		return supportedType(stack, x.Elem())
	case *types.Map:
		if b, ok := x.Key().Underlying().(*types.Basic); !ok || b.Kind() != types.String {
			return false
		}
		return supportedType(stack, x.Elem())
	case *types.Struct:
		// Eliminate structs with fields for which all fields are filtered.
		if x.NumFields() == 0 {
			return true
		}
		for i := 0; i < x.NumFields(); i++ {
			f := x.Field(i)
			if f.Exported() && supportedType(stack, f.Type()) {
				return true
			}
		}
	case *types.Interface:
		return true
	}
	return false
}

func (e *extractor) printField(name string, kind cuetoken.Token, expr types.Type, doc *ast.CommentGroup, newline bool) (typename string) {
	e.printDoc(doc, newline)
	colon := ": "
	switch kind {
	case cuetoken.ISA:
		colon = " :: "
	case cuetoken.OPTION:
		colon = "?: "
	}
	fmt.Fprint(e.w, name, colon)
	pos := e.w.Len()
	e.printType(expr)
	return e.w.String()[pos:]
}

func (e *extractor) printType(expr types.Type) {
	if x, ok := expr.(*types.Named); ok {
		obj := x.Obj()
		if obj.Pkg() == nil {
			fmt.Fprint(e.w, "_")
			return
		}
		// Check for builtin packages.
		// TODO: replace these literal types with a reference to the fixed
		// builtin type.
		switch obj.Type().String() {
		case "time.Time":
			e.usedInFile["time"] = true
			fmt.Fprint(e.w, e.pkgNames[obj.Pkg().Path()].name, ".", obj.Name())
			return

		case "math/big.Int":
			fmt.Fprint(e.w, "int")
			return

		default:
			if !strings.ContainsAny(obj.Pkg().Path(), ".") {
				// Drop any standard library type if they haven't been handled
				// above.
				if s := e.altType(obj.Type()); s != "" {
					fmt.Fprint(e.w, s)
					return
				}
			}
		}
		if pkg := obj.Pkg(); pkg != nil {
			if info := e.pkgNames[pkg.Path()]; info.name != "" {
				fmt.Fprint(e.w, info.name, ".")
				e.usedPkg(pkg.Path())
			}
		}
		fmt.Fprint(e.w, obj.Name())
		return
	}

	switch x := expr.(type) {
	case *types.Pointer:
		fmt.Fprintf(e.w, "null | ")
		e.printType(x.Elem())

	case *types.Struct:
		fmt.Fprint(e.w, "{")
		e.indent++
		e.printFields(x)
		e.indent--
		e.newLine()
		fmt.Fprint(e.w, "}")

	case *types.Slice:
		// TODO: should this be x.Elem().Underlying().String()? One could
		// argue either way.
		if x.Elem().String() == "byte" {
			fmt.Fprint(e.w, "bytes")
		} else {
			fmt.Fprint(e.w, "[...")
			e.printType(x.Elem())
			fmt.Fprint(e.w, "]")
		}

	case *types.Array:
		if x.Elem().String() == "byte" {
			// TODO: no way to constraint lengths of bytes for now, as regexps
			// operate on Unicode, not bytes. So we need
			//     fmt.Fprint(e.w, fmt.Sprintf("=~ '^\C{%d}$'", x.Len())),
			// but regexp does not support that.
			// But translate to bytes, instead of [...byte] to be consistent.
			fmt.Fprint(e.w, "bytes")
		} else {
			fmt.Fprintf(e.w, "%d*[", x.Len())
			e.printType(x.Elem())
			fmt.Fprint(e.w, "]")
		}

	case *types.Map:
		if b, ok := x.Key().Underlying().(*types.Basic); !ok || b.Kind() != types.String {
			panic(fmt.Sprintf("unsupported map key type %T", x.Key()))
		}
		fmt.Fprintf(e.w, "{ [string]: ")
		e.printType(x.Elem())
		fmt.Fprintf(e.w, " }")

	case *types.Basic:
		fmt.Fprint(e.w, x.String())

	case *types.Interface:
		fmt.Fprintf(e.w, "_")

	default:
		// record error
		panic(fmt.Sprintf("unsupported type %T", x))
	}
}

func (e *extractor) printFields(x *types.Struct) {
	s := e.orig[x]
	docs := []*ast.CommentGroup{}
	for _, f := range s.Fields.List {
		if len(f.Names) == 0 {
			docs = append(docs, f.Doc)
		} else {
			for range f.Names {
				docs = append(docs, f.Doc)
			}
		}
	}
	count := 0
	for i := 0; i < x.NumFields(); i++ {
		f := x.Field(i)
		if !ast.IsExported(f.Name()) {
			continue
		}
		if !supportedType(nil, f.Type()) {
			e.logf("    Dropped field %v for unsupported type %v", f.Name(), f.Type())
			continue
		}
		if f.Anonymous() && e.isInline(x.Tag(i)) {
			typ := f.Type()
			if _, ok := typ.(*types.Named); ok {
				// TODO: strongly consider allowing Expressions for embedded
				//       values. This ties in with using dots instead of spaces,
				//       comprehensions, and the ability to generate good
				//       error messages, so thread carefully.
				if i > 0 {
					fmt.Fprintln(e.w)
				}
				fmt.Fprint(e.w, "\n")
				e.printType(typ)
			} else {
				switch x := typ.(type) {
				case *types.Struct:
					e.printFields(x)
				default:
					panic(fmt.Sprintf("unimplemented embedding for type %T", x))
				}
			}
			continue
		}
		tag := x.Tag(i)
		name := getName(f.Name(), tag)
		if name == "-" {
			continue
		}
		if x, _ := cueast.QuoteIdent(name); x != name {
			name = strconv.Quote(name)
		}
		e.newLine()
		kind := cuetoken.COLON
		if e.isOptional(tag) {
			kind = cuetoken.OPTION
		}
		cueType := e.printField(name, kind, f.Type(), docs[i], count > 0)

		// Add field tag to convert back to Go.
		typeName := f.Type().String()
		// simplify type names:
		for path, info := range e.pkgNames {
			typeName = strings.Replace(typeName, path+".", info.name+".", -1)
		}
		typeName = strings.Replace(typeName, e.pkg.Types.Path()+".", "", -1)

		// TODO: remove fields in @go attr that are the same as printed?
		if name != f.Name() || typeName != cueType {
			fmt.Fprint(e.w, "@go(")
			if name != f.Name() {
				fmt.Fprint(e.w, f.Name())
			}
			if typeName != cueType {
				if strings.ContainsAny(typeName, `#"',()=`) {
					typeName = strconv.Quote(typeName)
				}
				fmt.Fprint(e.w, ",", typeName)
			}
			fmt.Fprintf(e.w, ")")
		}

		// Carry over protobuf field tags with modifications.
		if t := reflect.StructTag(tag).Get("protobuf"); t != "" {
			split := strings.Split(t, ",")
			k := 0
			for _, s := range split {
				if strings.HasPrefix(s, "name=") && s[len("name="):] == name {
					continue
				}
				split[k] = s
				k++
			}
			split = split[:k]

			// Put tag first, as type could potentially be elided and is
			// "more optional".
			if len(split) >= 2 {
				split[0], split[1] = split[1], split[0]
			}
			fmt.Fprintf(e.w, " @protobuf(%s)", strings.Join(split, ","))
		}

		// Carry over XML tags.
		if t := reflect.StructTag(tag).Get("xml"); t != "" {
			fmt.Fprintf(e.w, " @xml(%s)", t)
		}

		// Carry over TOML tags.
		if t := reflect.StructTag(tag).Get("toml"); t != "" {
			fmt.Fprintf(e.w, " @toml(%s)", t)
		}

		// TODO: should we in general carry over any unknown tag verbatim?

		count++
	}
}

func (e *extractor) isInline(tag string) bool {
	return hasFlag(tag, "json", "inline", 1) ||
		hasFlag(tag, "yaml", "inline", 1)
}

func (e *extractor) isOptional(tag string) bool {
	// TODO: also when the type is a list or other kind of pointer.
	return hasFlag(tag, "json", "omitempty", 1) ||
		hasFlag(tag, "yaml", "omitempty", 1)
}

func hasFlag(tag, key, flag string, offset int) bool {
	if t := reflect.StructTag(tag).Get(key); t != "" {
		split := strings.Split(t, ",")
		if offset >= len(split) {
			return false
		}
		for _, str := range split[offset:] {
			if str == flag {
				return true
			}
		}
	}
	return false
}

func getName(name string, tag string) string {
	tags := reflect.StructTag(tag)
	for _, s := range []string{"json", "yaml"} {
		if tag, ok := tags.Lookup(s); ok {
			if p := strings.Index(tag, ","); p >= 0 {
				tag = tag[:p]
			}
			if tag != "" {
				return tag
			}
		}
	}
	// TODO: should we also consider to protobuf name? Probably not.

	return name
}
