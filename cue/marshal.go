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

package cue

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"path/filepath"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/token"
)

// root.
type instanceData struct {
	Root  bool
	Path  string
	Files []fileData
}

type fileData struct {
	Name string
	Data []byte
}

const version = 1

type unmarshaller struct {
	ctxt    *build.Context
	imports map[string]*instanceData
}

func (b *unmarshaller) load(pos token.Pos, path string) *build.Instance {
	bi := b.imports[path]
	if bi == nil {
		return nil
	}
	return b.build(bi)
}

func (b *unmarshaller) build(bi *instanceData) *build.Instance {
	p := b.ctxt.NewInstance(bi.Path, b.load)
	p.ImportPath = bi.Path
	for _, f := range bi.Files {
		_ = p.AddFile(f.Name, f.Data)
	}
	p.Complete()
	return p
}

func compileInstances(r *Runtime, data []*instanceData) (instances []*Instance, err error) {
	b := unmarshaller{
		ctxt:    r.buildContext(),
		imports: map[string]*instanceData{},
	}
	for _, i := range data {
		if i.Path == "" {
			if !i.Root {
				return nil, errors.Newf(token.NoPos,
					"data contains non-root package without import path")
			}
			continue
		}
		b.imports[i.Path] = i
	}

	builds := []*build.Instance{}
	for _, i := range data {
		if !i.Root {
			continue
		}
		builds = append(builds, b.build(i))
	}

	return r.build(builds)
}

// Unmarshal creates an Instance from bytes generated by the MarshalBinary
// method of an instance.
func (r *Runtime) Unmarshal(b []byte) ([]*Instance, error) {
	if len(b) == 0 {
		return nil, errors.Newf(token.NoPos, "unmarshal failed: empty buffer")
	}

	switch b[0] {
	case version:
	default:
		return nil, errors.Newf(token.NoPos,
			"unmarshal failed: unsupported version %d, regenerate data", b[0])
	}

	reader, err := gzip.NewReader(bytes.NewReader(b[1:]))
	if err != nil {
		return nil, errors.Newf(token.NoPos, "unmarshal failed: %v", err)
	}

	data := []*instanceData{}
	err = gob.NewDecoder(reader).Decode(&data)
	if err != nil {
		return nil, errors.Newf(token.NoPos, "unmarshal failed: %v", err)
	}

	return compileInstances(r, data)
}

// Marshal creates bytes from a group of instances. Imported instances will
// be included in the emission.
//
// The stored instances are functionally the same, but preserving of file
// information is only done on a best-effort basis.
func (r *Runtime) Marshal(instances ...*Instance) (b []byte, err error) {
	ctx := r.index().newContext()

	staged := []instanceData{}
	done := map[string]int{}

	var errs errors.Error

	var stageInstance func(i *Instance) (pos int)
	stageInstance = func(i *Instance) (pos int) {
		if p, ok := done[i.ImportPath]; ok {
			return p
		}
		n, imports := export(ctx, i.rootValue, options{raw: true})

		file, ok := n.(*ast.File)
		if !ok {
			file = &ast.File{}
			if obj, ok := n.(*ast.StructLit); ok {
				file.Decls = append(file.Decls, obj.Elts...)
			} else {
				file.Decls = append(file.Decls, &ast.EmbedDecl{Expr: n.(ast.Expr)})
			}
		}
		if i.Name != "" {
			pkg := &ast.Package{Name: ast.NewIdent(i.Name)}
			file.Decls = append([]ast.Decl{pkg}, file.Decls...)
		}

		b, err := format.Node(file)
		errs = errors.Append(errs, errors.Promote(err, "marshal"))

		filename := "unmarshal"
		if i.inst != nil && len(i.inst.Files) == 1 {
			filename = i.inst.Files[0].Filename

			dir := i.Dir
			if i.inst != nil && i.inst.Root != "" {
				dir = i.inst.Root
			}
			if dir != "" {
				filename, _ = filepath.Rel(dir, filename)
				filename = filepath.ToSlash(filename)
			}
		}

		staged = append(staged, instanceData{
			Path:  i.ImportPath,
			Files: []fileData{{filename, b}},
		})

		p := len(staged) - 1

		for _, imp := range imports {
			i := ctx.importsByPath[imp]
			if i == nil {
				continue // a builtin package.
			}
			stageInstance(i)
		}

		return p
	}

	for _, i := range instances {
		staged[stageInstance(i)].Root = true
	}

	buf := &bytes.Buffer{}
	buf.WriteByte(version)

	zw := gzip.NewWriter(buf)
	if err := gob.NewEncoder(zw).Encode(staged); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil

}
