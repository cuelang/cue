// Copyright 2019 CUE Authors
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

// Package cuego allows using CUE constraints in Go programs.
//
// CUE constraints can be used to validate Go types as well as fill out
// missing fields that can be implied from the constraints and the values
// already defined within this type.
//
// CUE constraints can be added through field tags or by associating
// CUE code with a Go type. The field tags method follows the usual
// Go pattern:
//
//     type Sum struct {
//         A int `cue:"C-B,opt"`
//         B int `cue:"C-A,opt"`
//         C int `cue:"A+B,opt"`
//     }
//
//     func main() {
//         fmt.Println(cuego.Validate(&Sum{A: 1, B: 5, C: 7}))
//     }
//     // Validate verifies that all values comply to the constraints.
//     func (s *Sum) Validate() error { return cuego.Validate(s) }
//
//     // Update completes unspecified values to satisfy constraints or returns
//     // an error when this is not possible.
//     func (s *Sum) Update() error { return cuego.Update(s) }
//
//
// Defining Constraints
//
// There are two ways to annotate Go types with CUE constraints: through
// field tags and by associating CUE code with types.
//
// About field tags
//
//
// Parse and Register allow annotating CUE constraints with any Go types.
// Register associates constraints that will always apply when the respective
// type is processed. Compile allows the creation of self-contained constraints
// that may be checked only under certain circumstances.
//
//
// Validating Go Values
//
//    cuego.Validate(p)
//
// Validation assumes that all values are filled in correctly and will not
// infer values from an update. To automatically infer values, use Update.
//
//
// Updating Go Values
//
// Package cuego can also be used to infer unspecified values from a set of
// CUE constraints, for instance to fill out fields in a struct.
// An Update will implicitly validate a struct.
//
package cuego // import "cuelang.org/go/cuego"

// The first goal of this packages is to get the semantics right. After that,
// there are a lot of performance gains to be made:
// - cache the type info extracted during value (as opposed to type) conversion
// - remove the usage of mutex for value conversions
// - avoid the JSON round trip for Decode, as used in Update
// - generate native code for validating and updating
