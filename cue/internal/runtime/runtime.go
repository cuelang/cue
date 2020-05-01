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

type Runtime struct {
	*index
}

func New() *Runtime {
	return &Runtime{
		index: newIndex(sharedIndex),
	}
}

type StringIndexer interface {
	// ToIndex returns a unique positive index for s (0 < index < 2^28-1).
	//
	// For each pair of strings s and t it must return the same index if and
	// only if s == t.
	StringToIndex(s string) (index int64)

	// ToString returns a string s for index such that ToIndex(s) == index.
	IndexToString(index int64) string
}
