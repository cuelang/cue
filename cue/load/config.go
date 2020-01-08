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

package load

import (
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
)

const (
	cueSuffix  = ".cue"
	modDir     = "cue.mod"
	configFile = "module.cue"
	pkgDir     = "pkg"
)

// FromArgsUsage is a partial usage message that applications calling
// FromArgs may wish to include in their -help output.
//
// Some of the aspects of this documentation, like flags and handling '--' need
// to be implemented by the tools.
const FromArgsUsage = `
<args> is a list of arguments denoting a set of instances.
It may take one of two forms:

1. A list of *.cue source files.

   All of the specified files are loaded, parsed and type-checked
   as a single instance.

2. A list of relative directories to denote a package instance.

   Each directory matching the pattern is loaded as a separate instance.
   The instance contains all files in this directory and ancestor directories,
   up to the module root, with the same package name. The package name must
   be either uniquely determined by the files in the given directory, or
   explicitly defined using the '-p' flag.

   Files without a package clause are ignored.

   Files ending in *_test.cue files are only loaded when testing.

3. A list of import paths, each denoting a package.

   The package's directory is loaded from the package cache. The version of the
   package is defined in the modules cue.mod file.

A '--' argument terminates the list of packages.
`

// GenPath reports the directory in which to store generated
// files.
func GenPath(root string) string {
	info, err := os.Stat(filepath.Join(root, modDir))
	if err == nil && info.IsDir() {
		// TODO(legacy): support legacy cue.mod file.
		return filepath.Join(root, modDir, "gen")
	}
	return filepath.Join(root, "pkg")
}

// A Config configures load behavior.
type Config struct {
	// Context specifies the context for the load operation.
	// If the context is cancelled, the loader may stop early
	// and return an ErrCancelled error.
	// If Context is nil, the load cannot be cancelled.
	Context *build.Context

	loader *loader

	// A Module is a collection of packages and instances that are within the
	// directory hierarchy rooted at the module root. The module root can be
	// marked with a cue.mod file.
	ModuleRoot string

	// Module specifies the module prefix. If not empty, this value must match
	// the module field of an existing cue.mod file.
	Module string

	// Package defines the name of the package to be loaded. In this is not set,
	// the package must be uniquely defined from its context.
	Package string

	// Dir is the directory in which to run the build system's query tool
	// that provides information about the packages.
	// If Dir is empty, the tool is run in the current directory.
	Dir string

	// The build and release tags specify build constraints that should be
	// considered satisfied when processing +build lines. Clients creating a new
	// context may customize BuildTags, which defaults to empty, but it is
	// usually an error to customize ReleaseTags, which defaults to the list of
	// CUE releases the current release is compatible with.
	BuildTags   []string
	releaseTags []string

	// If Tests is set, the loader includes not just the packages
	// matching a particular pattern but also any related test packages.
	Tests bool

	// If Tools is set, the loader includes tool files associated with
	// a package.
	Tools bool

	// filesMode indicates that files are specified
	// explicitly on the command line.
	filesMode bool

	// If DataFiles is set, the loader includes entries for directories that
	// have no CUE files, but have recognized data files that could be converted
	// to CUE.
	DataFiles bool

	// StdRoot specifies an alternative directory for standard libaries.
	// This is mostly used for bootstrapping.
	StdRoot string

	// Overlay provides a mapping of absolute file paths to file contents.
	// If the file  with the given path already exists, the parser will use the
	// alternative file contents provided by the map.
	//
	// Overlays provide incomplete support for when a given file doesn't
	// already exist on disk. See the package doc above for more details.
	//
	// If the value must be of type string, []byte, io.Reader, or *ast.File.
	Overlay map[string]Source

	fileSystem

	loadFunc build.LoadFunc
}

func (c *Config) newInstance(pos token.Pos, p importPath) *build.Instance {
	dir, name, err := c.absDirFromImportPath(pos, p)
	i := c.Context.NewInstance(dir, c.loadFunc)
	i.Dir = dir
	i.PkgName = name
	i.DisplayPath = string(p)
	i.ImportPath = string(p)
	i.Root = c.ModuleRoot
	i.Module = c.Module
	i.Err = errors.Append(i.Err, err)

	return i
}

func (c *Config) newRelInstance(pos token.Pos, path, pkgName string) *build.Instance {
	fs := c.fileSystem

	var err errors.Error
	dir := path

	p := c.Context.NewInstance(path, c.loadFunc)
	p.PkgName = pkgName
	p.DisplayPath = filepath.ToSlash(path)
	// p.ImportPath = string(dir) // compute unique ID.
	p.Root = c.ModuleRoot
	p.Module = c.Module

	if isLocalImport(path) {
		p.Local = true
		if c.Dir == "" {
			err = errors.Append(err, errors.Newf(pos, "cwd unknown"))
		}
		dir = filepath.Join(c.Dir, filepath.FromSlash(path))
	}

	if path == "" {
		err = errors.Append(err, errors.Newf(pos,
			"import %q: invalid import path", path))
	} else if path != cleanImport(path) {
		err = errors.Append(err, c.loader.errPkgf(nil,
			"non-canonical import path: %q should be %q", path, pathpkg.Clean(path)))
	}

	if importPath, e := c.importPathFromAbsDir(fsPath(dir), path, c.Package); e != nil {
		// Detect later to keep error messages consistent.
	} else {
		p.ImportPath = string(importPath)
	}

	p.Dir = dir

	if fs.isAbsPath(path) || strings.HasPrefix(path, "/") {
		err = errors.Append(err, errors.Newf(pos,
			"absolute import path %q not allowed", path))
	}
	if err != nil {
		p.Err = errors.Append(p.Err, err)
		p.Incomplete = true
	}

	return p
}

func (c Config) newErrInstance(pos token.Pos, path importPath, err error) *build.Instance {
	i := c.newInstance(pos, path)
	i.Err = errors.Promote(err, "instance")
	return i
}

func toImportPath(dir string) importPath {
	return importPath(filepath.ToSlash(dir))
}

type importPath string

type fsPath string

func (c *Config) importPathFromAbsDir(absDir fsPath, key, name string) (importPath, errors.Error) {
	if c.ModuleRoot == "" {
		return "", errors.Newf(token.NoPos,
			"cannot determine import path for %q (root undefined)", key)
	}

	dir := filepath.Clean(string(absDir))
	if !strings.HasPrefix(dir, c.ModuleRoot) {
		return "", errors.Newf(token.NoPos,
			"cannot determine import path for %q (dir outside of root)", key)
	}

	pkg := filepath.ToSlash(dir[len(c.ModuleRoot):])
	switch {
	case strings.HasPrefix(pkg, "/cue.mod/"):
		pkg = pkg[len("/cue.mod/"):]
		if pkg == "" {
			return "", errors.Newf(token.NoPos,
				"invalid package %q (root of %s)", key, modDir)
		}

		// TODO(legacy): remove.
	case strings.HasPrefix(pkg, "/pkg/"):
		pkg = pkg[len("/pkg/"):]
		if pkg == "" {
			return "", errors.Newf(token.NoPos,
				"invalid package %q (root of %s)", key, pkgDir)
		}

	case c.Module == "":
		return "", errors.Newf(token.NoPos,
			"cannot determine import path for %q (no module)", key)
	default:
		pkg = c.Module + pkg
	}

	return addImportQualifier(importPath(pkg), name)
}

func addImportQualifier(pkg importPath, name string) (importPath, errors.Error) {
	if name != "" {
		s := string(pkg)
		if i := strings.LastIndexByte(s, '/'); i >= 0 {
			s = s[i+1:]
		}
		if i := strings.LastIndexByte(s, ':'); i >= 0 {
			// should never happen, but just in case.
			s = s[i+1:]
			if s != name {
				return "", errors.Newf(token.NoPos,
					"non-matching package names (%s != %s)", s, name)
			}
		} else if s != name {
			pkg += importPath(":" + name)
		}
	}

	return pkg, nil
}

// absDirFromImportPath converts a giving import path to an absolute directory
// and a package name. The root directory must be set.
//
// The returned directory may not exist.
func (c *Config) absDirFromImportPath(pos token.Pos, p importPath) (absDir, name string, err errors.Error) {
	if c.ModuleRoot == "" {
		return "", "", errors.Newf(pos, "cannot import %q (root undefined)", p)
	}

	// Extract the package name.

	name = string(p)
	switch i := strings.LastIndexAny(name, "/:"); {
	case i < 0:
	case p[i] == ':':
		name = string(p[i+1:])
		p = p[:i]

	default: // p[i] == '/'
		name = string(p[i+1:])
	}

	// TODO: fully test that name is a valid identifier.
	if name == "" {
		err = errors.Newf(pos, "empty package name in import path %q", p)
	} else if strings.IndexByte(name, '.') >= 0 {
		err = errors.Newf(pos,
			"cannot determine package name for %q (set explicitly with ':')", p)
	}

	// Determine the directory.

	sub := filepath.FromSlash(string(p))
	switch hasPrefix := strings.HasPrefix(string(p), c.Module); {
	case hasPrefix && len(sub) == len(c.Module):
		absDir = c.ModuleRoot

	case hasPrefix && p[len(c.Module)] == '/':
		absDir = filepath.Join(c.ModuleRoot, sub[len(c.Module)+1:])

	default:
		absDir = filepath.Join(GenPath(c.ModuleRoot), sub)
	}

	return absDir, name, nil
}

// Complete updates the configuration information. After calling complete,
// the following invariants hold:
//  - c.ModuleRoot != ""
//  - c.Module is set to the module import prefix if there is a cue.mod file
//    with the module property.
//  - c.loader != nil
//  - c.cache != ""
func (c Config) complete() (cfg *Config, err error) {
	// Each major CUE release should add a tag here.
	// Old tags should not be removed. That is, the cue1.x tag is present
	// in all releases >= CUE 1.x. Code that requires CUE 1.x or later should
	// say "+build cue1.x", and code that should only be built before CUE 1.x
	// (perhaps it is the stub to use in that case) should say "+build !cue1.x".
	c.releaseTags = []string{"cue0.1"}

	if c.Dir == "" {
		c.Dir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	} else if c.Dir, err = filepath.Abs(c.Dir); err != nil {
		return nil, err
	}

	// TODO: we could populate this already with absolute file paths,
	// but relative paths cannot be added. Consider what is reasonable.
	if err := c.fileSystem.init(&c); err != nil {
		return nil, err
	}

	// TODO: determine root on a package basis. Maybe we even need a
	// pkgname.cue.mod
	// Look to see if there is a cue.mod.
	if c.ModuleRoot == "" {
		// Only consider the current directory by default
		c.ModuleRoot = c.Dir
		if root := c.findRoot(c.Dir); root != "" {
			c.ModuleRoot = root
		}
	}

	c.loader = &loader{cfg: &c}

	// TODO: also make this work if run from outside the module?
	switch {
	case true:
		mod := filepath.Join(c.ModuleRoot, modDir)
		info, cerr := c.fileSystem.stat(mod)
		if cerr != nil {
			break
		}
		if info.IsDir() {
			mod = filepath.Join(mod, configFile)
		}
		f, cerr := c.fileSystem.openFile(mod)
		if cerr != nil {
			break
		}
		var r cue.Runtime
		inst, err := r.Compile(mod, f)
		if err != nil {
			return nil, errors.Wrapf(err, token.NoPos, "invalid cue.mod file")
		}
		prefix := inst.Lookup("module")
		if prefix.Exists() {
			name, err := prefix.String()
			if err != nil {
				return &c, err
			}
			if c.Module != "" && c.Module != name {
				return &c, errors.Newf(prefix.Pos(), "inconsistent modules: got %q, want %q", name, c.Module)
			}
			c.Module = name
		}
	}

	c.loadFunc = c.loader.loadFunc()

	if c.Context == nil {
		c.Context = build.NewContext(build.Loader(c.loadFunc))
	}

	return &c, nil
}

func (c Config) isRoot(dir string) bool {
	fs := &c.fileSystem
	// Note: cue.mod used to be a file. We still allow both to match.
	_, err := fs.stat(filepath.Join(dir, modDir))
	return err == nil
}

// findRoot returns the module root or "" if none was found.
func (c Config) findRoot(dir string) string {
	fs := &c.fileSystem

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	abs := absDir
	for {
		if c.isRoot(abs) {
			return abs
		}
		d := filepath.Dir(abs)
		if filepath.Base(filepath.Dir(abs)) == modDir {
			// The package was located within a "cue.mod" dir and there was
			// not cue.mod found until now. So there is no root.
			return ""
		}
		if len(d) >= len(abs) {
			break // reached top of file system, no cue.mod
		}
		abs = d
	}
	abs = absDir

	// TODO(legacy): remove this capability at some point.
	for {
		info, err := fs.stat(filepath.Join(abs, pkgDir))
		if err == nil && info.IsDir() {
			return abs
		}
		d := filepath.Dir(abs)
		if len(d) >= len(abs) {
			return "" // reached top of file system, no pkg dir.
		}
		abs = d
	}
}

func home() string {
	env := "HOME"
	if runtime.GOOS == "windows" {
		env = "USERPROFILE"
	} else if runtime.GOOS == "plan9" {
		env = "home"
	}
	return os.Getenv(env)
}
