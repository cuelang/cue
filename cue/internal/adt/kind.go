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

package adt

import (
	"fmt"
)

// Concreteness is a measure of the level of concreteness of a value, where
// lower values mean more concrete.
type Concreteness int

const (
	BottomLevel Concreteness = iota

	// Concrete indicates a concrete scalar value, list or struct.
	Concrete

	// Constraint indicates a non-concrete scalar value that is more specific,
	// than a top-level type.
	Constraint

	// PrimitiveType indicates a top-level specific type, for instance, string,
	// bytes, number, or bool.
	Type

	// Any indicates any value, or top.
	Any
)

// IsConcrete returns whether a value is concrete.
func IsConcrete(v Value) bool {
	return v.Concreteness() <= Concrete
}

// Kind reports the Value kind.
type Kind uint16

const (
	NullKind Kind = (1 << iota)
	BoolKind
	IntKind
	FloatKind
	StringKind
	BytesKind
	ListKind
	StructKind

	allKinds

	BottomKind = 0

	NumKind     = IntKind | FloatKind
	TopKind     = (allKinds - 1) // all kinds, but not references
	ScalarKinds = NullKind | BoolKind |
		IntKind | FloatKind | StringKind | BytesKind

	typeKinds = (allKinds - 1)

	atomKind    = (ListKind - 1)
	addableKind = (StructKind - 1)

	comparableKind = (ListKind - 1)
	stringableKind = ScalarKinds | StringKind
)

func isTop(v Value) bool {
	_, ok := v.(*Top)
	return ok
}

func isCustom(v Value) bool {
	_, ok := v.(*BuiltinValidator)
	return ok
}

// IsAnyOf reports whether k is any of the given kinds.
//
// For instances, k.IsAnyOf(String|Bytes) reports whether k overlaps with
// the String or Bytes kind.
func (k Kind) IsAnyOf(of Kind) bool {
	return k&of != BottomKind
}

// CanString reports whether the given type can convert to a string.
func (k Kind) CanString() bool {
	return k&StringKind|ScalarKinds != BottomKind
}

// String reports the string representation of k.
func (k Kind) String() string {
	str := ""
	if k&TopKind == TopKind {
		str = "_"
		goto finalize
	}
	for i := Kind(1); i < allKinds; i <<= 1 {
		t := ""
		switch k & i {
		case BottomKind:
			continue
		case NullKind:
			t = "null"
		case BoolKind:
			t = "bool"
		case IntKind:
			if k&FloatKind != 0 {
				t = "number"
			} else {
				t = "int"
			}
		case FloatKind:
			if k&IntKind != 0 {
				continue
			}
			t = "float"
		case StringKind:
			t = "string"
		case BytesKind:
			t = "bytes"
		case ListKind:
			t = "list"
		case StructKind:
			t = "struct"
		default:
			t = fmt.Sprintf("<unknown> %x", int(i))
		}
		if str != "" {
			str += "|"
		}
		str += t
	}
finalize:
	if str == "" {
		return "_|_"
	}
	return str
}
