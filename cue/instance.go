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

package cue

import (
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/internal"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/compile"
	"cuelang.org/go/internal/core/convert"
	"cuelang.org/go/internal/core/eval"
	"cuelang.org/go/internal/core/runtime"
)

// An Instance defines a single configuration based on a collection of
// underlying CUE files.
type Instance struct {
	index *runtime.Runtime

	root *adt.Vertex

	ImportPath  string
	Dir         string
	PkgName     string
	DisplayName string

	Incomplete bool         // true if Pkg and all its dependencies are free of errors
	Err        errors.Error // non-nil if the package had errors

	inst *build.Instance

	// complete bool // for cycle detection
}

func addInst(x *runtime.Runtime, p *Instance) *Instance {
	if p.inst == nil {
		p.inst = &build.Instance{
			ImportPath: p.ImportPath,
			PkgName:    p.PkgName,
		}
	}
	// fmt.Println(p.ImportPath, "XXX")
	x.AddInst(p.ImportPath, p.root, p.inst)
	x.Loaded[p.inst] = p
	p.index = x
	return p
}

func lookupInstance(x *runtime.Runtime, p *build.Instance) *Instance {
	if x, ok := x.Loaded[p]; ok {
		return x.(*Instance)
	}
	return nil
}

func getImportFromBuild(x *runtime.Runtime, p *build.Instance, v *adt.Vertex) *Instance {
	inst := lookupInstance(x, p)

	if inst != nil {
		return inst
	}

	inst = &Instance{
		ImportPath:  p.ImportPath,
		Dir:         p.Dir,
		PkgName:     p.PkgName,
		DisplayName: p.ImportPath,
		root:        v,
		inst:        p,
		index:       x,
	}
	if p.Err != nil {
		inst.setListOrError(p.Err)
	}

	x.Loaded[p] = inst

	return inst
}

func getImportFromNode(x *runtime.Runtime, v *adt.Vertex) *Instance {
	p := x.GetInstanceFromNode(v)
	if p == nil {
		return nil
	}

	return getImportFromBuild(x, p, v)
}

func getImportFromPath(x *runtime.Runtime, id string) *Instance {
	node, _ := x.LoadImport(id)
	if node == nil {
		return nil
	}
	b := x.GetInstanceFromNode(node)
	inst := lookupInstance(x, b)
	if inst == nil {
		inst = &Instance{
			ImportPath: b.ImportPath,
			PkgName:    b.PkgName,
			root:       node,
			inst:       b,
			index:      x,
		}
	}
	return inst
}

func init() {
	internal.MakeInstance = func(value interface{}) interface{} {
		v := value.(Value)
		x := v.eval(v.ctx())
		st, ok := x.(*adt.Vertex)
		if !ok {
			st = &adt.Vertex{}
			st.AddConjunct(adt.MakeRootConjunct(nil, x))
		}
		return addInst(v.idx, &Instance{
			root: st,
		})
	}
}

// newInstance creates a new instance. Use Insert to populate the instance.
func newInstance(x *runtime.Runtime, p *build.Instance, v *adt.Vertex) *Instance {
	// TODO: associate root source with structLit.
	inst := &Instance{
		root: v,
		inst: p,
	}
	if p != nil {
		inst.ImportPath = p.ImportPath
		inst.Dir = p.Dir
		inst.PkgName = p.PkgName
		inst.DisplayName = p.ImportPath
		if p.Err != nil {
			inst.setListOrError(p.Err)
		}
	}

	x.AddInst(p.ImportPath, v, p)
	x.Loaded[p] = inst
	inst.index = x
	return inst
}

func (inst *Instance) setListOrError(err errors.Error) {
	inst.Incomplete = true
	inst.Err = errors.Append(inst.Err, err)
}

func (inst *Instance) setError(err errors.Error) {
	inst.Incomplete = true
	inst.Err = errors.Append(inst.Err, err)
}

func (inst *Instance) eval(ctx *context) adt.Value {
	// TODO: remove manifest here?
	v := manifest(ctx, inst.root)
	return v
}

func init() {
	internal.EvalExpr = func(value, expr interface{}) interface{} {
		v := value.(Value)
		e := expr.(ast.Expr)
		ctx := newContext(v.idx)
		return newValueRoot(v.idx, ctx, evalExpr(ctx, v.v, e))
	}
}

// pkgID reports a package path that can never resolve to a valid package.
func pkgID() string {
	return "_"
}

// evalExpr evaluates expr within scope.
func evalExpr(ctx *context, scope *adt.Vertex, expr ast.Expr) adt.Value {
	cfg := &compile.Config{
		Scope: scope,
		Imports: func(x *ast.Ident) (pkgPath string) {
			if !isBuiltin(x.Name) {
				return ""
			}
			return x.Name
		},
	}

	c, err := compile.Expr(cfg, ctx, pkgID(), expr)
	if err != nil {
		return &adt.Bottom{Err: err}
	}
	return adt.Resolve(ctx, c)

	// scope.Finalize(ctx) // TODO: not appropriate here.
	// switch s := scope.Value.(type) {
	// case *bottom:
	// 	return s
	// case *adt.StructMarker:
	// default:
	// 	return ctx.mkErr(scope, "instance is not a struct, found %s", scope.Kind())
	// }

	// c := ctx

	// x, err := compile.Expr(&compile.Config{Scope: scope}, c.Runtime, expr)
	// if err != nil {
	// 	return c.NewErrf("could not evaluate %s: %v", c.Str(x), err)
	// }

	// env := &adt.Environment{Vertex: scope}

	// switch v := x.(type) {
	// case adt.Value:
	// 	return v
	// case adt.Resolver:
	// 	r, err := c.Resolve(env, v)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return r

	// case adt.Evaluator:
	// 	e, _ := c.Evaluate(env, x)
	// 	return e

	// }

	// return c.NewErrf("could not evaluate %s", c.Str(x))
}

// ID returns the package identifier that uniquely qualifies module and
// package name.
func (inst *Instance) ID() string {
	if inst == nil || inst.inst == nil {
		return ""
	}
	return inst.inst.ID()
}

// Doc returns the package comments for this instance.
func (inst *Instance) Doc() []*ast.CommentGroup {
	var docs []*ast.CommentGroup
	if inst.inst == nil {
		return nil
	}
	for _, f := range inst.inst.Files {
		if c := internal.FileComment(f); c != nil {
			docs = append(docs, c)
		}
	}
	return docs
}

// Value returns the root value of the configuration. If the configuration
// defines in emit value, it will be that value. Otherwise it will be all
// top-level values.
func (inst *Instance) Value() Value {
	ctx := newContext(inst.index)
	inst.root.Finalize(ctx)
	return newVertexRoot(inst.index, ctx, inst.root)
}

// Eval evaluates an expression within an existing instance.
//
// Expressions may refer to builtin packages if they can be uniquely identified.
func (inst *Instance) Eval(expr ast.Expr) Value {
	ctx := newContext(inst.index)
	v := inst.root
	v.Finalize(ctx)
	result := evalExpr(ctx, v, expr)
	return newValueRoot(inst.index, ctx, result)
}

// DO NOT USE.
//
// Deprecated: do not use.
func Merge(inst ...*Instance) *Instance {
	v := &adt.Vertex{}

	i := inst[0]
	ctx := newContext(i.index)

	// TODO: interesting test: use actual unification and then on K8s corpus.

	for _, i := range inst {
		w := i.Value()
		v.AddConjunct(adt.MakeRootConjunct(nil, w.v.ToDataAll()))
	}
	v.Finalize(ctx)

	p := addInst(i.index, &Instance{
		root: v,
		// complete: true,
	})
	return p
}

// Build creates a new instance from the build instances, allowing unbound
// identifier to bind to the top-level field in inst. The top-level fields in
// inst take precedence over predeclared identifier and builtin functions.
func (inst *Instance) Build(p *build.Instance) *Instance {
	p.Complete()

	idx := inst.index
	r := inst.index

	rErr := r.ResolveFiles(p)

	cfg := &compile.Config{Scope: inst.root}
	v, err := compile.Files(cfg, r, p.ID(), p.Files...)

	v.AddConjunct(adt.MakeRootConjunct(nil, inst.root))

	i := newInstance(idx, p, v)
	if rErr != nil {
		i.setListOrError(rErr)
	}
	if i.Err != nil {
		i.setListOrError(i.Err)
	}

	if err != nil {
		i.setListOrError(err)
	}

	// i.complete = true

	return i
}

func (inst *Instance) value() Value {
	return newVertexRoot(inst.index, newContext(inst.index), inst.root)
}

// Lookup reports the value at a path starting from the top level struct. The
// Exists method of the returned value will report false if the path did not
// exist. The Err method reports if any error occurred during evaluation. The
// empty path returns the top-level configuration struct. Use LookupDef for definitions or LookupField for
// any kind of field.
func (inst *Instance) Lookup(path ...string) Value {
	return inst.value().Lookup(path...)
}

// LookupDef reports the definition with the given name within struct v. The
// Exists method of the returned value will report false if the definition did
// not exist. The Err method reports if any error occurred during evaluation.
func (inst *Instance) LookupDef(path string) Value {
	return inst.value().LookupDef(path)
}

// LookupField reports a Field at a path starting from v, or an error if the
// path is not. The empty path returns v itself.
//
// It cannot look up hidden or unexported fields.
//
// Deprecated: this API does not work with new-style definitions. Use
// FieldByName defined on inst.Value().
func (inst *Instance) LookupField(path ...string) (f FieldInfo, err error) {
	v := inst.value()
	for _, k := range path {
		s, err := v.Struct()
		if err != nil {
			return f, err
		}

		f, err = s.FieldByName(k, true)
		if err != nil {
			return f, err
		}
		if f.IsHidden {
			return f, errNotFound
		}
		v = f.Value
	}
	return f, err
}

// Fill creates a new instance with the values of the old instance unified with
// the given value. It is not possible to update the emit value.
//
// Values may be any Go value that can be converted to CUE, an ast.Expr or
// a Value. In the latter case, it will panic if the Value is not from the same
// Runtime.
func (inst *Instance) Fill(x interface{}, path ...string) (*Instance, error) {
	for i := len(path) - 1; i >= 0; i-- {
		x = map[string]interface{}{path[i]: x}
	}
	a := make([]adt.Conjunct, len(inst.root.Conjuncts))
	copy(a, inst.root.Conjuncts)
	u := &adt.Vertex{Conjuncts: a}

	if v, ok := x.(Value); ok {
		if inst.index != v.idx {
			panic("value of type Value is not created with same Runtime as Instance")
		}
		for _, c := range v.v.Conjuncts {
			u.AddConjunct(c)
		}
	} else {
		ctx := eval.NewContext(inst.index, nil)
		expr := convert.GoValueToExpr(ctx, true, x)
		u.AddConjunct(adt.MakeRootConjunct(nil, expr))
		u.Finalize(ctx)
	}
	inst = addInst(inst.index, &Instance{
		root: u,
		inst: nil,

		// Omit ImportPath to indicate this is not an importable package.
		Dir:        inst.Dir,
		PkgName:    inst.PkgName,
		Incomplete: inst.Incomplete,

		// complete: true,
	})
	return inst, nil
}
