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

package parser

import (
	"sort"
	"strconv"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/token"
)

// sortImports sorts runs of consecutive import lines in import blocks in f.
// It also removes duplicate imports when it is possible to do so without data loss.
func sortImports(f *ast.File) {
	for _, d := range f.Decls {
		d, ok := d.(*ast.ImportDecl)
		if !ok {
			// Not an import declaration, so we're done.
			// Imports are always first.
			break
		}

		if !d.Lparen.IsValid() {
			// Not a block: sorted by default.
			continue
		}

		// Identify and sort runs of specs on successive lines.
		i := 0
		specs := d.Specs[:0]
		for j, s := range d.Specs {
			if j > i && s.Pos().Line() > 1+d.Specs[j-1].End().Line() {
				// j begins a new run. End this one.
				specs = append(specs, sortSpecs(f, d.Specs[i:j])...)
				i = j
			}
		}
		specs = append(specs, sortSpecs(f, d.Specs[i:])...)
		d.Specs = specs

		// Deduping can leave a blank line before the rparen; clean that up.
		if len(d.Specs) > 0 {
			lastSpec := d.Specs[len(d.Specs)-1]
			lastLine := lastSpec.Pos().Line()
			rParenLine := d.Rparen.Line()
			for rParenLine > lastLine+1 {
				rParenLine--
				d.Rparen.File().MergeLine(rParenLine)
			}
		}
	}
}

func importPath(s *ast.ImportSpec) string {
	t, err := strconv.Unquote(s.Path.Value)
	if err == nil {
		return t
	}
	return ""
}

func importName(s *ast.ImportSpec) string {
	n := s.Name
	if n == nil {
		return ""
	}
	return n.Name
}

func importComment(s *ast.ImportSpec) string {
	for _, c := range s.Comments() {
		if c.Line {
			return c.Text()
		}
	}
	return ""
}

// collapse indicates whether prev may be removed, leaving only next.
func collapse(prev, next *ast.ImportSpec) bool {
	if importPath(next) != importPath(prev) || importName(next) != importName(prev) {
		return false
	}
	for _, c := range prev.Comments() {
		if !c.Doc {
			return false
		}
	}
	return true
}

type posSpan struct {
	Start token.Pos
	End   token.Pos
}

func sortSpecs(f *ast.File, specs []*ast.ImportSpec) []*ast.ImportSpec {
	// Can't short-circuit here even if specs are already sorted,
	// since they might yet need deduplication.
	// A lone import, however, may be safely ignored.
	if len(specs) <= 1 {
		return specs
	}

	// Record positions for specs.
	pos := make([]posSpan, len(specs))
	for i, s := range specs {
		pos[i] = posSpan{s.Pos(), s.End()}
	}

	// Sort the import specs by import path.
	// Remove duplicates, when possible without data loss.
	// Reassign the import paths to have the same position sequence.
	// Reassign each comment to abut the end of its spec.
	// Sort the comments by new position.
	sort.Sort(byImportSpec(specs))

	// Dedup. Thanks to our sorting, we can just consider
	// adjacent pairs of imports.
	deduped := specs[:0]
	for i, s := range specs {
		if i == len(specs)-1 || !collapse(s, specs[i+1]) {
			deduped = append(deduped, s)
		} else {
			p := s.Pos()
			p.File().MergeLine(p.Line())
		}
	}
	specs = deduped

	return specs
}

type byImportSpec []*ast.ImportSpec

func (x byImportSpec) Len() int      { return len(x) }
func (x byImportSpec) Swap(i, j int) { x[i], x[j] = x[j], x[i] }
func (x byImportSpec) Less(i, j int) bool {
	ipath := importPath(x[i])
	jpath := importPath(x[j])
	if ipath != jpath {
		return ipath < jpath
	}
	iname := importName(x[i])
	jname := importName(x[j])
	if iname != jname {
		return iname < jname
	}
	return importComment(x[i]) < importComment(x[j])
}
