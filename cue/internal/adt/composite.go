// Copyright 2020 CUE Authors
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

package adt

import (
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
)

// An Environment links the parent scopes for identifier lookup to a composite
// node. Each conjunct that make up node in the tree can be associated with
// a different environment (although some conjuncts may share an Environment).
type Environment struct {
	Up     *Environment
	Vertex *Vertex

	// CloseID is a unique number that tracks a group of conjuncts that need
	// belong to a single originating definition.
	CloseID uint32

	cache map[Expr]Value
}

// evalCached is used to look up let expressions. Caching let expressions
// prevents a possible combinatorial explosion.
func (e *Environment) evalCached(c *OpContext, x Expr) Value {
	v, ok := e.cache[x]
	if !ok {
		if e.cache == nil {
			e.cache = map[Expr]Value{}
		}
		v = c.eval(x)
		e.cache[x] = v
	}
	return v
}

// A Vertex is a node in the value tree. It may be a leaf or internal node.
// It may have arcs to represent elements of a fully evaluated struct or list.
//
// For structs, it only contains definitions and concrete fields.
// optional fields are dropped.
//
// It maintains source information such as a list of conjuncts that contributed
// to the value.
type Vertex struct {
	Parent *Vertex // Do we need this?

	// Label is the feature leading to this vertex.
	Label Feature

	// Value is the value associated with this vertex. For lists and structs
	// this is a sentinel value indicating its kind.
	Value Value

	// The parent of nodes can be followed to determine the path within the
	// configuration of this node.
	// Value  Value
	Arcs []*Vertex // arcs are sorted in display order.

	// Conjuncts lists the structs that ultimately formed this Composite value.
	// This includes all selected disjuncts. This information is used to compute
	// the topological sort of arcs.
	Conjuncts []Conjunct

	// Structs is a slice of struct literals that contributed to this value.
	Structs []*StructLit

	// Closed contains information about how to interpret field labels for the
	// various conjuncts with respect to which fields are allowed in this
	// Vertex. If allows all fields if it is nil.
	// The evaluator will first check existing fields before using this. So for
	// simple cases, an Acceptor can always return false to close the Vertex.
	Closed Acceptor

	// arcLookup may be provided as a map if arcs is larger than a certain size.
	// arcLookup map[feature]int
}

// Acceptor is a single interface that reports whether feature f is a valid
// field label for this vertex.
//
// TODO(perf): combine this with the StructMarker functionality?
type Acceptor interface {
	Accept(ctx *OpContext, f Feature) bool
}

func (v *Vertex) Kind() Kind {
	return v.Value.Kind()
}

func (v *Vertex) IsList() bool {
	_, ok := v.Value.(*ListMarker)
	return ok
}

// lookup returns the Arc with label f if it exists or nil otherwise.
//
// It returns a pointer to the original arc to allow updating the cache.
// The pointer should not be used after inserting other arcs into the structure.
func (v *Vertex) lookup(f Feature) *Vertex {
	// TODO: special case int for lists.

	for _, a := range v.Arcs {
		if a.Label == f {
			return a
		}
	}
	return nil
}

func (v *Vertex) GetArc(f Feature) (arc *Vertex, isNew bool) {
	arc = v.lookup(f)
	if arc == nil {
		arc = &Vertex{Parent: v, Label: f}
		v.Arcs = append(v.Arcs, arc)
		isNew = true
	}
	return arc, isNew
}

func (v *Vertex) Source() ast.Node { return nil }

func (v *Vertex) AddConjunct(c Conjunct) *Bottom {
	if v.Value != nil {
		return &Bottom{Err: errors.Newf(token.NoPos, "cannot add conjunct")}
	}
	for _, x := range v.Conjuncts {
		if x == c {
			return nil
		}
	}
	v.Conjuncts = append(v.Conjuncts, c)
	return nil
}

func (v *Vertex) appendListArcs(arcs []*Vertex) (err *Bottom) {
	for _, a := range arcs {
		label, err := MakeLabel(a.Source(), int64(len(v.Arcs)), IntLabel)
		if err != nil {
			return &Bottom{Src: a.Source(), Err: err}
		}
		v.Arcs = append(v.Arcs, &Vertex{
			Parent:    v,
			Label:     label,
			Conjuncts: a.Conjuncts,
		})
	}
	return nil
}

// An Conjunct is an Environment-Expr pair. The Environment is the starting point
// for reference lookup for any reference contained in X.
type Conjunct struct {
	Env *Environment
	x   Node
}

// TODO(perf): replace with composite literal if this helps performance.
func MakeConjunct(env *Environment, x Node) Conjunct {
	switch x.(type) {
	case Expr, interface{ expr() Expr }:
	default:
		panic("invalid Node type")
	}
	return Conjunct{env, x}
}

func (c *Conjunct) Source() ast.Node {
	return c.x.Source()
}

func (c *Conjunct) Expr() Expr {
	switch x := c.x.(type) {
	case Expr:
		return x
	case interface{ expr() Expr }:
		return x.expr()
	default:
		panic("unreachable")
	}
}
