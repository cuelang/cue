// Copyright 2021 CUE Authors
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

// Package value contains functions for converting values to internal types
// and various other Value-related utilities.
package value

import (
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/runtime"
	"cuelang.org/go/internal/types"
)

func ToInternal(v cue.Value) (*runtime.Runtime, *adt.Vertex) {
	var t types.Value
	v.Core(&t)
	return t.R, t.V
}

// Make wraps cue.MakeValue.
func Make(ctx *adt.OpContext, v adt.Value) cue.Value {
	index := ctx.Impl().(*runtime.Runtime)
	return cue.MakeValue(index, v)
}

// func MakeError(r *runtime.Runtime, err error) cue.Value {
// 	return cue.Value{}
// }

// UnifyBuiltin returns the given Value unified with the given builtin template.
func UnifyBuiltin(v cue.Value, kind string) cue.Value {
	p := strings.Split(kind, ".")
	pkg, name := p[0], p[1]
	s, _ := runtime.SharedRuntime.LoadImport(pkg)
	if s == nil {
		return v
	}

	r, _ := ToInternal(v)
	a := s.Lookup(r.Label(name, false))
	if a == nil {
		return v
	}

	return v.Unify(cue.MakeValue(r, a))
}
