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
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal/core/adt"
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
	cloneID     adt.CloseInfo
}

type disjunct struct {
	expr      adt.Expr
	isDefault bool
}

func (n *nodeContext) addDisjunction(env *adt.Environment, x *adt.DisjunctionExpr, cloneID adt.CloseInfo) {
	a := []disjunct{}

	numDefaults := 0
	for _, v := range x.Values {
		isDef := v.Default // || n.hasDefaults(env, v.Val)
		if isDef {
			numDefaults++
		}
		a = append(a, disjunct{v.Val, isDef})
	}

	sort.SliceStable(a, func(i, j int) bool {
		return !a[j].isDefault && a[i].isDefault != a[j].isDefault
	})

	n.disjunctions = append(n.disjunctions,
		envDisjunct{env, a, numDefaults, cloneID})
}

func (n *nodeContext) addDisjunctionValue(env *adt.Environment, x *adt.Disjunction, cloneID adt.CloseInfo) {
	a := []disjunct{}

	for i, v := range x.Values {
		a = append(a, disjunct{v, i < x.NumDefaults})
	}

	n.disjunctions = append(n.disjunctions,
		envDisjunct{env, a, x.NumDefaults, cloneID})
}

func (n *nodeContext) updateResult(state adt.VertexStatus) {
	n.postDisjunct(state)

	if n.hasErr() {
		x := n.node
		err, ok := x.BaseValue.(*adt.Bottom)
		if !ok {
			err = n.getErr()
		}
		if err == nil {
			// TODO(disjuncts): Is this always correct? Especially for partial
			// evaluation it is okay for child errors to have incomplete errors.
			// Perhaps introduce an Err() method.
			err = x.ChildErrors
		}
		if err != nil {
			n.disjunctErrs = append(n.disjunctErrs, err)
		}
		return
	}

	n.touched = true
	d := &n.nodeShared.disjunct

	result := *n.node
	if result.BaseValue == nil {
		result.BaseValue = n.getValidators()
	}

	for _, v := range d.Values {
		if adt.Equal(n.ctx, v, &result) {
			return
		}
	}

	p := &result
	d.Values = append(d.Values, p)

	if n.done() {
		n.nodeShared.isDone = true
	}

	if n.defaultMode == isDefault {
		// Keep defaults sorted first.
		i := d.NumDefaults
		j := i + 1
		copy(d.Values[j:], d.Values[i:])
		d.Values[i] = p
		d.NumDefaults = j
	}

	switch {
	case !n.nodeShared.hasResult():

	case n.nodeShared.isDefault() && n.defaultMode != isDefault:
		return

	case !n.nodeShared.isDefault() && n.defaultMode == isDefault:

	default:
		return // n.defaultMode == isDefault
	}

	n.nodeShared.setResult(n.node)

	return
}

func (n *nodeContext) processDisjuncts(state adt.VertexStatus) {
	n.processDisjunct(state, 0, len(n.disjunctions))

	if n.nodeShared.hasResult() {
		return // found something
	}

	if len(n.disjunctions) > 0 {
		code := adt.IncompleteError

		if len(n.disjunctErrs) > 0 {
			code = adt.EvalError
			for _, c := range n.disjunctErrs {
				if c.Code > code {
					code = c.Code
				}
			}
		}

		b := &adt.Bottom{
			Code: code,
			Err:  n.disjunctError(),
		}
		n.node.SetValue(n.ctx, adt.Finalized, b)
	}
}

// TODO: move state to nodeShared.
func (n *nodeContext) processDisjunct(state adt.VertexStatus, k, sub int) {
	isSub := false
	var d envDisjunct
	switch {
	case sub < len(n.disjunctions):
		d = n.disjunctions[sub]
		sub++
		isSub = true

	case k < len(n.disjunctions):
		d = n.disjunctions[k]
		k++

	default:
		n.updateResult(state)
		return
	}

	// save current state of node and nodeContext
	nSaved := snapshotVertex(n.node)
	saved := *n

	for i, v := range d.values {
		n.eval.stats.DisjunctCount++

		if i > 0 {
			*n = saved
			*(n.node) = nSaved
			// restore state
		}

		// TODO: HACK ALERT: we ignore the default tags of the subexpression
		// if we already have a scalar value and can no longer change the
		// outcome.
		// This is not conform the spec, but mimics the old implementation.
		// It also results in nicer default semantics. Changing this will
		// break existing CUE code in awkward ways.
		// We probably should address this when we figure out how to change
		// the spec to accommodate for this. For instance, we could say
		// that if a disjunction only contributes a single disjunct to an
		// end result, default information is ignored. Not the greatest
		// definition, though.
		// Another alternative might be to have a special builtin that
		// mimics the good behavior.
		// Note that the same result can be obtained in CUE by adding
		// 0 to a referenced number (forces the default to be discarded).
		wasScalar := n.scalar != nil // Hack line 1

		c := adt.MakeConjunct(d.env, v.expr, d.cloneID)
		n.addExprConjunct(c)

		for n.expandOne() {
		}

		if n.hasErr() {
			continue
		}

		var mode defaultMode
		switch {
		case d.numDefaults == 0:
			mode = maybeDefault
		case v.isDefault:
			mode = isDefault
		default:
			mode = notDefault
		}

		if isSub {
			if !wasScalar { // Hack line 2.
				n.subMode = combineDefault(n.subMode, mode)
			}
		} else if sub == len(n.disjunctions) {
			n.defaultMode = combineDefault(n.defaultMode, n.subMode)
			n.defaultMode = combineDefault(n.defaultMode, mode)
			n.subMode = maybeDefault
		}

		n.processDisjunct(state, k, sub)
	}
}

// Clone makes a shallow copy of a Vertex. The purpose is to create different
// disjuncts from the same Vertex under computation. This allows the conjuncts
// of an arc to be reset to a previous position and the reuse of earlier
// computations.
//
// Notes: only Arcs need to be cloned recursively. Structs is assumed to not yet
// be computed at the time that a Clone is needed and must be nil. Conjuncts no
// longer needed and can become nil. All other fields can be copied shallowly.
//
// USE TO SAVE NODE BRANCH FOR DISJUNCTION, BUT BEFORE POSTDIJSUNCT.
func snapshotVertex(v *adt.Vertex) adt.Vertex {
	c := *v

	if len(v.Arcs) > 0 {
		c.Arcs = make([]*adt.Vertex, len(v.Arcs))
		for i, arc := range v.Arcs {
			// For child arcs, only Conjuncts are set and Arcs and
			// Structs will be nil.
			a := *arc
			c.Arcs[i] = &a

			a.Conjuncts = make([]adt.Conjunct, len(arc.Conjuncts))
			copy(a.Conjuncts, arc.Conjuncts)
		}
	}

	if len(v.Structs) > 0 {
		c.Structs = make([]*adt.StructInfo, len(v.Structs))
		copy(c.Structs, v.Structs)
	}

	return c
}

// // TODO: add proper conjuncts for the ones used by the disjunctions to replace
// // the original source.
// //
// func (n *nodeContext) insertDisjuncts() (inserted bool) {
// 	p := 0
// 	inserted = true

// 	n.subDisjunctions = n.subDisjunctions[:0]

// 	for _, d := range n.disjunctions {
// 		n.subDisjunctions = append(n.subDisjunctions, d)

// 		sub := len(n.disjunctions)
// 		defMode, ok := n.insertSingleDisjunct(p, d, false)
// 		p++
// 		if !ok {
// 			inserted = false
// 			break
// 		}

// 		subMode := maybeDefault
// 		for ; sub < len(n.disjunctions); sub++ {
// 			d := n.disjunctions[sub]

// 			// TODO: HACK ALERT: we ignore the default tags of the subexpression
// 			// if we already have a scalar value and can no longer change the
// 			// outcome.
// 			// This is not conform the spec, but mimics the old implementation.
// 			// It also results in nicer default semantics. Changing this will
// 			// break existing CUE code in awkward ways.
// 			// We probably should address this when we figure out how to change
// 			// the spec to accommodate for this. For instance, we could say
// 			// that if a disjunction only contributes a single disjunct to an
// 			// end result, default information is ignored. Not the greatest
// 			// definition, though.
// 			// Another alternative might be to have a special builtin that
// 			// mimics the good behavior.
// 			// Note that the same result can be obtained in CUE by adding
// 			// 0 to a referenced number (forces the default to be discarded).
// 			wasScalar := n.scalar != nil // Hack line 1

// 			n.subDisjunctions = append(n.subDisjunctions, d)
// 			mode, ok := n.insertSingleDisjunct(p, d, true)
// 			p++
// 			if !ok {
// 				inserted = false
// 				break
// 			}

// 			if !wasScalar { // Hack line 2.
// 				subMode = combineDefault(subMode, mode)
// 			}
// 		}
// 		defMode = combineSubDefault(defMode, subMode)

// 		n.defaultMode = combineDefault(n.defaultMode, defMode)
// 	}

// 	// Find last disjunction at which there is no overflow.
// 	for ; p > 0 && n.stack[p-1]+1 >= len(n.subDisjunctions[p-1].values); p-- {
// 	}
// 	if p > 0 {
// 		// Increment a valid position and set all subsequent entries to 0.
// 		n.stack[p-1]++
// 		n.stack = n.stack[:p]
// 	}
// 	return inserted
// }

// func (n *nodeContext) insertSingleDisjunct(p int, d envDisjunct, isSub bool) (mode defaultMode, ok bool) {
// 	if p >= len(n.stack) {
// 		n.stack = append(n.stack, 0)
// 	}

// 	k := n.stack[p]
// 	v := d.values[k]
// 	n.isFinal = n.isFinal && k == len(d.values)-1
// 	c := adt.MakeConjunct(d.env, v.expr, d.cloneID)
// 	n.addExprConjunct(c)

// 	for n.expandOne() {
// 	}

// 	switch {
// 	case d.numDefaults == 0:
// 		mode = maybeDefault
// 	case v.isDefault:
// 		mode = isDefault
// 	default:
// 		mode = notDefault
// 	}

// 	return mode, !n.hasErr()
// }

// Default rules from spec:
//
// U1: (v1, d1) & v2       => (v1&v2, d1&v2)
// U2: (v1, d1) & (v2, d2) => (v1&v2, d1&d2)
//
// D1: (v1, d1) | v2       => (v1|v2, d1)
// D2: (v1, d1) | (v2, d2) => (v1|v2, d1|d2)
//
// M1: *v        => (v, v)
// M2: *(v1, d1) => (v1, d1)
//
// NOTE: M2 cannot be *(v1, d1) => (v1, v1), as this has the weird property
// of making a value less specific. This causes issues, for instance, when
// trimming.
//
// The old implementation does something similar though. It will discard
// default information after first determining if more than one conjunct
// has survived.
//
// def + maybe -> def
// not + maybe -> def
// not + def   -> def

type defaultMode int

const (
	maybeDefault defaultMode = iota
	notDefault
	isDefault
)

// combineDefaults combines default modes for unifying conjuncts.
//
// Default rules from spec:
//
// U1: (v1, d1) & v2       => (v1&v2, d1&v2)
// U2: (v1, d1) & (v2, d2) => (v1&v2, d1&d2)
func combineDefault(a, b defaultMode) defaultMode {
	if a > b {
		a, b = b, a
	}
	switch {
	case a == maybeDefault && b == maybeDefault:
		return maybeDefault
	case a == maybeDefault && b == notDefault:
		return notDefault
	case a == maybeDefault && b == isDefault:
		return isDefault
	case a == notDefault && b == notDefault:
		return notDefault
	case a == notDefault && b == isDefault:
		return notDefault
	case a == isDefault && b == isDefault:
		return isDefault
	default:
		panic("unreachable")
	}
}

// disjunctError returns a compound error for a failed disjunction.
//
// TODO(perf): the set of errors is now computed during evaluation. Eventually,
// this could be done lazily.
func (n *nodeContext) disjunctError() (errs errors.Error) {
	ctx := n.ctx

	disjuncts := selectErrors(n.disjunctErrs)

	if disjuncts == nil {
		errs = ctx.Newf("empty disjunction")
	} else {
		disjuncts = errors.Sanitize(disjuncts)
		k := len(errors.Errors(disjuncts))
		// prefix '-' to sort to top
		errs = ctx.Newf("%d errors in empty disjunction:", k)
	}

	errs = errors.Append(errs, disjuncts)

	return errs
}

func selectErrors(a []*adt.Bottom) (errs errors.Error) {
	// return all errors if less than a certain number.
	if len(a) <= 2 {
		for _, b := range a {
			errs = errors.Append(errs, b.Err)

		}
		return errs
	}

	// First select only relevant errors.
	isIncomplete := false
	k := 0
	for _, b := range a {
		if !isIncomplete && b.Code >= adt.IncompleteError {
			k = 0
			isIncomplete = true
		}
		a[k] = b
		k++
	}
	a = a[:k]

	// filter errors
	positions := map[token.Pos]bool{}

	add := func(b *adt.Bottom, p token.Pos) bool {
		if positions[p] {
			return false
		}
		positions[p] = true
		errs = errors.Append(errs, b.Err)
		return true
	}

	for _, b := range a {
		// TODO: Should we also distinguish by message type?
		if add(b, b.Err.Position()) {
			continue
		}
		for _, p := range b.Err.InputPositions() {
			if add(b, p) {
				break
			}
		}
	}

	return errs
}
