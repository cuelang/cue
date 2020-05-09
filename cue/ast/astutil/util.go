// Copyright 2019 CUE Authors
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

package astutil

import (
	"path"
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/token"
)

func ImportPathName(id string) string {
	name := path.Base(id)
	if p := strings.LastIndexByte(name, ':'); p > 0 {
		name = name[p+1:]
	}
	return name
}

// ImportInfo returns the name and full path of an ImportSpec.
func ImportInfo(spec *ast.ImportSpec) (name, id string) {
	s, _ := strconv.Unquote(spec.Path.Value)
	if spec.Name != nil {
		return spec.Name.Name, s
	}
	name = ImportPathName(s)
	return name, s
}

// CopyComments associates comments of one node with another.
// It may change the relative position of comments.
func CopyComments(to, from ast.Node) {
	if from == nil {
		return
	}
	ast.SetComments(to, from.Comments())
}

// CopyPosition sets the position of one node to another.
func CopyPosition(to, from ast.Node) {
	if from == nil {
		return
	}
	ast.SetPos(to, from.Pos())
}

// CopyMeta copies comments and position information from one node to another.
// It returns the destination node.
func CopyMeta(to, from ast.Node) ast.Node {
	if from == nil {
		return to
	}
	ast.SetComments(to, from.Comments())
	ast.SetPos(to, from.Pos())
	return to
}

// insertImport looks up an existing import with the given name and path or will
// add spec if it doesn't exist. It returns a spec in decls matching spec.
func insertImport(decls *[]ast.Decl, spec *ast.ImportSpec) *ast.ImportSpec {
	name, id := ImportInfo(spec)

	a := *decls

	var imports *ast.ImportDecl
	var orig *ast.ImportSpec
	i := 0
outer:
	for ; i < len(a); i++ {
		d := a[i]
		switch t := d.(type) {
		default:
			break outer

		case *ast.Package:
		case *ast.CommentGroup:
		case *ast.ImportDecl:
			imports = t
			for _, s := range t.Specs {
				x, y := ImportInfo(s)
				if y != id {
					continue
				}
				orig = s
				if name == "" || x == name {
					return s
				}
			}
		}
	}

	// Import not found, add one.
	if imports == nil {
		imports = &ast.ImportDecl{}
		preamble := append(a[:i:i], imports)
		a = append(preamble, a[i:]...)
		*decls = a
	}

	if orig != nil {
		CopyComments(spec, orig)
	}
	imports.Specs = append(imports.Specs, spec)
	ast.SetRelPos(imports.Specs[0], token.NoRelPos)

	return spec
}
