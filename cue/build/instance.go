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

package build

import (
	pathpkg "path"
	"path/filepath"
	"strings"
	"unicode"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal"
)

// An Instance describes the collection of files, and its imports, necessary
// to build a CUE instance.
//
// A typical way to create an Instance is to use the cue/load package.
type Instance struct {
	ctxt *Context

	BuildFiles    []*File // files to be inclduded in the build
	IgnoredFiles  []*File // files excluded for this build
	OrphanedFiles []*File // recognized file formats not part of any build
	InvalidFiles  []*File // could not parse these files
	UnknownFiles  []*File // unknown file types

	// Files contains the AST for all files part of this instance.
	// TODO: the intent is to deprecate this in favor of BuildFiles.
	Files []*ast.File

	loadFunc LoadFunc
	done     bool

	// Scope is another instance that may be used to resolve any unresolved
	// reference of this instance. For instance, tool and test instances
	// may refer to top-level fields in their package scope.
	Scope *Instance

	// PkgName is the name specified in the package clause.
	PkgName string
	hasName bool

	// ImportPath returns the unique path to identify an imported instance.
	//
	// Instances created with NewInstance do not have an import path.
	ImportPath string

	// Imports lists the instances of all direct imports of this instance.
	Imports []*Instance

	// The Err for loading this package or nil on success. This does not
	// include any errors of dependencies. Incomplete will be set if there
	// were any errors in dependencies.
	Err errors.Error

	// Incomplete reports whether any dependencies had an error.
	Incomplete bool

	parent *Instance // TODO: for cycle detection

	// The following fields are for informative purposes and are not used by
	// the cue package to create an instance.

	// ImportComment is the path in the import comment on the package statement.
	ImportComment string

	// DisplayPath is a user-friendly version of the package or import path.
	DisplayPath string

	// Dir is the package directory. Note that a package may also include files
	// from ancestor directories, up to the module file.
	Dir string

	// Module defines the module name of a package. It must be defined if
	// the packages within the directory structure of the module are to be
	// imported by other packages, including those within the module.
	Module string

	// Root is the root of the directory hierarchy, it may be "" if this an
	// instance has no imports.
	// If Module != "", this corresponds to the module root.
	// Root/pkg is the directory that holds third-party packages.
	Root string // root directory of hierarchy ("" if unknown)

	// AllTags are the build tags that can influence file selection in this
	// directory.
	AllTags []string

	Standard bool // Is a builtin package
	User     bool // True if package was created from individual files.

	// Deprecated: use BuildFiles
	CUEFiles []string // .cue source files
	// Deprecated: use BuildFiles and OrphanedFiles
	DataFiles []string // recognized data files (.json, .yaml, etc.)

	// The intent is to also deprecate the following fields in favor of
	// IgnoredFiles and UnknownFiles.
	TestCUEFiles    []string // .cue test files (_test.cue)
	ToolCUEFiles    []string // .cue tool files (_tool.cue)
	IgnoredCUEFiles []string // .cue source files ignored for this build
	InvalidCUEFiles []string // .cue source files with detected problems (parse error, wrong package name, and so on)

	// Dependencies
	ImportPaths []string
	ImportPos   map[string][]token.Pos // line information for Imports

	Deps       []string
	DepsErrors []error
	Match      []string
}

// Dependencies reports all Instances on which this instance depends.
func (inst *Instance) Dependencies() []*Instance {
	// TODO: as cyclic dependencies are not allowed, we could just not check.
	// Do for safety now and remove later if needed.
	return appendDependencies(nil, inst, map[*Instance]bool{})
}

func appendDependencies(a []*Instance, inst *Instance, done map[*Instance]bool) []*Instance {
	for _, d := range inst.Imports {
		if done[d] {
			continue
		}
		a = append(a, d)
		done[d] = true
		a = appendDependencies(a, d, done)
	}
	return a
}

// Abs converts relative path used in the one of the file fields to an
// absolute one.
func (inst *Instance) Abs(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(inst.Root, path)
}

func (inst *Instance) setPkg(pkg string) bool {
	if !inst.hasName {
		inst.hasName = true
		inst.PkgName = pkg
		return true
	}
	return false
}

// ReportError reports an error processing this instance.
func (inst *Instance) ReportError(err errors.Error) {
	inst.Err = errors.Append(inst.Err, err)
}

// Context defines the build context for this instance. All files defined
// in Syntax as well as all imported instances must be created using the
// same build context.
func (inst *Instance) Context() *Context {
	return inst.ctxt
}

func (inst *Instance) parse(name string, src interface{}) (*ast.File, error) {
	if inst.ctxt != nil && inst.ctxt.parseFunc != nil {
		return inst.ctxt.parseFunc(name, src)
	}
	return parser.ParseFile(name, src, parser.ParseComments)
}

// LookupImport defines a mapping from an ImportSpec's ImportPath to Instance.
func (inst *Instance) LookupImport(path string) *Instance {
	path = inst.expandPath(path)
	for _, inst := range inst.Imports {
		if inst.ImportPath == path {
			return inst
		}
	}
	return nil
}

func (inst *Instance) addImport(imp *Instance) {
	for _, inst := range inst.Imports {
		if inst.ImportPath == imp.ImportPath {
			if inst != imp {
				panic("import added multiple times with different instances")
			}
			return
		}
	}
	inst.Imports = append(inst.Imports, imp)
}

// AddFile adds the file with the given name to the list of files for this
// instance. The file may be loaded from the cache of the instance's context.
// It does not process the file's imports. The package name of the file must
// match the package name of the instance.
func (inst *Instance) AddFile(filename string, src interface{}) error {
	file, err := inst.parse(filename, src)
	if err != nil {
		// should always be an errors.List, but just in case.
		err := errors.Promote(err, "error adding file")
		inst.ReportError(err)
		return err
	}

	return inst.AddSyntax(file)
}

// AddSyntax adds the given file to list of files for this instance. The package
// name of the file must match the package name of the instance.
func (inst *Instance) AddSyntax(file *ast.File) errors.Error {
	astutil.Resolve(file, func(pos token.Pos, msg string, args ...interface{}) {
		inst.Err = errors.Append(inst.Err, errors.Newf(pos, msg, args...))
	})
	_, pkg, pos := internal.PackageInfo(file)
	if pkg != "" && pkg != "_" && !inst.setPkg(pkg) && pkg != inst.PkgName {
		err := errors.Newf(pos,
			"package name %q conflicts with previous package name %q",
			pkg, inst.PkgName)
		inst.ReportError(err)
		return err
	}
	inst.Files = append(inst.Files, file)
	return nil
}

func (inst *Instance) expandPath(path string) string {
	isLocal := IsLocalImport(path)
	if isLocal {
		path = dirToImportPath(filepath.Join(inst.Dir, path))
	}
	return path
}

// dirToImportPath returns the pseudo-import path we use for a package
// outside the CUE path. It begins with _/ and then contains the full path
// to the directory. If the package lives in c:\home\gopher\my\pkg then
// the pseudo-import path is _/c_/home/gopher/my/pkg.
// Using a pseudo-import path like this makes the ./ imports no longer
// a special case, so that all the code to deal with ordinary imports works
// automatically.
func dirToImportPath(dir string) string {
	return pathpkg.Join("_", strings.Map(makeImportValid, filepath.ToSlash(dir)))
}

func makeImportValid(r rune) rune {
	// Should match Go spec, compilers, and ../../go/parser/parser.go:/isValidImport.
	const illegalChars = `!"#$%&'()*,:;<=>?[\]^{|}` + "`\uFFFD"
	if !unicode.IsGraphic(r) || unicode.IsSpace(r) || strings.ContainsRune(illegalChars, r) {
		return '_'
	}
	return r
}

// IsLocalImport reports whether the import path is
// a local import path, like ".", "..", "./foo", or "../foo".
func IsLocalImport(path string) bool {
	return path == "." || path == ".." ||
		strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")
}
