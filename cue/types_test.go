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

package cue

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func getInstance(t *testing.T, body ...string) *Instance {
	t.Helper()

	insts := Build(makeInstances([]*bimport{{files: body}}))
	if insts[0].Err != nil {
		t.Fatalf("unexpected parse error: %v", insts[0].Err)
	}
	return insts[0]
}

func TestValueType(t *testing.T) {
	testCases := []struct {
		value          string
		kind           Kind
		incompleteKind Kind
		json           string
		valid          bool
		// pos            token.Pos
	}{{ // Not a concrete value.
		value:          `_`,
		kind:           BottomKind,
		incompleteKind: nextKind - 1,
	}, {
		value:          `_|_`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
	}, {
		value:          `1&2`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
	}, { // TODO: should be error{
		value:          `b`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
	}, {
		value:          `(b[a])`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
	}, { // TODO: should be error{
		value: `(b)
			b: bool`,
		kind:           BottomKind,
		incompleteKind: BoolKind,
	}, {
		value:          `([][b])`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
	}, {
		value:          `null`,
		kind:           NullKind,
		incompleteKind: NullKind,
	}, {
		value:          `true`,
		kind:           BoolKind,
		incompleteKind: BoolKind,
	}, {
		value:          `false`,
		kind:           BoolKind,
		incompleteKind: BoolKind,
	}, {
		value:          `bool`,
		kind:           BottomKind,
		incompleteKind: BoolKind,
	}, {
		value:          `2`,
		kind:           NumberKind,
		incompleteKind: NumberKind,
	}, {
		value:          `2.0`,
		kind:           NumberKind,
		incompleteKind: NumberKind,
	}, {
		value:          `2.0Mi`,
		kind:           NumberKind,
		incompleteKind: NumberKind,
	}, {
		value:          `14_000`,
		kind:           NumberKind,
		incompleteKind: NumberKind,
	}, {
		value:          `0..5`,
		kind:           BottomKind,
		incompleteKind: NumberKind,
	}, {
		value:          `float`,
		kind:           BottomKind,
		incompleteKind: NumberKind,
	}, {
		value:          `"str"`,
		kind:           StringKind,
		incompleteKind: StringKind,
	}, {
		value:          "'''\n'''",
		kind:           BytesKind,
		incompleteKind: BytesKind,
	}, {
		value:          "string",
		kind:           BottomKind,
		incompleteKind: StringKind,
	}, {
		value:          `{}`,
		kind:           StructKind,
		incompleteKind: StructKind,
	}, {
		value:          `[]`,
		kind:           ListKind,
		incompleteKind: ListKind,
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			inst := getInstance(t, tc.value)
			v := inst.Value()
			if got := v.Kind(); got != tc.kind {
				t.Errorf("Kind: got %x; want %x", got, tc.kind)
			}
			want := tc.incompleteKind | BottomKind
			if got := v.IncompleteKind(); got != want {
				t.Errorf("IncompleteKind: got %x; want %x", got, want)
			}
			incomplete := tc.incompleteKind != tc.kind
			if got := v.IsIncomplete(); got != incomplete {
				t.Errorf("IsIncomplete: got %v; want %v", got, incomplete)
			}
			invalid := tc.kind == BottomKind
			if got := v.IsValid(); got != !invalid {
				t.Errorf("IsValid: got %v; want %v", got, !invalid)
			}
			// if got, want := v.Pos(), tc.pos+1; got != want {
			// 	t.Errorf("pos: got %v; want %v", got, want)
			// }
		})
	}
}

func TestInt(t *testing.T) {
	testCases := []struct {
		value  string
		int    int64
		uint   uint64
		base   int
		err    string
		errU   string
		notInt bool
	}{{
		value: "1",
		int:   1,
		uint:  1,
	}, {
		value: "-1",
		int:   -1,
		uint:  0,
		errU:  ErrAbove.Error(),
	}, {
		value: "-111222333444555666777888999000",
		int:   math.MinInt64,
		uint:  0,
		err:   ErrAbove.Error(),
		errU:  ErrAbove.Error(),
	}, {
		value: "111222333444555666777888999000",
		int:   math.MaxInt64,
		uint:  math.MaxUint64,
		err:   ErrBelow.Error(),
		errU:  ErrBelow.Error(),
	}, {
		value:  "1.0",
		err:    "not of right kind (float vs int)",
		errU:   "not of right kind (float vs int)",
		notInt: true,
	}, {
		value:  "int",
		err:    "non-concrete value (int)*",
		errU:   "non-concrete value (int)*",
		notInt: true,
	}, {
		value:  "_|_",
		err:    "from source",
		errU:   "from source",
		notInt: true,
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			n := getInstance(t, tc.value).Value()
			base := 10
			if tc.base > 0 {
				base = tc.base
			}
			b, err := n.AppendInt(nil, base)
			if checkFailed(t, err, tc.err, "append") {
				want := tc.value
				if got := string(b); got != want {
					t.Errorf("append: got %v; want %v", got, want)
				}
			}

			if got := n.IsInt(); got != !tc.notInt {
				t.Errorf("isInt: got %v; want %v", got, !tc.notInt)
			}

			isUint := !tc.notInt && tc.int >= 0
			if got := n.IsUint(); got != isUint {
				t.Errorf("isUint: got %v; want %v", got, isUint)
			}

			vi, err := n.Int64()
			checkErr(t, err, tc.err, "Int64")
			if vi != tc.int {
				t.Errorf("Int64: got %v; want %v", vi, tc.int)
			}

			vu, err := n.Uint64()
			checkErr(t, err, tc.errU, "Uint64")
			if vu != uint64(tc.uint) {
				t.Errorf("Uint64: got %v; want %v", vu, tc.uint)
			}
		})
	}
}

func TestFloat(t *testing.T) {
	testCases := []struct {
		value   string
		float   string
		float64 float64
		mant    string
		exp     int
		fmt     byte
		prec    int
		err     string
	}{{
		value:   "1",
		float:   "1",
		mant:    "1",
		exp:     0,
		float64: 1,
		fmt:     'g',
	}, {
		value:   "-1",
		float:   "-1",
		mant:    "-1",
		exp:     0,
		float64: -1,
		fmt:     'g',
	}, {
		value:   "1.0",
		float:   "1.0",
		mant:    "10",
		exp:     -1,
		float64: 1.0,
		fmt:     'g',
	}, {
		value:   "2.6",
		float:   "2.6",
		mant:    "26",
		exp:     -1,
		float64: 2.6,
		fmt:     'g',
	}, {
		value:   "20.600",
		float:   "20.60",
		mant:    "20600",
		exp:     -3,
		float64: 20.60,
		prec:    2,
		fmt:     'f',
	}, {
		value:   "1/0",
		float:   "∞",
		float64: math.Inf(1),
		prec:    2,
		fmt:     'f',
		err:     ErrAbove.Error(),
	}, {
		value:   "-1/0",
		float:   "-∞",
		float64: math.Inf(-1),
		prec:    2,
		fmt:     'f',
		err:     ErrBelow.Error(),
	}, {
		value:   "1.797693134862315708145274237317043567982e+308",
		float:   "1.8e+308",
		mant:    "1797693134862315708145274237317043567982",
		exp:     269,
		float64: math.Inf(1),
		prec:    2,
		fmt:     'g',
		err:     ErrAbove.Error(),
	}, {
		value:   "-1.797693134862315708145274237317043567982e+308",
		float:   "-1.8e+308",
		mant:    "-1797693134862315708145274237317043567982",
		exp:     269,
		float64: math.Inf(-1),
		prec:    2,
		fmt:     'g',
		err:     ErrBelow.Error(),
	}, {
		value:   "4.940656458412465441765687928682213723650e-324",
		float:   "4.941e-324",
		mant:    "4940656458412465441765687928682213723650",
		exp:     -363,
		float64: 0,
		prec:    4,
		fmt:     'g',
		err:     ErrBelow.Error(),
	}, {
		value:   "-4.940656458412465441765687928682213723650e-324",
		float:   "-4.940656458412465441765687928682213723650e-324",
		mant:    "-4940656458412465441765687928682213723650",
		exp:     -363,
		float64: 0,
		prec:    -1,
		fmt:     'g',
		err:     ErrAbove.Error(),
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			n := getInstance(t, tc.value).Value()
			if n.Kind() != NumberKind {
				t.Fatal("Not a number")
			}

			var mant big.Int
			exp, err := n.MantExp(&mant)
			mstr := ""
			if err == nil {
				mstr = mant.String()
			}
			if exp != tc.exp || mstr != tc.mant {
				t.Errorf("mantExp: got %s %d; want %s %d", mstr, exp, tc.mant, tc.exp)
			}

			b, err := n.AppendFloat(nil, tc.fmt, tc.prec)
			want := tc.float
			if got := string(b); got != want {
				t.Errorf("append: got %v; want %v", got, want)
			}

			f, err := n.Float64()
			checkErr(t, err, tc.err, "Float64")
			if f != tc.float64 {
				t.Errorf("Float64: got %v; want %v", f, tc.float64)
			}
		})
	}
}

func TestString(t *testing.T) {
	testCases := []struct {
		value string
		str   string
		err   string
	}{{
		value: `""`,
		str:   ``,
	}, {
		value: `"Hello world!"`,
		str:   `Hello world!`,
	}, {
		value: `"Hello \(world)!"
		world: "world"`,
		str: `Hello world!`,
	}, {
		value: `string`,
		err:   "non-concrete value (string)*",
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			str, err := getInstance(t, tc.value).Value().String()
			checkFatal(t, err, tc.err, "init")
			if str != tc.str {
				t.Errorf("String: got %q; want %q", str, tc.str)
			}

			b, err := getInstance(t, tc.value).Value().Bytes()
			checkFatal(t, err, tc.err, "init")
			if got := string(b); got != tc.str {
				t.Errorf("Bytes: got %q; want %q", got, tc.str)
			}

			r, err := getInstance(t, tc.value).Value().Reader()
			checkFatal(t, err, tc.err, "init")
			b, _ = ioutil.ReadAll(r)
			if got := string(b); got != tc.str {
				t.Errorf("Reader: got %q; want %q", got, tc.str)
			}
		})
	}
}

func TestError(t *testing.T) {
	testCases := []struct {
		value string
		err   string
	}{{
		value: `_|_`,
		err:   "from source",
	}, {
		value: `"Hello world!"`,
	}, {
		value: `string`,
		err:   "non-concrete value (string)*",
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			err := getInstance(t, tc.value).Value().Err()
			checkErr(t, err, tc.err, "init")
		})
	}
}

func TestNull(t *testing.T) {
	testCases := []struct {
		value string
		err   string
	}{{
		value: `_|_`,
		err:   "from source",
	}, {
		value: `"str"`,
		err:   "not of right kind (string vs null)",
	}, {
		value: `null`,
	}, {
		value: `_`,
		err:   "non-concrete value (_)*",
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			err := getInstance(t, tc.value).Value().Null()
			checkErr(t, err, tc.err, "init")
		})
	}
}

func TestBool(t *testing.T) {
	testCases := []struct {
		value string
		bool  bool
		err   string
	}{{
		value: `_|_`,
		err:   "from source",
	}, {
		value: `"str"`,
		err:   "not of right kind (string vs bool)",
	}, {
		value: `true`,
		bool:  true,
	}, {
		value: `false`,
	}, {
		value: `bool`,
		err:   "non-concrete value (bool)*",
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			got, err := getInstance(t, tc.value).Value().Bool()
			if checkErr(t, err, tc.err, "init") {
				if got != tc.bool {
					t.Errorf("got %v; want %v", got, tc.bool)
				}
			}
		})
	}
}

func TestList(t *testing.T) {
	testCases := []struct {
		value string
		res   string
		err   string
	}{{
		value: `_|_`,
		err:   "from source",
	}, {
		value: `"str"`,
		err:   "not of right kind (string vs list)",
	}, {
		value: `[]`,
		res:   "[]",
	}, {
		value: `[1,2,3]`,
		res:   "[1,2,3,]",
	}, {
		value: `(0..5)*[1,2,3, ...int]`,
		res:   "[1,2,3,]",
	}, {
		value: `[x for x in y if x > 1]
		y: [1,2,3]`,
		res: "[2,3,]",
	}, {
		value: `[int]`,
		err:   "cannot convert incomplete value",
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			l, err := getInstance(t, tc.value).Value().List()
			checkFatal(t, err, tc.err, "init")

			buf := []byte{'['}
			for l.Next() {
				b, err := l.Value().MarshalJSON()
				checkFatal(t, err, tc.err, "list.Value")
				buf = append(buf, b...)
				buf = append(buf, ',')
			}
			buf = append(buf, ']')
			if got := string(buf); got != tc.res {
				t.Errorf("got %v; want %v", got, tc.res)
			}
		})
	}
}

func TestFields(t *testing.T) {
	testCases := []struct {
		value string
		res   string
		err   string
	}{{
		value: `_|_`,
		err:   "from source",
	}, {
		value: `"str"`,
		err:   "not of right kind (string vs struct)",
	}, {
		value: `{}`,
		res:   "{}",
	}, {
		value: `{a:1,b:2,c:3}`,
		res:   "{a:1,b:2,c:3,}",
	}, {
		value: `{a:1,"_b":2,c:3,_d:4}`,
		res:   "{a:1,_b:2,c:3,}",
	}, {
		value: `{_a:"a"}`,
		res:   "{}",
	}, {
		value: `{"\(k)": v for k, v in y if v > 1}
		y: {a:1,b:2,c:3}`,
		res: "{b:2,c:3,}",
	}, {
		value: `{a:1,b:2,c:int}`,
		err:   "cannot convert incomplete value",
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			obj := getInstance(t, tc.value).Value()

			iter, err := obj.Fields()
			checkFatal(t, err, tc.err, "init")

			buf := []byte{'{'}
			for iter.Next() {
				buf = append(buf, iter.Label()...)
				buf = append(buf, ':')
				b, err := iter.Value().MarshalJSON()
				checkFatal(t, err, tc.err, "Obj.At")
				buf = append(buf, b...)
				buf = append(buf, ',')
			}
			buf = append(buf, '}')
			if got := string(buf); got != tc.res {
				t.Errorf("got %v; want %v", got, tc.res)
			}

			iter, _ = obj.Fields()
			for iter.Next() {
				want, err := iter.Value().MarshalJSON()
				checkFatal(t, err, tc.err, "Obj.At2")

				got, err := obj.Lookup(iter.Label()).MarshalJSON()
				checkFatal(t, err, tc.err, "Obj.At2")

				if !bytes.Equal(got, want) {
					t.Errorf("Lookup: got %q; want %q", got, want)
				}
			}
			v := obj.Lookup("non-existing")
			checkErr(t, v.Err(), "not found", "non-existing")
		})
	}
}

func TestAllFields(t *testing.T) {
	testCases := []struct {
		value string
		res   string
		err   string
	}{{
		value: `{a:1,"_b":2,c:3,_d:4}`,
		res:   "{a:1,_b:2,c:3,_d:4,}",
	}, {
		value: `{_a:"a"}`,
		res:   `{_a:"a",}`,
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			obj := getInstance(t, tc.value).Value()

			iter, err := obj.AllFields()
			checkFatal(t, err, tc.err, "init")

			buf := []byte{'{'}
			for iter.Next() {
				buf = append(buf, iter.Label()...)
				buf = append(buf, ':')
				b, err := iter.Value().MarshalJSON()
				checkFatal(t, err, tc.err, "Obj.At")
				buf = append(buf, b...)
				buf = append(buf, ',')
			}
			buf = append(buf, '}')
			if got := string(buf); got != tc.res {
				t.Errorf("got %v; want %v", got, tc.res)
			}
		})
	}
}

func TestTemplate(t *testing.T) {
	testCases := []struct {
		value string
		path  []string
		want  string
	}{{
		value: `
		a <Name>: Name
		`,
		path: []string{"a", ""},
		want: `"label"`,
	}, {
		value: `
		<Name>: { a: Name }
		`,
		path: []string{"", "a"},
		want: `"label"`,
	}, {
		value: `
		<Name>: { a: Name }
		`,
		path: []string{""},
		want: `{"a":"label"}`,
	}, {
		value: `
		a <Foo> <Bar>: { b: Foo+Bar }
		`,
		path: []string{"a", "", ""},
		want: `{"b":"labellabel"}`,
	}, {
		value: `
		a <Foo> b <Bar>: { c: Foo+Bar }
		a foo b <Bar>: { d: Bar }
		`,
		path: []string{"a", "foo", "b", ""},
		want: `{"c":"foolabel","d":"label"}`,
	}}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			v := getInstance(t, tc.value).Value()
			for _, p := range tc.path {
				if p == "" {
					v = v.Template()("label")
				} else {
					v = v.Lookup(p)
				}
			}
			b, err := v.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if got := string(b); got != tc.want {
				t.Errorf("\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestSubsumes(t *testing.T) {
	a := []string{"a"}
	b := []string{"b"}
	testCases := []struct {
		value string
		pathA []string
		pathB []string
		want  bool
	}{{
		value: `4`,
		want:  true,
	}, {
		value: `a: string, b: "foo"`,
		pathA: a,
		pathB: b,
		want:  true,
	}, {
		value: `a: string, b: "foo"`,
		pathA: b,
		pathB: a,
		want:  false,
	}, {
		value: `a: {a: string, b: 4}, b: {a: "foo", b: 4}`,
		pathA: a,
		pathB: b,
		want:  true,
	}, {
		value: `a: [string,  4], b: ["foo", 4]`,
		pathA: a,
		pathB: b,
		want:  true,
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			v := getInstance(t, tc.value)
			a := v.Lookup(tc.pathA...)
			b := v.Lookup(tc.pathB...)
			got := a.Subsumes(b)
			if got != tc.want {
				t.Errorf("got %v (%v); want %v (%v)", got, a, tc.want, b)
			}
		})
	}
}

func TestUnify(t *testing.T) {
	a := []string{"a"}
	b := []string{"b"}
	testCases := []struct {
		value string
		pathA []string
		pathB []string
		want  string
	}{{
		value: `4`,
		want:  `4`,
	}, {
		value: `a: string, b: "foo"`,
		pathA: a,
		pathB: b,
		want:  `"foo"`,
	}, {
		value: `a: string, b: "foo"`,
		pathA: b,
		pathB: a,
		want:  `"foo"`,
	}, {
		value: `a: {a: string, b: 4}, b: {a: "foo", b: 4}`,
		pathA: a,
		pathB: b,
		want:  `{"a":"foo","b":4}`,
	}, {
		value: `a: [string,  4], b: ["foo", 4]`,
		pathA: a,
		pathB: b,
		want:  `["foo",4]`,
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			v := getInstance(t, tc.value).Value()
			x := v.Lookup(tc.pathA...)
			y := v.Lookup(tc.pathB...)
			b, err := x.Unify(y).MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if got := string(b); got != tc.want {
				t.Errorf("got %v; want %v", got, tc.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	type fields struct {
		A int `json:"A"`
		B int `json:"B"`
		C int `json:"C"`
	}
	intList := func(ints ...int) *[]int {
		ints = append([]int{}, ints...)
		return &ints
	}
	testCases := []struct {
		value string
		dst   interface{}
		want  interface{}
		err   string
	}{{
		value: `_|_`,
		err:   "from source",
	}, {
		value: `"str"`,
		dst:   "",
		want:  "str",
	}, {
		value: `"str"`,
		dst:   new(int),
		err:   "cannot unmarshal string into Go value of type int",
	}, {
		value: `{}`,
		dst:   &fields{},
		want:  &fields{},
	}, {
		value: `{a:1,b:2,c:3}`,
		dst:   &fields{},
		want:  &fields{A: 1, B: 2, C: 3},
	}, {
		value: `{"\(k)": v for k, v in y if v > 1}
		y: {a:1,b:2,c:3}`,
		dst:  &fields{},
		want: &fields{B: 2, C: 3},
	}, {
		value: `{a:1,b:2,c:int}`,
		dst:   new(fields),
		err:   "cannot convert incomplete value",
	}, {
		value: `[]`,
		dst:   intList(),
		want:  intList(),
	}, {
		value: `[1,2,3]`,
		dst:   intList(),
		want:  intList(1, 2, 3),
	}, {
		value: `[x for x in y if x > 1]
				y: [1,2,3]`,
		dst:  intList(),
		want: intList(2, 3),
	}, {
		value: `[int]`,
		err:   "cannot convert incomplete value",
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			err := getInstance(t, tc.value).Value().Decode(&tc.dst)
			checkFatal(t, err, tc.err, "init")

			if !cmp.Equal(tc.dst, tc.want) {
				t.Error(cmp.Diff(tc.dst, tc.want))
				t.Errorf("\n%#v\n%#v", tc.dst, tc.want)
			}
		})
	}
}

func TestValueLookup(t *testing.T) {
	config := `
		a: {
			a: 0
			b: 1
			c: 2
		}
		b: {
			d: a.a
			e: int
		}
	`

	strList := func(s ...string) []string { return s }

	testCases := []struct {
		config    string
		path      []string
		str       string
		notExists bool
	}{{
		config: "_|_",
		path:   strList(""),
		str:    "from source",
	}, {
		config: "_|_",
		path:   strList("a"),
		str:    "from source",
	}, {
		config: config,
		path:   strList(),
		str:    "<0>{a: <1>{a: 0, b: 1, c: 2}, b: <2>{d: <0>.a.a, e: int}",
	}, {
		config: config,
		path:   strList("a", "a"),
		str:    "0",
	}, {
		config: config,
		path:   strList("a"),
		str:    "<0>{a: 0, b: 1, c: 2}",
	}, {
		config: config,
		path:   strList("b", "d"),
		str:    "0",
	}, {
		config:    config,
		path:      strList("c", "non-existing"),
		str:       "not found",
		notExists: true,
	}, {
		config: config,
		path:   strList("b", "d", "lookup in non-struct"),
		str:    "not of right kind (int vs struct)",
	}}
	for _, tc := range testCases {
		t.Run(tc.str, func(t *testing.T) {
			v := getInstance(t, tc.config).Value().Lookup(tc.path...)
			if got := !v.Exists(); got != tc.notExists {
				t.Errorf("exists: got %v; want %v", got, tc.notExists)
			}

			got := fmt.Sprint(v)
			if tc.str == "" {
				t.Fatalf("str empty, got %q", got)
			}
			if !strings.Contains(got, tc.str) {
				t.Errorf("\n got %v\nwant %v", got, tc.str)
			}
		})
	}
}

func TestMashalJSON(t *testing.T) {
	testCases := []struct {
		value string
		json  string
		err   string
	}{{
		value: `""`,
		json:  `""`,
	}, {
		value: `null`,
		json:  `null`,
	}, {
		value: `_|_`,
		err:   "from source",
	}, {
		value: `(a.b)
		a: {}`,
		err: "undefined field",
	}, {
		value: `true`,
		json:  `true`,
	}, {
		value: `false`,
		json:  `false`,
	}, {
		value: `bool`,
		json:  `bool`,
		err:   "cannot convert incomplete value",
	}, {
		value: `"str"`,
		json:  `"str"`,
	}, {
		value: `12_000`,
		json:  `12000`,
	}, {
		value: `12.000`,
		json:  `12.000`,
	}, {
		value: `12M`,
		json:  `12000000`,
	}, {
		value: `3.0e100`,
		json:  `3.0E+100`,
	}, {
		value: `[]`,
		json:  `[]`,
	}, {
		value: `[1, 2, 3]`,
		json:  `[1,2,3]`,
	}, {
		value: `[int]`,
		err:   `cannot convert incomplete value`,
	}, {
		value: `((0..3) * [1, 2])`,
		json:  `[1,2]`,
	}, {
		value: `{}`,
		json:  `{}`,
	}, {
		value: `{a: 2, b: 3, c: ["A", "B"]}`,
		json:  `{"a":2,"b":3,"c":["A","B"]}`,
	}}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d/%v", i, tc.value), func(t *testing.T) {
			inst := getInstance(t, tc.value)
			b, err := inst.Value().MarshalJSON()
			checkFatal(t, err, tc.err, "init")

			if got := string(b); got != tc.json {
				t.Errorf("\n got %v;\nwant %v", got, tc.json)
			}
		})
	}
}

func TestWalk(t *testing.T) {
	testCases := []struct {
		value string
		out   string
	}{{
		value: `""`,
		out:   `""`,
	}, {
		value: `null`,
		out:   `null`,
	}, {
		value: `_|_`,
		out:   "_|_(from source)",
	}, {
		value: `(a.b)
		a: {}`,
		out: `_|_(<0>.a.b:undefined field "b")`,
	}, {
		value: `true`,
		out:   `true`,
	}, {
		value: `false`,
		out:   `false`,
	}, {
		value: `bool`,
		out:   "bool",
	}, {
		value: `"str"`,
		out:   `"str"`,
	}, {
		value: `12_000`,
		out:   `12000`,
	}, {
		value: `12.000`,
		out:   `12.000`,
	}, {
		value: `12M`,
		out:   `12000000`,
	}, {
		value: `3.0e100`,
		out:   `3.0e+100`,
	}, {
		value: `[]`,
		out:   `[]`,
	}, {
		value: `[1, 2, 3]`,
		out:   `[1,2,3]`,
	}, {
		value: `[int]`,
		out:   `[int]`,
	}, {
		value: `((0..3) * [1, 2])`,
		out:   `[1,2]`,
	}, {
		value: `{}`,
		out:   `{}`,
	}, {
		value: `{a: 2, b: 3, c: ["A", "B"]}`,
		out:   `{a:2,b:3,c:["A","B"]}`,
	}}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d/%v", i, tc.value), func(t *testing.T) {
			inst := getInstance(t, tc.value)
			buf := []byte{}
			stripComma := func() {
				if n := len(buf) - 1; buf[n] == ',' {
					buf = buf[:n]
				}
			}
			inst.Value().Walk(func(v Value) bool {
				if k, ok := v.Label(); ok {
					buf = append(buf, k+":"...)
				}
				switch v.Kind() {
				case StructKind:
					buf = append(buf, '{')
				case ListKind:
					buf = append(buf, '[')
				default:
					buf = append(buf, fmt.Sprint(v, ",")...)
				}
				return true
			}, func(v Value) {
				switch v.Kind() {
				case StructKind:
					stripComma()
					buf = append(buf, "},"...)
				case ListKind:
					stripComma()
					buf = append(buf, "],"...)
				}
			})
			stripComma()
			if got := string(buf); got != tc.out {
				t.Errorf("\n got %v;\nwant %v", got, tc.out)
			}
		})
	}
}

func TestTrimZeros(t *testing.T) {
	testCases := []struct {
		in  string
		out string
	}{
		{"", ""},
		{"2", "2"},
		{"2.0", "2.0"},
		{"2.000000000000", "2.0"},
		{"2000000000000", "2e+12"},
		{"2000000", "2e+6"},
	}
	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			if got := trimZeros(tc.in); got != tc.out {
				t.Errorf("got %q; want %q", got, tc.out)
			}
		})
	}
}

func TestReferences(t *testing.T) {
	config1 := `
	a: {
		b: 3
	}
	c: {
		d: a.b
		e: c.d
		f: a
	}
	`
	config2 := `
	a: { c: 3 }
	b: { c: int, d: 4 }
	r: (a & b).c
	`
	testCases := []struct {
		config string
		in     string
		out    string
	}{
		{config1, "c.d", "a.b"},
		{config1, "c.e", "c.d"},
		{config1, "c.f", "a"},

		{config2, "r", "a.c b.c"},
	}
	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			ctx, st := compileFile(t, tc.config)
			v := newValueRoot(ctx, st)
			for _, k := range strings.Split(tc.in, ".") {
				obj, err := v.structVal(ctx)
				if err != nil {
					t.Fatal(err)
				}
				v = obj.Lookup(k)
			}
			got := []string{}
			for _, r := range v.References() {
				got = append(got, strings.Join(r, "."))
			}
			want := strings.Split(tc.out, " ")
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %v; want %v", got, want)
			}
		})
	}
}

func checkErr(t *testing.T, err error, str, name string) bool {
	t.Helper()
	if err == nil {
		if str != "" {
			t.Errorf(`err:%s: got ""; want %q`, name, str)
		}
		return true
	}
	return checkFailed(t, err, str, name)
}

func checkFatal(t *testing.T, err error, str, name string) {
	t.Helper()
	if !checkFailed(t, err, str, name) {
		t.SkipNow()
	}
}

func checkFailed(t *testing.T, err error, str, name string) bool {
	t.Helper()
	if err != nil {
		got := err.Error()
		if str == "" {
			t.Fatalf(`err:%s: got %q; want ""`, name, got)
		}
		if !strings.Contains(got, str) {
			t.Errorf(`err:%s: got %q; want %q`, name, got, str)
		}
		return false
	}
	return true
}
