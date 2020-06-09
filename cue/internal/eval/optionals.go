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

import "cuelang.org/go/cue/internal/adt"

// FieldSet represents the fields for a single struct literal, along
// the constraints of fields that may be added.
type FieldSet struct {
	// TODO: look at consecutive identical environments to figure out
	// what belongs to same definition?
	env *adt.Environment

	// field marks the optional conjuncts of all explicit fields.
	// Required fields are marked as empty
	fields []field

	// literal map[adt.Feature][]adt.Node

	// excluded are all literal fields that already exist.
	bulk       []bulkField
	additional []adt.Expr
	isOpen     bool // has a ...
}

type field struct {
	label    adt.Feature
	optional []adt.Node
}

type bulkField struct {
	check fieldMatcher
	expr  adt.Node // Conjunct
}

func (o *FieldSet) Match(c *adt.OpContext, f adt.Feature) bool {
	if len(o.additional) > 0 {
		return true
	}
	if o.fieldIndex(f) >= 0 {
		return true
	}
	for _, b := range o.bulk {
		if b.check.Match(c, f) {
			return true
		}
	}
	return false
}

// MatchAndInsert finds matching optional parts for a given Arc and adds its
// conjuncts. Bulk fields are only applied if no fields match, and additional
// constraints are only added if neither regular nor bulk fields match.
func (o *FieldSet) MatchAndInsert(c *adt.OpContext, arc *adt.Vertex) {
	// Match normal fields
	p := 0
	for ; p < len(o.fields); p++ {
		if o.fields[p].label == arc.Label {
			break
		}
	}
	if p < len(o.fields) {
		for _, e := range o.fields[p].optional {
			arc.AddConjunct(adt.MakeConjunct(o.env, e))
		}
		return
	}

	// match bulk optional fields / pattern properties
	matched := false
	for _, f := range o.bulk {
		if f.check.Match(c, arc.Label) {
			matched = true
			if f.expr != nil {
				arc.AddConjunct(adt.MakeConjunct(o.env, f.expr))
			}
		}
	}
	if matched {
		return
	}

	// match others
	for _, x := range o.additional {
		arc.AddConjunct(adt.MakeConjunct(o.env, x))
	}
}

func (o *FieldSet) fieldIndex(f adt.Feature) int {
	for i := range o.fields {
		if o.fields[i].label == f {
			return i
		}
	}
	return -1
}

func (o *FieldSet) MarkField(c *adt.OpContext, x *adt.Field) {
	if o.fieldIndex(x.Label) < 0 {
		o.fields = append(o.fields, field{label: x.Label})
	}
}

func (o *FieldSet) AddOptional(c *adt.OpContext, x *adt.OptionalField) {
	p := o.fieldIndex(x.Label)
	if p < 0 {
		p = len(o.fields)
		o.fields = append(o.fields, field{label: x.Label})
	}
	o.fields[p].optional = append(o.fields[p].optional, x)
}

func (o *FieldSet) AddDynamic(c *adt.OpContext, env *adt.Environment, x *adt.DynamicField) {
	// not in bulk: count as regular field?
	o.bulk = append(o.bulk, bulkField{dynamicMatcher{env, x.Key}, nil})
}

func (o *FieldSet) AddBulk(c *adt.OpContext, x *adt.BulkOptionalField) {
	v, ok := c.Evaluate(o.env, x.Filter)
	if !ok {
		// TODO: handle dynamically
		return
	}
	switch f := v.(type) {
	case *adt.Num:
		// Just assert an error. Lists have not been expanded yet at
		// this point, so there is no need to check for existing
		//fields.
		l, err := adt.MakeLabel(x.Src, c.Int64(f), adt.IntLabel)
		if err != nil {
			c.AddErr(err)
			return
		}
		o.bulk = append(o.bulk, bulkField{labelMatcher(l), x})

	case *adt.Top:
		o.bulk = append(o.bulk, bulkField{typeMatcher(adt.TopKind), x})

	case *adt.BasicType:
		o.bulk = append(o.bulk, bulkField{typeMatcher(f.K), x})

	case *adt.String:
		l := c.Label(f)
		o.bulk = append(o.bulk, bulkField{labelMatcher(l), x})

	case adt.Validator:
		o.bulk = append(o.bulk, bulkField{validateMatcher{f}, x})

	default:
		// TODO(err): not allowed type
	}
}

func (o *FieldSet) AddEllipsis(c *adt.OpContext, x *adt.Ellipsis) {
	if x.Value == nil {
		o.isOpen = true
	} else {
		o.additional = append(o.additional, x.Value)
	}
}

type fieldMatcher interface {
	Match(c *adt.OpContext, f adt.Feature) bool
}

type labelMatcher adt.Feature

func (m labelMatcher) Match(c *adt.OpContext, f adt.Feature) bool {
	return adt.Feature(m) == f
}

type typeMatcher adt.Kind

func (m typeMatcher) Match(c *adt.OpContext, f adt.Feature) bool {
	switch f.Typ() {
	case adt.StringLabel:
		return adt.Kind(m)&adt.StringKind != 0

	case adt.IntLabel:
		return adt.Kind(m)&adt.IntKind != 0
	}
	return false
}

type validateMatcher struct {
	adt.Validator
}

func (m validateMatcher) Match(c *adt.OpContext, f adt.Feature) bool {
	v := f.ToValue(c)
	return c.Validate(m.Validator, v) == nil
}

type dynamicMatcher struct {
	env  *adt.Environment
	expr adt.Expr
}

func (m dynamicMatcher) Match(c *adt.OpContext, f adt.Feature) bool {
	if !f.IsRegular() || !f.IsString() {
		return false
	}
	v, ok := c.Evaluate(m.env, m.expr)
	if !ok {
		return false
	}
	s, ok := v.(*adt.String)
	if !ok {
		return false
	}
	return f.ToString(c) == s.Str
}
