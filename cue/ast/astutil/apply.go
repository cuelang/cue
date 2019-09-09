// Copyright 2018 The CUE Authors
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

package astutil

import (
	"fmt"
	"reflect"

	"cuelang.org/go/cue/ast"
)

// A Cursor describes a node encountered during Apply.
// Information about the node and its parent is available
// from the Node, Parent, and Index methods.
//
// The methods Replace, Delete, InsertBefore, and InsertAfter
// can be used to change the AST without disrupting Apply.
// Delete, InsertBefore, and InsertAfter are only defined for modifying
// a StructLit and will panic in any other context.
type Cursor interface {
	// Node returns the current Node.
	Node() ast.Node

	// Parent returns the parent of the current Node.
	Parent() Cursor

	// Index reports the index >= 0 of the current Node in the slice of Nodes
	// that contains it, or a value < 0 if the current Node is not part of a
	// list.
	Index() int

	// Replace replaces the current Node with n.
	// The replacement node is not walked by Apply.
	Replace(n ast.Node)

	// Delete deletes the current Node from its containing struct.
	// If the current Node is not part of a struct, Delete panics.
	Delete()

	// InsertAfter inserts n after the current Node in its containing struct.
	// If the current Node is not part of a struct, InsertAfter panics.
	// Apply does not walk n.
	InsertAfter(n ast.Node)

	// InsertBefore inserts n before the current Node in its containing struct.
	// If the current Node is not part of a struct, InsertBefore panics.
	// Apply will not walk n.
	InsertBefore(n ast.Node)
}

type cursor struct {
	parent   Cursor
	node     ast.Node
	typ      interface{} // the type of the node
	index    int         // position of any of the sub types.
	replaced bool
}

func newCursor(parent Cursor, n ast.Node, typ interface{}) *cursor {
	return &cursor{
		parent: parent,
		typ:    typ,
		node:   n,
		index:  -1,
	}
}

func (c *cursor) Parent() Cursor { return c.parent }
func (c *cursor) Index() int     { return c.index }
func (c *cursor) Node() ast.Node { return c.node }

func (c *cursor) Replace(n ast.Node) {
	// panic if the value cannot convert to the original type.
	reflect.ValueOf(n).Convert(reflect.TypeOf(c.typ).Elem())
	c.node = n
	c.replaced = true
}

func (c *cursor) InsertAfter(n ast.Node)  { panic("unsupported") }
func (c *cursor) InsertBefore(n ast.Node) { panic("unsupported") }
func (c *cursor) Delete()                 { panic("unsupported") }

// Walk traverses an AST in depth-first order: It starts by calling f(node);
// node must not be nil. If before returns true, Walk invokes f recursively for
// each of the non-nil children of node, followed by a call of after. Both
// functions may be nil. If before is nil, it is assumed to always return true.
//

// Apply traverses a syntax tree recursively, starting with root,
// and calling pre and post for each node as described below.
// Apply returns the syntax tree, possibly modified.
//
// If pre is not nil, it is called for each node before the node's
// children are traversed (pre-order). If pre returns false, no
// children are traversed, and post is not called for that node.
//
// If post is not nil, and a prior call of pre didn't return false,
// post is called for each node after its children are traversed
// (post-order). If post returns false, traversal is terminated and
// Apply returns immediately.
//
// Only fields that refer to AST nodes are considered children;
// i.e., token.Pos, Scopes, Objects, and fields of basic types
// (strings, etc.) are ignored.
//
// Children are traversed in the order in which they appear in the
// respective node's struct definition.
//
func Apply(node ast.Node, before, after func(Cursor) bool) ast.Node {
	walk(&inspector{before: before, after: after}, nil, &node)
	return node
}

// A visitor's before method is invoked for each node encountered by Walk.
// If the result visitor w is true, Walk visits each of the children
// of node with the visitor w, followed by a call of w.After.
type visitor interface {
	Before(Cursor) visitor
	After(Cursor) bool
}

// Helper functions for common node lists. They may be empty.

func walkExprList(v visitor, parent Cursor, ptr interface{}, list []ast.Expr) {
	c := newCursor(parent, nil, nil)
	for i, x := range list {
		c.index = i
		c.node = x
		c.typ = &list[i]
		walkCursor(v, c)
		if x != c.node {
			list[i] = c.node.(ast.Expr)
		}
	}
}

type declsCursor struct {
	*cursor
	decls, after []ast.Decl
	delete       bool
}

func (c *declsCursor) InsertAfter(n ast.Node) {
	c.after = append(c.after, n.(ast.Decl))
}

func (c *declsCursor) InsertBefore(n ast.Node) {
	c.decls = append(c.decls, n.(ast.Decl))
}

func (c *declsCursor) Delete() { c.delete = true }

func walkDeclList(v visitor, parent Cursor, list []ast.Decl) []ast.Decl {
	c := &declsCursor{
		cursor: newCursor(parent, nil, nil),
		decls:  make([]ast.Decl, 0, len(list)),
	}
	for i, x := range list {
		c.node = x
		c.typ = &list[i]
		walkCursor(v, c)
		if !c.delete {
			c.decls = append(c.decls, c.node.(ast.Decl))
		}
		c.delete = false
		c.decls = append(c.decls, c.after...)
		c.after = c.after[:0]
	}
	return c.decls
}

func walk(v visitor, parent Cursor, nodePtr interface{}) {
	res := reflect.Indirect(reflect.ValueOf(nodePtr))
	n := res.Interface()
	node := n.(ast.Node)
	c := newCursor(parent, node, nodePtr)
	walkCursor(v, c)
	if node != c.node {
		res.Set(reflect.ValueOf(c.node))
	}
}

// walkCursor traverses an AST in depth-first order: It starts by calling
// v.Visit(node); node must not be nil. If the visitor w returned by
// v.Visit(node) is not nil, walk is invoked recursively with visitor
// w for each of the non-nil children of node, followed by a call of
// w.Visit(nil).
//
func walkCursor(v visitor, c Cursor) {
	if v = v.Before(c); v == nil {
		return
	}

	node := c.Node()

	// TODO: record the comment groups and interleave with the values like for
	// parsing and printing?
	comments := node.Comments()
	for _, cm := range comments {
		walk(v, c, &cm)
	}

	// walk children
	// (the order of the cases matches the order
	// of the corresponding node types in go)
	switch n := node.(type) {
	// Comments and fields
	case *ast.Comment:
		// nothing to do

	case *ast.CommentGroup:
		for _, cg := range n.List {
			walk(v, c, &cg)
		}

	case *ast.Attribute:
		// nothing to do

	case *ast.Field:
		walk(v, c, &n.Label)
		if n.Value != nil {
			walk(v, c, &n.Value)
		}
		for _, a := range n.Attrs {
			walk(v, c, &a)
		}

	case *ast.StructLit:
		n.Elts = walkDeclList(v, c, n.Elts)

	// Expressions
	case *ast.BottomLit, *ast.BadExpr, *ast.Ident, *ast.BasicLit:
		// nothing to do

	case *ast.TemplateLabel:
		walk(v, c, &n.Ident)

	case *ast.Interpolation:
		walkExprList(v, c, &n, n.Elts)

	case *ast.ListLit:
		walkExprList(v, c, &n, n.Elts)

	case *ast.Ellipsis:
		if n.Type != nil {
			walk(v, c, &n.Type)
		}

	case *ast.ParenExpr:
		walk(v, c, &n.X)

	case *ast.SelectorExpr:
		walk(v, c, &n.X)
		walk(v, c, &n.Sel)

	case *ast.IndexExpr:
		walk(v, c, &n.X)
		walk(v, c, &n.Index)

	case *ast.SliceExpr:
		walk(v, c, &n.X)
		if n.Low != nil {
			walk(v, c, &n.Low)
		}
		if n.High != nil {
			walk(v, c, &n.High)
		}

	case *ast.CallExpr:
		walk(v, c, &n.Fun)
		walkExprList(v, c, &n, n.Args)

	case *ast.UnaryExpr:
		walk(v, c, &n.X)

	case *ast.BinaryExpr:
		walk(v, c, &n.X)
		walk(v, c, &n.Y)

	// Declarations
	case *ast.ImportSpec:
		if n.Name != nil {
			walk(v, c, &n.Name)
		}
		walk(v, c, &n.Path)

	case *ast.BadDecl:
		// nothing to do

	case *ast.ImportDecl:
		for _, s := range n.Specs {
			walk(v, c, &s)
		}

	case *ast.EmbedDecl:
		walk(v, c, &n.Expr)

	case *ast.Alias:
		walk(v, c, &n.Ident)
		walk(v, c, &n.Expr)

	case *ast.Comprehension:
		clauses := n.Clauses
		for i := range n.Clauses {
			walk(v, c, &clauses[i])
		}
		walk(v, c, &n.Struct)

	// Files and packages
	case *ast.File:
		n.Decls = walkDeclList(v, c, n.Decls)

	case *ast.Package:
		walk(v, c, &n.Name)

	case *ast.ListComprehension:
		walk(v, c, &n.Expr)
		clauses := n.Clauses
		for i := range clauses {
			walk(v, c, &clauses[i])
		}

	case *ast.ForClause:
		if n.Key != nil {
			walk(v, c, &n.Key)
		}
		walk(v, c, &n.Value)
		walk(v, c, &n.Source)

	case *ast.IfClause:
		walk(v, c, &n.Condition)

	default:
		panic(fmt.Sprintf("Walk: unexpected node type %T", n))
	}

	v.After(c)
}

type inspector struct {
	before func(Cursor) bool
	after  func(Cursor) bool

	commentStack []commentFrame
	current      commentFrame
}

type commentFrame struct {
	cg  []*ast.CommentGroup
	pos int8
}

func (f *inspector) Before(c Cursor) visitor {
	node := c.Node()
	if f.before == nil || (f.before(c) && node == c.Node()) {
		f.commentStack = append(f.commentStack, f.current)
		f.current = commentFrame{cg: node.Comments()}
		f.visitComments(c, f.current.pos)
		return f
	}
	return nil
}

func (f *inspector) After(c Cursor) bool {
	f.visitComments(c, 127)
	p := len(f.commentStack) - 1
	f.current = f.commentStack[p]
	f.commentStack = f.commentStack[:p]
	f.current.pos++
	if f.after != nil {
		f.after(c)
	}
	return true
}

func (f *inspector) visitComments(p Cursor, pos int8) {
	c := &f.current
	for i := 0; i < len(c.cg); i++ {
		cg := c.cg[i]
		if cg.Position == pos {
			continue
		}
		cursor := newCursor(p, cg, cg)
		if f.before == nil || (f.before(cursor) && !cursor.replaced) {
			for j, c := range cg.List {
				cursor := newCursor(p, c, &c)
				if f.before == nil || (f.before(cursor) && !cursor.replaced) {
					if f.after != nil {
						f.after(cursor)
					}
				}
				cg.List[j] = cursor.node.(*ast.Comment)
			}
			if f.after != nil {
				f.after(cursor)
			}
		}
		c.cg[i] = cursor.node.(*ast.CommentGroup)
	}
}
