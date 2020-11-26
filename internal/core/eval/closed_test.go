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
	"fmt"
	"strings"
	"testing"

	"cuelang.org/go/internal/core/adt"
)

func TestInsert(t *testing.T) {
	testCases := []struct {
		tree   []CloseDef
		typ    func(c *acceptor, at adt.ID, p adt.Node) adt.ID
		at     adt.ID
		wantID adt.ID
		out    string
	}{{
		tree:   nil,
		at:     0,
		typ:    (*acceptor).InsertDefinition,
		wantID: 1,
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 0, def: true, close: false}`,
	}, {
		tree:   []CloseDef{{}},
		at:     0,
		typ:    (*acceptor).InsertDefinition,
		wantID: 1,
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 0, def: true, close: false}`,
	}, {
		tree:   []CloseDef{0: {And: 1}, {And: 0, IsDef: true}},
		at:     0,
		typ:    (*acceptor).InsertDefinition,
		wantID: 2,
		out: `
		0:{and: 2, embed: 0, def: false, close: false}
		1:{and: 0, embed: 0, def: true, close: false}
		2:{and: 1, embed: 0, def: true, close: false}`,
	}, {
		tree:   []CloseDef{0: {And: 1}, 1: {And: 0, IsDef: true}},
		at:     1,
		typ:    (*acceptor).InsertDefinition,
		wantID: 2,
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 2, embed: 0, def: false, close: false}
		2:{and: 0, embed: 0, def: true, close: false}`,
	}, {
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 2, IsDef: true},
			2: {And: 0, IsDef: true},
		},
		at:     1,
		typ:    (*acceptor).InsertDefinition,
		wantID: 3,
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 3, embed: 0, def: false, close: false}
		2:{and: 0, embed: 0, def: true, close: false}
		3:{and: 2, embed: 0, def: true, close: false}`,
	}, {
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 0, NextEmbed: 2, IsDef: true},
			2: {And: embedRoot},
			3: {And: 3},
		},
		at:     1,
		typ:    (*acceptor).InsertDefinition,
		wantID: 4,
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 4, embed: 2, def: false, close: false}
		2:{and: -, embed: 0, def: false, close: false}
		3:{and: 3, embed: 0, def: false, close: false}
		4:{and: 0, embed: 0, def: true, close: false}`,
	}, {
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 0, NextEmbed: 2, IsDef: true},
			2: {And: embedRoot},
			3: {And: 3},
		},
		at:     3,
		typ:    (*acceptor).InsertDefinition,
		wantID: 4,
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 2, def: true, close: false}
		2:{and: -, embed: 0, def: false, close: false}
		3:{and: 4, embed: 0, def: false, close: false}
		4:{and: 3, embed: 0, def: true, close: false}`,
	}, {
		tree:   nil,
		at:     0,
		typ:    (*acceptor).InsertEmbed,
		wantID: 2,
		out: `
		0:{and: 0, embed: 1, def: false, close: false}
		1:{and: -, embed: 0, def: false, close: false}
		2:{and: 2, embed: 0, def: false, close: false}`,
	}, {
		tree:   []CloseDef{{}},
		at:     0,
		typ:    (*acceptor).InsertEmbed,
		wantID: 2,
		out: `
		0:{and: 0, embed: 1, def: false, close: false}
		1:{and: -, embed: 0, def: false, close: false}
		2:{and: 2, embed: 0, def: false, close: false}`,
	}, {
		tree:   []CloseDef{0: {And: 1}, 1: {And: 0, IsDef: true}},
		at:     1,
		typ:    (*acceptor).InsertEmbed,
		wantID: 3,
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 2, def: true, close: false}
		2:{and: -, embed: 0, def: false, close: false}
		3:{and: 3, embed: 0, def: false, close: false}`,
	}, {
		tree:   []CloseDef{0: {NextEmbed: 1}, 1: {And: embedRoot}, 2: {And: 2}},
		at:     0,
		typ:    (*acceptor).InsertEmbed,
		wantID: 4,
		out: `
		0:{and: 0, embed: 3, def: false, close: false}
		1:{and: -, embed: 0, def: false, close: false}
		2:{and: 2, embed: 0, def: false, close: false}
		3:{and: -, embed: 1, def: false, close: false}
		4:{and: 4, embed: 0, def: false, close: false}`,
	}, {
		tree:   []CloseDef{0: {NextEmbed: 1}, 1: {And: embedRoot}, 2: {And: 2}},
		at:     1,
		typ:    (*acceptor).InsertEmbed,
		wantID: 4,
		out: `
		0:{and: 0, embed: 1, def: false, close: false}
		1:{and: -, embed: 3, def: false, close: false}
		2:{and: 2, embed: 0, def: false, close: false}
		3:{and: -, embed: 0, def: false, close: false}
		4:{and: 4, embed: 0, def: false, close: false}`,
	}, {
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 2},
			2: {And: 0},
		},
		at:     2,
		typ:    (*acceptor).InsertEmbed,
		wantID: 4,
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 2, embed: 0, def: false, close: false}
		2:{and: 0, embed: 3, def: false, close: false}
		3:{and: -, embed: 0, def: false, close: false}
		4:{and: 4, embed: 0, def: false, close: false}`,
	}}
	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			c := &acceptor{Canopy: tc.tree}
			gotID := tc.typ(c, tc.at, nil)

			if gotID != tc.wantID {
				t.Errorf("id: got %d; want %d", gotID, tc.wantID)
			}

			w := &strings.Builder{}
			// fmt.Fprintf(w, "%#v", c.Canopy)
			writeConjuncts(w, c)

			got := strings.TrimSpace(w.String())
			got = strings.ReplaceAll(got, "\t", "")
			want := strings.TrimSpace(tc.out)
			want = strings.ReplaceAll(want, "\t", "")

			if got != want {
				t.Errorf("id: got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestInsertSubtree(t *testing.T) {
	testCases := []struct {
		tree []CloseDef
		at   adt.ID
		sub  []CloseDef
		out  string
	}{{
		tree: nil,
		at:   0,
		sub:  nil,
		out: `
		0:{and: 0, embed: 1, def: false, close: false}
		1:{and: -, embed: 0, def: false, close: false}
		2:{and: 2, embed: 0, def: false, close: false}`,
	}, {
		tree: []CloseDef{{}},
		at:   0,
		sub:  nil,
		out: `
		0:{and: 0, embed: 1, def: false, close: false}
		1:{and: -, embed: 0, def: false, close: false}
		2:{and: 2, embed: 0, def: false, close: false}`,
	}, {
		tree: []CloseDef{0: {And: 1}, {And: 0, IsDef: true}},
		at:   0,
		sub:  []CloseDef{{}},
		out: `
		0:{and: 1, embed: 2, def: false, close: false}
		1:{and: 0, embed: 0, def: true, close: false}
		2:{and: -, embed: 0, def: false, close: false}
		3:{and: 3, embed: 0, def: false, close: false}`,
	}, {
		tree: []CloseDef{0: {And: 1}, {And: 0, IsDef: true}},
		at:   0,
		sub:  []CloseDef{{And: 1}, {And: 0, IsDef: true}},
		out: `
		0:{and: 1, embed: 2, def: false, close: false}
		1:{and: 0, embed: 0, def: true, close: false}
		2:{and: -, embed: 0, def: false, close: false}
		3:{and: 4, embed: 0, def: false, close: false}
		4:{and: 3, embed: 0, def: true, close: false}`,
	}, {
		tree: []CloseDef{0: {NextEmbed: 1}, 1: {And: embedRoot}, 2: {And: 2}},
		at:   0,
		sub:  []CloseDef{0: {NextEmbed: 1}, 1: {And: embedRoot}, 2: {And: 2}},
		out: `
		0:{and: 0, embed: 3, def: false, close: false}
		1:{and: -, embed: 0, def: false, close: false}
		2:{and: 2, embed: 0, def: false, close: false}
		3:{and: -, embed: 1, def: false, close: false}
		4:{and: 4, embed: 5, def: false, close: false}
		5:{and: -, embed: 0, def: false, close: false}
		6:{and: 6, embed: 0, def: false, close: false}`,
	}, {
		tree: []CloseDef{0: {NextEmbed: 1}, 1: {And: embedRoot}, 2: {And: 2}},
		at:   2,
		sub:  []CloseDef{0: {NextEmbed: 1}, 1: {And: embedRoot}, 2: {And: 2}},
		out: `
		0:{and: 0, embed: 1, def: false, close: false}
		1:{and: -, embed: 0, def: false, close: false}
		2:{and: 2, embed: 3, def: false, close: false}
		3:{and: -, embed: 0, def: false, close: false}
		4:{and: 4, embed: 5, def: false, close: false}
		5:{and: -, embed: 0, def: false, close: false}
		6:{and: 6, embed: 0, def: false, close: false}`,
	}}
	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			c := &acceptor{Canopy: tc.tree}
			d := &acceptor{Canopy: tc.sub}
			n := &nodeContext{nodeShared: &nodeShared{node: &adt.Vertex{}}}
			c.InsertSubtree(tc.at, n, &adt.Vertex{Closed: d}, false)

			w := &strings.Builder{}
			// fmt.Fprintf(w, "%#v", c.Canopy)
			writeConjuncts(w, c)

			got := strings.TrimSpace(w.String())
			got = strings.ReplaceAll(got, "\t", "")
			want := strings.TrimSpace(tc.out)
			want = strings.ReplaceAll(want, "\t", "")

			if got != want {
				t.Errorf("id: got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
func TestVerifyArcAllowed(t *testing.T) {
	fields := func(a ...adt.Feature) []adt.Feature { return a }
	results := func(a ...bool) []bool { return a }
	fieldSets := func(a ...[]adt.Feature) [][]adt.Feature { return a }

	testCases := []struct {
		desc     string
		isClosed bool
		tree     []CloseDef
		fields   [][]adt.Feature
		check    []adt.Feature
		want     []bool
	}{{
		desc:     "required and remains closed with embedding",
		isClosed: true,
		tree: []CloseDef{
			{And: 0},
		},
		fields: fieldSets(
			fields(1),
		),
		check: fields(2),
		want:  results(false),
	}, {

		// 	desc:  "empty tree accepts everything",
		// 	tree:  nil,
		// 	check: feats(1),
		// 	want:  results(true),
		// }, {
		desc: "feature required in one",
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 0, IsDef: true},
		},
		fields: fieldSets(
			fields(1),
			fields(2),
		),
		check: fields(1, 2, 3, 4),
		want:  results(false, true, false, false),
	}, {
		desc: "feature required in both",
		tree: []CloseDef{
			0: {And: 1, IsDef: true},
			1: {And: 0, IsDef: true},
		},
		fields: fieldSets(
			fields(1, 3),
			fields(2, 3),
		),
		check: fields(1, 2, 3, 4),
		want:  results(false, false, true, false),
	}, {
		desc: "feature required in neither",
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 0},
		},
		fields: fieldSets(
			fields(1, 3),
			fields(2, 3),
		),
		check: fields(1, 2, 3, 4),
		want:  results(true, true, true, true),
	}, {
		desc: "closedness of embed",
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 0, IsDef: true, NextEmbed: 2},
			2: {And: -1},
			3: {And: 4},
			4: {And: 3, IsDef: true},
		},
		fields: fieldSets(
			fields(3, 4),
			fields(),
			fields(),
			fields(),
			fields(3),
		),
		check: fields(1, 3, 4),
		want:  results(false, true, false),
	}, {
		desc: "implied required through embedding",
		tree: []CloseDef{
			0: {And: 0, NextEmbed: 1},
			1: {And: -1},
			2: {And: 3},
			3: {And: 2, IsDef: true},
		},
		fields: fieldSets(
			fields(3, 4),
			fields(),
			fields(),
			fields(3, 2),
		),
		check: fields(1, 2, 3, 4),
		want:  results(false, true, true, true),
	}, {
		desc: "implied required through recursive embedding",
		tree: []CloseDef{
			0: {And: 0, NextEmbed: 1},
			1: {And: -1},
			2: {And: 2, NextEmbed: 3},
			3: {And: -1},
			4: {And: 5},
			5: {And: 4, IsDef: true},
		},
		fields: fieldSets(
			fields(3, 4),
			fields(),
			fields(),
			fields(),
			fields(),
			fields(3, 2),
		),
		check: fields(1, 2, 3, 4),
		want:  results(false, true, true, true),
	}, {
		desc: "nil fieldSets",
		tree: []CloseDef{
			0: {And: 0, NextEmbed: 1},
			1: {And: -1},
			2: {And: 3},
			3: {And: 2, IsDef: true},
		},
		fields: fieldSets(
			nil,
			nil,
			fields(1),
			fields(2),
		),
		check: fields(1, 2, 3),
		want:  results(false, true, false),
	}, {
		desc: "required and remains closed with embedding",
		tree: []CloseDef{
			{And: 1},
			{And: 0, NextEmbed: 2, IsDef: true},
			{And: -1},
			{And: 3},
		},
		fields: fieldSets(
			fields(1),
			fields(),
			nil,
			fields(2),
		),
		check: fields(0, 1, 2, 3),
		want:  results(false, false, true, false),
	}}
	for i, tc := range testCases {
		t.Run(fmt.Sprint(i, "/", tc.desc), func(t *testing.T) {
			c := &acceptor{Canopy: tc.tree, isClosed: tc.isClosed}
			for _, f := range tc.fields {
				if f == nil {
					c.Fields = append(c.Fields, nil)
					continue
				}
				fs := &fieldSet{}
				c.Fields = append(c.Fields, fs)
				for _, id := range f {
					fs.fields = append(fs.fields, field{label: id})
				}
			}

			ctx := &adt.OpContext{}

			for i, f := range tc.check {
				got := c.verify(ctx, f)
				if got != tc.want[i] {
					t.Errorf("%d/%d: got %v; want %v", i, f, got, tc.want[i])
				}
			}
		})
	}
}

func TestCompact(t *testing.T) {
	testCases := []struct {
		desc      string
		tree      []CloseDef
		conjuncts []adt.ID
		allowAll  bool
		out       string
	}{{
		desc:      "leave nil as is",
		tree:      nil,
		conjuncts: []adt.ID{0, 0},
		out:       "nil",
	}, {
		desc:      "simplest case",
		tree:      []CloseDef{{}},
		conjuncts: []adt.ID{0, 0},
		out:       "0:{and: 0, embed: 0, def: false, close: false}",
	}, {
		desc:      "required ands should not be removed",
		tree:      []CloseDef{{And: 1}, {And: 0, IsDef: true}},
		conjuncts: []adt.ID{0, 0},
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 0, def: true, close: false}`,
	}, {
		desc:      "clear def flag for hidden fields",
		tree:      []CloseDef{{And: 1}, {And: 0, IsDef: true}},
		conjuncts: []adt.ID{0, 0},
		allowAll:  true,
		out: `
			0:{and: 1, embed: 0, def: false, close: false}
			1:{and: 0, embed: 0, def: false, close: false}`,
	}, {
		desc:      "required ands should not be removed",
		tree:      []CloseDef{{And: 1}, {And: 0, IsClosed: true}},
		conjuncts: []adt.ID{0},
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 0, def: false, close: false}`,
	}, {
		// TODO: used ands that are not required could be collapsed.
		desc:      "remove secondary unused and",
		tree:      []CloseDef{{And: 1}, {And: 0}},
		conjuncts: []adt.ID{},
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 0, def: false, close: false}`,
		// TODO: should be
		// out: `
		// 0:{and: 0, embed: 0, def: false, close: false}`,
	}, {
		// TODO: used ands that are not required could be collapsed.
		desc:      "don't remove used and",
		tree:      []CloseDef{{And: 1}, {And: 0}},
		conjuncts: []adt.ID{1},
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 0, def: false, close: false}`,
	}, {
		// TODO: redundant ands are not removed for the moment.
		desc: "remove redundant top-level and",
		tree: []CloseDef{{And: 1}, {And: 2}, {And: 0, IsDef: true}},
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 2, embed: 0, def: false, close: false}
		2:{and: 0, embed: 0, def: true, close: false}`,
		// TODO: should be
		// out: `
		// 0:{and: 1, embed: 0, def: false, close: false}
		// 1:{and: 0, embed: 0, def: true, close: false}`,
	}, {
		desc: "remove redundant top-level closed and",
		tree: []CloseDef{{And: 1}, {And: 2}, {And: 0, IsClosed: true}},
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 2, embed: 0, def: false, close: false}
		2:{and: 0, embed: 0, def: false, close: false}`,
		// TODO: should be
		// out: `
		// 0:{and: 1, embed: 0, def: false, close: false}
		// 1:{and: 0, embed: 0, def: false, close: false}`,
	}, {
		desc:      "remove embedding",
		tree:      []CloseDef{{NextEmbed: 1}, {And: embedRoot}, {And: 2}},
		conjuncts: []adt.ID{0},
		out:       "0:{and: 0, embed: 0, def: false, close: false}",
	}, {
		desc:      "keep embedding if used",
		tree:      []CloseDef{{NextEmbed: 1}, {And: embedRoot}, {And: 2}},
		conjuncts: []adt.ID{0, 2},
		out: `
		0:{and: 0, embed: 1, def: false, close: false}
		1:{and: -, embed: 0, def: false, close: false}
		2:{and: 2, embed: 0, def: false, close: false}`,
	}, {
		desc: "keep nested and",
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 0, NextEmbed: 2},
			2: {And: embedRoot, NextEmbed: 6},
			3: {And: 4},
			4: {And: 5},
			5: {And: 3},
			6: {And: embedRoot},
			7: {And: 7},
		},
		conjuncts: []adt.ID{5},
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 2, def: false, close: false}
		2:{and: -, embed: 0, def: false, close: false}
		3:{and: 4, embed: 0, def: false, close: false}
		4:{and: 5, embed: 0, def: false, close: false}
		5:{and: 3, embed: 0, def: false, close: false}`,
	}, {
		desc: "keep nested and",
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 0, NextEmbed: 2},
			2: {And: embedRoot, NextEmbed: 6},
			3: {And: 4},
			4: {And: 5},
			5: {And: 3},
			6: {And: embedRoot},
			7: {And: 7},
		},
		conjuncts: []adt.ID{7},
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 2, def: false, close: false}
		2:{and: -, embed: 0, def: false, close: false}
		3:{and: 3, embed: 0, def: false, close: false}`,
	}, {
		desc: "keep nested and",
		tree: []CloseDef{
			0: {And: 1},
			1: {And: 0, NextEmbed: 2},
			2: {And: embedRoot, NextEmbed: 6},
			3: {And: 4},
			4: {And: 5},
			5: {And: 3},
			6: {And: embedRoot},
			7: {And: 9},
			8: {And: 7},
			9: {And: 8},
		},
		conjuncts: []adt.ID{8},
		out: `
		0:{and: 1, embed: 0, def: false, close: false}
		1:{and: 0, embed: 2, def: false, close: false}
		2:{and: -, embed: 0, def: false, close: false}
		3:{and: 5, embed: 0, def: false, close: false}
		4:{and: 3, embed: 0, def: false, close: false}
		5:{and: 4, embed: 0, def: false, close: false}`,
	}}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			v := &adt.Vertex{
				Parent: &adt.Vertex{
					Closed: &acceptor{Canopy: tc.tree},
				},
			}
			for _, id := range tc.conjuncts {
				v.Conjuncts = append(v.Conjuncts, adt.Conjunct{CloseID: id})
			}

			Compact(v, tc.allowAll)
			c := closedInfo(v)

			w := &strings.Builder{}
			// fmt.Fprintf(w, "%#v", c.Canopy)
			writeConjuncts(w, c)
			got := strings.TrimSpace(w.String())
			got = strings.ReplaceAll(got, "\t", "")
			want := strings.TrimSpace(tc.out)
			want = strings.ReplaceAll(want, "\t", "")
			if got != want {
				t.Errorf("id: got:\n%s\nwant:%s", got, want)
			}
		})
	}
}

func writeConjuncts(w *strings.Builder, c *acceptor) {
	if len(c.Canopy) == 0 {
		w.WriteString("nil")
		return
	}

	idStr := func(d adt.ID) interface{} {
		if d < 0 {
			return "-"
		}
		return d
	}

	for i, c := range c.Canopy {
		fmt.Fprintf(w, "%d:{", i)
		fmt.Fprintf(w, "and: %v, ", idStr(c.And))
		fmt.Fprintf(w, "embed: %d, ", c.NextEmbed)
		fmt.Fprintf(w, "def: %v, ", c.IsDef)
		fmt.Fprintf(w, "close: %v", c.IsClosed)
		w.WriteString("}\n")
	}
	// writeAnd(w, c, 0)
}

func writeAnd(w *strings.Builder, c *acceptor, id adt.ID) {
	w.WriteString("&(")
	c.visitAnd(id, func(id adt.ID, n CloseDef) bool {
		w.WriteString(" ")
		if n.IsDef || n.IsClosed {
			w.WriteString("*")
		}
		fmt.Fprintf(w, "%d[", id)
		c.visitEmbed(id, func(id adt.ID, n CloseDef) bool {
			writeAnd(w, c, id)
			return true
		})
		w.WriteString("]")
		return true
	})
	w.WriteString(")")
}
