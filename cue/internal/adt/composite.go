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

type Environment struct {
	Up   *Environment
	Node *Composite
}

// A Composite is a fully evaluated struct or list.
//
// For structs, it only contains definitions and concrete fields.
// optional fields are dropped.
type Composite struct {
	// The parent of nodes can be followed to determine the path within the
	// configuration of this node.
	Parent *Composite
	Name   Feature
	Arcs   []Arc // arcs are sorted in display order.

	// definitions []arc // definitions this node conforms to

	// isClosed bool // isValidating or isComplete may be a better name.
	// isList   bool
	// active   bool // no need to wrap if active.

	// Temporary scratch area.
	env *Environment

	// Error context: information for better error messages.
	// optionalFields []feature

	// arcLookup may be provided as a map if arcs is larger than a certain size.
	// arcLookup map[feature]int
}

// func (x *Node) address() value              { return x }
// func (x *Node) concreteness() concreteLevel { return concrete }
// func (x *Node) specialize(k kind) value     { return x }
// func (x *Node) kind() kind {
// 	if x.isList {
// 		return listKind
// 	}
// 	return structKind
// }

// lexical reference is in reference node.
// parent chaining is in node.

// type docInfo struct{}

type Arc struct {
	Label     Feature   // comp/embed/value/bulk
	Conjuncts []EnvExpr // a value, which may be an embedding or comprehension
	Cache     Value     // anything but reference.
	// doc       *docInfo
}

func (a *Arc) addConjunct(env *Environment, v Expr) {
	for _, x := range a.Conjuncts {
		if x.Env == env && x.X == v {
			return
		}
	}
	a.Conjuncts = append(a.Conjuncts, EnvExpr{env, v})
}

type EnvExpr struct {
	Env *Environment
	X   Expr
}
