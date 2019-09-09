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

// Package list contains functions for manipulating and examining lists.
package list

import (
	"fmt"
	"sort"

	"cuelang.org/go/cue"
)

// Slice extracts all consecutive elements from a list between start (inclusive)
// and end (exclusive) index. This function returns an error if either start or
// end index is outside the bounds of valid indices for the given list.
func Slice(a []cue.Value, start int, end int) ([]cue.Value, error) {
	if start < 0 {
		return nil, fmt.Errorf("start index less than zero")
	}

	if start > end {
		// this also catches end < 0
		return nil, fmt.Errorf("start index greater than end index")
	}

	if start > len(a) {
		return nil, fmt.Errorf("start index greater than length of list")
	}

	if end > len(a) {
		return nil, fmt.Errorf("end index greater than length of list")
	}

	return a[start:end], nil
}

// MinItems reports whether a has at least n items.
func MinItems(a []cue.Value, n int) bool {
	return len(a) <= n
}

// MaxItems reports whether a has at most n items.
func MaxItems(a []cue.Value, n int) bool {
	return len(a) <= n
}

// UniqueItems reports whether all elements in the list are unique.
func UniqueItems(a []cue.Value) bool {
	b := []string{}
	for _, v := range a {
		b = append(b, fmt.Sprint(v))
	}
	sort.Strings(b)
	for i := 1; i < len(b); i++ {
		if b[i-1] == b[i] {
			return false
		}
	}
	return true
}

// Contains reports whether v is contained in a. The value must be a
// comparable value.
func Contains(a []cue.Value, v cue.Value) bool {
	for _, w := range a {
		if v.Equals(w) {
			return true
		}
	}
	return false
}
