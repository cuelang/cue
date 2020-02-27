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

package cue

import (
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
)

// MarshalJSON marshalls this value into valid JSON.
func (v Value) MarshalJSON() (b []byte, err error) {
	b, err = v.marshalJSON()
	if err != nil {
		return nil, unwrapJSONError(err)
	}
	return b, nil
}

func (v Value) marshalJSON() (b []byte, err error) {
	v, _ = v.Default()
	if v.path == nil {
		return json.Marshal(nil)
	}
	ctx := v.idx.newContext()
	x := v.eval(ctx)
	// TODO: implement marshalles in value.
	switch k := x.kind(); k {
	case nullKind:
		return json.Marshal(nil)
	case boolKind:
		return json.Marshal(x.(*boolLit).b)
	case intKind, floatKind, numKind:
		return x.(*numLit).v.MarshalText()
	case stringKind:
		return json.Marshal(x.(*stringLit).str)
	case bytesKind:
		return json.Marshal(x.(*bytesLit).b)
	case listKind:
		l := x.(*list)
		i := Iterator{ctx: ctx, val: v, iter: l, len: len(l.elem.arcs)}
		return marshalList(&i)
	case structKind:
		obj, _ := v.structValData(ctx)
		return obj.marshalJSON()
	case bottomKind:
		return nil, toMarshalErr(v, x.(*bottom))
	default:
		if k.hasReferences() {
			return nil, marshalErrf(v, x, codeIncomplete, "value %q contains unresolved references", ctx.str(x))
		}
		if !k.isGround() {
			return nil, marshalErrf(v, x, codeIncomplete, "cannot convert incomplete value %q to JSON", ctx.str(x))
		}
		return nil, marshalErrf(v, x, 0, "cannot convert value %q of type %T to JSON", ctx.str(x), x)
	}
}

// MarshalJSON returns a valid JSON encoding or reports an error if any of the
// fields is invalid.
func (o *structValue) marshalJSON() (b []byte, err errors.Error) {
	b = append(b, '{')
	n := o.Len()
	for i := 0; i < n; i++ {
		k, v := o.At(i)
		s, err := json.Marshal(k)
		if err != nil {
			return nil, unwrapJSONError(err)
		}
		b = append(b, s...)
		b = append(b, ':')
		bb, err := json.Marshal(v)
		if err != nil {
			return nil, unwrapJSONError(err)
		}
		b = append(b, bb...)
		if i < n-1 {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return b, nil
}

// marshalJSON iterates over the list and generates JSON output. HasNext
// will return false after this operation.
func marshalList(l *Iterator) (b []byte, err errors.Error) {
	b = append(b, '[')
	if l.Next() {
		for i := 0; ; i++ {
			x, err := json.Marshal(l.Value())
			if err != nil {
				return nil, unwrapJSONError(err)
			}
			b = append(b, x...)
			if !l.Next() {
				break
			}
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return b, nil
}

var _ errors.Error = &marshalError{}

type marshalError struct {
	err errors.Error
	b   *bottom
}

func toMarshalErr(v Value, b *bottom) error {
	return &marshalError{v.toErr(b), b}
}

func marshalErrf(v Value, src source, code errCode, msg string, args ...interface{}) error {
	arguments := append([]interface{}{code, msg}, args...)
	b := v.idx.mkErr(src, arguments...)
	return toMarshalErr(v, b)
}

func (e *marshalError) Error() string {
	return fmt.Sprintf("cue: marshal error: %v", e.err)
}

func (e *marshalError) Path() []string               { return e.err.Path() }
func (e *marshalError) Msg() (string, []interface{}) { return e.err.Msg() }
func (e *marshalError) Position() token.Pos          { return e.err.Position() }
func (e *marshalError) InputPositions() []token.Pos {
	return e.err.InputPositions()
}

func unwrapJSONError(err error) errors.Error {
	switch x := err.(type) {
	case *json.MarshalerError:
		return unwrapJSONError(x.Err)
	case *marshalError:
		return x
	case errors.Error:
		return &marshalError{x, nil}
	default:
		return &marshalError{errors.Wrapf(err, token.NoPos, "json error"), nil}
	}
}
