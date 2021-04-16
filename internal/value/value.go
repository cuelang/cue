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

// TODO:
// Make wraps cue.MakeValue.
func Make(ctx *adt.OpContext, v adt.Value) cue.Value {
	return (*cue.Context)(ctx.Impl().(*runtime.Runtime)).Encode(v)
}

//
// func Make(r *runtime.Runtime, v *adt.Vertex) cue.Value {
// 	return cue.Value{}
// }

// func MakeError(r *runtime.Runtime, err error) cue.Value {
// 	return cue.Value{}
// }
