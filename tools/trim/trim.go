// Copyright 2018 The CUE Authors
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

// Package trim removes definitions that may be inferred from
// templates.
//
// A field, struct, or list is removed if it is implied by a constraint, such
// as from an optional field matching a required field, a list type value,
// a comprehension or any other implied content. It will modify the files in place.
//
// Limitations
//
// Removal is on a best effort basis. Some caveats:
// - Fields in implied content may refer to fields within the struct in which
//   they are included, but are only resolved on a best-effort basis.
// - Disjunctions that contain structs in implied content cannot be used to
//   remove fields.
// - There is currently no verification step: manual verification is required.
//
// Examples:
//
// 	light: [string]: {
// 		room:          string
// 		brightnessOff: *0.0 | >=0 & <=100.0
// 		brightnessOn:  *100.0 | >=0 & <=100.0
// 	}
//
// 	light: ceiling50: {
// 		room:          "MasterBedroom"
// 		brightnessOff: 0.0    // this line
// 		brightnessOn:  100.0  // and this line will be removed
// 	}
//
// Results in:
//
// 	light: [string]: {
// 		room:          string
// 		brightnessOff: *0.0 | >=0 & <=100.0
// 		brightnessOn:  *100.0 | >=0 & <=100.0
// 	}
//
// 	light: ceiling50: {
// 		room: "MasterBedroom"
// 	}
//
package trim

import (
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/internal"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/runtime"
	"cuelang.org/go/internal/core/subsume"
)

// Config configures trim options.
type Config struct {
	Trace bool
}

// Files trims fields in the given files that can be implied from other fields,
// as can be derived from the evaluated values in inst.
// Trimming is done on a best-effort basis and only when the removed field
// is clearly implied by another field, rather than equal sibling fields.
func Files(files []*ast.File, inst *cue.Instance, cfg *Config) error {
	rx, vx := internal.CoreValue(inst.Value())
	r := rx.(*runtime.Runtime)
	v := vx.(*adt.Vertex)

	t := &trimmer{
		Config: *cfg,
		ctx:    adt.NewContext(r, v),
		remove: map[ast.Node]bool{},
	}

	t.findSubordinates(v)

	// Remove subordinate values from files.
	for _, f := range files {
		astutil.Apply(f, func(c astutil.Cursor) bool {
			if f, ok := c.Node().(*ast.Field); ok && t.remove[f.Value] {
				c.Delete()
			}
			return true
		}, nil)
	}

	return nil
}

type trimmer struct {
	Config

	ctx    *adt.OpContext
	remove map[ast.Node]bool
}

func (t *trimmer) markRemove(c adt.Conjunct) {
	if src := c.Expr().Source(); src != nil {
		t.remove[src] = true
	}
}

const dominatorNode = adt.ComprehensionSpan | adt.DefinitionSpan | adt.ConstraintSpan

func isDominator(c adt.Conjunct) bool {
	return c.CloseInfo.IsInOneOf(dominatorNode)
}

// Roots of constraints are not allowed to strip conjuncts by
// themselves as it will eliminate the reason for the trigger.
func allowRemove(v *adt.Vertex) bool {
	for _, c := range v.Conjuncts {
		if isDominator(c) &&
			(c.CloseInfo.Location() != c.Expr() ||
				c.CloseInfo.RootSpanType() != adt.ConstraintSpan) {
			return true
		}
	}
	return false
}

// A parent may be removed if there is not a "no" and there is at least one
// match, confirming we are not in a definition or constraint ourselves and
// cannot have a no
const (
	no = 1 << iota
	maybe
	yes
)

func (t *trimmer) findSubordinates(v *adt.Vertex) int {
	// TODO(structure sharing): do not descend into vertices whose parent is not
	// equal to the parent. This is not relevant at this time, but may be so in
	// the future.

	if len(v.Arcs) > 0 {
		var match int
		for _, a := range v.Arcs {
			match |= t.findSubordinates(a)
		}

		// This also skips embedded scalars if not all fields are removed. In
		// this case we need to preserve the scalar to keep the type of the
		// struct intact, which might as well be done by not removing the scalar
		// type.
		if match&yes == 0 || match&no != 0 {
			return match
		}
	}

	if !allowRemove(v) {
		return no
	}

	switch v.BaseValue.(type) {
	case *adt.StructMarker, *adt.ListMarker:
		// Rely on previous processing of the Arcs and the fact that we take the
		// default value to check dominator subsumption, meaning that we don't
		// have to check additional optional constraints to pass subsumption.
		for _, c := range v.Conjuncts {
			if !isDominator(c) {
				t.markRemove(c)
			}
		}
		return yes
	}

	doms := &adt.Vertex{}
	subs := &adt.Vertex{}

	for _, c := range v.Conjuncts {
		if !isDominator(c) {
			subs.AddConjunct(c)
		} else {
			doms.AddConjunct(c)
		}
	}

	if len(subs.Conjuncts) == 0 {
		return maybe // only if there are siblings to be removed.
	}

	doms.Finalize(t.ctx)
	subs.Finalize(t.ctx)

	doms = doms.Default()

	if doms.IsErr() || subs.IsErr() {
		return no
	}

	if err := subsume.Value(t.ctx, subs, doms); err == nil {
		for _, c := range subs.Conjuncts {
			t.markRemove(c)
		}
		return yes
	}

	return no
}
