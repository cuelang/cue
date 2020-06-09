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
	"sort"

	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/internal/adt"
)

// Nodes man not reenter a disjunction.
//
// Copy one layer deep; throw away items on failure.

// DISJUNCTION ALGORITHM
//
// The basic concept of the algorithm is to use backtracking to find valid
// disjunctions. The algorithm can stop if two matching disjuncts are found
// where one does not subsume the other.
//
// At a later point, we can introduce a filter step to filter out possible
// disjuncts based on, say, discriminator fields or field exclusivity (oneOf
// fields in Protobuf).
//
// To understand the details of the algorithm, it is important to understand
// some properties of disjunction.
//
//
// EVALUATION OF A DISJUNCTION IS SELF CONTAINED
//
// In other words, fields outside of a disjunction cannot bind to values within
// a disjunction whilst evaluating that disjunction. This allows the computation
// of disjunctions to be isolated from side effects.
//
// The intuition behind this is as follows: as a disjunction is not a concrete
// value, it is not possible to lookup a field within a disjunction if it has
// not yet been evaluated. So if a reference within a disjunction that is needed
// to disambiguate that disjunction refers to a field outside the scope of the
// disjunction which, in turn, refers to a field within the disjunction, this
// results in a cycle error. We achieve this by not removing the cycle marker of
// the Vertex of the disjunction until the disjunction is resolved.
//
// Note that the following disjunct is still allowed:
//
//    a: 1
//    b: a
//
// Even though `a` refers to the root of the disjunction, it does not _select
// into_ the disjunction. Implementation-wise, it also doesn't have to, as the
// respective vertex is available within the Environment. Referencing a node
// outside the disjunction that in turn selects the disjunction root, however,
// will result in a detected cycle.
//
// As usual, cycle detection should be interpreted marked as incomplete, so that
// the referring node will not be fixed to an error prematurely.
//
//
// SUBSUMPTION OF AMBIGUOUS DISJUNCTS
//
// A disjunction can be evaluated to a concrete value if only one disjunct
// remains. Aside from disambiguating through unification failure, disjuncts
// may also be disambiguated by taking the least specific of two disjuncts.
// For instance, if a subsumes b, then the result of disjunction may be a.
//
//   NEW ALGORITHM NO LONGER VERIFIES SUBSUMPTION. SUBSUMPTION IS INHERENTLY
//   IMPRECISE (DUE TO BULK OPTIONAL FIELDS). OTHER THAN THAT, FOR SCALAR VALUES
//   IT JUST MEANS THERE IS AMBIGUITY, AND FOR STRUCTS IT CAN LEAD TO STRANGE
//   CONSEQUENCES.
//
//   USE EQUALITY INSTEAD:
//     - Undefined == error for optional fields.
//     - So only need to check exact labels for vertices.

type envDisjunct struct {
	env         *adt.Environment
	values      []disjunct
	numDefaults int
	cloneID     uint32
	isEmbed     bool
}

type disjunct struct {
	expr      adt.Expr
	isDefault bool
}

func (n *nodeContext) addDisjunction(env *adt.Environment, x *adt.Disjunction, cloneID uint32, isEmbed bool) {
	a := []disjunct{}

	numDefaults := 0
	for _, v := range x.Values {
		isDef := v.Default || n.hasDefaults(env, v.Val)
		if isDef {
			numDefaults++
		}
		a = append(a, disjunct{v.Val, isDef})
	}

	sort.SliceStable(a, func(i, j int) bool {
		return !a[j].isDefault && a[i].isDefault != a[j].isDefault
	})

	n.disjunctions = append(n.disjunctions,
		envDisjunct{env, a, numDefaults, cloneID, isEmbed})
}

// TODO(fix): by evaluating disjunction recursively within tryDisjuncts, the defaults can be properly computed and this function is unnecessary.
func (n *nodeContext) hasDefaults(env *adt.Environment, expr adt.Expr) (has bool) {
	switch x := expr.(type) {
	case *adt.BinaryExpr:
		if x.Op == adt.AndOp {
			return n.hasDefaults(env, x.X) || n.hasDefaults(env, x.Y)
		}

	case *adt.Disjunction:
		if x.HasDefaults {
			return true
		}
		for _, v := range x.Values {
			if n.hasDefaults(env, v.Val) {
				return true
			}
		}

	case adt.Resolver:
		// TODO: right now, associativity is only resolved lexically. That is,
		// for defaults of the form:
		//
		//   a: *foo | ref
		//
		// it is assumed that ref does not resolve to a disjunction with a
		// default value. This violates the the substitution principle and is
		// against the spec. This semantics has some big advantages, though:
		//
		//    1) allows users to override a default quite easily
		//    2) locally clear whether a default applies or not
		//    3) much easier to implement.
		//
		// The will have to be considered carefully, though.
		return false

		// TODO(call):
		//case adt.Evaluator: // For call expressions mostly.
	}

	return false
}

func (n *nodeContext) updateResult() (isFinal bool) {
	n.postDisjunct()

	switch {
	case n.hasErr():
		return n.isFinal

	case !n.nodeShared.hasResult():

	case n.nodeShared.isDefault() && !n.isDefault:
		return n.isFinal

	case !n.nodeShared.isDefault() && n.isDefault:

	default:
		if Equal(n.ctx, n.node, &n.result) {
			return n.isFinal
		}

		// TODO: Compute fancy error message.
		n.nodeShared.resultNode = n
		n.nodeShared.result.Value = &adt.Bottom{
			Code: adt.IncompleteError,
			Err:  errors.Newf(n.ctx.Pos(), "ambiguous disjunction"),
		}
		n.nodeShared.result.Arcs = nil
		n.nodeShared.result.Structs = nil
		return n.isDefault
	}

	n.nodeShared.resultNode = n
	n.nodeShared.result = *n.node

	return n.isFinal
}

func (n *nodeContext) tryDisjuncts() (finished bool) {
	if !n.insertDisjuncts() || !n.updateResult() {
		if !n.isFinal {
			return false // More iterations to do.
		}
	}

	if n.nodeShared.hasResult() {
		return true // found something
	}

	if len(n.disjunctions) > 0 {
		n.node.Value = n.ctx.NewErrf("all disjunctions failed error")
	}
	return true
}

func (n *nodeContext) insertDisjuncts() (inserted bool) {
	p := 0
	inserted = true
	for p < len(n.disjunctions) {

		if p >= len(n.stack) {
			n.stack = append(n.stack, 0)
		}

		d := n.disjunctions[p]
		k := n.stack[p]
		v := d.values[k]

		n.isFinal = n.isFinal && k == len(d.values)-1
		n.isDefault = n.isDefault && (v.isDefault || d.numDefaults == 0)

		n.addExprConjunct(adt.MakeConjunct(d.env, v.expr), d.cloneID, d.isEmbed)

		for n.expandOne() {
		}

		p++

		if n.hasErr() {
			inserted = false
			break
		}
	}

	// Find last disjunction at which there is no overflow.
	for ; p > 0 && n.stack[p-1]+1 >= len(n.disjunctions[p-1].values); p-- {
	}
	if p > 0 {
		// Increment a valid position and set all subsequent entries to 0.
		n.stack[p-1]++
		n.stack = n.stack[:p]
	}
	return inserted
}
