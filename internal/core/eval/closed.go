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

// The file implements the majority of the closed struct semantics. The data is
// recorded in the Closed field of a Vertex.
//
// Each vertex has a set of conjuncts that make up the values of the vertex.
// Each Conjunct may originate from various sources, like an embedding, field
// definition or regular value. For the purpose of computing the value, the
// source of the conjunct is irrelevant. The origin does matter, however, for
// determining whether a field is allowed in a closed struct. The Closed field
// keeps track of the kind of origin for this purpose.
//
// More precisely, the CloseDef struct explains how the conjuncts of an arc were
// combined for instance due to a conjunction with closed struct or through an
// embedding. Each Vertex may be associated with a slice of CloseDefs. The
// position of a CloseDef in a file corresponds to an adt.ID.
//
// While evaluating each conjunct, new CloseDefs are added to indicate how a
// conjunct relates to its parent as needed. For instance, if a field references
// a definition, all other previous checks are useless, as the newly referred to
// definitions define an upper bound and will contain all the information that
// is necessary to determine whether a field may be included.
//
// Most of the logic in this file concerns itself with the combination of
// multiple CloseDef values as well as traversing the structure to validate
// whether an arc is allowed. The actual fieldSet logic is in optional.go The
// overall control and use of the functionality in this file is used in eval.go.

import (
	"fmt"
	"strings"

	"cuelang.org/go/internal/core/adt"
)

// acceptor implements adt.Acceptor.
//
// Note that it keeps track of whether it represents a closed struct. An
// acceptor is also used to associate an CloseDef with a Vertex, and not
// all CloseDefs represent a closed struct: a value that contains embeddings may
// eventually turn into a closed struct. Consider
//
//    a: {
//       b
//       d: e: int
//    }
//    b: d: {
//       #A & #B
//    }
//
// At the point of evaluating `a`, the struct is not yet closed. However,
// descending into `d` will trigger the inclusion of definitions which in turn
// causes the struct to be closed. At this point, it is important to know that
// `b` originated from an embedding, as otherwise `e` may not be allowed.
//
type acceptor struct {
	Canopy []CloseDef
	Fields []*fieldSet

	// TODO: isClosed could be removed if we can include closedness fully
	// in the ClosedDefs representing conjuncts. This, in turn, will allow
	// various optimizations, like having to record field sets for data
	// vertices.
	isClosed bool
	isList   bool
	openList bool

	// ignore tells to the closedness check for one level. This allows
	// constructed parent nodes that do not have field sets defined.
	ignore bool
}

func (a *acceptor) clone() *acceptor {
	canopy := make([]CloseDef, len(a.Canopy))
	copy(canopy, a.Canopy)
	for i := range canopy {
		canopy[i].IsClosed = false
	}
	return &acceptor{
		Canopy:   canopy,
		isClosed: a.isClosed,
	}
}

func (a *acceptor) Accept(c *adt.OpContext, f adt.Feature) bool {
	if a.isList {
		return a.openList
	}

	// TODO: remove these two checks and always pass InvalidLabel.
	if !a.isClosed {
		return true
	}
	if f == adt.InvalidLabel {
		return false
	}
	if f.IsInt() {
		return a.openList
	}
	return a.verifyArcAllowed(c, f, nil)
}

func (a *acceptor) MatchAndInsert(c *adt.OpContext, v *adt.Vertex) {
	a.visitAllFieldSets(func(fs *fieldSet) {
		fs.MatchAndInsert(c, v)
	})
}

func (a *acceptor) OptionalTypes() (mask adt.OptionalType) {
	a.visitAllFieldSets(func(f *fieldSet) {
		mask |= f.OptionalTypes()
	})
	return mask
}

func (a *acceptor) IsOptional(label adt.Feature) bool {
	optional := false
	a.visitAllFieldSets(func(f *fieldSet) {
		optional = optional || f.IsOptional(label)
	})
	return optional
}

// A disjunction acceptor represents a disjunction of all possible fields. Note
// that this is never used in evaluation as evaluation stops at incomplete nodes
// and a disjunction is incomplete. When the node is referenced, the original
// conjuncts are used instead.
//
// The value may be used in the API, though, where it may be an argument to
// UnifyAccept.
//
// TODO(perf): it would be sufficient to only implement the Accept method of an
// Acceptor. This could be implemented as an allocation-free wrapper type around
// a Disjunction. This will require a bit more API cleaning, though.
func newDisjunctionAcceptor(x *adt.Disjunction) adt.Acceptor {
	n := &acceptor{}

	for _, d := range x.Values {
		if a, _ := d.Closed.(*acceptor); a != nil {
			offset := n.InsertSubtree(0, nil, d, false)
			a.visitAllFieldSets(func(f *fieldSet) {
				g := *f
				g.id += offset
				n.insertFieldSet(g.id, &g)
			})
		}
	}

	return n
}

// CloseDef defines how individual fieldSets (corresponding to conjuncts)
// combine to determine whether a field is contained in a closed set.
//
// A CloseDef combines multiple conjuncts and embeddings. All CloseDefs are
// stored in slice. References to other CloseDefs are indices within this slice.
// Together they define the top of the tree of the expression tree of how
// conjuncts combine together (a canopy).
type CloseDef struct {
	Src adt.Node

	// And is used to track the IDs of a set of conjuncts. If IsDef or IsClosed
	// is true, a field is only allowed if at least one of the corresponding
	// fieldsets associated with this node or its embeddings allows it.
	//
	// And nodes are linked in a ring, meaning that the last node points back
	// to the first node. This allows a traversal of all and nodes to commence
	// at any point in the ring.
	And adt.ID

	// NextEmbed indicates the first ID for a linked list of embedded
	// expressions. The node corresponding to the actual embedding is at
	// position NextEmbed+1. The linked-list nodes all have a value of -1 for
	// And. NextEmbed is 0 for the last element in the list.
	NextEmbed adt.ID

	// IsDef indicates this node is associated with a definition and that all
	// expressions are recursively closed. This value is "sticky" when a child
	// node copies the closedness data from a parent node.
	IsDef bool

	// IsClosed indicates this node is associated with the result of close().
	// A child vertex should not "inherit" this value.
	IsClosed bool
}

func (n *CloseDef) isRequired() bool {
	return n.IsDef || n.IsClosed
}

const embedRoot adt.ID = -1

type Entry = fieldSet

// TODO: this may be an idea to get rid of acceptor.isClosed. There
// are various more things to consider, though.
//
// func (c *acceptor) isRequired(id adt.ID) bool {
// 	req := false
// 	c.visitAnd(id, func(id adt.ID, n CloseDef) bool {
// 		if n.isRequired() {
// 			req = true
// 			return false
// 		}
// 		c.visitEmbed(id, func(id adt.ID, n CloseDef) bool {
// 			if n.isRequired() {
// 				req = true
// 				return false
// 			}
// 			req = req || c.isRequired(id)
// 			return true
// 		})

// 		return true
// 	})
// 	return req
// }

func (c *acceptor) visitAllFieldSets(f func(f *fieldSet)) {
	for _, set := range c.Fields {
		for ; set != nil; set = set.next {
			f(set)
		}
	}
}

func (c *acceptor) visitAnd(id adt.ID, f func(id adt.ID, n CloseDef) bool) bool {
	for i := id; ; {
		x := c.Canopy[i]

		if !f(i, x) {
			return false
		}

		if i = x.And; i == id {
			break
		}
	}
	return true
}

func (c *acceptor) visitOr(id adt.ID, f func(id adt.ID, n CloseDef) bool) bool {
	if !f(id, c.Canopy[id]) {
		return false
	}
	return c.visitEmbed(id, f)
}

func (c *acceptor) visitEmbed(id adt.ID, f func(id adt.ID, n CloseDef) bool) bool {
	for i := c.Canopy[id].NextEmbed; i != 0; i = c.Canopy[i].NextEmbed {
		if id := i + 1; !f(id, c.Canopy[id]) {
			return false
		}
	}
	return true
}

func (c *acceptor) node(id adt.ID) *CloseDef {
	if len(c.Canopy) == 0 {
		c.Canopy = append(c.Canopy, CloseDef{})
	}
	return &c.Canopy[id]
}

func (c *acceptor) fieldSet(at adt.ID) *fieldSet {
	if int(at) >= len(c.Fields) {
		return nil
	}
	return c.Fields[at]
}

func (c *acceptor) insertFieldSet(at adt.ID, e *fieldSet) {
	c.node(0) // Ensure the canopy is at least length 1.
	if len(c.Fields) < len(c.Canopy) {
		a := make([]*fieldSet, len(c.Canopy))
		copy(a, c.Fields)
		c.Fields = a
	}
	e.next = c.Fields[at]
	c.Fields[at] = e
}

// InsertDefinition appends a new CloseDef to Canopy representing a reference to
// a definition at the given position. It returns the position of the new
// CloseDef.
func (c *acceptor) InsertDefinition(at adt.ID, src adt.Node) (id adt.ID) {
	if len(c.Canopy) == 0 {
		c.Canopy = append(c.Canopy, CloseDef{})
	}
	if int(at) >= len(c.Canopy) {
		panic(fmt.Sprintf("at >= len(canopy) (%d >= %d)", at, len(c.Canopy)))
	}
	// New there is a new definition, the parent location (invariant) is no
	// longer a required entry and could be dropped if there were no more
	// fields.
	//    #orig: #d     // only fields in #d are sufficient to check.
	//    #orig: {a: b}
	c.Canopy[at].IsDef = false

	id = adt.ID(len(c.Canopy))
	y := CloseDef{
		Src:       src,
		And:       c.Canopy[at].And,
		NextEmbed: 0,
		IsDef:     true,
	}
	c.Canopy[at].And = id
	c.Canopy = append(c.Canopy, y)

	return id
}

// InsertEmbed appends a new CloseDef to Canopy representing the use of an
// embedding at the given position. It returns the position of the new CloseDef.
func (c *acceptor) InsertEmbed(at adt.ID, src adt.Node) (id adt.ID) {
	if len(c.Canopy) == 0 {
		c.Canopy = append(c.Canopy, CloseDef{})
	}
	if int(at) >= len(c.Canopy) {
		panic(fmt.Sprintf("at >= len(canopy) (%d >= %d)", at, len(c.Canopy)))
	}

	id = adt.ID(len(c.Canopy))
	y := CloseDef{
		And:       -1,
		NextEmbed: c.Canopy[at].NextEmbed,
	}
	z := CloseDef{Src: src, And: id + 1}
	c.Canopy[at].NextEmbed = id
	c.Canopy = append(c.Canopy, y, z)

	return id + 1
}

// isComplexStruct reports whether the Closed information should be copied as a
// subtree into the parent node using InsertSubtree. If not, the conjuncts can
// just be inserted at the current ID.
func isComplexStruct(v *adt.Vertex) bool {
	m, _ := v.BaseValue.(*adt.StructMarker)
	if m == nil {
		return false
	}
	a, _ := v.Closed.(*acceptor)
	if a == nil {
		return false
	}
	if a.isClosed {
		return true
	}
	switch len(a.Canopy) {
	case 0:
		return false
	case 1:
		// TODO: should we check for closedness?
		x := a.Canopy[0]
		return x.isRequired()
	}
	return true
}

// InsertSubtree inserts the closedness information of v into c as an embedding
// at the current position and inserts conjuncts of v into n (if not nil). It
// inserts it as an embedding and not and to cover either case. The idea is that
// one of the values were supposed to be closed, a separate node entry would
// already have been created.
//
// TODO: get rid of this. This is now only used in newDisjunctionAcceptor,
// which, in turn, is rarely used for analyzing disjunction values in the API.
// This code is not ideal and buggy (see comment below), but it doesn't seem
// worth improving it and we can probably do without.
func (c *acceptor) InsertSubtree(at adt.ID, n *nodeContext, v *adt.Vertex, cyclic bool) adt.ID {
	if len(c.Canopy) == 0 {
		c.Canopy = append(c.Canopy, CloseDef{})
	}
	if int(at) >= len(c.Canopy) {
		panic(fmt.Sprintf("at >= len(canopy) (%d >= %d)", at, len(c.Canopy)))
	}

	// TODO: like with AddVertex, this really should use the acceptor of the
	// parent. This seems not to work, though.
	//
	// var a *acceptor
	// if v.Parent != nil && v.Parent.Closed != nil {
	// 	a = closedInfo(v.Parent)
	// } else {
	// 	a = &acceptor{}
	// }
	a := closedInfo(v)
	a.node(0)

	id := adt.ID(len(c.Canopy))
	y := CloseDef{
		And:       embedRoot,
		NextEmbed: c.Canopy[at].NextEmbed,
	}
	c.Canopy[at].NextEmbed = id

	c.Canopy = append(c.Canopy, y)
	id = adt.ID(len(c.Canopy))

	// First entry is at the embedded node location.
	c.Canopy = append(c.Canopy, a.Canopy...)

	// Shift all IDs for the new offset.
	for i := int(id); i < len(c.Canopy); i++ {
		x := c.Canopy[i]
		if x.And != -1 {
			c.Canopy[i].And += id
		}
		if x.NextEmbed != 0 {
			c.Canopy[i].NextEmbed += id
		}
	}

	if n != nil {
		for _, c := range v.Conjuncts {
			c = updateCyclic(c, cyclic, nil, nil)
			c.CloseID += id
			n.addExprConjunct(c)
		}
	}

	return id
}

func appendConjuncts(v *adt.Vertex, a []adt.Conjunct) {
	for _, c := range a {
		v.AddConjunct(c)
	}
}

// AddVertex add a Vertex to a new destination node. The caller may
// call AddVertex multiple times on dst. None of the fields of dst
// should be set by the caller. AddVertex takes care of setting the
// Label and Parent.
func AddVertex(dst, src *adt.Vertex) {
	if dst.Parent == nil {
		// Create "fake" parent that holds the combined closed data.
		// We do not set the parent until here as we don't want to "inherit" the
		// closedness setting from v.
		dst.Parent = &adt.Vertex{Parent: src.Parent}
		dst.Label = src.Label
	}

	if src.IsData() {
		dst.AddConjunct(adt.MakeConjunct(nil, src, 0))
		return
	}

	var srcC *acceptor
	if src.Parent != nil && src.Parent.Closed != nil {
		srcC = closedInfo(src.Parent)
	} else {
		srcC = &acceptor{}
	}
	dstC := closedInfo(dst.Parent)
	dstC.ignore = true

	isDef := src.Label.IsDef()
	addClose := isDef || srcC.isClosed

	if addClose {
		dstC.node(0)
	} else if len(dstC.Canopy) == 0 {
		// we can copy it as is (assuming 0 is )
		appendConjuncts(dst, src.Conjuncts)
		dstC.Canopy = append(dstC.Canopy, srcC.Canopy...)
		return
	}

	offset := adt.ID(len(dstC.Canopy))
	top := offset

	switch {
	case len(srcC.Canopy) == 0 && !addClose:
		appendConjuncts(dst, src.Conjuncts)
		return

	case len(srcC.Canopy) == 1:
		c := srcC.Canopy[0]
		if !addClose && !c.IsDef && !c.IsClosed {
			appendConjuncts(dst, src.Conjuncts)
			return
		}
		c.IsDef = addClose
		dstC.Canopy = append(dstC.Canopy, c)

	case addClose:
		srcC.node(0)
		isClosed := false
		srcC.visitAnd(0, func(id adt.ID, c CloseDef) bool {
			if c.IsClosed || c.IsDef {
				isClosed = true
				return false
			}
			return true
		})

		if !isClosed {
			// need to embed and close.
			dstC.Canopy = append(dstC.Canopy,
				CloseDef{
					And:       offset,
					IsDef:     true,
					NextEmbed: offset + 1,
				},
				CloseDef{
					And: embedRoot,
				})

			offset += 2
		}
		fallthrough

	default:
		dstC.Canopy = append(dstC.Canopy, srcC.Canopy...)
	}

	// Shift all IDs for the new offset.
	for i := int(offset); i < len(dstC.Canopy); i++ {
		x := dstC.Canopy[i]
		if x.And != -1 {
			dstC.Canopy[i].And += offset
		}
		if x.NextEmbed != 0 {
			dstC.Canopy[i].NextEmbed += offset
		}
	}

	topAnd := dstC.Canopy[top].And
	atAnd := dstC.Canopy[0].And
	dstC.Canopy[top].And = atAnd
	dstC.Canopy[0].And = topAnd

	for _, c := range src.Conjuncts {
		c.CloseID += offset
		if err := dst.AddConjunct(c); err != nil {
			panic(err)
		}
	}
}

// TODO: This is roughly the equivalent of AddVertex for use with
// UnifyAccept. It is based on the old InsertSubTree. We should use
// this at some point, but ideally we should have a better way to
// represent closedness in the first place that is more flexible in
// handling API usage.
//
// func EmbedVertex(dst, src *adt.Vertex) {
// 	if dst.Parent == nil {
// 		// Create "fake" parent that holds the combined closed data.
// 		// We do not set the parent until here as we don't want to "inherit" the
// 		// closedness setting from v.
// 		dst.Parent = &adt.Vertex{Parent: src.Parent}
// 		dst.Label = src.Label
// 	}
//
// 	if src.IsData() {
// 		dst.AddConjunct(adt.MakeConjunct(nil, src, 0))
// 		return
// 	}
//
// 	var a *acceptor
// 	if src.Parent != nil && src.Parent.Closed != nil {
// 		a = closedInfo(src.Parent)
// 	} else {
// 		a = &acceptor{}
// 	}
// 	a.node(0)
//
// 	c := closedInfo(dst.Parent)
// 	c.ignore = true
// 	c.node(0)
//
// 	id := adt.ID(len(c.Canopy))
// 	y := CloseDef{
// 		And:       embedRoot,
// 		NextEmbed: c.Canopy[0].NextEmbed,
// 	}
// 	c.Canopy[0].NextEmbed = id
//
// 	c.Canopy = append(c.Canopy, y)
// 	id = adt.ID(len(c.Canopy))
//
// 	// First entry is at the embedded node location.
// 	c.Canopy = append(c.Canopy, a.Canopy...)
//
// 	// Shift all IDs for the new offset.
// 	for i := int(id); i < len(c.Canopy); i++ {
// 		x := c.Canopy[i]
// 		if x.And != -1 {
// 			c.Canopy[i].And += id
// 		}
// 		if x.NextEmbed != 0 {
// 			c.Canopy[i].NextEmbed += id
// 		}
// 	}
//
// 	for _, c := range src.Conjuncts {
// 		c.CloseID += id
// 		if err := dst.AddConjunct(c); err != nil {
// 			panic(err)
// 		}
// 	}
// }

func (c *acceptor) verifyArc(ctx *adt.OpContext, f adt.Feature, v *adt.Vertex) (found bool, err *adt.Bottom) {

	defer ctx.ReleasePositions(ctx.MarkPositions())

	c.node(0) // ensure at least a size of 1.
	if c.verify(ctx, f) {
		return true, nil
	}

	// TODO: also disallow non-hidden definitions.
	if !f.IsString() && f != adt.InvalidLabel {
		return false, nil
	}

	if v != nil {
		for _, c := range v.Conjuncts {
			if pos := c.Field(); pos != nil {
				ctx.AddPosition(pos)
			}
		}
	}

	// collect positions from tree.
	for _, c := range c.Canopy {
		if c.Src != nil {
			ctx.AddPosition(c.Src)
		}
	}

	label := f.SelectorString(ctx)
	return false, ctx.NewErrf("field `%s` not allowed", label)
}

func (c *acceptor) verifyArcAllowed(ctx *adt.OpContext, f adt.Feature, v *adt.Vertex) bool {

	// TODO: also disallow non-hidden definitions.
	if !f.IsString() && f != adt.InvalidLabel {
		return true
	}

	defer ctx.ReleasePositions(ctx.MarkPositions())

	c.node(0) // ensure at least a size of 1.
	return c.verify(ctx, f)
}

func (c *acceptor) verify(ctx *adt.OpContext, f adt.Feature) bool {
	ok, required := c.verifyAnd(ctx, 0, f)
	return ok || (!required && !c.isClosed)
}

// verifyAnd reports whether f is contained in all closed conjuncts at id and,
// if not, whether the precense of at least one entry is required.
func (c *acceptor) verifyAnd(ctx *adt.OpContext, id adt.ID, f adt.Feature) (found, required bool) {
	for i := id; ; {
		x := c.Canopy[i]

		if ok, req := c.verifySets(ctx, i, f); ok {
			found = true
		} else if ok, isClosed := c.verifyEmbed(ctx, i, f); ok {
			found = true
		} else if req || x.isRequired() {
			// Not found for a closed entry so this indicates a failure.
			return false, true
		} else if isClosed {
			// The node itself isn't closed, but an embedding indicates it
			// should. See cue/testdata/definitions/embed.txtar.
			required = true
		}

		if i = x.And; i == id {
			break
		}
	}

	return found, required
}

// verifyEmbed reports whether any of the embeddings for the node at id allows f
// and, if not, whether the embeddings imply that the enclosing node should be
// closed. The latter is the case when embedded struct itself is closed.
func (c *acceptor) verifyEmbed(ctx *adt.OpContext, id adt.ID, f adt.Feature) (found, isClosed bool) {

	for i := c.Canopy[id].NextEmbed; i != 0; i = c.Canopy[i].NextEmbed {
		ok, req := c.verifyAnd(ctx, i+1, f)
		if ok {
			return true, false
		}
		if req {
			isClosed = true
		}
	}
	return false, isClosed
}

func (c *acceptor) verifySets(ctx *adt.OpContext, id adt.ID, f adt.Feature) (found, required bool) {
	o := c.fieldSet(id)
	if o == nil {
		return false, false
	}
	for isRegular := f.IsRegular(); o != nil; o = o.next {
		if isRegular && (len(o.additional) > 0 || o.isOpen) {
			return true, false
		}

		for _, g := range o.fields {
			if f == g.label {
				return true, false
			}
		}

		if !isRegular {
			continue
		}

		for _, b := range o.bulk {
			if b.check.Match(ctx, f) {
				return true, false
			}
		}
	}

	// TODO: this is the same location where code is registered as the old code,
	// but
	for o := c.Fields[id]; o != nil; o = o.next {
		if o.pos != nil {
			ctx.AddPosition(o.pos)
		}
	}
	return false, false
}

type info struct {
	referred bool
	up       adt.ID
	replace  adt.ID
	reverse  adt.ID
}

// Compact updates closedness info by cloning and compacting this info from
// the parent.
//
// A node is used if it itself or any of its descendant nodes are used.
//
// A CloseDef node can be removed from the list if it:
//   - is not in use by any of the conjuncts
//   - has no descendants that are marked
//   - is a top-level "and" node that is required
//
func Compact(v *adt.Vertex, allowAll bool) {
	if v.Parent == nil {
		return
	}

	c := closedInfo(v.Parent)
	if len(c.Canopy) == 0 {
		return
	}

	marked := make([]info, len(c.Canopy))

	c.markParents(0, marked)

	// Mark top-level required ands.
	c.visitAnd(0, func(id adt.ID, n CloseDef) bool {
		// TODO: Ideally we should only marked required nodes, but that
		// seems to lead to an error in one of the tests. Investigate.
		// if n.isRequired() {
		marked[id].referred = true
		// }
		return true
	})

	// // Mark first
	// marked[0].referred = true

	// Mark all entries that cannot be dropped.
	marked[0].referred = true
	for _, x := range v.Conjuncts {
		c.markUsed(x.CloseID, marked)
	}

	// Compute compact numbers and reverse.
	closed := false
	k := adt.ID(0)
	for i, x := range marked {
		if x.referred {
			marked[i].replace = k
			marked[k].reverse = adt.ID(i)
			k++
			closed = closed || c.Canopy[i].IsClosed
		}
	}

	// if int(k) == len(c.Canopy) && !closed && !allowAll &&
	// 	v.Parent.Status() == adt.Finalized {
	// 	v.Closed = &acceptor{
	// 		Canopy:   c.Canopy,
	// 		isClosed: c.isClosed,
	// 	}
	// 	return
	// }

	compacted := make([]CloseDef, k)
	v.Closed = &acceptor{
		Canopy:   compacted,
		isClosed: c.isClosed,
	}

	for i := range compacted {
		orig := c.Canopy[marked[i].reverse]

		and := orig.And
		if and != embedRoot {
			for !marked[and].referred {
				and = c.Canopy[and].And
			}
			and = marked[and].replace
		}
		compacted[i] = CloseDef{
			Src:   orig.Src,
			And:   and,
			IsDef: orig.IsDef && !allowAll,
		}

		last := adt.ID(i)
		for or := orig.NextEmbed; or != 0; or = c.Canopy[or].NextEmbed {
			if marked[or].referred {
				compacted[last].NextEmbed = marked[or].replace
				last = marked[or].replace
			}
		}
	}

	// Update conjuncts
	for i, x := range v.Conjuncts {
		v.Conjuncts[i].CloseID = marked[x.ID()].replace
	}
}

func (c *acceptor) markParents(parent adt.ID, info []info) {
	// Ands are arranged in a ring, so check for parent, not 0.
	c.visitAnd(parent, func(i adt.ID, x CloseDef) bool {
		info[i].up = parent
		c.visitEmbed(i, func(j adt.ID, x CloseDef) bool {
			c.markParents(j, info)
			info[j-1].up = i // embedRoot
			info[j].up = j - 1
			return true
		})
		return true
	})
}

func (c *acceptor) markUsed(id adt.ID, marked []info) {
	if marked[id].referred {
		return
	}

	c.markUsed(marked[id].up, marked)

	// TODO: mark only first and required.
	for i := id; i != -1 && !marked[i].referred; i = c.Canopy[i].And {
		marked[i].referred = true
	}
}

func acceptorString(c *acceptor) string {
	if c == nil {
		return "null"
	}
	if c == nil || len(c.Canopy) == 0 {
		return "nil"
	}

	w := &strings.Builder{}
	idStr := func(d adt.ID) interface{} {
		if d < 0 {
			return "-"
		}
		return d
	}

	for i, c := range c.Canopy {
		fmt.Fprintf(w, "%d:{", i)
		fmt.Fprintf(w, "and: %v, ", idStr(c.And))
		fmt.Fprintf(w, "embed: %d, ", c.NextEmbed)
		fmt.Fprintf(w, "def: %v, ", c.IsDef)
		fmt.Fprintf(w, "close: %v", c.IsClosed)
		w.WriteString("}\n")
	}
	return w.String()
}
