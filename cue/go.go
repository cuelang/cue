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

package cue

import (
	"encoding"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal"
	"github.com/cockroachdb/apd/v2"
)

// This file contains functionality for converting Go to CUE.
//
// The code in this file is a prototype implementation and is far from
// optimized.

func init() {
	internal.FromGoValue = func(runtime, x interface{}, allowDefault bool) interface{} {
		return convertValue(runtime.(*Runtime), x, allowDefault)
	}

	internal.FromGoType = func(runtime, x interface{}) interface{} {
		return convertType(runtime.(*Runtime), x)
	}
}

func convertValue(r *Runtime, x interface{}, allowDefault bool) Value {
	ctx := r.index().newContext()
	v := convert(ctx, baseValue{}, allowDefault, x)
	return newValueRoot(ctx, v)
}

func convertType(r *Runtime, x interface{}) Value {
	ctx := r.index().newContext()
	v := convertGoType(r, reflect.TypeOf(x))
	return newValueRoot(ctx, v)

}

// parseTag parses a CUE expression from a cue tag.
func parseTag(ctx *context, obj *structLit, field label, tag string) value {
	if p := strings.Index(tag, ","); p >= 0 {
		tag = tag[:p]
	}
	if tag == "" {
		return &top{}
	}
	expr, err := parser.ParseExpr("<field:>", tag)
	if err != nil {
		field := ctx.labelStr(field)
		return ctx.mkErr(baseValue{}, "invalid tag %q for field %q: %v", tag, field, err)
	}
	v := newVisitor(ctx.index, nil, nil, obj)
	return v.walk(expr)
}

// TODO: should we allow mapping names in cue tags? This only seems like a good
// idea if we ever want to allow mapping CUE to a different name than JSON.
var tagsWithNames = []string{"json", "yaml", "protobuf"}

func getName(f *reflect.StructField) string {
	name := f.Name
	for _, s := range tagsWithNames {
		if tag, ok := f.Tag.Lookup(s); ok {
			if p := strings.Index(tag, ","); p >= 0 {
				tag = tag[:p]
			}
			if tag != "" {
				name = tag
				break
			}
		}
	}
	return name
}

// isOptional indicates whether a field should be marked as optional.
func isOptional(f *reflect.StructField) bool {
	isOptional := false
	switch f.Type.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Interface, reflect.Slice:
		// Note: it may be confusing to distinguish between an empty slice and
		// a nil slice. However, it is also surprizing to not be able to specify
		// a default value for a slice. So for now we will allow it.
		isOptional = true
	}
	if tag, ok := f.Tag.Lookup("cue"); ok {
		// TODO: only if first field is not empty.
		isOptional = false
		for _, f := range strings.Split(tag, ",")[1:] {
			switch f {
			case "opt":
				isOptional = true
			case "req":
				return false
			}
		}
	} else if tag, ok = f.Tag.Lookup("json"); ok {
		isOptional = false
		for _, f := range strings.Split(tag, ",")[1:] {
			if f == "omitempty" {
				return true
			}
		}
	}
	return isOptional
}

// isOmitEmpty means that the zero value is interpreted as undefined.
func isOmitEmpty(f *reflect.StructField) bool {
	isOmitEmpty := false
	switch f.Type.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Chan, reflect.Interface, reflect.Slice:
		// Note: it may be confusing to distinguish between an empty slice and
		// a nil slice. However, it is also surprizing to not be able to specify
		// a default value for a slice. So for now we will allow it.
		isOmitEmpty = true

	default:
		// TODO: we can also infer omit empty if a type cannot be nil if there
		// is a constraint that unconditionally disallows the zero value.
	}
	tag, ok := f.Tag.Lookup("json")
	if ok {
		isOmitEmpty = false
		for _, f := range strings.Split(tag, ",")[1:] {
			if f == "omitempty" {
				return true
			}
		}
	}
	return isOmitEmpty
}

// parseJSON parses JSON into a CUE value. b must be valid JSON.
func parseJSON(ctx *context, b []byte) evaluated {
	expr, err := parser.ParseExpr("json", b)
	if err != nil {
		panic(err) // cannot happen
	}
	v := newVisitor(ctx.index, nil, nil, nil)
	return v.walk(expr).evalPartial(ctx)
}

func isZero(v reflect.Value) bool {
	x := v.Interface()
	if x == nil {
		return true
	}
	switch k := v.Kind(); k {
	case reflect.Struct, reflect.Array:
		// we never allow optional values for these types.
		return false

	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Slice:
		// Note that for maps we preserve the distinction between a nil map and
		// an empty map.
		return v.IsNil()

	case reflect.String:
		return v.Len() == 0

	default:
		return x == reflect.Zero(v.Type()).Interface()
	}
}

func convert(ctx *context, src source, allowDefault bool, x interface{}) evaluated {
	v := convertRec(ctx, src, allowDefault, x)
	if v == nil {
		return ctx.mkErr(baseValue{}, "unsupported Go type (%v)", v)
	}
	return v
}

func convertRec(ctx *context, src source, allowDefault bool, x interface{}) evaluated {
	switch v := x.(type) {
	case nil:
		if !allowDefault {
			return &nullLit{src.base()}
		}
		// Interpret a nil pointer as an undefined value that is only
		// null by default, but may still be set: *null | _.
		return makeNullable(&top{src.base()}, false).(evaluated)

	case ast.Expr:
		x := newVisitorCtx(ctx, nil, nil, nil)
		return ctx.manifest(x.walk(v))

	case *big.Int:
		n := newNum(src, intKind)
		n.v.Coeff.Set(v)
		if v.Sign() < 0 {
			n.v.Coeff.Neg(&n.v.Coeff)
			n.v.Negative = true
		}
		return n

	case *big.Rat:
		// should we represent this as a binary operation?
		n := newNum(src, numKind)
		_, err := ctx.Quo(&n.v, apd.NewWithBigInt(v.Num(), 0), apd.NewWithBigInt(v.Denom(), 0))
		if err != nil {
			return ctx.mkErr(src, err)
		}
		if !v.IsInt() {
			n.k = floatKind
		}
		return n

	case *big.Float:
		n := newNum(src, floatKind)
		_, _, _ = n.v.SetString(v.String())
		return n

	case *apd.Decimal:
		n := newNum(src, floatKind|intKind)
		n.v.Set(v)
		if !n.isInt(ctx) {
			n.k = floatKind
		}
		return n

	case json.Marshaler:
		b, err := v.MarshalJSON()
		if err != nil {
			return ctx.mkErr(src, err)
		}

		return parseJSON(ctx, b)

	case encoding.TextMarshaler:
		b, err := v.MarshalText()
		if err != nil {
			return ctx.mkErr(src, err)
		}
		b, err = json.Marshal(string(b))
		if err != nil {
			return ctx.mkErr(src, err)
		}
		return parseJSON(ctx, b)

	case error:
		return ctx.mkErr(src, v.Error())
	case bool:
		return &boolLit{src.base(), v}
	case string:
		return &stringLit{src.base(), v, nil}
	case []byte:
		return &bytesLit{src.base(), v, nil}
	case int:
		return toInt(ctx, src, int64(v))
	case int8:
		return toInt(ctx, src, int64(v))
	case int16:
		return toInt(ctx, src, int64(v))
	case int32:
		return toInt(ctx, src, int64(v))
	case int64:
		return toInt(ctx, src, int64(v))
	case uint:
		return toUint(ctx, src, uint64(v))
	case uint8:
		return toUint(ctx, src, uint64(v))
	case uint16:
		return toUint(ctx, src, uint64(v))
	case uint32:
		return toUint(ctx, src, uint64(v))
	case uint64:
		return toUint(ctx, src, uint64(v))
	case uintptr:
		return toUint(ctx, src, uint64(v))
	case float64:
		r := newNum(src, floatKind)
		_, _, _ = r.v.SetString(fmt.Sprintf("%g", v))
		return r
	case float32:
		r := newNum(src, floatKind)
		_, _, _ = r.v.SetString(fmt.Sprintf("%g", v))
		return r

	case reflect.Value:
		if v.CanInterface() {
			return convertRec(ctx, src, allowDefault, v.Interface())
		}

	default:
		value := reflect.ValueOf(v)
		switch value.Kind() {
		case reflect.Bool:
			return &boolLit{src.base(), value.Bool()}

		case reflect.String:
			return &stringLit{src.base(), value.String(), nil}

		case reflect.Int, reflect.Int8, reflect.Int16,
			reflect.Int32, reflect.Int64:
			return toInt(ctx, src, value.Int())

		case reflect.Uint, reflect.Uint8, reflect.Uint16,
			reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return toUint(ctx, src, value.Uint())

		case reflect.Float32, reflect.Float64:
			return convertRec(ctx, src, allowDefault, value.Float())

		case reflect.Ptr:
			if value.IsNil() {
				if !allowDefault {
					return &nullLit{src.base()}
				}
				// Interpret a nil pointer as an undefined value that is only
				// null by default, but may still be set: *null | _.
				elem := goTypeToValue(ctx, false, reflect.TypeOf(v).Elem())
				return makeNullable(elem, false).(evaluated)
			}
			return convertRec(ctx, src, allowDefault, value.Elem().Interface())

		case reflect.Struct:
			obj := newStruct(src)
			t := value.Type()
			for i := 0; i < value.NumField(); i++ {
				t := t.Field(i)
				if t.PkgPath != "" {
					continue
				}
				val := value.Field(i)
				if isOmitEmpty(&t) && isZero(val) {
					continue
				}
				sub := convertRec(ctx, src, allowDefault, val.Interface())
				if sub == nil {
					// mimic behavior of encoding/json: skip fields of unsupported types
					continue
				}
				if isBottom(sub) {
					return sub
				}

				// leave errors like we do during normal evaluation or do we
				// want to return the error?
				name := getName(&t)
				if name == "-" {
					continue
				}
				f := ctx.strLabel(name)
				obj.arcs = append(obj.arcs, arc{feature: f, v: sub})
			}
			sort.Sort(obj)
			return obj

		case reflect.Map:
			obj := newStruct(src)

			sorted := []string{}
			keys := []string{}
			t := value.Type()
			switch key := t.Key(); key.Kind() {
			case reflect.String,
				reflect.Int, reflect.Int8, reflect.Int16,
				reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16,
				reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				for _, k := range value.MapKeys() {
					s := fmt.Sprint(k)
					keys = append(keys, s)
					sorted = append(sorted, s)

					val := value.MapIndex(k).Interface()
					sub := convertRec(ctx, src, allowDefault, val)
					// mimic behavior of encoding/json: report error of
					// unsupported type.
					if sub == nil {
						return ctx.mkErr(baseValue{}, "unsupported Go type (%v)", val)
					}
					if isBottom(sub) {
						return sub
					}

					// Set feature later.
					obj.arcs = append(obj.arcs, arc{feature: 0, v: sub})
				}

			default:
				return ctx.mkErr(baseValue{}, "unsupported Go type for map key (%v)", key)
			}

			// Assign label in normalized order.
			sort.Strings(sorted)
			for _, k := range sorted {
				ctx.strLabel(k)
			}

			// Now assign the labels to the arcs.
			for i, k := range keys {
				obj.arcs[i].feature = ctx.strLabel(k)
			}
			sort.Sort(obj)
			return obj

		case reflect.Slice, reflect.Array:
			list := &list{baseValue: src.base()}
			arcs := []arc{}
			for i := 0; i < value.Len(); i++ {
				val := value.Index(i).Interface()
				x := convertRec(ctx, src, allowDefault, val)
				if x == nil {
					return ctx.mkErr(baseValue{}, "unsupported Go type (%v)", val)
				}
				if isBottom(x) {
					return x
				}
				arcs = append(arcs, arc{feature: label(len(arcs)), v: x})
			}
			list.elem = &structLit{baseValue: list.baseValue, arcs: arcs}
			list.initLit()
			// There is no need to set the type of the list, as the list will
			// be of fixed size and all elements will already have a defined
			// value.
			return list
		}
	}
	return nil
}

func toInt(ctx *context, src source, x int64) evaluated {
	n := newNum(src, intKind)
	n.v.SetInt64(x)
	return n
}

func toUint(ctx *context, src source, x uint64) evaluated {
	n := newNum(src, floatKind)
	n.v.Coeff.SetUint64(x)
	return n
}

func convertGoType(r *Runtime, t reflect.Type) value {
	ctx := r.index().newContext()
	// TODO: this can be much more efficient.
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	return goTypeToValue(ctx, true, t)
}

var (
	jsonMarshaler = reflect.TypeOf(new(json.Marshaler)).Elem()
	textMarshaler = reflect.TypeOf(new(encoding.TextMarshaler)).Elem()
	topSentinel   = &top{}
)

// goTypeToValue converts a Go Type to a value.
//
// TODO: if this value will always be unified with a concrete type in Go, then
// many of the fields may be omitted.
func goTypeToValue(ctx *context, allowNullDefault bool, t reflect.Type) value {
	v := goTypeToValueRec(ctx, allowNullDefault, t)
	if v == nil {
		return ctx.mkErr(baseValue{}, "unsupported Go type (%v)", t)
	}
	return v
}

func goTypeToValueRec(ctx *context, allowNullDefault bool, t reflect.Type) (e value) {
	if e, ok := ctx.typeCache.Load(t); ok {
		return e.(value)
	}

	switch reflect.Zero(t).Interface().(type) {
	case *big.Int, big.Int:
		e = &basicType{k: intKind}
		goto store

	case *big.Float, big.Float, *big.Rat, big.Rat:
		e = &basicType{k: numKind}
		goto store

	case *apd.Decimal, apd.Decimal:
		e = &basicType{k: numKind}
		goto store
	}

	// Even if this is for types that we know cast to a certain type, it can't
	// hurt to return top, as in these cases the concrete values will be
	// strict instances and there cannot be any tags that further constrain
	// the values.
	if t.Implements(jsonMarshaler) || t.Implements(textMarshaler) {
		return topSentinel
	}

	switch k := t.Kind(); k {
	case reflect.Ptr:
		elem := t.Elem()
		for elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		e = goTypeToValueRec(ctx, false, elem)
		if allowNullDefault {
			e = wrapOrNull(e)
		}

	case reflect.Interface:
		switch t.Name() {
		case "error":
			// This is really null | _|_. There is no error if the error is null.
			e = &nullLit{} // null
		default:
			e = topSentinel // `_`
		}

	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		e = predefinedRanges[t.Kind().String()]

	case reflect.Uint, reflect.Uintptr:
		e = predefinedRanges["uint64"]

	case reflect.Int:
		e = predefinedRanges["int64"]

	case reflect.String:
		e = &basicType{k: stringKind}

	case reflect.Bool:
		e = &basicType{k: boolKind}

	case reflect.Float32, reflect.Float64:
		e = &basicType{k: floatKind}

	case reflect.Struct:
		// First iterate to create struct, then iterate another time to
		// resolve field tags to allow field tags to refer to the struct fields.
		tags := map[label]string{}
		obj := newStruct(baseValue{})
		ctx.typeCache.Store(t, obj)

		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			_, ok := f.Tag.Lookup("cue")
			elem := goTypeToValueRec(ctx, !ok, f.Type)
			if elem == nil || isBottom(elem) {
				continue // Ignore fields for unsupported types
			}

			// leave errors like we do during normal evaluation or do we
			// want to return the error?
			name := getName(&f)
			if name == "-" {
				continue
			}
			l := ctx.strLabel(name)
			obj.arcs = append(obj.arcs, arc{
				feature: l,
				// The GO JSON decoder always allows a value to be undefined.
				optional: isOptional(&f),
				v:        elem,
			})

			if tag, ok := f.Tag.Lookup("cue"); ok {
				tags[l] = tag
			}
		}
		sort.Sort(obj)

		for label, tag := range tags {
			v := parseTag(ctx, obj, label, tag)
			if isBottom(v) {
				return v
			}
			for i, a := range obj.arcs {
				if a.feature == label {
					// Instead of unifying with the existing type, we substitute
					// with the constraints from the tags. The type constraints
					// will be implied when unified with a concrete value.
					obj.arcs[i].v = mkBin(ctx, token.NoPos, opUnify, a.v, v)
				}
			}
		}

		return obj

	case reflect.Array, reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			e = &basicType{k: bytesKind}
		} else {
			elem := goTypeToValueRec(ctx, allowNullDefault, t.Elem())
			if elem == nil {
				return ctx.mkErr(baseValue{}, "unsupported Go type (%v)", t.Elem())
			}

			var ln value = &top{}
			if t.Kind() == reflect.Array {
				ln = toInt(ctx, baseValue{}, int64(t.Len()))
			}
			e = &list{elem: &structLit{}, typ: elem, len: ln}
		}
		if k == reflect.Slice {
			e = wrapOrNull(e)
		}

	case reflect.Map:
		switch key := t.Key(); key.Kind() {
		case reflect.String, reflect.Int, reflect.Int8, reflect.Int16,
			reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8,
			reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		default:
			return ctx.mkErr(baseValue{}, "unsupported Go type for map key (%v)", key)
		}

		obj := newStruct(baseValue{})
		sig := &params{}
		sig.add(ctx.label("_", true), &basicType{k: stringKind})
		v := goTypeToValueRec(ctx, allowNullDefault, t.Elem())
		if v == nil {
			return ctx.mkErr(baseValue{}, "unsupported Go type (%v)", t.Elem())
		}
		if isBottom(v) {
			return v
		}
		obj.template = &lambdaExpr{params: sig, value: v}

		e = wrapOrNull(obj)
	}

store:
	// TODO: store error if not nil?
	if e != nil {
		ctx.typeCache.Store(t, e)
	}
	return e
}

func wrapOrNull(e value) value {
	if e == nil || isBottom(e) || e.kind().isAnyOf(nullKind) {
		return e
	}
	return makeNullable(e, true)
}

func makeNullable(e value, nullIsDefault bool) value {
	return &disjunction{
		baseValue: baseValue{e},
		values: []dValue{
			{val: &nullLit{}, marked: nullIsDefault},
			{val: e}},
		errors:      nil,
		hasDefaults: nullIsDefault,
	}
}
