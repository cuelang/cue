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

package export

import (
	"sort"

	"cuelang.org/go/internal/core/adt"
)

// TODO: topological sort should go arguably in a more fundamental place as it
// may be needed to sort inputs for comprehensions.

// sortDecls does a topological sort of an ast.Decl list d based on their
// assigned features f. If a declaration has no feature, it should be assigned
// the same feature as the previous element. The use of stable sort means that
// these elements will keep the same order. Otherwise unknown features will be
// grouped at the end.
//
// Slice d and a must have the same length.
// func (e *Exporter) sortDecls(d []ast.Decl, a []adt.Feature, s []*adt.StructLit) {
// 	f := e.extractFeatures(s)
// 	if len(f) == 0 {
// 		return
// 	}

// 	m := sortArcs(f)

// 	sort.SliceStable(d, func(i, j int) bool {
// 		pi, ok := m[a[i]]
// 		if !ok {
// 			return false
// 		}
// 		pj, ok := m[a[j]]
// 		if !ok {
// 			return true
// 		}
// 		return pi > pj
// 	})
// }

func (e *exporter) sortedArcs(v *adt.Vertex) (sorted []*adt.Vertex) {
	a := e.extractFeatures(v.Structs)
	if len(a) == 0 {
		return v.Arcs
	}

	sorted = make([]*adt.Vertex, len(v.Arcs))
	copy(sorted, v.Arcs)

	m := sortArcs(a)
	sort.SliceStable(sorted, func(i, j int) bool {
		if m[sorted[i].Label] == 0 {
			return m[sorted[j].Label] != 0
		}
		return m[sorted[i].Label] > m[sorted[j].Label]
	})

	return sorted
}

func (e *exporter) extractFeatures(in []*adt.StructLit) (a [][]adt.Feature) {
	for _, s := range in {
		sorted := []adt.Feature{}
		for _, e := range s.Decls {
			switch x := e.(type) {
			case *adt.Field:
				sorted = append(sorted, x.Label)
			}
		}

		if len(sorted) > 1 {
			a = append(a, sorted)
		}
	}
	return a
}

// sortArcs does a topological sort of arcs based on a variant of Kahn's
// algorithm. See
// https://www.geeksforgeeks.org/topological-sorting-indegree-based-solution/
//
// It returns a map from feature to int where the feature with the highest
// number should be sorted first.
func sortArcs(fronts [][]adt.Feature) map[adt.Feature]int {
	counts := map[adt.Feature]int{}
	for _, a := range fronts {
		if len(a) <= 1 {
			continue // no dependencies
		}
		for _, f := range a[1:] {
			counts[f]++
		}
	}

	// We could use a Heap instead of simple linear search here if we are
	// concerned about the time complexity.

	index := -1
outer:
	for {
	lists:
		for i, a := range fronts {
			for len(a) > 0 {
				f := a[0]
				n := counts[f]
				if n > 0 {
					continue lists
				}

				// advance list and decrease dependency.
				a = a[1:]
				fronts[i] = a
				if len(a) > 1 && counts[a[0]] > 0 {
					counts[a[0]]--
				}

				if n == 0 { // may be head of other lists as well
					counts[f] = index
					index--
				}
				continue outer // progress
			}
		}

		for _, a := range fronts {
			if len(a) > 0 {
				// Detected a cycle. Fire at will to make progress.
				counts[a[0]] = 0
				continue outer
			}
		}
		break
	}

	return counts
}
