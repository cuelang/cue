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

package eval

import (
	"cuelang.org/go/internal/core/adt"
)

// TODO: find better name than Finalize:
//  - concretize

// FinalizeAll recursively finalizes all values in the Vertex v.
func FinalizeAll(r adt.Runtime, v *adt.Vertex) *adt.Vertex {
	e := New(r)
	c := adt.NewContext(r, e, v)
	return e.finalizeAll(c, v)
}

// FinalizeValue returns a new Vertex that representing only the concrete values
// of a Vertex.
func FinalizeValue(r adt.Runtime, v *adt.Vertex) *adt.Vertex {
	e := New(r)
	c := adt.NewContext(r, e, v)
	return e.finalize(c, v, v.Arcs)
}

func (e *Evaluator) finalizeAll(c *adt.OpContext, v *adt.Vertex) *adt.Vertex {
	arcs := make([]*adt.Vertex, len(v.Arcs))

	for i, a := range v.Arcs {
		arcs[i] = e.finalizeAll(c, a)
	}

	return e.finalize(c, v, arcs)
}

func (e *Evaluator) finalize(c *adt.OpContext, v *adt.Vertex, arcs []*adt.Vertex) *adt.Vertex {
	// v.Finalize(c)
	// n := e.Evaluate(c, v)

	// for _, c := range v.Conjuncts {
	// 	// if n == c.Expr() {
	// 	// 	return v
	// 	// }
	// }

	w := *v

	w.Arcs = arcs
	// w.MakeData() = true

	// w := &adt.Vertex{
	// 	Parent:  v.Parent,
	// 	Label:   v.Label,
	// 	Value:   v.Value,
	// 	Arcs:    arcs,
	// 	Structs: v.Structs,
	// }
	// if n != v {
	// 	Conjuncts: []adt.Conjunct{
	// 		adt.MakeConjunct(nil, n),
	// 		}, // deliberately leave out, forcing final value.
	// 	}
	// w.UpdateStatus(adt.Finalized)

	// switch x := n.(type) {
	// case *adt.Vertex:
	// 	w.Value = x.Value

	// case adt.SingleValue:
	// 	w.Value = x

	// default:
	// 	panic("unreachable")
	// }

	return &w
}
