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
	"fmt"
	"log"
	"strings"
	"testing"

	"cuelang.org/go/cue/format"
)

func TestExport(t *testing.T) {
	testCases := []struct {
		raw     bool // skip evaluation the root, fully raw
		eval    bool // evaluate the full export
		noOpt   bool
		in, out string
	}{{
		in:  `"hello"`,
		out: `"hello"`,
	}, {
		in:  `'hello'`,
		out: `'hello'`,
	}, {
		in: `'hello\nworld'`,
		out: "'''" +
			multiSep + "hello" +
			multiSep + "world" +
			multiSep + "'''",
	}, {
		in: `"hello\nworld"`,
		out: `"""` +
			multiSep + "hello" +
			multiSep + "world" +
			multiSep + `"""`,
	}, {
		in: `{
			$type: 3
			"_": int
			"_foo": int
			_bar: int
		}`,
		out: unindent(`
		{
			$type:  3
			"_":    int
			"_foo": int
			_bar:   int
		}`),
	}, {
		in: "{ a: 1, b: a + 2, c: null, d: true, e: _, f: string }",
		out: unindent(`
			{
				a: 1
				b: 3
				c: null
				d: true
				e: _
				f: string
			}`),
	}, {
		// Here the failed lookups are not considered permanent
		// failures, as the structs are open.
		in: `{ a: { b: 2.0, s: "abc" }, b: a.b, c: a.c, d: a["d"], e: a.t[2:3] }`,
		out: unindent(`
			{
				a: {
					b: 2.0
					s: "abc"
				}
				b: 2.0
				c: a.c
				d: a["d"]
				e: a.t[2:3]
			}`),
	}, {
		// Here the failed lookups are permanent failures as the structs are
		// closed.
		in: `{ a :: { b: 2.0, s: "abc" }, b: a.b, c: a.c, d: a["d"], e: a.t[2:3] }`,
		out: unindent(`
			{
				a :: {
					b: 2.0
					s: "abc"
				}
				b: 2.0
				c: _|_ // undefined field "c"
				d: _|_ // undefined field "d"
				e: _|_ // undefined field "t"
			}`),
	}, {
		// Insert comma between error and inserted message.
		in: `{ a: [ 3&4] }`,
		out: unindent(`
		{
			a: [_|_, // conflicting values 3 and 4
			]
		}`),
	}, {
		in: `{
			a: 5*[int]
			a: [1, 2, ...]
			b: <=5*[int]
			b: [1, 2, ...]
			c: (>=3 & <=5)*[int]
			c: [1, 2, ...]
			d: >=2*[int]
			d: [1, 2, ...]
			e: [...int]
			e: [1, 2, ...]
			f: [1, 2, ...]
		}`,
		out: unindent(`
		{
			a: [1, 2, int, int, int]
			b: <=5*[int] & [1, 2, ...]
			c: (>=3 & <=5)*[int] & [1, 2, ...]
			d: >=2*[int] & [1, 2, ...]
			e: [1, 2]
			f: [1, 2]
		}`),
	}, {
		raw: true,
		in: `{
			a: 5*[int]
			a: [1, 2, ...]
			b: <=5*[int]
			b: [1, 2, ...]
			c: (>=3 & <=5)*[int]
			c: [1, 2, ...]
			d: >=2*[int]
			d: [1, 2, ...]
			e: [...int]
			e: [1, 2, ...]
			f: [1, 2, ...]
		}`,
		out: unindent(`
		{
			a: 5*[int] & [1, 2, ...]
			b: <=5*[int] & [1, 2, ...]
			c: (>=3 & <=5)*[int] & [1, 2, ...]
			d: >=2*[int] & [1, 2, ...]
			e: [...int] & [1, 2, ...]
			f: [1, 2, ...]
		}`),
	}, {
		raw: true,
		in:  `{ a: { b: [] }, c: a.b, d: a["b"] }`,
		out: unindent(`
			{
				a: b: []
				c: a.b
				d: a["b"]
			}`),
	}, {
		raw: true,
		in:  `{ a: *"foo" | *"bar" | *string | int, b: a[2:3] }`,
		out: unindent(`
			{
				a: *"foo" | *"bar" | *string | int
				b: a[2:3]
			}`),
	}, {
		in: `{
			a: >=0 & <=10 & !=1
		}`,
		out: unindent(`
			{
				a: >=0 & <=10 & !=1
			}`),
	}, {
		raw: true,
		in: `{
				a: >=0 & <=10 & !=1
			}`,
		out: unindent(`
			{
				a: >=0 & <=10 & !=1
			}`),
	}, {
		raw:  true,
		eval: true,
		in: `{
				a: (*1 | 2) & (1 | *2)
				b: [(*1 | 2) & (1 | *2)]
			}`,
		out: unindent(`
			{
				a: 1 | 2 | *_|_
				b: [1 | 2 | *_|_]
			}`),
	}, {
		raw:  true,
		eval: true,
		in: `{
				u16: int & >=0 & <=65535
				u32: uint32
				u64: uint64
				u128: uint128
				u8: uint8
				ua: uint16 & >0
				us: >=0 & <10_000 & int
				i16: >=-32768 & int & <=32767
				i32: int32 & > 0
				i64:  int64
				i128: int128
				f64:  float64
				fi:  float64 & int
			}`,
		out: unindent(`
			{
				u16:  uint16
				u32:  uint32
				u64:  uint64
				u128: uint128
				u8:   uint8
				ua:   uint16 & >0
				us:   uint & <10000
				i16:  int16
				i32:  int32 & >0
				i64:  int64
				i128: int128
				f64:  float64
				fi:   int & float64
			}`),
	}, {
		raw: true,
		in:  `{ a: [1, 2], b: { for k, v in a if v > 1 { "\(k)": v } } }`,
		out: unindent(`
			{
				a: [1, 2]
				b: {
					for k, v in a if v > 1 {
						"\(k)": v
					}
				}
			}`),
	}, {
		raw: true,
		in:  `{ a: [1, 2], b: [ for k, v in a { v }] }`,
		out: unindent(`
			{
				a: [1, 2]
				b: [ for k, v in a { v } ]
			}`),
	}, {
		raw: true,
		in:  `{ a: >=0 & <=10, b: "Count: \(a) times" }`,
		out: unindent(`
			{
				a: >=0 & <=10
				b: "Count: \(a) times"
			}`),
	}, {
		raw: true,
		in:  `{ a: "", b: len(a) }`,
		out: unindent(`
				{
					a: ""
					b: len(a)
				}`),
	}, {
		raw:  true,
		eval: true,
		in: `{
			b: {
				idx: a[str]
				str: string
			}
			b: a: b: 4
			a: b: 3
		}`,
		// reference to a must be redirected to outer a through alias
		out: unindent(`
		{
			A = a
			b: {
				idx: A[str]
				a: b: 4
				str: string
			}
			a: b: 3
		}`),
	}, {
		raw:   true,
		eval:  true,
		noOpt: true,
		in: `{
			job: [Name=_]: {
				name:     Name
				replicas: uint | *1 @protobuf(10)
				command:  string
			}

			job: list: command: "ls"

			job: nginx: {
				command:  "nginx"
				replicas: 2
			}
		}`,
		out: unindent(`
		{
			job: {
				list: {
					name:     "list"
					replicas: >=0 | *1 @protobuf(10)
					command:  "ls"
				}
				nginx: {
					name:     "nginx"
					replicas: 2 @protobuf(10)
					command:  "nginx"
				}
			}
		}`),
	}, {
		// TODO: positions of embedded structs is not preserved. Use some kind
		// of topological sort to preserve order.
		raw: true,
		in: `{
			emb :: {
				a: 1

				sub: {
					f: 3
				}
			}
			def :: {
				emb

				b: 2
			}
			f :: { a: 10 }
			e :: {
				f

				b: int
				[_]: <100
			}
		}`,
		out: unindent(`
		{
			emb :: {
				a: 1
				sub: f: 3
			}
			f :: {
				a: 10
			}
			def :: {
				b: 2
				emb
			}
			e :: {
				[string]: <100
				b:        int
				f
			}
		}`),
	}, {
		raw:   true,
		eval:  true,
		noOpt: true,
		in: `{
				reg: { foo: 1, bar: { baz: 3 } }
				def :: {
					a: 1

					sub: reg
				}
				val: def
				def2 :: {
					a: { b: int }
				}
				val2: def2
			}`,
		out: unindent(`
		{
			reg: {
				foo: 1
				bar: baz: 3
			}
			def :: {
				a:   1
				sub: reg
			}
			val: def
			def2 :: {
				a: b: int
			}
			val2: def2
		}`),
	}, {
		raw:  true,
		eval: true,
		in: `{
			b: [{
				[X=_]: int
				if a > 4 {
					f: 4
				}
			}][a]
			a: int
			c: *1 | 2
		}`,
		// reference to a must be redirected to outer a through alias
		out: unindent(`
		{
			b: [{
				[X=string]: int
				if a > 4 {
					f: 4
				}
			}][a]
			a: int
			c: *1 | 2
		}`),
	}, {
		raw: true,
		in: `{
			if false {
				{ a: 1 } | { b: 1 }
			}
		}`,
		// reference to a must be redirected to outer a through alias
		out: unindent(`
		{
			if false {
				{
					a: 1
				} | {
					b: 1
				}
			}
		}`),
	}, {
		raw:  true,
		eval: true,
		in: `{
			Foo :: {
			Bar :: Foo | string
			}
		}`,
		out: unindent(`
		{
			Foo :: {
				Bar :: Foo | string
			}
		}`),
	}, {
		raw:  true,
		eval: true,
		in: `{
				FindInMap :: {
					"Fn::FindInMap" :: [string | FindInMap]
				}
				a: [...string]
			}`,
		out: unindent(`
			{
				FindInMap :: {
					"Fn::FindInMap" :: [string | FindInMap]
				}
				a: [...string]
			}`)}, {
		raw:   true,
		eval:  true,
		noOpt: true,
		in: `{
				And :: {
					"Fn::And": [...(3 | And)]
				}
				Ands: And & {
					"Fn::And" : [_]
				}
			}`,
		out: unindent(`
			{
				And :: {
					"Fn::And": [...3 | And]
				}
				Ands: And & {
					"Fn::And": [_]
				}
			}`),
	}, {
		raw:  true,
		eval: true,
		in: `{
			Foo :: {
				sgl: Bar
				ref: null | Foo
				ext: Bar | null
				ref: null | Foo
				ref2: null | Foo.sgl
				...
			}
			Foo :: {
				Foo: 2
				...
			}
			Bar :: string
		}`,
		out: unindent(`
		{
			FOO = Foo
			Foo :: {
				Foo:  2
				sgl:  Bar
				ref:  (null | FOO) & (null | FOO)
				ext:  Bar | null
				ref2: null | FOO.sgl
				...
			}
			Bar :: string
		}`),
	}, {
		raw:  true,
		eval: true,
		in: `{
			A: [uint]
			B: A & ([10] | [192])
		}`,
		out: unindent(`
		{
			A: [>=0]
			B: A & ([10] | [192])
		}`),
	}, {
		in: `{
			[string]: _
			foo: 3
		}`,
		out: unindent(`
		{
			foo: 3
			...
		}`),
	}}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			body := fmt.Sprintf("Test: %s", tc.in)
			ctx, obj := compileFile(t, body)
			ctx.trace = *traceOn
			var root value = obj
			if !tc.raw {
				root = testResolve(ctx, obj, evalFull)
			}
			t.Log(debugStr(ctx, root))

			n := root.(*structLit).arcs[0].v
			v := newValueRoot(ctx, n)

			opts := options{raw: !tc.eval, omitOptional: tc.noOpt}
			node, _ := export(ctx, nil, v.eval(ctx), opts)
			b, err := format.Node(node, format.Simplify())
			if err != nil {
				log.Fatal(err)
			}
			if got := string(b); got != tc.out {
				t.Errorf("\ngot  %v;\nwant %v", got, tc.out)
			}
		})
	}
}

func TestExportFile(t *testing.T) {
	testCases := []struct {
		eval    bool // evaluate the full export
		in, out string
		opts    []Option
	}{{
		eval: true,
		opts: []Option{Docs(true)},
		in: `
		// Hello foo
		package foo

		import "strings"

		a: strings.ContainsAny("c")
		`,
		out: unindent(`
		// Hello foo
		package foo

		import "strings"

		a: strings.ContainsAny("c")`),
	}, {
		eval: true,
		in: `
		// Drop this comment
		package foo

		import "time"

		a: time.Time
		`,
		out: unindent(`
		package foo

		import "time"

		a: time.Time`),
	}, {
		in: `
		import "time"

		{
			a: time.Time
		} & {
			time: int
		}		`,
		out: unindent(`
		import timex "time"

		time: int
		a:    timex.Time`),
	}, {
		in: `
		import time2 "time"

		a:    time2.Time`,
		out: unindent(`
		import "time"

		a: time.Time`),
	}, {
		in: `
		import time2 "time"

		time: int
		a:    time2.Time`,
		out: unindent(`
		import time2 "time"

		time: int
		a:    time2.Time`),
	}, {
		in: `
		import "strings"

		a: strings.TrimSpace("  c  ")
		`,
		out: unindent(`
		import "strings"

		a: strings.TrimSpace("  c  ")`),
	}, {
		in: `
		import "strings"

		let stringsx = strings

		a: {
			strings: stringsx.ContainsAny("c")
		}
		`,
		out: unindent(`
		import "strings"

		let STRINGS = strings
		a: strings: STRINGS.ContainsAny("c")`),
	}, {
		in: `
			a: b - 100
			b: a + 100
		`,
		out: unindent(`
		a: b - 100
		b: a + 100`),
	}, {
		in: `A: {
			[_]: B
		} @protobuf(1,"test")

		B: {}
		B: {a: int} | {b: int}

		C :: [D]: int
		D :: string
		`,
		out: unindent(`
		A: {
			[string]: B
		} @protobuf(1,"test")
		B: {} & ({
			a: int
		} | {
			b: int
		})
		C :: {
			[D]: int
			[D]: int
		}
		D :: string`),
	}, {
		in: `
		import "time"

		a: { b: time.Duration } | { c: time.Duration }
		`,
		out: unindent(`
		import "time"

		a: {
			b: time.Duration
		} | {
			c: time.Duration
		}`),
	}, {
		// a closed struct unified with a struct with a template restrictions is
		// exported as a conjunction of two structs.
		eval: true,
		opts: []Option{ResolveReferences(true)},
		in: `
		A :: { b: int }
		a: A & { [string]: <10 }
		B :: a
		`,
		out: unindent(`
		A :: {
			b: int
		}
		a: close({
			b: <10
		})
		B :: {
			b: <10
		}`),
	}, {
		eval: true,
		opts: []Option{Final()},
		in: `{
			reg: { foo: 1, bar: { baz: 3 } }
			def :: {
				a: 1

				sub: reg
			}
			val: def
		}`,
		out: unindent(`
		{
			reg: {
				foo: 1
				bar: baz: 3
			}
			val: {
				a: 1
				sub: {
					foo: 1
					bar: baz: 3
				}
			}
		}`),
	}, {
		eval: true,
		opts: []Option{ResolveReferences(true)},
		in: `
			T :: {
				[_]: int64
			}
			X :: {
				x: int
			} & T
			x: X
			`,
		out: unindent(`
		T :: {
			[string]: int64
		}
		x: {
			[string]: int64
			x:        int64
		}
		X :: {
			[string]: int64
			x:        int64
		}`),
	}, {
		eval: true,
		opts: []Option{Optional(false), ResolveReferences(true)},
		in: `
		T :: {
			[_]: int64
		}
		X :: {
			x: int
		} & T
		x: X
		`,
		out: unindent(`
		T :: {}
		x: x: int64
		X :: {
			x: int64
		}`),
	}, {
		eval: true,
		opts: []Option{ResolveReferences(true)},
		in: `{
				reg: { foo: 1, bar: { baz: 3 } }
				def :: {
					a: 1
	
					sub: reg
				}
				val: def
				def2 :: {
					a: { b: int }
				}
				val2: def2
			}`,
		out: unindent(`
			{
				reg: {
					foo: 1
					bar: baz: 3
				}
				def :: {
					a: 1
					sub: {
						foo: 1
						bar: {
							baz: 3
							...
						}
						...
					}
				}
				val: close({
					a: 1
					sub: {
						foo: 1
						bar: baz: 3
					}
				})
				def2 :: {
					a: b: int
				}
				val2: close({
					a: close({
						b: int
					})
				})
			}`),
	}, {
		eval: true,
		in: `
				a?: 1
				b?: 2
				b?: 2
				c?: 3
				c: 3`,
		out: unindent(`
		a?: 1
		b?: 2
		c:  3`),
	}, {
		eval: true,
		opts: []Option{ResolveReferences(true)},
		in: `
		A :: {
			[=~"^[a-s]*$"]: int
		}
		B :: {
			[=~"^[m-z]+"]: int
		}
		C: {A & B}
		D :: {A & B}
		`,
		// TODO: the outer close of C could be optimized away.
		out: unindent(`
		A :: {
			[=~"^[a-s]*$"]: int
		}
		B :: {
			[=~"^[m-z]+"]: int
		}
		C: close({
			close({
				[=~"^[a-s]*$"]: int
			}) & close({
				[=~"^[m-z]+"]: int
			})
		})
		D :: {
			close({
				[=~"^[a-s]*$"]: int
			}) & close({
				[=~"^[m-z]+"]: int
			})
		}`),
	}, {
		eval: true,
		opts: []Option{Docs(true)},
		in: `
		// Definition
		A :: {
			// TODO: support
			[string]: int
		}
		// Field
		a: A

		// Pick first comment only.
		a: _
		`,
		out: unindent(`
		// Definition
		A :: {
			[string]: int
		}

		// Field
		a: A`),
	}, {
		eval: true,
		opts: []Option{Docs(true)},
		// It is okay to allow bulk-optional fields along-side definitions.
		in: `
		"#A": {
			[string]: int
			"#B": 4
		}
		// Definition
		#A: {
			[string]: int
			#B: 4
		}
		a: #A.#B
		`,
		out: unindent(`
		"#A": {
			[string]: int
			"#B":     4
		}
		// Definition
		#A: {
			[string]: int
			#B:       4
		}
		a: 4`),
	}, {
		in: `
		#A: {
			#B: 4
		}
		a: #A.#B
		`,
		out: unindent(`
		#A: #B: 4
		a: #A.#B`),
	}, {
		eval: true,
		in: `
		x: [string]: int
		a: [P=string]: {
			b: x[P]
			c: P
			e: len(P)
		}
		`,
		out: unindent(`
		x: [string]: int
		a: [P=string]: {
			b: x[P]
			c: P
			e: len(P)
		}`),
	}, {
		eval: true,
		in: `
		list: [...string]
		foo: 1 | 2 | *3
		foo: int
		`,
		out: unindent(`
		list: [...string]
		foo: 1 | 2 | *3`),
	}, {
		eval: true,
		opts: []Option{Final()},
		in: `
		list: [...string]
		foo: 1 | 2 | *3
		foo: int
		`,
		out: unindent(`
		{
			list: []
			foo: 3
		}`),
	}, {
		// Expand final values, not values that may still be incomplete.
		eval: true,
		in: `
		import "math"
		import "tool/exec"

		A :: {
			b: 3
		}

		a:   A
		pi:  math.Pi
		run: exec.Run
		`,
		out: unindent(`
		import "tool/exec"

		A :: {
			b: 3
		}
		a:   A
		pi:  3.14159265358979323846264338327950288419716939937510582097494459
		run: exec.Run`),
	}, {
		// Change mode midway to break cycle.
		eval: true,
		opts: []Option{Final(), Optional(true)},
		in: `
		Foo: {
			foo?: Foo
			bar:  string
			baz:  bar + "2"
		}

		foo: Foo & {
			foo: {
				bar: "barNested"
			}
			bar: "barParent"
		}`,
		out: unindent(`
		{
			Foo: {
				foo?: Foo
				bar:  string
				baz:  bar + "2"
			}
			foo: {
				foo: {
					foo?: Foo
					bar:  "barNested"
					baz:  "barNested2"
				}
				bar: "barParent"
				baz: "barParent2"
			}
		}`),
	}, {
		eval: true,
		opts: []Option{Final(), Definitions(true)},
		in: `
		package tst

		x: {
			a: A
			b: A
		}
		A :: string | [A]

		`,
		out: unindent(`
		{
			x: {
				a: A
				b: A
			}
			A :: string | [A]
		}`),
	}, {
		eval: true,
		opts: []Option{Definitions(true)},
		in: `
		body?: {
			a: int
			b?: string
		}
		`,
		out: unindent(`
		body?: {
			a:  int
			b?: string
		}`),
	}, {
		// Drop comprehensions in final mode.
		eval: true,
		opts: []Option{Final(), Concrete(true)},
		in: `
		foo: _
		a: {
			if len(foo.bar) > 0 {
				command: ["envoy-lifecycle", string]
			}
		}
		`,
		out: unindent(`
		{
			foo: _
			a: {}
		}`),
	}, {
		// Don't output hidden fields in concrete and final mode.
		eval: true,
		opts: []Option{Final(), Concrete(true)},
		in: `
			_foo: _
			`,
		out: `{}`,
	}, {
		eval: true,
		in: `
		A :: {
			foo: int @tag2(2)
		} @tag2(1)

		A :: {
			foo: 2 @tag(2)
		} @tag(1)

		`,
		out: `A :: {
	foo: 2 @tag(2) @tag2(2)
} @tag(1) @tag2(1)`,
	}}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			var r Runtime
			inst, err := r.Compile("test", tc.in)
			if err != nil {
				t.Fatal(err)
			}
			opts := tc.opts
			if !tc.eval {
				opts = []Option{Raw()}
			}
			b, err := format.Node(inst.Value().Syntax(opts...), format.Simplify())
			if err != nil {
				log.Fatal(err)
			}
			if got := strings.TrimSpace(string(b)); got != tc.out {
				t.Errorf("\ngot:\n%v\nwant:\n%v", got, tc.out)
			}
		})
	}
}

func unindent(s string) string {
	lines := strings.Split(s, "\n")[1:]
	ws := lines[0][:len(lines[0])-len(strings.TrimLeft(lines[0], " \t"))]
	for i, s := range lines {
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, ws) {
			panic("invalid indentation")
		}
		lines[i] = lines[i][len(ws):]
	}
	return strings.Join(lines, "\n")
}
