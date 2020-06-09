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

// Package eval contains the high level CUE evaluation strategy.
//
// CUE allows for a significant amount of freedom in order of evaluation due to
// the commutativity of the unification operation. This package implements one
// of the possible strategies.
package eval

// TODO:
//   - result should be nodeContext: this allows optionals info to be extracted
//     and computed.
//

import (
	"fmt"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/internal/adt"
	"cuelang.org/go/cue/internal/runtime"
	"cuelang.org/go/cue/token"
)

var structSentinel = &adt.StructMarker{}

var incompleteSentinel = &adt.Bottom{
	Code: adt.IncompleteError,
	Err:  errors.Newf(token.NoPos, "incomplete"),
}

// func Eval(r *runtime.Runtime, v *adt.Vertex) errors.Error {
// 	if v.Value == nil {
// 		ctx := adt.NewContext(e.r, e, v)
// 		e.Unify(ctx, v)
// 	}

// 	// extract error if needed.
// 	return nil
// }

type evaluator struct {
	r       *runtime.Runtime
	index   adt.StringIndexer
	closeID uint32
}

func (e *evaluator) nextID() uint32 {
	e.closeID++
	return e.closeID
}

func (e *evaluator) Eval(v *adt.Vertex) errors.Error {
	if v.Value == nil {
		ctx := adt.NewContext(e.r, e, v)
		e.Unify(ctx, v)
	}

	// extract error if needed.
	return nil
}

// Evaluate is used to evaluate a sub expression while evaluating a Vertex
// with Unify. It may or may not return the original Vertex. It may also
// terminate evaluation early if it has enough evidence that a certain value
// can be the only value in a valid configuration. This means that an error
// may go undetected at this point, as long as it is caught later.
//
func (e *evaluator) Evaluate(c *adt.OpContext, v *adt.Vertex) adt.Value {
	if v.Value == nil {
		save := *v
		// Use node itself to allow for cycle detection.
		s := e.evalVertex(c, v, true)
		if s == nil {
			*v = save
			return &adt.Bottom{Code: adt.IncompleteError}
		}
		err, _ := v.Value.(*adt.Bottom)
		if !s.done() && (err == nil || err.IsIncomplete()) {
			// Clear values afterwards
			*v = save
		}
		if !s.done() && s.hasDisjunction() {
			return &adt.Bottom{Code: adt.IncompleteError}
		}
		if s.hasResult() {
			if b, _ := v.Value.(*adt.Bottom); b != nil {
				*v = save
				return b
			}
			// TODO: Only use result when not a cycle.
			v = &s.result
		}
		// TODO: Store if concrete and fully resolved.

	} else {
		b, _ := v.Value.(*adt.Bottom)
		if b != nil {
			return b
		}
	}

	switch v.Value.(type) {
	case nil:
		// TODO(XXX) should not happen: use incompleteSentinel instead
		return nil // in complete

	case *adt.ListMarker, *adt.StructMarker:
		return v

	default:
		return v.Value
	}
}

// Unify implements adt.Unifier.
//
// May not evaluate the entire value, but just enough to be able to compute.
//
// Phase one: record everything concrete
// Phase two: record incomplete
// Phase three: record cycle.
func (e *evaluator) Unify(c *adt.OpContext, v *adt.Vertex) {
	// defer c.PopVertex(c.PushVertex(v))

	if v.Value != nil {
		return
	}

	n := e.evalVertex(c, v, false)
	if n.result.Value != nil {
		*v = n.result
	}
	// Else set it to something.

	// Check whether result is done.
}

func (e *evaluator) evalVertex(c *adt.OpContext, v *adt.Vertex, partial bool) *nodeShared {
	// fmt.Println(debug.NodeString(c.StringIndexer, v, nil))
	shared := &nodeShared{
		ctx:   c,
		eval:  e,
		node:  v,
		stack: nil, // silence linter
	}

	closed := v.Closed

	for i := 0; ; i++ {
		_ = c.Err() // Clear any remaining error.
		// Set the cache to a cycle error to ensure a cyclic reference will result
		// in an error if applicable. A cyclic error may be ignored for
		// non-expression references. The cycle error may also be removed as soon
		// as there is evidence what a correct value must be, but before all
		// validation has taken place.
		v.Value = cycle
		v.Arcs = nil // TODO: use v.Arcs[:0] when arcs cleared in updateResult
		v.Structs = nil
		v.Closed = closed

		// If the result is a struct, it needs to be closed if:
		//   1) this node introduces a definition
		//   2) this node is a child of a node that introduces a definition,
		//      recursively.
		//   3) this node embeds a closed struct.
		needClose := v.Label.IsDef()

		n := &nodeContext{
			kind:       adt.TopKind,
			nodeShared: shared,
			needClose:  needClose,

			// These get cleared upon proof to the contrary.
			isDefault: true,
			isFinal:   true,
		}

		closeID := uint32(0)

		for _, x := range v.Conjuncts {
			closeID := closeID
			// TODO: needed for reentrancy. Investigate usefulness for cycle
			// detection.
			if x.Env != nil && x.Env.CloseID != 0 {
				closeID = x.Env.CloseID
			}
			n.addExprConjunct(x, closeID, true)
		}

		if i == 0 {
			// Use maybeSetCache for cycle breaking
			for n.maybeSetCache(); n.expandOne(); n.maybeSetCache() {
			}
			if v.Value != cycle && partial {
				// We have found a partial result. There may still be errors
				// down the line which may result from further evaluating this
				// field, but that will be caught when evaluating this field
				// for real.
				shared.result = *v
				shared.hasResult_ = true
				return shared
			}
			if !n.done() && len(n.disjunctions) > 0 && v.Value == cycle {
				// We disallow entering computations of disjunctions with
				// incomplete data.
				v.Value = &adt.Bottom{Code: adt.IncompleteError}
				shared.result = *v
				shared.hasResult_ = true
				return shared
			}
		}

		// Handle disjunctions. If there are no disjunctions, this call is
		// equivalent to calling n.postDisjunct.
		if n.tryDisjuncts() {
			break
		}
	}

	return shared
}

func (n *nodeContext) postDisjunct() {
	ctx := n.ctx

	// Use maybeSetCache for cycle breaking
	for n.maybeSetCache(); n.expandOne(); n.maybeSetCache() {
	}

	// TODO: preparation for association lists:
	// We assume that association types may not be created dynamically for now.
	// So we add lists
	n.addLists(ctx)

	if err := n.getErr(); err != nil {
		n.node.Value = err
		n.errs = nil
	} else if n.node.Value == cycle {
		// TODO: this does not yet validate all values.
		n.node.Value = nil
		n.finalize()
		// Either set to Conjunction or error.
	}

	v := n.node.Value
	if v != nil && v.Concreteness() == adt.Concrete {
		kind := n.kind
		if n.scalar != nil {
			kind = n.scalar.Kind()
		}
		if v.Kind()&^kind != 0 {
			p := token.NoPos
			if src := v.Source(); src != nil {
				p = src.Pos()
			}
			n.addErr(errors.Newf(p,
				// TODO(err): position of all value types.
				"conflicting types",
			))
		}
		if n.lowerBound != nil {
			if b := ctx.Validate(n.lowerBound, v); b != nil {
				n.addBottom(b)
			}
		}
		if n.upperBound != nil {
			if b := ctx.Validate(n.upperBound, v); b != nil {
				n.addBottom(b)
			}
		}
		for _, v := range n.checks {
			if b := ctx.Validate(v, n.node.Value); b != nil {
				n.addBottom(b)
			}
		}
	}

	// Visit arcs recursively to validate and compute error.
	for _, a := range n.node.Arcs {
		switch {
		case v != nil && v.Kind() == adt.ListKind:
			if a.Label.Typ() != adt.IntLabel {
				n.addErr(errors.Newf(token.NoPos,
					// TODO(err): add positions for list and arc definitions.
					"list may only have integer indices arcs"))
			}

		case v != nil && v != structSentinel && v.Kind() != adt.BottomKind:
			n.addErr(errors.Newf(token.NoPos,
				// TODO(err): add positions of non-struct values and arcs.
				"cannot combine scalar values with arcs"))
		}
	}

	var c *CloseDef
	if a, _ := n.node.Closed.(*acceptor); a != nil {
		c = a.tree
		n.needClose = n.needClose || a.isClosed
	}
	updated := updateClosed(c, n.replace)
	if updated == nil && n.needClose {
		updated = &CloseDef{}
	}

	// TODO retrieve from env.

	if err := n.getErr(); err != nil {
		if b, _ := n.node.Value.(*adt.Bottom); b != nil {
			err = adt.CombineErrors(nil, b, err)
		}
		n.node.Value = err
		// TODO: add return: if evaluation of arcs is important it can be done
		// later. Logically we're done.
	}

	m := &acceptor{updated, n.optionals, n.needClose, n.openList}
	if updated != nil {
		n.node.Closed = m
	}

	if len(n.optionals) > 0 && updated != nil && m.isClosed {
		n.node.Closed = m
	}

	for _, a := range n.node.Arcs {
		if updated != nil {
			a.Closed = m
		}
		if updated != nil && m.isClosed {
			if err := m.verifyArcAllowed(n.ctx, a.Label); err != nil {
				n.node.Value = err
			}
			// TODO: use continue to not process already failed fields,
			// or at least don't record recursive error.
			// continue
		}
		n.eval.Unify(ctx, a)
		if err, _ := a.Value.(*adt.Bottom); err != nil {
			n.node.Value = adt.CombineRecursiveError(n.node.Value, err)
		}
	}
}

var cycle = &adt.Bottom{Code: adt.CycleError}

type nodeShared struct {
	eval *evaluator
	ctx  *adt.OpContext
	sub  []*adt.Environment // Environment cache
	node *adt.Vertex

	// Disjunction handling
	resultNode *nodeContext
	result     adt.Vertex
	hasResult_ bool
	stack      []int
}

func (n *nodeShared) hasResult() bool {
	return n.resultNode != nil || n.hasResult_
}

func (n *nodeShared) done() bool {
	if n.resultNode == nil {
		return false
	}
	return n.resultNode.done()
}

func (n *nodeShared) hasDisjunction() bool {
	if n.resultNode == nil {
		return false
	}
	return len(n.resultNode.disjunctions) > 0
}

func (n *nodeShared) isDefault() bool {
	if n.resultNode == nil {
		return false
	}
	return n.resultNode.isDefault
}

// A nodeContext is used to collate all conjuncts of a value to facilitate
// unification. Conceptually order of unification does not matter. However,
// order has relevance when performing checks of non-monotic properities. Such
// checks should only be performed once the full value is known.
type nodeContext struct {
	*nodeShared

	// TODO:
	// filter *adt.Vertex a subset of composite with concrete fields for
	// bloom-like filtering of disjuncts. We should first verify, however,
	// whether some breath-first search gives sufficient performance, as this
	// should already ensure a quick-fail for struct disjunctions with
	// discriminators.

	// Current value (may be under construction)
	scalar adt.Value // TODO: use Value in node.

	// Concrete conjuncts
	kind       adt.Kind
	lowerBound *adt.BoundValue // > or >=
	upperBound *adt.BoundValue // < or <=
	checks     []adt.Validator // BuiltinValidator, other bound values.
	errs       *adt.Bottom

	// Struct information
	dynamicFields []envDynamic
	ifClauses     []envYield
	forClauses    []envYield
	optionals     []FieldSet // env + field
	// NeedClose:
	// - node starts definition
	// - embeds a definition
	// - parent node is closing
	needClose bool
	openList  bool
	newClose  *CloseDef
	// closeID   uint32 // from parent, or if not exist, new if introducing a def.
	replace map[uint32]*CloseDef

	// Expression conjuncts
	lists []envList
	exprs []adt.Conjunct

	// Disjunction handling
	disjunctions []envDisjunct
	isDefault    bool
	isFinal      bool
}

func (n *nodeContext) done() bool {
	return len(n.dynamicFields) == 0 &&
		len(n.ifClauses) == 0 &&
		len(n.forClauses) == 0 &&
		len(n.exprs) == 0
}

func (n *nodeContext) hasErr() bool {
	if n.node.Value != cycle {
		if _, ok := n.node.Value.(*adt.Bottom); ok {
			return true
		}
	}
	return n.ctx.HasErr() || n.errs != nil
}

func (n *nodeContext) getErr() *adt.Bottom {
	n.errs = adt.CombineErrors(nil, n.errs, n.ctx.Err())
	return n.errs
}

func (n *nodeContext) finalize() {
	ctx := n.ctx

	if !n.done() {
		if !ctx.IsTentative() {
			n.node.Value = incompleteSentinel
		}
		return
	}

	a := []adt.Value{}
	if n.node.Value != nil {
		a = append(a, n.node.Value)
	}
	kind := adt.TopKind
	if n.lowerBound != nil {
		a = append(a, n.lowerBound)
		kind &= n.lowerBound.Kind()
	}
	if n.upperBound != nil {
		a = append(a, n.upperBound)
		kind &= n.upperBound.Kind()
	}
	for _, c := range n.checks {
		if b, _ := c.(*adt.BoundValue); b != nil && b.Op == adt.NotEqualOp {
			if n.upperBound != nil &&
				adt.SimplifyBounds(ctx, n.kind, n.upperBound, b) != nil {
				continue
			}
			if n.lowerBound != nil &&
				adt.SimplifyBounds(ctx, n.kind, n.lowerBound, b) != nil {
				continue
			}
		}
		a = append(a, c)
		kind &= c.Kind()
	}
	if kind&^n.kind != 0 {
		a = append(a, &adt.BasicType{K: n.kind})
	}

	var v adt.Value
	switch len(a) {
	case 0:
		// Src is the combined input.
		v = &adt.BasicType{K: n.kind}

		if len(n.node.Structs) > 0 {
			v = structSentinel

		}

	case 1:
		v = a[0]

	default:
		v = &adt.Conjunction{Values: a}
	}

	// TODO: handle incomplete
	if ctx.IsTentative() && v.Concreteness() > adt.Concrete {
		return
	}

	n.node.Value = v
}

func (n *nodeContext) maybeSetCache() {
	if n.node.Value != cycle {
		return
	}
	if n.scalar != nil {
		n.node.Value = n.scalar
	}
	if n.errs != nil {
		n.node.Value = n.errs
	}
}

type envDynamic struct {
	env   *adt.Environment
	field *adt.DynamicField
}

type envYield struct {
	env   *adt.Environment
	yield adt.Yielder
}

type envList struct {
	env    *adt.Environment
	list   *adt.ListLit
	n      int64    // recorded length after evaluator
	expr   adt.Expr // element type
	isOpen bool
}

func (n *nodeContext) addBottom(b *adt.Bottom) {
	n.errs = adt.CombineErrors(nil, n.errs, b)
}

func (n *nodeContext) addErr(err errors.Error) {
	if err != nil {
		n.errs = adt.CombineErrors(nil, n.errs, &adt.Bottom{
			Err: err,
		})
	}
}

// addExprConjuncts will attempt to evaluate an adt.Expr and insert the value
// into the nodeContext if successful or queue it for later evaluation if it is
// incomplete or is not value.
func (n *nodeContext) addExprConjunct(v adt.Conjunct, def uint32, top bool) {
	env := v.Env
	if env != nil && env.CloseID != def {
		e := *env
		e.CloseID = def
		env = &e
	}
	switch x := v.Expr().(type) {
	case adt.Value:
		// TODO: insert in vertex as well.
		n.addValueConjunct(env, x)

	case *adt.BinaryExpr:
		if x.Op == adt.AndOp {
			n.addExprConjunct(adt.MakeConjunct(env, x.X), def, false)
			n.addExprConjunct(adt.MakeConjunct(env, x.Y), def, false)
		} else {
			n.evalExpr(v, def, top)
		}

	case *adt.StructLit:
		n.addStruct(env, x, def, top)

	case *adt.ListLit:
		n.lists = append(n.lists, envList{env: env, list: x})

	case *adt.Disjunction:
		n.addDisjunction(env, x, def, top)

	default:
		// Must be Resolver or Evaluator.
		n.evalExpr(v, def, top)
	}

	if top {
		n.updateReplace(v.Env)
	}
}

func (n *nodeContext) evalExpr(v adt.Conjunct, closeID uint32, top bool) {
	ctx := n.ctx

	switch x := v.Expr().(type) {
	case adt.Resolver:
		arc, err := ctx.Resolve(v.Env, x)
		if err != nil {
			if !err.IsIncomplete() {
				n.addBottom(err)
				break
			}

			// If this is a cycle error, we have reached a fixed point and adding
			// conjuncts at this point will not change the value. Also, continuing
			// to pursue this value will result in an infinite loop.
			//
			// TODO: add a mechanism so that the computation will only have to be
			// one once?
			if err.Code == adt.CycleError {
				break
			}
		}
		if arc == nil {
			n.exprs = append(n.exprs, v)
			break
		}

		// If this is a cycle error, we have reached a fixed point and adding
		// conjuncts at this point will not change the value. Also, continuing
		// to pursue this value will result in an infinite loop.
		//
		// TODO: add a mechanism so that the computation will only have to be
		// one once?
		if arc.Value == cycle {
			break
		}

		// TODO: detect structural cycles here. A structural cycle can occur
		// if it is not a reference cycle, but refers to a parent. node.
		// This should only be allowed if it is unified with a finite structure.

		if arc.Label.IsDef() {
			id := n.eval.nextID()
			n.needClose = true

			current := n.newClose
			n.newClose = nil

			for _, a := range arc.Conjuncts {
				n.addExprConjunct(a, id, false)
			}

			current, n.newClose = n.newClose, current

			if current == nil {
				current = &CloseDef{ID: id}
			}
			n.addAnd(current)

		} else {
			for _, a := range arc.Conjuncts {
				n.addExprConjunct(a, closeID, top)
			}
		}

	case adt.Evaluator:
		// adt.Interpolation, adt.UnaryExpr, adt.BinaryExpr, adt.CallExpr
		val, complete := ctx.Evaluate(v.Env, v.Expr())
		if !complete {
			n.exprs = append(n.exprs, v)
			break
		}

		// TODO: insert in vertex as well
		n.addValueConjunct(v.Env, val)

	default:
		panic(fmt.Sprintf("unknown expression of type %T", x))
	}
}

func (n *nodeContext) addValueConjunct(env *adt.Environment, v adt.Value) {
	if b, ok := v.(*adt.Bottom); ok {
		n.addBottom(b)
		return
	}

	ctx := n.ctx
	n.kind = n.kind & v.Kind()
	if n.kind == adt.BottomKind {
		// TODO: how to get conflicting values?
		n.addErr(errors.Newf(token.NoPos, "incompatible values v"))
		return
	}

	switch x := v.(type) {
	case *adt.Conjunction:
		for _, x := range x.Values {
			n.addValueConjunct(env, x)
		}

	case *adt.Top:
		n.optionals = append(n.optionals, FieldSet{env: env, isOpen: true})

	case *adt.BasicType:

	case *adt.BoundValue:
		switch x.Op {
		case adt.LessThanOp, adt.LessEqualOp:
			if y := n.upperBound; y != nil {
				n.upperBound = nil
				n.addValueConjunct(env, adt.SimplifyBounds(ctx, n.kind, x, y))
				return
			}
			n.upperBound = x

		case adt.GreaterThanOp, adt.GreaterEqualOp:
			if y := n.lowerBound; y != nil {
				n.lowerBound = nil
				n.addValueConjunct(env, adt.SimplifyBounds(ctx, n.kind, x, y))
				return
			}
			n.lowerBound = x

		case adt.EqualOp, adt.NotEqualOp, adt.MatchOp, adt.NotMatchOp:
			n.checks = append(n.checks, x)
			return
		}

	case adt.Validator:
		n.checks = append(n.checks, x)

	case *adt.Vertex: // handle lists or perhaps structs returned from a builtin.
		for _, a := range x.Arcs {
			for _, c := range a.Conjuncts {
				n.insertField(a.Label, c)
			}
		}

	case adt.Value: // *NullLit, *BoolLit, *NumLit, *StringLit, *BytesLit
		if y := n.scalar; y != nil {
			if b, ok := adt.BinOp(ctx, adt.EqualOp, x, y).(*adt.Bool); !ok || !b.B {
				n.addErr(errors.Newf(ctx.Pos(), "incompatible values %s and %s", ctx.Str(x), ctx.Str(y)))
			}
			// TODO: do we need to explicitly add again?
			// n.scalar = nil
			// n.addValueConjunct(c, adt.BinOp(c, adt.EqualOp, x, y))
			break
		}
		n.scalar = x

	default:
		panic(fmt.Sprintf("unknown value type %T", x))
	}

	if n.lowerBound != nil && n.upperBound != nil {
		if u := adt.SimplifyBounds(ctx, n.kind, n.lowerBound, n.upperBound); u != nil {
			n.lowerBound = nil
			n.upperBound = nil
			n.addValueConjunct(env, u)
		}
	}
}

// addStruct collates the declarations of a struct.
//
// addStruct fulfills two additional pivotal functions:
//   1) Implement vertex unification (this happends through De Bruijn indices
//      combined with proper set up of Environments).
//   2) Implied closedness for definitions.
//
func (n *nodeContext) addStruct(
	env *adt.Environment,
	s *adt.StructLit,
	newDef uint32,
	top bool) {

	ctx := n.ctx
	n.node.Structs = append(n.node.Structs, s)

	// Inherit closeID from environment, unless this is a new definition.
	closeID := newDef
	if closeID == 0 && env != nil {
		closeID = env.CloseID
	}

	// fmt.Println("ADDDST", n.node.Label.ToString(n.ctx))

	// NOTE: This is a crucial point in the code:
	// Unification derferencing happens here. The child nodes are set to
	// an Environment linked to the current node. Together with the De Bruijn
	// indices, this determines to which Vertex a reference resolves.

	// TODO(perf): consider using environment cache:
	// var childEnv *adt.Environment
	// for _, s := range n.nodeCache.sub {
	// 	if s.Up == env {
	// 		childEnv = s
	// 	}
	// }
	childEnv := &adt.Environment{
		Up:      env,
		Vertex:  n.node,
		CloseID: closeID,
	}

	var hasOther, hasBulk adt.Node

	opt := FieldSet{env: childEnv}

	for _, d := range s.Decls {
		switch x := d.(type) {
		case *adt.Field:
			opt.MarkField(ctx, x)
			// handle in next iteration.

		case *adt.OptionalField:
			opt.AddOptional(ctx, x)

		case *adt.DynamicField:
			hasOther = x
			n.dynamicFields = append(n.dynamicFields, envDynamic{childEnv, x})
			opt.AddDynamic(ctx, childEnv, x)

		case *adt.ForClause:
			hasOther = x
			n.forClauses = append(n.forClauses, envYield{childEnv, x})

		case adt.Yielder:
			hasOther = x
			n.ifClauses = append(n.ifClauses, envYield{childEnv, x})

		case adt.Expr:
			// push and opo embedding type.
			id := n.eval.nextID()

			current := n.newClose
			n.newClose = nil

			hasOther = x
			n.addExprConjunct(adt.MakeConjunct(childEnv, x), id, false)

			current, n.newClose = n.newClose, current

			if current == nil {
				current = &CloseDef{ID: id} // TODO: isClosed?
			} else {
				n.needClose = true
			}
			n.addOr(closeID, current)

		case *adt.BulkOptionalField:
			hasBulk = x
			opt.AddBulk(ctx, x)

		case *adt.Ellipsis:
			hasBulk = x
			opt.AddEllipsis(ctx, x)

		default:
			panic("unreachable")
		}
	}

	if hasBulk != nil && hasOther != nil {
		n.addErr(errors.Newf(token.NoPos, "cannot mix bulk optional fields with dynamic fields, embeddings, or comprehensions within the same struct"))
	}

	// Apply existing fields
	for _, arc := range n.node.Arcs {
		opt.MatchAndInsert(ctx, arc)
	}

	n.optionals = append(n.optionals, opt)

	for _, d := range s.Decls {
		switch x := d.(type) {
		case *adt.Field:
			n.insertField(x.Label, adt.MakeConjunct(childEnv, x))
		}
	}
}

func (n *nodeContext) insertField(f adt.Feature, x adt.Conjunct) *adt.Vertex {
	ctx := n.ctx
	arc, isNew := n.node.GetArc(f)

	// TODO: disallow adding conjuncts when cache set?
	arc.AddConjunct(x)

	if isNew {
		for _, o := range n.optionals {
			o.MatchAndInsert(ctx, arc)
		}
	}
	return arc
}

// expandOne adds dynamic fields to a node until a fixed point is reached.
// On each iteration, dynamic fields that cannot resolve due to incomplete
// values are skipped. They will be retried on the next iteration until no
// progress can be made. Note that a dynamic field may add more dynamic fields.
//
// forClauses are processed after all other clauses. A struct may be referenced
// before it is complete, meaning that fields added by other forms of injection
// may influence the result of a for clause _after_ it has already been
// processed. We could instead detect such insertion and feed it to the
// ForClause to generate another entry or have the for clause be recomputed.
// This seems to be too complicated and lead to iffy edge cases.
// TODO(error): detect when a field is added to a struct that is already used
// in a for clause.
func (n *nodeContext) expandOne() (done bool) {
	if n.done() {
		return false
	}

	var progress bool

	if progress = n.injectDynamic(); progress {
		return true
	}

	if n.ifClauses, progress = n.injectEmbedded(n.ifClauses); progress {
		return true
	}

	if n.forClauses, progress = n.injectEmbedded(n.forClauses); progress {
		return true
	}

	// Do expressions after comprehensions, as comprehensions can never
	// refer to embedded scalars, whereas expressions may refer to generated
	// fields if we were to allow attributes to be defined alongside
	// scalars.
	exprs := n.exprs
	n.exprs = n.exprs[:0]
	for _, x := range exprs {
		n.evalExpr(x, 0, true)
		n.updateReplace(x.Env)

		// collect and and or
	}
	if len(n.exprs) < len(exprs) {
		return true
	}

	// No progress, report error later if needed: unification with
	// disjuncts may resolve this later later on.
	return false
}

// injectDynamic evaluates and inserts dynamic declarations.
func (n *nodeContext) injectDynamic() (progress bool) {
	ctx := n.ctx
	k := 0

	a := n.dynamicFields
	for _, d := range n.dynamicFields {
		var f adt.Feature
		v, complete := ctx.Evaluate(d.env, d.field.Key)
		if !complete {
			a[k] = d
			k++
			continue
		}
		f = ctx.Label(v)
		n.insertField(f, adt.MakeConjunct(d.env, d.field))
	}

	progress = k < len(n.dynamicFields)

	n.dynamicFields = a[:k]

	return progress
}

// injectEmbedded evaluates and inserts embeddings. It first evaluates all
// embeddings before inserting the results to ensure that the order of
// evaluation does not matter.
func (n *nodeContext) injectEmbedded(all []envYield) (a []envYield, progress bool) {
	ctx := n.ctx
	type envStruct struct {
		env *adt.Environment
		s   *adt.StructLit
	}
	var sa []envStruct
	f := func(env *adt.Environment, st *adt.StructLit) {
		sa = append(sa, envStruct{env, st})
	}

	k := 0
	for _, d := range all {
		sa = sa[:0]

		if err := ctx.Yield(d.env, d.yield, f); err != nil {
			if err.IsIncomplete() {
				all[k] = d
				k++
			} else {
				// continue to collect other errors.
				n.addBottom(err)
			}
			continue
		}

		for _, st := range sa {
			n.addStruct(st.env, st.s, 0, true)
		}
	}

	return all[:k], k < len(all)
}

// addLists
//
// TODO: association arrays:
// If an association array marker was present in a struct, create a struct node
// instead of a list node. In either case, a node may only have list fields
// or struct fields and not both.
//
// addLists should be run after the fixpoint expansion:
//    - it enforces that comprehensions may not refer to the list itself
//    - there may be no other fields within the list.
//
// TODO(embeddedScalars): for embedded scalars, there should be another pass
// of evaluation expressions after expanding lists.
func (n *nodeContext) addLists(c *adt.OpContext) {
	if len(n.lists) == 0 {
		return
	}

	if len(n.node.Arcs) > 0 && !n.node.Arcs[0].Label.IsInt() {
		// p := combinedPos(c.node.Arcs[0].conjuncts[0].expr, c.lists[0])
		n.addErr(errors.Newf(token.NoPos, "conflicting types list and struct"))
	}

	for i, l := range n.lists {
		index := int64(0)
		for j, elem := range l.list.Elems {
			switch x := elem.(type) {
			case adt.Yielder:
				err := c.Yield(l.env, x, func(e *adt.Environment, st *adt.StructLit) {
					label, err := adt.MakeLabel(x.Source(), index, adt.IntLabel)
					n.addErr(err)
					index++
					n.insertField(label, adt.MakeConjunct(e, st))
				})
				if err.IsIncomplete() {

				}

			case *adt.Ellipsis:
				if j != len(l.list.Elems)-1 {
					n.addErr(errors.Newf(token.NoPos,
						"ellipsis must be last element in list"))
				}

				// TODO: add as optional.
				n.lists[i].expr = x.Value
				n.lists[i].isOpen = true

			default:
				label, err := adt.MakeLabel(x.Source(), index, adt.IntLabel)
				n.addErr(err)
				index++
				n.insertField(label, adt.MakeConjunct(l.env, x))
			}
		}

		n.lists[i].n = index
	}

	// Check list lengths and find the max.
	var max envList = envList{isOpen: true}
	for _, l := range n.lists {
		if l.n < max.n && l.isOpen {
			continue
		}

		if l.n > max.n && max.isOpen {
			max = l
			continue
		}

		if l.n == max.n {
			if max.isOpen {
				max = l // may now be closed
			}
			continue
		}

		n.addErr(errors.Newf(c.Pos(),
			"incompatible list lengths (%d and %d)", max.n, l.n))
	}

	sources := []ast.Expr{}
	// Add conjuncts for additional items.
	for _, l := range n.lists {
		if !l.isOpen {
			continue
		}
		if l.expr == nil {
			l.expr = &adt.Top{}
		}
		if src, _ := l.expr.Source().(ast.Expr); src != nil {
			sources = append(sources, src)
		}
		for p := l.n; p < max.n; p++ {
			label, err := adt.MakeLabel(l.list.Src, p, adt.IntLabel)
			n.addErr(err)
			n.insertField(label, adt.MakeConjunct(l.env, l.expr))
		}
	}

	n.openList = max.isOpen

	n.node.Value = &adt.ListMarker{
		Src: ast.NewBinExpr(token.AND, sources...),
	}
}
