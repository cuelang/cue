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

package runtime

import "cuelang.org/go/cue/build"

// A Runtime maintains data structures for indexing and resuse for evaluation.
type Runtime struct {
	index *index

	Loaded map[*build.Instance]interface{}
}

// New creates a new Runtime. The builtins registered with RegisterBuiltin
// are available for
func New() *Runtime {
	return &Runtime{
		index: sharedIndex,
	}
}
