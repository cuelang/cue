// Copyright 2021 CUE Authors
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

package eval

import (
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/debug"
)

func Evaluate(r adt.Runtime, v *adt.Vertex) {
	format := func(n adt.Node) string {
		return debug.NodeString(r, n, printConfig)
	}
	e := adt.NewEngine(r)
	c := adt.New(v, &adt.Config{
		Runtime: r,
		Unifier: e,
		Format:  format,
	})
	e.Unify(c, v, adt.Finalized)
}

func New(r adt.Runtime) *Engine {
	return &Engine{r: r, e: adt.NewEngine(r)}
}

func NewEngine(r adt.Runtime) *Engine {
	return &Engine{r: r, e: adt.NewEngine(r)}
}

type Engine struct {
	r adt.Runtime
	e *adt.Engine
}

func (e *Engine) Evaluate(ctx *adt.OpContext, v *adt.Vertex) adt.Value {
	return e.e.Evaluate(ctx, v)
}

func (e *Engine) Unify(ctx *adt.OpContext, v *adt.Vertex, state adt.VertexStatus) {
	e.e.Unify(ctx, v, state)
}

func (e *Engine) Stats() *adt.Stats {
	return e.e.Stats()
}

// TODO: Note: NewContext takes essentially a cue.Value. By making this
// type more central, we can perhaps avoid context creation.
func NewContext(r adt.Runtime, v *adt.Vertex) *adt.OpContext {
	e := NewEngine(r)
	return e.NewContext(v)
}

func (e *Engine) NewContext(v *adt.Vertex) *adt.OpContext {
	format := func(n adt.Node) string {
		return debug.NodeString(e.r, n, printConfig)
	}
	return adt.New(v, &adt.Config{
		Runtime: e.r,
		Unifier: e.e,
		Format:  format,
	})
}

var printConfig = &debug.Config{Compact: true}
