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

import "cuelang.org/go/cue/ast"

// A Node is any abstract data type representing an value or expression.
type Node interface {
	Source() ast.Node
}

// A Value represents a node in the evaluated data graph.
type Value interface {
	Node
	value()
}

// A Decl represents all valid StructLit elements.
type Decl interface {
	Node
	declNode()
}

// An Elem represents all value ListLit elements.
//
// All Elem values can be used as a Decl.
type Elem interface {
	Decl
	elemNode()
}

// An Expr corresponds to an ast.Expr.
//
// All Expr values can be used as an Elem or Decl.
type Expr interface {
	Elem
	expr()
}

// An Evaluator provides a method to convert to a value.
type Evaluator interface {
	Node
	// TODO: Eval(c Context, env *Environment) Value
}

// A Resolver represents a reference somewhere else within a tree that resolves
// a value.
type Resolver interface {
	Node
	// TODO: Resolve(c Context, env *Environment) Arc
}

// A Yielder represents 0 or more labeled values of structs or lists.
type Yielder interface {
	Node
	yielderNode()
	// TODO: Yield()
}

// A Validator validates a Value. All Validators are Values.
type Validator interface {
	Node
	// TODO: Validate(c Context, v Value) *Bottom
}

// Value

func (*Composite) value()        {}
func (*Bottom) value()           {}
func (*Null) value()             {}
func (*Bool) value()             {}
func (*Num) value()              {}
func (*String) value()           {}
func (*Bytes) value()            {}
func (*Top) value()              {}
func (*BasicType) value()        {}
func (*BoundValue) value()       {}
func (*BuiltinValidator) value() {}

// Expr

func (*StructLit) expr()        {}
func (*ListLit) expr()          {}
func (*Bottom) expr()           {}
func (*Null) expr()             {}
func (*Bool) expr()             {}
func (*Num) expr()              {}
func (*String) expr()           {}
func (*Bytes) expr()            {}
func (*Top) expr()              {}
func (*BasicType) expr()        {}
func (*BoundExpr) expr()        {}
func (*FieldReference) expr()   {}
func (*LabelReference) expr()   {}
func (*DynamicReference) expr() {}
func (*ImportReference) expr()  {}
func (*LetReference) expr()     {}
func (*SelectorExpr) expr()     {}
func (*IndexExpr) expr()        {}
func (*SliceExpr) expr()        {}
func (*Interpolation) expr()    {}
func (*UnaryExpr) expr()        {}
func (*BinaryExpr) expr()       {}
func (*CallExpr) expr()         {}
func (*Disjunction) expr()      {}

// Decl

func (*Field) declNode()             {}
func (*OptionalField) declNode()     {}
func (*BulkOptionalField) declNode() {}
func (*DynamicField) declNode()      {}

// Decl and Yielder

func (*LetClause) declNode()    {}
func (*LetClause) yielderNode() {}

// Decl and Elem

func (*Ellipsis) elemNode()         {}
func (*Ellipsis) declNode()         {}
func (*Bottom) declNode()           {}
func (*Bottom) elemNode()           {}
func (*Null) declNode()             {}
func (*Null) elemNode()             {}
func (*Bool) declNode()             {}
func (*Bool) elemNode()             {}
func (*Num) declNode()              {}
func (*Num) elemNode()              {}
func (*String) declNode()           {}
func (*String) elemNode()           {}
func (*Bytes) declNode()            {}
func (*Bytes) elemNode()            {}
func (*Top) declNode()              {}
func (*Top) elemNode()              {}
func (*BasicType) declNode()        {}
func (*BasicType) elemNode()        {}
func (*BoundExpr) declNode()        {}
func (*BoundExpr) elemNode()        {}
func (*FieldReference) declNode()   {}
func (*FieldReference) elemNode()   {}
func (*LabelReference) declNode()   {}
func (*LabelReference) elemNode()   {}
func (*DynamicReference) declNode() {}
func (*DynamicReference) elemNode() {}
func (*ImportReference) declNode()  {}
func (*ImportReference) elemNode()  {}
func (*LetReference) declNode()     {}
func (*LetReference) elemNode()     {}
func (*SelectorExpr) declNode()     {}
func (*SelectorExpr) elemNode()     {}
func (*IndexExpr) declNode()        {}
func (*IndexExpr) elemNode()        {}
func (*SliceExpr) declNode()        {}
func (*SliceExpr) elemNode()        {}
func (*Interpolation) declNode()    {}
func (*Interpolation) elemNode()    {}
func (*UnaryExpr) declNode()        {}
func (*UnaryExpr) elemNode()        {}
func (*BinaryExpr) declNode()       {}
func (*BinaryExpr) elemNode()       {}
func (*CallExpr) declNode()         {}
func (*CallExpr) elemNode()         {}
func (*Disjunction) declNode()      {}
func (*Disjunction) elemNode()      {}

// Decl, Elem, and Yielder

func (*StructLit) declNode()    {}
func (*StructLit) elemNode()    {}
func (*StructLit) yielderNode() {}
func (*ListLit) declNode()      {}
func (*ListLit) elemNode()      {}
func (*ListLit) yielderNode()   {}
func (*ForClause) declNode()    {}
func (*ForClause) elemNode()    {}
func (*ForClause) yielderNode() {}
func (*IfClause) declNode()     {}
func (*IfClause) elemNode()     {}
func (*IfClause) yielderNode()  {}

// Yielder

func (*ValueClause) yielderNode() {}
