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
	"strconv"
	"strings"
	"testing"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
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
		concrete       bool
		// pos            token.Pos
	}{{ // Not a concrete value.
		value:          `_`,
		kind:           BottomKind,
		incompleteKind: nextKind - 1,
	}, {
		value:          `_|_`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
		concrete:       true,
	}, {
		value:          `1&2`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
		concrete:       true,
	}, { // TODO: should be error{
		value:          `b`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
		concrete:       true,
	}, {
		value:          `(b[a])`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
		concrete:       true,
	}, { // TODO: should be error{
		value: `(b)
			b: bool`,
		kind:           BottomKind,
		incompleteKind: BoolKind,
	}, {
		value:          `([][b])`,
		kind:           BottomKind,
		incompleteKind: BottomKind,
		concrete:       true,
	}, {
		value:          `null`,
		kind:           NullKind,
		incompleteKind: NullKind,
		concrete:       true,
	}, {
		value:          `true`,
		kind:           BoolKind,
		incompleteKind: BoolKind,
		concrete:       true,
	}, {
		value:          `false`,
		kind:           BoolKind,
		incompleteKind: BoolKind,
		concrete:       true,
	}, {
		value:          `bool`,
		kind:           BottomKind,
		incompleteKind: BoolKind,
	}, {
		value:          `2`,
		kind:           IntKind,
		incompleteKind: IntKind,
		concrete:       true,
	}, {
		value:          `2.0`,
		kind:           FloatKind,
		incompleteKind: FloatKind,
		concrete:       true,
	}, {
		value:          `2.0Mi`,
		kind:           IntKind,
		incompleteKind: IntKind,
		concrete:       true,
	}, {
		value:          `14_000`,
		kind:           IntKind,
		incompleteKind: IntKind,
		concrete:       true,
	}, {
		value:          `>=0 & <5`,
		kind:           BottomKind,
		incompleteKind: NumberKind,
	}, {
		value:          `float`,
		kind:           BottomKind,
		incompleteKind: FloatKind,
	}, {
		value:          `"str"`,
		kind:           StringKind,
		incompleteKind: StringKind,
		concrete:       true,
	}, {
		value:          "'''\n'''",
		kind:           BytesKind,
		incompleteKind: BytesKind,
		concrete:       true,
	}, {
		value:          "string",
		kind:           BottomKind,
		incompleteKind: StringKind,
	}, {
		value:          `{}`,
		kind:           StructKind,
		incompleteKind: StructKind,
		concrete:       true,
	}, {
		value:          `[]`,
		kind:           ListKind,
		incompleteKind: ListKind,
		concrete:       true,
	}, {
		value:    `{a: int, b: [1][a]}.b`,
		kind:     BottomKind,
		concrete: false,
	}, {
		value: `import "time"
			{a: time.Time}.a`,
		kind:           BottomKind,
		incompleteKind: StringKind,
		concrete:       false,
	}, {
		value: `import "time"
			{a: time.Time & string}.a`,
		kind:           BottomKind,
		incompleteKind: StringKind,
		concrete:       false,
	}, {
		value: `import "strings"
			{a: strings.ContainsAny("D")}.a`,
		kind:           BottomKind,
		incompleteKind: StringKind,
		concrete:       false,
	}, {
		value: `import "struct"
		{a: struct.MaxFields(2) & {}}.a`,
		kind:           StructKind, // Can determine a valid struct already.
		incompleteKind: StructKind,
		concrete:       true,
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
			if got := v.IsConcrete(); got != tc.concrete {
				t.Errorf("IsConcrete: got %v; want %v", got, tc.concrete)
			}
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
		err:    "cannot use value 1.0 (type float) as int",
		errU:   "cannot use value 1.0 (type float) as int",
		notInt: true,
	}, {
		value:  "int",
		err:    "non-concrete value int",
		errU:   "non-concrete value int",
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
		kind    Kind
		err     string
	}{{
		value:   "1",
		float:   "1",
		mant:    "1",
		exp:     0,
		float64: 1,
		fmt:     'g',
		kind:    IntKind,
	}, {
		value:   "-1",
		float:   "-1",
		mant:    "-1",
		exp:     0,
		float64: -1,
		fmt:     'g',
		kind:    IntKind,
	}, {
		value:   "1.0",
		float:   "1.0",
		mant:    "10",
		exp:     -1,
		float64: 1.0,
		fmt:     'g',
		kind:    FloatKind,
	}, {
		value:   "2.6",
		float:   "2.6",
		mant:    "26",
		exp:     -1,
		float64: 2.6,
		fmt:     'g',
		kind:    FloatKind,
	}, {
		value:   "20.600",
		float:   "20.60",
		mant:    "20600",
		exp:     -3,
		float64: 20.60,
		prec:    2,
		fmt:     'f',
		kind:    FloatKind,
	}, {
		value:   "1/0",
		float:   "",
		float64: 0,
		prec:    2,
		fmt:     'f',
		err:     "division by zero",
		kind:    BottomKind,
	}, {
		value:   "1.797693134862315708145274237317043567982e+308",
		float:   "1.8e+308",
		mant:    "1797693134862315708145274237317043567982",
		exp:     269,
		float64: math.Inf(1),
		prec:    2,
		fmt:     'g',
		err:     ErrAbove.Error(),
		kind:    FloatKind,
	}, {
		value:   "-1.797693134862315708145274237317043567982e+308",
		float:   "-1.8e+308",
		mant:    "-1797693134862315708145274237317043567982",
		exp:     269,
		float64: math.Inf(-1),
		prec:    2,
		fmt:     'g',
		kind:    FloatKind,
		err:     ErrBelow.Error(),
	}, {
		value:   "4.940656458412465441765687928682213723650e-324",
		float:   "4.941e-324",
		mant:    "4940656458412465441765687928682213723650",
		exp:     -363,
		float64: 0,
		prec:    4,
		fmt:     'g',
		kind:    FloatKind,
		err:     ErrBelow.Error(),
	}, {
		value:   "-4.940656458412465441765687928682213723650e-324",
		float:   "-4.940656458412465441765687928682213723650e-324",
		mant:    "-4940656458412465441765687928682213723650",
		exp:     -363,
		float64: 0,
		prec:    -1,
		fmt:     'g',
		kind:    FloatKind,
		err:     ErrAbove.Error(),
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			n := getInstance(t, tc.value).Value()
			if n.Kind() != tc.kind {
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

			b, _ := n.AppendFloat(nil, tc.fmt, tc.prec)
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
		err:   "non-concrete value string",
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
		err:   "",
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
		err:   "cannot use value \"str\" (type string) as null",
	}, {
		value: `null`,
	}, {
		value: `_`,
		err:   "non-concrete value _",
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
		err:   "cannot use value \"str\" (type string) as bool",
	}, {
		value: `true`,
		bool:  true,
	}, {
		value: `false`,
	}, {
		value: `bool`,
		err:   "non-concrete value bool",
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
		err:   "cannot use value \"str\" (type string) as list",
	}, {
		value: `[]`,
		res:   "[]",
	}, {
		value: `[1,2,3]`,
		res:   "[1,2,3,]",
	}, {
		value: `>=5*[1,2,3, ...int]`,
		err:   "incomplete",
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
		err:   "cannot use value \"str\" (type string) as struct",
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
		value: `{ for k, v in y if v > 1 {"\(k)": v} }
		y: {a:1,b:2,c:3}`,
		res: "{b:2,c:3,}",
	}, {
		value: `{ def :: 1, _hidden: 2, opt?: 3, reg: 4 }`,
		res:   "{reg:4,}",
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

			iter, err := obj.Fields(All())
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

func TestDefaults(t *testing.T) {
	testCases := []struct {
		value string
		def   string
		val   string
		ok    bool
	}{{
		value: `number | *1`,
		def:   "1",
		val:   "number",
		ok:    true,
	}, {
		value: `1 | 2 | *3`,
		def:   "3",
		val:   "1|2|3",
		ok:    true,
	}, {
		value: `*{a:1,b:2}|{a:1}|{b:2}`,
		def:   "{a: 1, b: 2}",
		val:   "{a: 1}|{b: 2}",
		ok:    true,
	}, {
		value: `{a:1}&{b:2}`,
		def:   `{a: 1, b: 2}`,
		val:   ``,
		ok:    false,
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			v := getInstance(t, "a: "+tc.value).Lookup("a")

			d, ok := v.Default()
			if ok != tc.ok {
				t.Errorf("hasDefault: got %v; want %v", ok, tc.ok)
			}

			if got := fmt.Sprint(d); got != tc.def {
				t.Errorf("default: got %v; want %v", got, tc.def)
			}

			op, val := v.Expr()
			if op != OrOp {
				return
			}
			vars := []string{}
			for _, v := range val {
				vars = append(vars, fmt.Sprint(v))
			}
			if got := strings.Join(vars, "|"); got != tc.val {
				t.Errorf("value: got %v; want %v", got, tc.val)
			}
		})
	}
}

func TestLen(t *testing.T) {
	testCases := []struct {
		input  string
		length string
	}{{
		input:  "[1, 3]",
		length: "2",
	}, {
		input:  "[1, 3, ...]",
		length: "int & >=2",
	}, {
		input:  `"foo"`,
		length: "3",
	}, {
		input:  `'foo'`,
		length: "3",
		// TODO: Currently not supported.
		// }, {
		// 	input:  "{a:1, b:3, a:1, c?: 3, _hidden: 4}",
		// 	length: "2",
	}, {
		input:  "3",
		length: "_|_(len not supported for type 4)", // TODO: fix kind name
	}}
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			v := getInstance(t, "a: "+tc.input).Lookup("a")

			length := v.Len()
			if got := fmt.Sprint(length); got != tc.length {
				t.Errorf("length: got %v; want %v", got, tc.length)
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

func TestEquals(t *testing.T) {
	testCases := []struct {
		a, b string
		want bool
	}{{
		`4`, `4`, true,
	}, {
		`"str"`, `2`, false,
	}, {
		`2`, `3`, false,
	}, {
		`[1]`, `[3]`, false,
	}, {
		`[]`, `[]`, true,
	}, {
		`{
			a: b,
			b: a,
		}`,
		`{
			a: b,
			b: a,
		}`,
		true,
	}, {
		`{
			a: "foo",
			b: "bar",
		}`,
		`{
			a: "foo",
		}`,
		false,
	}}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			var r Runtime
			a, err := r.Compile("a", tc.a)
			if err != nil {
				t.Fatal(err)
			}
			b, err := r.Compile("b", tc.b)
			if err != nil {
				t.Fatal(err)
			}
			got := a.Value().Equals(b.Value())
			if got != tc.want {
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
		dst:   new(string),
		want:  "str",
	}, {
		value: `"str"`,
		dst:   new(int),
		err:   "cannot unmarshal string into Go value of type int",
	}, {
		value: `{}`,
		dst:   &fields{},
		want:  fields{},
	}, {
		value: `{a:1,b:2,c:3}`,
		dst:   &fields{},
		want:  fields{A: 1, B: 2, C: 3},
	}, {
		value: `{for k, v in y if v > 1 {"\(k)": v} }
		y: {a:1,b:2,c:3}`,
		dst:  &fields{},
		want: fields{B: 2, C: 3},
	}, {
		value: `{a:1,b:2,c:int}`,
		dst:   new(fields),
		err:   "cannot convert incomplete value",
	}, {
		value: `[]`,
		dst:   intList(),
		want:  *intList(),
	}, {
		value: `[1,2,3]`,
		dst:   intList(),
		want:  *intList(1, 2, 3),
	}, {
		value: `[x for x in y if x > 1]
				y: [1,2,3]`,
		dst:  intList(),
		want: *intList(2, 3),
	}, {
		value: `[int]`,
		err:   "cannot convert incomplete value",
	}}
	for _, tc := range testCases {
		t.Run(tc.value, func(t *testing.T) {
			err := getInstance(t, tc.value).Value().Decode(tc.dst)
			checkFatal(t, err, tc.err, "init")

			got := reflect.ValueOf(tc.dst).Elem().Interface()
			if !cmp.Equal(got, tc.want) {
				t.Error(cmp.Diff(got, tc.want))
				t.Errorf("\n%#v\n%#v", got, tc.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc string
		in   string
		err  bool
		opts []Option
	}{{
		desc: "issue #51",
		in: `
		a <Name>: foo
		a b: {}
		`,
		err: true,
	}, {
		desc: "concrete",
		in: `
		a: 1
		b: { c: 2, d: 3 }
		c d e f: 5
		`,
		opts: []Option{Concrete(true)},
	}, {
		desc: "disjunction",
		in:   `a: 1 | 2`,
	}, {
		desc: "disjunction concrete",
		in:   `a: 1 | 2`,
		opts: []Option{Concrete(true)},
		err:  true,
	}, {
		desc: "incomplete concrete",
		in:   `a: string`,
	}, {
		desc: "incomplete",
		in:   `a: string`,
		opts: []Option{Concrete(true)},
		err:  true,
	}, {
		desc: "list",
		in:   `a: [{b: string}, 3]`,
	}, {
		desc: "list concrete",
		in:   `a: [{b: string}, 3]`,
		opts: []Option{Concrete(true)},
		err:  true,
	}, {
		desc: "allow cycles",
		in: `
			a: b - 100
			b: a + 100
			c: [c[1], c[0]]
			`,
	}, {
		desc: "disallow cycles",
		in: `
			a: b - 100
			b: a + 100
			c: [c[1], c[0]]
			`,
		opts: []Option{DisallowCycles(true)},
		err:  true,
	}, {
		desc: "builtins are okay",
		in: `
		import "time"

		a: { b: time.Duration } | { c: time.Duration }
		`,
	}}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			r := Runtime{}
			inst, err := r.Parse("validate", tc.in)
			if err != nil {
				t.Fatal(err)
			}
			err = inst.Value().Validate(tc.opts...)
			if gotErr := err != nil; gotErr != tc.err {
				t.Errorf("got %v; want %v", err, tc.err)
			}
		})
	}
}

func TestPath(t *testing.T) {
	config := `
	a b c: 5
	b: {
		b1: 3
		b2: 4
		"b 3": 5
		"4b": 6
		l: [
			{a: 2},
			{c: 2},
		]
	}
	`
	mkpath := func(p ...string) []string { return p }
	testCases := [][]string{
		mkpath("a", "b", "c"),
		mkpath("b", "l", "1", "c"),
		mkpath("b", `"b 3"`),
		mkpath("b", `"4b"`),
	}
	for _, tc := range testCases {
		r := Runtime{}
		inst, err := r.Parse("config", config)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(strings.Join(tc, "."), func(t *testing.T) {
			v := inst.Lookup(tc[0])
			for _, e := range tc[1:] {
				if '0' <= e[0] && e[0] <= '9' {
					i, err := strconv.Atoi(e)
					if err != nil {
						t.Fatal(err)
					}
					iter, err := v.List()
					if err != nil {
						t.Fatal(err)
					}
					for c := 0; iter.Next(); c++ {
						if c == i {
							v = iter.Value()
							break
						}
					}
				} else if e[0] == '"' {
					v = v.Lookup(e[1 : len(e)-1])
				} else {
					v = v.Lookup(e)
				}
			}
			got, _ := v.path.appendPath(nil, v.idx)
			if !reflect.DeepEqual(got, tc) {
				t.Errorf("got %v; want %v", got, tc)
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
		str:    "{a: {a: 0, b: 1, c: 2}, b: {d: a.a, e: int}",
	}, {
		config: config,
		path:   strList("a", "a"),
		str:    "0",
	}, {
		config: config,
		path:   strList("a"),
		str:    "{a: 0, b: 1, c: 2}",
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
		str:    "cannot use value 0 (type int) as struct",
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

func cmpError(a, b error) bool {
	if a == nil {
		return b == nil
	}
	if b == nil {
		return a == nil
	}
	return a.Error() == b.Error()
}

func TestAttributeErr(t *testing.T) {
	const config = `
	a: {
		a: 0 @foo(a,b,c=1)
		b: 1 @bar(a,b,c,d=1) @foo(a,,d=1)
	}
	`
	testCases := []struct {
		path string
		attr string
		err  error
	}{{
		path: "a",
		attr: "foo",
		err:  nil,
	}, {
		path: "a",
		attr: "bar",
		err:  errors.New(`attribute "bar" does not exist`),
	}, {
		path: "xx",
		attr: "bar",
		err:  errors.New(`attribute "bar" does not exist`),
	}, {
		path: "e",
		attr: "bar",
		err:  errors.New(`attribute "bar" does not exist`),
	}}
	for _, tc := range testCases {
		t.Run(tc.path+"-"+tc.attr, func(t *testing.T) {
			v := getInstance(t, config).Value().Lookup("a", tc.path)
			a := v.Attribute(tc.attr)
			err := a.Err()
			if !cmpError(err, tc.err) {
				t.Errorf("got %v; want %v", err, tc.err)
			}
		})
	}
}

func TestAttributeString(t *testing.T) {
	const config = `
	a: {
		a: 0 @foo(a,b,c=1)
		b: 1 @bar(a,b,c,d=1) @foo(a,,d=1)
	}
	`
	testCases := []struct {
		path string
		attr string
		pos  int
		str  string
		err  error
	}{{
		path: "a",
		attr: "foo",
		pos:  0,
		str:  "a",
	}, {
		path: "a",
		attr: "foo",
		pos:  2,
		str:  "c=1",
	}, {
		path: "b",
		attr: "bar",
		pos:  3,
		str:  "d=1",
	}, {
		path: "e",
		attr: "bar",
		err:  errors.New(`attribute "bar" does not exist`),
	}, {
		path: "b",
		attr: "foo",
		pos:  4,
		err:  errors.New("field does not exist"),
	}}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s.%s:%d", tc.path, tc.attr, tc.pos), func(t *testing.T) {
			v := getInstance(t, config).Value().Lookup("a", tc.path)
			a := v.Attribute(tc.attr)
			got, err := a.String(tc.pos)
			if !cmpError(err, tc.err) {
				t.Errorf("err: got %v; want %v", err, tc.err)
			}
			if got != tc.str {
				t.Errorf("str: got %v; want %v", got, tc.str)
			}
		})
	}
}

func TestAttributeInt(t *testing.T) {
	const config = `
	a: {
		a: 0 @foo(1,3,c=1)
		b: 1 @bar(a,-4,c,d=1) @foo(a,,d=1)
	}
	`
	testCases := []struct {
		path string
		attr string
		pos  int
		val  int64
		err  error
	}{{
		path: "a",
		attr: "foo",
		pos:  0,
		val:  1,
	}, {
		path: "b",
		attr: "bar",
		pos:  1,
		val:  -4,
	}, {
		path: "e",
		attr: "bar",
		err:  errors.New(`attribute "bar" does not exist`),
	}, {
		path: "b",
		attr: "foo",
		pos:  4,
		err:  errors.New("field does not exist"),
	}, {
		path: "a",
		attr: "foo",
		pos:  2,
		err:  errors.New(`strconv.ParseInt: parsing "c=1": invalid syntax`),
	}}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s.%s:%d", tc.path, tc.attr, tc.pos), func(t *testing.T) {
			v := getInstance(t, config).Value().Lookup("a", tc.path)
			a := v.Attribute(tc.attr)
			got, err := a.Int(tc.pos)
			if !cmpError(err, tc.err) {
				t.Errorf("err: got %v; want %v", err, tc.err)
			}
			if got != tc.val {
				t.Errorf("val: got %v; want %v", got, tc.val)
			}
		})
	}
}

func TestAttributeFlag(t *testing.T) {
	const config = `
	a: {
		a: 0 @foo(a,b,c=1)
		b: 1 @bar(a,b,c,d=1) @foo(a,,d=1)
	}
	`
	testCases := []struct {
		path string
		attr string
		pos  int
		flag string
		val  bool
		err  error
	}{{
		path: "a",
		attr: "foo",
		pos:  0,
		flag: "a",
		val:  true,
	}, {
		path: "b",
		attr: "bar",
		pos:  1,
		flag: "a",
		val:  false,
	}, {
		path: "b",
		attr: "bar",
		pos:  0,
		flag: "c",
		val:  true,
	}, {
		path: "e",
		attr: "bar",
		err:  errors.New(`attribute "bar" does not exist`),
	}, {
		path: "b",
		attr: "foo",
		pos:  4,
		err:  errors.New("field does not exist"),
	}}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s.%s:%d", tc.path, tc.attr, tc.pos), func(t *testing.T) {
			v := getInstance(t, config).Value().Lookup("a", tc.path)
			a := v.Attribute(tc.attr)
			got, err := a.Flag(tc.pos, tc.flag)
			if !cmpError(err, tc.err) {
				t.Errorf("err: got %v; want %v", err, tc.err)
			}
			if got != tc.val {
				t.Errorf("val: got %v; want %v", got, tc.val)
			}
		})
	}
}

func TestAttributeLookup(t *testing.T) {
	const config = `
	a: {
		a: 0 @foo(a,b,c=1)
		b: 1 @bar(a,b,e=-5,d=1) @foo(a,,d=1)
	}
	`
	testCases := []struct {
		path string
		attr string
		pos  int
		key  string
		val  string
		err  error
	}{{
		path: "a",
		attr: "foo",
		pos:  0,
		key:  "c",
		val:  "1",
	}, {
		path: "b",
		attr: "bar",
		pos:  1,
		key:  "a",
		val:  "",
	}, {
		path: "b",
		attr: "bar",
		pos:  0,
		key:  "e",
		val:  "-5",
	}, {
		path: "b",
		attr: "bar",
		pos:  0,
		key:  "d",
		val:  "1",
	}, {
		path: "b",
		attr: "foo",
		pos:  2,
		key:  "d",
		val:  "1",
	}, {
		path: "b",
		attr: "foo",
		pos:  2,
		key:  "f",
		val:  "",
	}, {
		path: "e",
		attr: "bar",
		err:  errors.New(`attribute "bar" does not exist`),
	}, {
		path: "b",
		attr: "foo",
		pos:  4,
		err:  errors.New("field does not exist"),
	}}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s.%s:%d", tc.path, tc.attr, tc.pos), func(t *testing.T) {
			v := getInstance(t, config).Value().Lookup("a", tc.path)
			a := v.Attribute(tc.attr)
			got, _, err := a.Lookup(tc.pos, tc.key)
			if !cmpError(err, tc.err) {
				t.Errorf("err: got %v; want %v", err, tc.err)
			}
			if got != tc.val {
				t.Errorf("val: got %v; want %v", got, tc.val)
			}
		})
	}
}

func TestValueDoc(t *testing.T) {
	const config = `
	// foobar defines at least foo.
	package foobar

	// A Foo fooses stuff.
	Foo: {
		// field1 is an int.
		field1: int

		field2: int

		// duplicate field comment
		dup3: int
	}

	// foos are instances of Foo.
	foos <foo>: Foo

	// My first little foo.
	foos MyFoo: {
		// local field comment.
		field1: 0

		// Dangling comment.

		// other field comment.
		field2: 1

		// duplicate field comment
		dup3: int
	}

	bar: {
		// comment from bar on field 1
		field1: int
		// comment from bar on field 2
		field2: int // don't include this
	}

	baz: bar & {
		// comment from baz on field 1
		field1: int
		field2: int
	}
	`
	config2 := `
	// Another Foo.
	Foo: {}
	`
	var r Runtime
	getInst := func(name, body string) *Instance {
		inst, err := r.Compile("dir/file1.cue", body)
		if err != nil {
			t.Fatal(err)
		}
		return inst
	}

	inst := getInst("config", config)

	v1 := inst.Value()
	v2 := getInst("config2", config2).Value()
	both := v1.Unify(v2)

	testCases := []struct {
		val  Value
		path string
		doc  string
	}{{
		val:  v1,
		path: "foos",
		doc:  "foos are instances of Foo.\n",
	}, {
		val:  v1,
		path: "foos MyFoo",
		doc:  "My first little foo.\n",
	}, {
		val:  v1,
		path: "foos MyFoo field1",
		doc: `field1 is an int.

local field comment.
`,
	}, {
		val:  v1,
		path: "foos MyFoo field2",
		doc:  "other field comment.\n",
	}, {
		val:  v1,
		path: "foos MyFoo dup3",
		doc: `duplicate field comment

duplicate field comment
`,
	}, {
		val:  v1,
		path: "bar field1",
		doc:  "comment from bar on field 1\n",
	}, {
		val:  v1,
		path: "baz field1",
		doc: `comment from baz on field 1

comment from bar on field 1
`,
	}, {
		val:  v1,
		path: "baz field2",
		doc:  "comment from bar on field 2\n",
	}, {
		val:  v2,
		path: "Foo",
		doc: `Another Foo.
`,
	}, {
		val:  both,
		path: "Foo",
		doc: `Another Foo.

A Foo fooses stuff.
`,
	}}
	for _, tc := range testCases {
		t.Run("field:"+tc.path, func(t *testing.T) {
			v := tc.val.Lookup(strings.Split(tc.path, " ")...)
			doc := docStr(v.Doc())
			if doc != tc.doc {
				t.Errorf("doc: got:\n%vwant:\n%v", doc, tc.doc)
			}
		})
	}
	want := "foobar defines at least foo.\n"
	if got := docStr(inst.Doc()); got != want {
		t.Errorf("pkg: got:\n%vwant:\n%v", got, want)
	}
}

func docStr(docs []*ast.CommentGroup) string {
	doc := ""
	for _, d := range docs {
		if doc != "" {
			doc += "\n"
		}
		doc += d.Text()
	}
	return doc
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
		value: `0/0`,
		err:   "division undefined",
	}, {
		value: `[]`,
		json:  `[]`,
	}, {
		value: `[1, 2, 3]`,
		json:  `[1,2,3]`,
	}, {
		value: `[int]`,
		err:   `0: cannot convert incomplete value`,
	}, {
		value: `(>=3 * [1, 2])`,
		err:   "incomplete error", // TODO: improve error
	}, {
		value: `{}`,
		json:  `{}`,
	}, {
		value: `{a: 2, b: 3, c: ["A", "B"]}`,
		json:  `{"a":2,"b":3,"c":["A","B"]}`,
	}, {
		value: `{a: 2, b: 3, c: [string, "B"]}`,
		err:   `c.0: cannot convert incomplete value`,
	}, {
		value: `{a: [{b: [0, {c: string}] }] }`,
		err:   `path a.0.b.1.c: cannot convert incomplete value`,
	}, {
		value: `{foo?: 1, bar?: 2, baz: 3}`,
		json:  `{"baz":3}`,
	}, {
		// Has an unresolved cycle, but should not matter as all fields involved
		// are optional
		value: `{foo?: bar, bar?: foo, baz: 3}`,
		json:  `{"baz":3}`,
	}, {
		// Issue #107
		value: `a: 1.0/1`,
		json:  `{"a":1}`,
	}, {
		// Issue #108
		value: `
		a: int
		a: >0
		a: <2

		b: int
		b: >=0.9
		b: <1.1

		c: int
		c: >1
		c: <=2

		d: int
		d: >=1
		d: <=1.5

		e: int
		e: >=1
		e: <=1.32

		f: >=1.1 & <=1.1
		`,
		json: `{"a":1,"b":1,"c":2,"d":1,"e":1,"f":1.1}`,
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
		out: `_|_(undefined field "b")`,
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
		value: `3 * [1, 2]`,
		out:   `[1,2,1,2,1,2]`,
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

func TestReference(t *testing.T) {
	testCases := []struct {
		input string
		want  string
	}{{
		input: "v: _|_",
		want:  "",
	}, {
		input: "v: 2",
		want:  "",
	}, {
		input: "v: a, a: 1",
		want:  "a",
	}, {
		input: "v: a.b.c, a b c: 1",
		want:  "a b c",
	}}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			v := getInstance(t, tc.input).Lookup("v")
			inst, a := v.Reference()
			if got := strings.Join(a, " "); got != tc.want {
				t.Errorf("\n got %v;\nwant %v", got, tc.want)
			}

			if tc.want != "" {
				v := inst.Lookup(a...)
				if x, _ := v.Int64(); x != 1 {
					t.Errorf("path resolved to %s; want 1", v)
				}
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
	c: {args: s1 + s2}.args
	s1: string
	s2: string
	d: ({arg: b}).arg.c
	e: f.arg.c
	f: {arg: b}
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
		{config2, "c", "s1 s2"},
		// {config2, "d", "b.c"}, // TODO: make this work as well.
		{config2, "e", "f.arg.c"}, // TODO: should also report b.c.
	}
	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			ctx, st := compileFile(t, tc.config)
			v := newValueRoot(ctx, st)
			for _, k := range strings.Split(tc.in, ".") {
				obj, err := v.structValFull(ctx)
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

func TestExpr(t *testing.T) {
	testCases := []struct {
		input string
		want  string
	}{{
		input: "v: 3",
		want:  " 3",
	}, {
		input: "v: 3 + 4",
		want:  "+ 3 4",
	}, {
		input: "v: !a, a: 3",
		want:  "! <0>.a",
	}, {
		input: "v: 1 | 2 | 3 | *4",
		want:  "| 1 2 3 4",
	}, {
		input: "v: 2 & 5",
		want:  "& 2 5",
	}, {
		input: "v: 2 | 5",
		want:  "| 2 5",
	}, {
		input: "v: 2 && 5",
		want:  "&& 2 5",
	}, {
		input: "v: 2 || 5",
		want:  "|| 2 5",
	}, {
		input: "v: 2 == 5",
		want:  "== 2 5",
	}, {
		input: "v: !b, b: true",
		want:  "! <0>.b",
	}, {
		input: "v: 2 != 5",
		want:  "!= 2 5",
	}, {
		input: "v: <5",
		want:  "< 5",
	}, {
		input: "v: 2 <= 5",
		want:  "<= 2 5",
	}, {
		input: "v: 2 > 5",
		want:  "> 2 5",
	}, {
		input: "v: 2 >= 5",
		want:  ">= 2 5",
	}, {
		input: "v: 2 =~ 5",
		want:  "=~ 2 5",
	}, {
		input: "v: 2 !~ 5",
		want:  "!~ 2 5",
	}, {
		input: "v: 2 + 5",
		want:  "+ 2 5",
	}, {
		input: "v: 2 - 5",
		want:  "- 2 5",
	}, {
		input: "v: 2 * 5",
		want:  "* 2 5",
	}, {
		input: "v: 2 / 5",
		want:  "/ 2 5",
	}, {
		input: "v: 2 quo 5",
		want:  "quo 2 5",
	}, {
		input: "v: 2 rem 5",
		want:  "rem 2 5",
	}, {
		input: "v: 2 div 5",
		want:  "div 2 5",
	}, {
		input: "v: 2 mod 5",
		want:  "mod 2 5",
	}, {
		input: "v: a.b, a b: 4",
		want:  `. <0>.a "b"`,
	}, {
		input: `v: a["b"], a b: 3 `,
		want:  `[] <0>.a "b"`,
	}, {
		input: "v: a[2:5], a: [1, 2, 3, 4, 5]",
		want:  "[:] <0>.a 2 5",
	}, {
		input: "v: len([])",
		want:  "() len []",
	}, {
		input: `v: "Hello, \(x)! Welcome to \(place)", place: string, x: string`,
		want:  `\() "Hello, " <0>.x "! Welcome to " <0>.place ""`,
	}}
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			v := getInstance(t, tc.input).Lookup("v")
			op, operands := v.Expr()
			got := opToString[op]
			for _, v := range operands {
				got += " "
				got += debugStr(v.ctx(), v.path.v)
			}
			if got != tc.want {
				t.Errorf("\n got %v;\nwant %v", got, tc.want)
			}
		})
	}
}
