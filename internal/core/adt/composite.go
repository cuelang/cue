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
	"fmt"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
)

// TODO: unanswered questions about structural cycles:
//
// 1. When detecting a structural cycle, should we consider this as:
//    a) an unevaluated value,
//    b) an incomplete error (which does not affect parent validity), or
//    c) a special value.
//
// Making it an error is the simplest way to ensure reentrancy is disallowed:
// without an error it would require an additional mechanism to stop reentrancy
// from continuing to process. Even worse, in some cases it may only partially
// evaluate, resulting in unexpected results. For this reason, we are taking
// approach `b` for now.
//
// This has some consequences of how disjunctions are treated though. Consider
//
//     list: {
//        head: _
//        tail: list | null
//     }
//
// When making it an error, evaluating the above will result in
//
//     list: {
//        head: _
//        tail: null
//     }
//
// because list will result in a structural cycle, and thus an error, it will be
// stripped from the disjunction. This may or may not be a desirable property. A
// nice thing is that it is not required to write `list | *null`. A disadvantage
// is that this is perhaps somewhat inexplicit.
//
// When not making it an error (and simply cease evaluating child arcs upon
// cycle detection), the result would be:
//
//     list: {
//        head: _
//        tail: list | null
//     }
//
// In other words, and evaluation would result in a cycle and thus an error.
// Implementations can recognize such cases by having unevaluated arcs. An
// explicit structure cycle marker would probably be less error prone.
//
// Note that in both cases, a reference to list will still use the original
// conjuncts, so the result will be the same for either method in this case.
//
//
// 2. Structural cycle allowance.
//
// Structural cycle detection disallows reentrancy as well. This means one
// cannot use structs for recursive computation. This probably will evaluation
// of some configuration. Given that there is no real alternative yet, we could
// allow structural cycle detection to be optionally disabled.

// An Environment links the parent scopes for identifier lookup to a composite
// node. Each conjunct that make up node in the tree can be associated with
// a different environment (although some conjuncts may share an Environment).
type Environment struct {
	Up     *Environment
	Vertex *Vertex

	// DynamicLabel is only set when instantiating a field from a pattern
	// constraint. It is used to resolve label references.
	DynamicLabel Feature

	// TODO(perf): make the following public fields a shareable struct as it
	// mostly is going to be the same for child nodes.

	// CloseID is a unique number that tracks a group of conjuncts that need
	// belong to a single originating definition.
	CloseID uint32

	// Cyclic indicates a structural cycle was detected for this conjunct or one
	// of its ancestors.
	Cyclic bool

	// Deref keeps track of nodes that should dereference to Vertex. It is used
	// for the detecting structural cycle.
	//
	// The detection algorithm is based on Tomabechi's quasi-destructive graph
	// unification. This detection requires dependencies to be resolved into
	// fully dereferenced vertices. This is not the case in our algorithm:
	// the result of evaluating conjuncts is placed into dereferenced vertices
	// _after_ they are evaluated, but the Environment still points to the
	// non-dereferenced context.
	//
	// In order to be able to detect structural cycles, we need to ensure that
	// at least one node that is part of a cycle in the context in which
	// conjunctions are evaluated dereferences correctly.
	//
	// To only field necessary to detect a structural cycle, however, is
	// the Status field of the Vertex. So rather than dereferencing a node
	// proper, it is sufficient to mimic the Status of the dereferenced nodes
	// to these nodes (will always be EvaluatingArcs).
	Deref []*Vertex

	// Cycles contains vertices for which cycles are detected. It is used
	// for tracking self-references within a structural cycles.
	//
	// Unlike Deref, Cycles does not persist with child nodes.
	// TODO: Cycles is always a tail end of Deref, so this can be optimized.
	Cycles []*Vertex

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
	// Parent links to a parent Vertex. This parent should only be used to
	// access the parent's Label field to find the relative location within a
	// tree.
	Parent *Vertex

	// Label is the feature leading to this vertex.
	Label Feature

	// TODO: move the following status fields to a separate struct.

	// status indicates the evaluation progress of this vertex.
	status VertexStatus

	// EvalCount keeps track of temporary dereferencing during evaluation.
	// If EvalCount > 0, status should be considered to be EvaluatingArcs.
	EvalCount int

	// SelfCount is used for tracking self-references.
	SelfCount int

	// Value is the value associated with this vertex. For lists and structs
	// this is a sentinel value indicating its kind.
	Value Value

	// ChildErrors is the collection of all errors of children.
	ChildErrors *Bottom

	// The parent of nodes can be followed to determine the path within the
	// configuration of this node.
	// Value  Value
	Arcs []*Vertex // arcs are sorted in display order.

	// Conjuncts lists the structs that ultimately formed this Composite value.
	// This includes all selected disjuncts.
	//
	// This value may be nil, in which case the Arcs are considered to define
	// the final value of this Vertex.
	Conjuncts []Conjunct

	// Structs is a slice of struct literals that contributed to this value.
	// This information is used to compute the topological sort of arcs.
	Structs []*StructLit

	// Closed contains information about how to interpret field labels for the
	// various conjuncts with respect to which fields are allowed in this
	// Vertex. If allows all fields if it is nil.
	// The evaluator will first check existing fields before using this. So for
	// simple cases, an Acceptor can always return false to close the Vertex.
	Closed Acceptor
}

// VertexStatus indicates the evaluation progress of a Vertex.
type VertexStatus int8

const (
	// Unprocessed indicates a Vertex has not been processed before.
	// Value must be nil.
	Unprocessed VertexStatus = iota

	// Evaluating means that the current Vertex is being evaluated. If this is
	// encountered it indicates a reference cycle. Value must be nil.
	Evaluating

	// Partial indicates that the result was only partially evaluated. It will
	// need to be fully evaluated to get a complete results.
	//
	// TODO: this currently requires a renewed computation. Cache the
	// nodeContext to allow reusing the computations done so far.
	Partial

	// EvaluatingArcs indicates that the arcs of the Vertex are currently being
	// evaluated. If this is encountered it indicates a structural cycle.
	// Value does not have to be nil
	EvaluatingArcs

	// Finalized means that this node is fully evaluated and that the results
	// are save to use without further consideration.
	Finalized
)

func (v *Vertex) Status() VertexStatus {
	if v.EvalCount > 0 {
		return EvaluatingArcs
	}
	return v.status
}

func (v *Vertex) UpdateStatus(s VertexStatus) {
	if v.status > s {
		panic(fmt.Sprintf("attempt to regress status from %d to %d", v.Status(), s))
	}
	v.status = s
}

// ToVertex wraps v in a new Vertex, if necessary.
func ToVertex(v Value) *Vertex {
	switch x := v.(type) {
	case *Vertex:
		return x
	default:
		n := &Vertex{Value: x}
		n.AddConjunct(MakeConjunct(nil, v))
		return n
	}
}

// Unwrap returns the possibly non-concrete scalar value of v or nil if v is
// a list, struct or of undefined type.
func Unwrap(v Value) Value {
	x, ok := v.(*Vertex)
	if !ok {
		return v
	}
	switch x.Value.(type) {
	case *StructMarker, *ListMarker:
		return v
	default:
		return x.Value
	}
}

// Acceptor is a single interface that reports whether feature f is a valid
// field label for this vertex.
//
// TODO: combine this with the StructMarker functionality?
type Acceptor interface {
	// Accept reports whether a given field is accepted as output.
	// Pass an InvalidLabel to determine whether this is always open.
	Accept(ctx *OpContext, f Feature) bool

	// MatchAndInsert finds the conjuncts for optional fields, pattern
	// constraints, and additional constraints that match f and inserts them in
	// arc. Use f is 0 to match all additional constraints only.
	MatchAndInsert(c *OpContext, arc *Vertex)
}

func (v *Vertex) Kind() Kind {
	return v.Value.Kind()
}

func (v *Vertex) IsClosed(ctx *OpContext) bool {
	switch x := v.Value.(type) {
	case *ListMarker:
		// TODO: use one mechanism.
		if x.IsOpen {
			return false
		}
		if v.Closed == nil {
			return true
		}
		return v.Closed.Accept(ctx, InvalidLabel)

	case *StructMarker:
		if x.NeedClose {
			return true
		}
		if v.Closed == nil {
			return false
		}
		return !v.Closed.Accept(ctx, InvalidLabel)
	}
	return false
}

func (v *Vertex) Accept(ctx *OpContext, f Feature) bool {
	if !v.IsClosed(ctx) || v.Lookup(f) != nil {
		return true
	}
	if v.Closed != nil {
		return v.Closed.Accept(ctx, f)
	}
	return false
}

func (v *Vertex) MatchAndInsert(ctx *OpContext, arc *Vertex) {
	if v.Closed == nil {
		return
	}
	if !v.Accept(ctx, arc.Label) {
		return
	}
	v.Closed.MatchAndInsert(ctx, arc)
}

func (v *Vertex) IsList() bool {
	_, ok := v.Value.(*ListMarker)
	return ok
}

// Lookup returns the Arc with label f if it exists or nil otherwise.
func (v *Vertex) Lookup(f Feature) *Vertex {
	for _, a := range v.Arcs {
		if a.Label == f {
			return a
		}
	}
	return nil
}

// Elems returns the regular elements of a list.
func (v *Vertex) Elems() []*Vertex {
	// TODO: add bookkeeping for where list arcs start and end.
	a := make([]*Vertex, 0, len(v.Arcs))
	for _, x := range v.Arcs {
		if x.Label.IsInt() {
			a = append(a, x)
		}
	}
	return a
}

// GetArc returns a Vertex for the outgoing arc with label f. It creates and
// ads one if it doesn't yet exist.
func (v *Vertex) GetArc(f Feature) (arc *Vertex, isNew bool) {
	arc = v.Lookup(f)
	if arc == nil {
		arc = &Vertex{Parent: v, Label: f}
		v.Arcs = append(v.Arcs, arc)
		isNew = true
	}
	return arc, isNew
}

func (v *Vertex) Source() ast.Node { return nil }

// AddConjunct adds the given Conjuncts to v if it doesn't already exist.
func (v *Vertex) AddConjunct(c Conjunct) *Bottom {
	if v.Value != nil {
		// This is likely a bug in the evaluator and should not happen.
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

// Path computes the sequence of Features leading from the root to of the
// instance to this Vertex.
func (v *Vertex) Path() []Feature {
	return appendPath(nil, v)
}

func appendPath(a []Feature, v *Vertex) []Feature {
	if v.Parent == nil {
		return a
	}
	a = appendPath(a, v.Parent)
	return append(a, v.Label)
}

func (v *Vertex) Add(w *Vertex) {
	if len(w.Conjuncts) > 0 {
		for _, c := range w.Conjuncts {
			v.AddConjunct(c)
		}
	} else {
		st := &StructLit{}
		v.AddConjunct(MakeConjunct(nil, st))
		for _, a := range w.Arcs {
			st.Decls = append(st.Decls, &Field{
				Label: a.Label,
				Value: a,
			})
		}
	}
}

func (v *Vertex) appendListArcs(arcs []*Vertex) (err *Bottom) {
	for _, a := range arcs {
		// TODO(list): BUG this only works if lists do not have definitions
		// fields.
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

// An Conjunct is an Environment-Expr pair. The Environment is the starting
// point for reference lookup for any reference contained in X.
type Conjunct struct {
	Env *Environment
	x   Node
}

// TODO(perf): replace with composite literal if this helps performance.

// MakeConjunct creates a conjunct from the given environment and node.
// It panics if x cannot be used as an expression.
func MakeConjunct(env *Environment, x Node) Conjunct {
	switch x.(type) {
	case Expr, interface{ expr() Expr }:
	default:
		panic(fmt.Sprintf("invalid Node type %T", x))
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
