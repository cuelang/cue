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

	"cuelang.org/go/cue/format"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
)

// extractGoCmd represents the go command
var extractGoCmd = &cobra.Command{
	Use:   "go",
	Short: "extract type definitions from a Go program",
	Long: `extract go converts Go types into CUE definitions

The extracted packages are put in the CUE module's pkg directory at the path
of the corresponding Go package. The extracted definitions are available to
any CUE file within the CUE module by importing this path.

The generated definitions are written to the gen.cue file in the corresponding
package directory. Users can add additional files in these package directories
to apply further constraints. For instance, consider the gen.cue contains
a definition

	package foo

	Host: {
		IP4:  string
		Port: int
	}

To tighten the allowed values for IP4 without modifying the generated file,
add another file, say manual.cue, within the same directory.

	package foo

	Host IP4: =~ "^(\(Byte)\\.){3}\(Byte)$"

	Byte = #"([01]?\d?\d|2[0-4]\d|25[0-5])"#

Regenerating the defintions will then not discard the additional constraints.


Rules of Conversion

Go structs are converted to cue structs adhering to the following conventions:

	- field names are translated based on the definition of a "cue", json", or
	  or "protobuf" tag, in that order.
	- embedded structs marked with a json inline tag unify with struct
	  definition. For instance, the Go struct

	    struct MyStruct {
			Common  ` + "json:\",inline\"" + `
			Field string
		 }

	  translantes to the CUE struct

		 MyStruct: Common & {
			 Field: string
		 }

Slices and arrays convert to CUE lists, except when the element type is byte,
in which case it translates to the CUE bytes type. In the case of arrays, the
length of the CUE value is constraint accordingly.

Maps translate to a CUE struct, where all elements are constraint to be of
Go map element type.

Pointers translate to a sum type with the default value of null and the Go
type as an alternative value.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return extract(cmd, args)
	},
}

func init() {
	extractCmd.AddCommand(extractGoCmd)

	exclude = extractGoCmd.Flags().StringP("exclude", "e", "", "comma-separated list of regexps of entries")
	replace = extractGoCmd.Flags().String("as", "", "comma-separated list of type mappings")
}

var (
	exclude *string
	replace *string

	exclusions []*regexp.Regexp
	asMap      = map[string]dstUsed{}
)

type dstUsed struct {
	dst  string
	used bool
}

func initExclusions() {
	for _, re := range strings.Split(*exclude, ",") {
		if re != "" {
			exclusions = append(exclusions, regexp.MustCompile(re))
		}
	}

	for _, m := range strings.Split(*replace, ",") {
		split := strings.Split(m, "=")
		if len(split) > 1 {
			asMap[split[0]] = dstUsed{dst: split[1]}
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

type extractor struct {
	stderr io.Writer
	err    error
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
	pkgNames   map[string]string
	usedInFile map[string]bool
	indent     int
}

func (e *extractor) usedPkg(pkg string) {
	e.usedPkgs[pkg] = true
	e.usedInFile[pkg] = true
}

func (e *extractor) errorf(format string, args ...interface{}) {
	err := fmt.Errorf(format, args...)
	fmt.Fprintln(e.stderr, err)
	if e.err == nil {
		e.err = err
	}
}

// TODO:
// - consider not including types with any dropped fields.

func extract(cmd *cobra.Command, args []string) error {
	// determine module root:
	binst := loadFromArgs(cmd, []string{"."})[0]
	// TODO: require explicitly set root.
	root := binst.Root

	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax,
	}
	pkgs, err := packages.Load(cfg, args...)
	if err != nil {
		return err
	}

	e := extractor{
		stderr: cmd.OutOrStderr(),
		pkgs:   pkgs,
		orig:   map[types.Type]*ast.StructType{},
	}

	initExclusions()

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
	fmt.Println("---", p.PkgPath)

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
	dir := filepath.Join(root, "pkg", filepath.FromSlash(pkg))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	e.usedPkgs = map[string]bool{}

	for i, f := range p.Syntax {
		fmt.Println("    ---", p.CompiledGoFiles[i])
		e.w = &bytes.Buffer{}

		e.cmap = ast.NewCommentMap(p.Fset, f, f.Comments)

		e.pkgNames = map[string]string{}
		e.usedInFile = map[string]bool{}

		for _, spec := range f.Imports {
			key, _ := strconv.Unquote(spec.Path.Value)
			if spec.Name != nil {
				e.pkgNames[key] = spec.Name.Name
			} else {
				// TODO: incorrect, should be name of package clause
				e.pkgNames[key] = path.Base(key)
			}
		}

		// e.cmap = ast.NewCommentMap(e.prog.Fset, f, f.Comments)
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

		args := pkg
		if *exclude != "" {
			args += " --exclude=" + *exclude
		}
		if *replace != "" {
			args += " --as=" + *replace
		}

		fmt.Fprintln(w, "// Code generated by cue extract go. DO NOT EDIT.")
		fmt.Fprintln(w)
		fmt.Fprintln(w, "//cue:generate cue extract go", args)
		fmt.Fprintln(w)
		if f.Doc != nil {
			for _, c := range f.Doc.List {
				fmt.Fprintln(w, c.Text)
			}
		}
		fmt.Fprintf(w, "package %s\n", p.Name)
		fmt.Fprintln(w)
		if len(pkgs) > 0 {
			fmt.Fprintln(w, "import (")
			for _, s := range pkgs {
				name := e.pkgNames[s]
				if p.Imports[s].Name == name {
					fmt.Fprintf(w, "%q\n", s)
				} else {
					fmt.Fprintf(w, "%v %q\n", name, s)
				}
			}
			fmt.Fprintln(w, ")")
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w)
		io.Copy(w, e.w)

		file := filepath.Base(p.CompiledGoFiles[i])
		file = file[:len(file)-len(".go")]
		file += "_gen.cue"
		b, err := format.Source(w.Bytes())
		if err != nil {
			ioutil.WriteFile(filepath.Join(dir, file), w.Bytes(), 0644)
			fmt.Println(dir, file)
			panic(err)
			return err
		}
		err = ioutil.WriteFile(filepath.Join(dir, file), b, 0644)
		if err != nil {
			return err
		}
	}

	for path := range e.usedPkgs {
		fmt.Println(path, "PKG")

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
			if !ok || filter(v.Name.Name) {
				continue
			}

			typ := e.pkg.TypesInfo.TypeOf(v.Name)
			enums := e.consts[typ.String()]
			name := v.Name.Name
			switch tn, ok := e.pkg.TypesInfo.Defs[v.Name].(*types.TypeName); {
			case ok:
				if altType := altType(tn.Type()); altType != "" {
					// TODO: add the underlying tag as a Go tag once we have
					// proper string escaping for CUE.
					e.printDoc(x.Doc, true)
					fmt.Fprintf(e.w, "%s: %s", name, altType)
					added = true
					break
				}
				fallthrough

			default:
				if !supportedType(nil, typ) {
					continue
				}
				added = true

				// TODO: only print original type if value is not marked as enum.
				underlying := e.pkg.TypesInfo.TypeOf(v.Type)
				e.printField(name, false, underlying, x.Doc, true)
			}

			e.indent++
			for _, v := range enums {
				fmt.Fprint(e.w, " |")
				e.newLine()
				fmt.Fprint(e.w, v)
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
				fmt.Fprint(e.w, name.Name, ": ")
				if e.printAs(name.Name) {
					e.newLine()
					continue
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
	}
	return added
}

func altType(typ types.Type) string {
	// If the type implements a JSON unmarshaller, we allow the type to accept
	// an arbitrary type and we add a
	ms := types.NewMethodSet(typ)
	var isJSON, isText bool
	for i := 0; i < ms.Len(); i++ {
		if fn, ok := ms.At(i).Obj().(*types.Func); ok {
			switch fn.Name() {
			case "UnmarshalJSON", "UnmarshalYAML",
				"MarshalJSON", "MarshalYAML":
				// JSON or YAML marshal-related methods may changes the
				// underlying representation of Go versus JSON. So in this case
				// we can no longer make assumptions about the type and we will
				// need create a general template.
				isJSON = true
			case "UnmarshalText", "MarshalText":
				isText = true
			}
		}
	}
	// TODO: See GroupVersion in apimachinery/pkg/apis/meta/v1/group_version.go
	// If recursive type still has json tags, make that the type regardless.
	// However, beware that some code uses MarshalJSON to provide a new API
	// on top of old code (like Prometheus), so it should be considered
	// carefully how and when this is allowed.
	// TODO: at the very least include the original code as documentation.
	// For instance, see Fields in types_gen.go in the same package.
	if isJSON {
		// tags += fmt.Sprintf(" @json(,raw)")
		return "_"
	} else if isText {
		// tags += fmt.Sprintf(" @json(,string)")
		return "string"
	}
	return ""
}

func (e *extractor) printDoc(doc *ast.CommentGroup, newline bool) {
	if doc == nil {
		return
	}
	if newline {
		e.newLine()
	}
	for _, c := range doc.List {
		fmt.Fprint(e.w, c.Text)
		e.newLine()
	}
}

func (e *extractor) newLine() {
	fmt.Fprintln(e.w)
	fmt.Fprint(e.w, strings.Repeat("    ", e.indent))
}

func supportedType(stack []types.Type, t types.Type) bool {
	// handle recursive types
	for _, t0 := range stack {
		if t0 == t {
			return true
		}
	}
	stack = append(stack, t)

	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()

		if _, ok := asMap[named.String()]; ok {
			return true
		}

		// Redirect or drop Go standard library types.
		if obj.Pkg() == nil {
			fmt.Println("UNSUPPORTED", obj.Id())
			return false
		}
		switch obj.Pkg().Path() {
		case "time":
			switch named.Obj().Name() {
			case "Time", "Duration", "Location", "Month", "Weekday":
				return true
			}
			return false
		case "net":
			// TODO: IP, Host, SRV, etc.
		case "url":
			// TODO: URL and Values
		}

		if !strings.ContainsAny(obj.Pkg().Path(), ".") {
			// Drop any standard library type if they haven't been handled
			// above.
			return false
		}
	}

	t = t.Underlying()
	switch x := t.(type) {
	case *types.Basic, *types.Named:
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
			// fmt.Println(f.Name(), "ppp")
		}
	}
	return false
}

func (e *extractor) printField(name string, opt bool, expr types.Type, doc *ast.CommentGroup, newline bool) (typename string) {
	e.printDoc(doc, newline)
	colon := ": "
	if opt {
		colon = "?: "
	}
	fmt.Fprint(e.w, name, colon)
	if e.printAs(name) {
		return
	}
	pos := e.w.Len()
	e.printType(expr)
	return e.w.String()[pos:]
}

func (e *extractor) printAs(name string) bool {
	key := fmt.Sprintf("%s.%s", e.pkg.PkgPath, name)
	alt, ok := asMap[key]
	if !ok {
		return false
	}

	dst := alt.dst
	alt.used = true
	asMap[key] = alt

	if strings.Contains(dst, "/") {
		e.usedPkg(dst[:len(dst)-len(path.Ext(dst))])
	}
	fmt.Fprint(e.w, dst)
	return true
}

func (e *extractor) printType(expr types.Type) {
	if x, ok := expr.(*types.Named); ok && x.Obj().Pkg() != nil {
		// Check for builtin packages.
		// TODO: replace these literal types with a reference to the fixed
		// builtin type.
		switch x.Obj().Type().String() {
		case "time.Time":
			e.usedInFile["time"] = true
			fmt.Fprint(e.w, `time.Time`) // builtin time
			return

		case "big.Int":
			fmt.Fprint(e.w, `null | int`)
			return
		}
		if pkg := x.Obj().Pkg(); pkg != nil {
			if name := e.pkgNames[pkg.Path()]; name != "" {
				fmt.Fprint(e.w, name, ".")
				e.usedPkg(pkg.Path())
			}
		}
		fmt.Fprint(e.w, x.Obj().Name())
		return
	}

	switch x := expr.(type) {
	case *types.Pointer:
		fmt.Fprintf(e.w, "null | ")
		e.printType(x.Elem())

	case *types.Struct:
		for i := 0; i < x.NumFields(); i++ {
			f := x.Field(i)
			if f.Anonymous() && e.isInline(x.Tag(i)) {
				typ := f.Type()
				if named, ok := typ.(*types.Named); ok {
					fmt.Fprintf(e.w, "%s & ", named.Obj().Name())
				}
			}
		}
		fmt.Fprint(e.w, "{")
		e.indent++
		e.printFields(x)
		e.indent--
		e.newLine()
		fmt.Fprint(e.w, "}")
		fmt.Fprintln(e.w)

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
			fmt.Println("TYPE", x)
			log.Panicf("unsupported map key type %T", x.Key())
		}
		fmt.Fprintf(e.w, "{ <_>: ")
		e.printType(x.Elem())
		fmt.Fprintf(e.w, " }")

	case *types.Basic:
		fmt.Fprint(e.w, x.String())

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
			continue
		}
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
		tag := x.Tag(i)
		name := getName(f.Name(), tag)
		if name == "-" {
			continue
		}
		e.newLine()
		cueType := e.printField(name, e.isOptional(tag), f.Type(), docs[i], count > 0)

		// Add field tag to convert back to Go.
		typeName := f.Type().String()
		// simplify type names:
		for path, name := range e.pkgNames {
			typeName = strings.Replace(typeName, path+".", name+".", -1)
		}
		typeName = strings.Replace(typeName, e.pkg.Types.Path()+".", "", -1)

		// TODO: remove fields in @go attr that are the same as printed?
		if name != f.Name() || typeName != cueType {
			fmt.Fprint(e.w, "@go(")
			if name != f.Name() {
				fmt.Fprint(e.w, f.Name())
			}
			if typeName != cueType {
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
	if t := reflect.StructTag(tag).Get("json"); t != "" {
		for _, str := range strings.Split(t, ",")[1:] {
			if str == "inline" {
				return true
			}
		}
	}
	return false
}

func (e *extractor) isOptional(tag string) bool {
	return hasFlag(tag, "json", "omitempty", 1) ||
		hasFlag(tag, "protobuf", "opt", 2)
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
