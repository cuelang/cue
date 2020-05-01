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
)

// An Environment links the parent scopes for identifier lookup to a composite
// node. Each conjunct that make up node in the tree can be associated with
// a different environment (although some conjuncts may share an Environment).
type Environment struct {
	Up     *Environment
	Vertex *Vertex
	cache  map[Expr]Value
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

	// Def Feature // set if Arc originated from definition.

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

	// Temporary scratch area.
	// env *Environment

	// Error context: information for better error messages.
	// optionalFields []feature

	// arcLookup may be provided as a map if arcs is larger than a certain size.
	// arcLookup map[feature]int
}

func (v *Vertex) Kind() Kind {
	return v.Value.Kind()
}

// Lookup returns the Arc with label f if it exists or nil otherwise.
//
// It returns a pointer to the original arc to allow updating the cache.
// The pointer should not be used after inserting other arcs into the structure.
func (v *Vertex) Lookup(f Feature) *Vertex {
	// TODO: special case int for lists.

	for _, a := range v.Arcs {
		if a.Label == f {
			return a
		}
	}
	return nil
}

func (v *Vertex) GetArc(parent *Vertex, f Feature) *Vertex {
	arc := v.Lookup(f)
	if arc == nil {
		arc = &Vertex{Parent: parent, Label: f}
		v.Arcs = append(v.Arcs, arc)
	}
	return arc
}

func (v *Vertex) Source() ast.Node { return nil }
func (v *Vertex) AddConjunct(c Conjunct) *Bottom {
	for _, x := range v.Conjuncts {
		if x == c {
			return nil
		}
	}
	if v.Value != nil {
		return &Bottom{} // Connot add conjunct.
	}
	v.Conjuncts = append(v.Conjuncts, c)
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
