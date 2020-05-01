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
	"strconv"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
)

// Feature
type Feature uint32

const InvalidLabel Feature = 0x7 // 0xb111

const MaxIndex = 1<<28 - 1

type StringIndexer interface {
	// ToIndex returns a unique positive index for s (0 < index < 2^28-1).
	//
	// For each pair of strings s and t it must return the same index if and
	// only if s == t.
	StringToIndex(s string) (index int64)

	// ToString returns a string s for index such that ToIndex(s) == index.
	IndexToString(index int64) string
}

func (f Feature) ToString(index StringIndexer) string {
	if f == 0 {
		return "_"
	}
	x := f.Index()
	switch f.Typ() {
	case IntLabel:
		return strconv.Itoa(int(x))
	case StringLabel:
		s := index.IndexToString(int64(x))
		if ast.IsValidIdent(s) &&
			!strings.HasPrefix(s, "#") &&
			!strings.HasPrefix(s, "_") {
			return s
		}
		return strconv.Quote(s)
	default:
		return index.IndexToString(int64(x))
	}
}

func MakeLabel(p token.Pos, index int64, f FeatureType) (Feature, errors.Error) {
	if 0 > index || index > MaxIndex {
		return InvalidLabel,
			errors.Newf(p, "int label out of range (%d not >=0 and <= %d)",
				index, MaxIndex)
	}
	return Feature(index)<<indexShift | Feature(f), nil
}

type FeatureType int8

const (
	StringLabel           FeatureType = 0 // 0b000
	IntLabel              FeatureType = 1 // 0b001
	DefinitionLabel       FeatureType = 3 // 0b011
	HiddenLabel           FeatureType = 6 // 0b110
	HiddenDefinitionLabel FeatureType = 7 // 0b111

	// letLabel              FeatureType = 0b010

	fTypeMask Feature = 7 // 0b111

	indexShift = 3
)

func (f Feature) IsValid() bool    { return f != 0 }
func (f Feature) Typ() FeatureType { return FeatureType(f & fTypeMask) }
func (f Feature) IsDef() bool      { return f.Typ()&DefinitionLabel == DefinitionLabel }
func (f Feature) IsInt() bool      { return f.Typ() == IntLabel }
func (f Feature) IsHidden() bool   { return f.Typ()&HiddenLabel == HiddenLabel }
func (f Feature) Index() int       { return int(f >> indexShift) }

// func (f Feature) isLet() bool  { return f.typ() == letLabel }
