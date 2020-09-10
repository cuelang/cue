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

package eval

import (
	"testing"

	"cuelang.org/go/internal/core/adt"
	"github.com/google/go-cmp/cmp"
)

func TestRewriteClosed(t *testing.T) {
	testCases := []struct {
		desc    string
		close   *CloseDef
		replace map[adt.ID]*CloseDef
		want    *CloseDef
	}{{
		desc:  "introduce new",
		close: nil,
		replace: map[adt.ID]*CloseDef{
			0: {ID: 2, IsAnd: false, List: nil},
		},
		want: &CloseDef{
			ID: 0x02,
		},
	}, {
		desc: "auto insert missing 0",
		close: &CloseDef{
			ID: 1,
		},
		replace: map[adt.ID]*CloseDef{
			0: {ID: 2, IsAnd: false, List: nil},
			1: nil, // keep 1
		},
		want: &CloseDef{
			IsAnd: true,
			List:  []*CloseDef{{ID: 1}, {ID: 2}},
		},
	}, {
		desc: "a: #A & #B",
		close: &CloseDef{
			ID: 1,
		},
		replace: map[adt.ID]*CloseDef{
			1: {ID: 1, IsAnd: true, List: []*CloseDef{{ID: 2}, {ID: 3}}},
		},
		want: &CloseDef{
			ID:    0x01,
			IsAnd: true,
			List:  []*CloseDef{{ID: 2}, {ID: 3}},
		},
	}, {
		desc: "eliminateUnusedToEmpty",
		close: &CloseDef{
			ID: 1,
		},
		replace: map[adt.ID]*CloseDef{
			0: nil,
		},
		want: nil,
	}, {
		// Eliminate an embedding for which there are no more entries.
		desc: "eliminateOneEmbedding",
		close: &CloseDef{
			ID: 0,
			List: []*CloseDef{
				{ID: 2},
				{ID: 3},
			},
		},
		replace: map[adt.ID]*CloseDef{2: nil},
		want:    &CloseDef{ID: 2},
	}, {
		desc: "eliminateAllEmbeddings",
		close: &CloseDef{
			ID: 2,
			List: []*CloseDef{
				{ID: 2},
				{ID: 3},
			},
		},
		replace: map[adt.ID]*CloseDef{0: {ID: 4}, 4: nil},
		want:    &CloseDef{ID: 4},
	}, {
		// Do not eliminate an embedding that has a replacement.
		desc: "eliminateOneEmbeddingByMultiple",
		close: &CloseDef{
			ID: 0,
			List: []*CloseDef{
				{ID: 2},
				{ID: 3},
			},
		},
		replace: map[adt.ID]*CloseDef{
			2: nil,
			3: {ID: 3, IsAnd: true, List: []*CloseDef{{ID: 4}, {ID: 5}}},
		},
		want: &CloseDef{
			ID: 0x00,
			List: []*CloseDef{
				{ID: 2},
				{ID: 3, IsAnd: true, List: []*CloseDef{{ID: 4}, {ID: 5}}},
			},
		},
	}, {
		// Select b within a
		// a: {      // ID: 0
		//     #A    // ID: 1
		//     #B    // ID: 2
		//     b: #C // ID: 0
		// }
		// #C: {
		//     b: #D // ID: 3
		// }
		//
		desc: "embeddingOverruledByField",
		close: &CloseDef{
			ID: 0,
			List: []*CloseDef{
				{ID: 1},
				{ID: 2},
				{ID: 0},
			},
		},
		replace: map[adt.ID]*CloseDef{0: {ID: 3}},
		want:    &CloseDef{ID: 3},
	}, {
		// Select b within a
		// a: {      // ID: 0
		//     #A    // ID: 1
		//     #B    // ID: 2
		//     b: #C // ID: 0
		// }
		// #C: {
		//     b: #D & #E // ID: 3 & 4
		// }
		//
		desc: "embeddingOverruledByMultiple",
		close: &CloseDef{
			ID: 0,
			List: []*CloseDef{
				{ID: 1},
				{ID: 2},
				{ID: 0},
			},
		},
		replace: map[adt.ID]*CloseDef{
			0: {IsAnd: true, List: []*CloseDef{{ID: 3}, {ID: 4}}},
		},
		want: &CloseDef{
			ID:    0,
			IsAnd: true,
			List:  []*CloseDef{{ID: 3}, {ID: 4}},
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := updateClosed(tc.close, tc.replace)
			if !cmp.Equal(got, tc.want) {
				t.Error(cmp.Diff(got, tc.want))
			}
		})
	}
}
