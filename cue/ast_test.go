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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/internal"
)

func TestCompile(t *testing.T) {
	testCases := []struct {
		in  string
		out string
	}{{
		in: `{
		  foo: 1,
		}`,
		out: "<0>{<1>{foo: 1}, }", // emitted value, but no top-level fields
	}, {
		in: `
		foo: 1
		`,
		out: "<0>{foo: 1}",
	}, {
		in: `
		a: true
		b: 2K
		c: 4_5
		d: "abc"
		e: 3e2 // 3h1m2ss
		"": 8
		`,
		out: `<0>{"": 8, a: true, b: 2000, c: 45, d: "abc", e: 3e+2}`,
	}, {
		in: `
		a: null
		b: true
		c: false
		`,
		out: "<0>{a: null, b: true, c: false}",
	}, {
		in: `
		a: <1
		b: >= 0 & <= 10
		c: != null
		d: >100
		`,
		out: `<0>{a: <1, b: (>=0 & <=10), c: !=null, d: >100}`,
	}, {
		in: "" +
			`a: "\(4)",
			b: "one \(a) two \(  a + c  )",
			c: "one"`,
		out: `<0>{a: ""+4+"", b: "one "+<0>.a+" two "+(<0>.a + <0>.c)+"", c: "one"}`,
	}, {
		in: "" +
			`a: """
				multi
				""",
			b: '''
				hello world
				goodbye globe
				welcome back planet
				'''`,
		out: `<0>{a: "multi", b: 'hello world\ngoodbye globe\nwelcome back planet'}`,
	}, {
		in: "" +
			`a: """
				multi \(4)
				""",
			b: """
				hello \("world")
				goodbye \("globe")
				welcome back \("planet")
				"""`,
		out: `<0>{a: "multi "+4+"", b: "hello "+"world"+"\ngoodbye "+"globe"+"\nwelcome back "+"planet"+""}`,
	}, {
		in: `
		a: _
		b: int
		c: float
		d: bool
		e: string
		`,
		out: "<0>{a: _, b: int, c: float, d: bool, e: string}",
	}, {
		in: `
		a: null
		b: true
		c: false
		`,
		out: "<0>{a: null, b: true, c: false}",
	}, {
		in: `
		null: null
		true: true
		false: false
		`,
		out: "<0>{null: null, true: true, false: false}",
	}, {
		in: `
		a: 1 + 2
		b: -2 - 3
		c: !d
		d: true
		`,
		out: "<0>{a: (1 + 2), b: (-2 - 3), c: !<0>.d, d: true}",
	}, {
		in: `
			l0: 3*[int]
			l0: [1, 2, 3]
			l1: <=5*[string]
			l1: ["a", "b"]
			l2: (<=5)*[{ a: int }]
			l2: [{a: 1}, {a: 2, b: 3}]
			l3: (<=10)*[int]
			l3: [1, 2, 3, ...]
			l4: [1, 2, ...]
			l4: [...int]
			l5: [1, ...int]

			s1: ((<=6)*[int])[2:3]
			s2: [0,2,3][1:2]

			e0: (>=2 & <=5)*[{}]
			e0: [{}]
			`,
		out: `<0>{l0: ((3 * [int]) & [1,2,3]), l1: ((<=5 * [string]) & ["a","b"]), l2: ((<=5 * [<1>{a: int}]) & [<2>{a: 1},<3>{a: 2, b: 3}]), l3: ((<=10 * [int]) & [1,2,3, ...]), l4: ([1,2, ...] & [, ...int]), l5: [1, ...int], s1: (<=6 * [int])[2:3], s2: [0,2,3][1:2], e0: (((>=2 & <=5) * [<4>{}]) & [<5>{}])}`,
	}, {
		in: `
		a: 5 | "a" | true
		aa: 5 | *"a" | true
		b: c: {
			cc: { ccc: 3 }
		}
		d: true
		`,
		out: "<0>{a: (5 | \"a\" | true), aa: (5 | *\"a\" | true), b: <1>{c: <2>{cc: <3>{ccc: 3}}}, d: true}",
	}, {
		in: `
		a: a: { b: a } // referencing ancestor nodes is legal.
		a: b: a.a      // do lookup before merging of nodes
		b: a.a        // different node as a.a.b, as first node counts
		c: a          // same node as b, as first node counts
		d: a["a"]
		`,
		out: `<0>{a: (<1>{a: <2>{b: <1>.a}} & <3>{b: <0>.a.a}), b: <0>.a.a, c: <0>.a, d: <0>.a["a"]}`,
		// TODO(#152): should be
		// out: `<0>{a: (<1>{a: <2>{b: <2>}} & <3>{b: <3>.a}), b: <0>.a.a, c: <0>.a, d: <0>.a["a"]}`,
	}, {
		// bunch of aliases
		in: `
		let a1 = a2
		let a2 = 5
		b: a1
		let a3 = d
		c: {
			d: {
				r: a3
			}
			r: a3
		}
		d: { e: 4 }
		`,
		out: `<0>{b: 5, c: <1>{d: <2>{r: <0>.d}, r: <0>.d}, d: <3>{e: 4}}`,
	}, {
		// aliases with errors
		in: `
		let e1 = 1
		let e1 = 2
		e1v: e1
		e2: "a"
		let e2 = "a"
		`,
		out: `alias "e1" redeclared in same scope:` + "\n" +
			"    test:3:3\n" +
			`cannot have both alias and field with name "e2" in same scope:` + "\n" +
			"    test:6:3\n" +
			"<0>{}",
	}, {
		in: `
		let a = b
		b: {
			c: a // reference to own root.
		}
		`,
		out: `<0>{b: <1>{c: <0>.b}}`,
		// }, {
		// 	// TODO: Support this:
		// 	// optional fields
		// 	in: `
		// 		X=[string]: { chain: X | null }
		// 		`,
		// 	out: `
		// 		`,
	}, {
		// optional fields
		in: `
			[ID=string]: { name: ID }
			A="foo=bar": 3
			a: A
			B=bb: 4
			b1: B
			b1: bb
			C="\(a)": 5
			c: C
			`,
		out: `<0>{[]: <1>(ID: string)-><2>{name: <1>.ID}, "foo=bar": 3, a: <0>."foo=bar", bb: 4, b1: (<0>.bb & <0>.bb), c: <0>[""+<0>.a+""]""+<0>.a+"": 5}`,
	}, {
		// optional fields with key filters
		in: `
			JobID: =~"foo"
			a: [JobID]: { name: string }

			[<"s"]: { other: string }
			`,
		out: `<0>{` +
			`[<"s"]: <1>(_: string)-><2>{other: string}, ` +
			`JobID: =~"foo", a: <3>{` +
			`[<0>.JobID]: <4>(_: string)-><5>{name: string}, ` +
			`}}`,
	}, {
		// Issue #251
		// TODO: the is one of the cases where it is relatively easy to catch
		// a structural cycle. We should be able, however, to break the cycle
		// with a post-validation constraint. Clean this up with the evaluator
		// update.
		in: `
		{
			[x]: 3
		}
		x:   "x"
		`,
		out: "reference \"x\" in label expression refers to field against which it would be matched:\n    test:3:5\n<0>{}",
	}, {
		// illegal alias usage
		in: `
			[X=string]: { chain: X | null }
			a: X
			Y=[string]: 3
			a: X
			`,
		out: `a: invalid label: cannot reference fields with square brackets labels outside the field value:
    test:3:7
a: invalid label: cannot reference fields with square brackets labels outside the field value:
    test:5:7
<0>{}`,
	}, {
		// detect duplicate aliases, even if illegal
		in: `
		[X=string]: int
		X=[string]: int
		Y=foo: int
		let Y=3
		Z=[string]: { Z=3, a: int } // allowed
		`,
		out: `alias "X" redeclared in same scope:
    test:3:3
alias "Y" redeclared in same scope:
    test:5:3
<0>{}`,
	}, {
		in: `
		a: {
			[name=_]: { n: name }
			k: 1
		}
		b: {
			[X=_]: { x: 0, y: 1 }
			v: {}
		}
		`,
		out: `<0>{a: <1>{[]: <2>(name: string)-><3>{n: <2>.name}, k: 1}, b: <4>{[]: <5>(X: string)-><6>{x: 0, y: 1}, v: <7>{}}}`,
	}, {
		in: `
		a: {
			for k, v in b if b.a < k {
				"\(k)": v
			}
		}
		b: {
			a: 1
			b: 2
			c: 3
		}
		`,
		out: `<0>{a: <1>{ <2>for k, v in <0>.b if (<0>.b.a < <2>.k) yield <3>{""+<2>.k+"": <2>.v}}, b: <4>{a: 1, b: 2, c: 3}}`,
	}, {
		in: `
			a: { for k, v in b {"\(v)": v} }
			b: { a: "aa", b: "bb", c: "cc" }
			`,
		out: `<0>{a: <1>{ <2>for k, v in <0>.b yield <3>{""+<2>.v+"": <2>.v}}, b: <4>{a: "aa", b: "bb", c: "cc"}}`,
	}, {
		in: `
			a: [ for _, v in b { v } ]
			b: { a: 1, b: 2, c: 3 }
			`,
		out: `<0>{a: [ <1>for _, v in <0>.b yield <1>.v ], b: <2>{a: 1, b: 2, c: 3}}`,
	}, {
		in: `
			a: >=1 & <=2
			b: >=1 & >=2 & <=3
			c: >="a" & <"b"
			d: >(2+3) & <(4+5)
			`,
		out: `<0>{a: (>=1 & <=2), b: ((>=1 & >=2) & <=3), c: (>="a" & <"b"), d: (>(2 + 3) & <(4 + 5))}`,
	}, {
		in: `
			a: *1,
			b: **1 | 2
		`,
		out: `a: preference mark not allowed at this position:
    test:2:7
b: preference mark not allowed at this position:
    test:3:8
<0>{}`,
	}, {
		in: `
			a: int @foo(1,"str")
		`,
		out: "<0>{a: int @foo(1,\"str\")}",
	}, {
		in: `
			a: int @b([,b) // invalid
		`,
		out: "unexpected ')':\n    test:2:18\nattribute missing ')':\n    test:3:3\n<0>{}",
	}, {
		in: `
		a: d: {
			base
			info :: {
				...
			}
			Y: info.X
		}

		base :: {
			info :: {...}
		}

		a: [Name=string]: { info :: {
			X: "foo"
		}}
		`,
		out: `<0>{` +
			`a: (<1>{d: <2>{info :: <3>{...}, Y: <2>.info.X}, <0>.base} & <4>{[]: <5>(Name: string)-><6>{info :: <7>C{X: "foo"}}, }), ` +
			`base :: <8>C{info :: <9>{...}}}`,
	}, {
		in: `
		a: d: {
			#base
			#info: {
				...
			}
			Y: #info.X
		}

		#base: {
			#info: {...}
		}

		a: [Name=string]: { #info: {
			X: "foo"
		}}
		`,
		out: `<0>{` +
			`a: (<1>{d: <2>{#info: <3>{...}, Y: <2>.#info.X}, <0>.#base} & <4>{[]: <5>(Name: string)-><6>{#info: <7>C{X: "foo"}}, }), ` +
			`#base: <8>C{#info: <9>{...}}}`,
	}, {
		in: `
		def :: {
			Type: string
			Text: string
			Size: int
		}

		def :: {
			Type: "B"
			Size: 0
		} | {
			Type: "A"
			Size: 1
		}
		`,
		out: `<0>{` +
			`def :: (<1>C{Size: int, Type: string, Text: string} & (<2>C{Size: 0, Type: "B"} | <3>C{Size: 1, Type: "A"}))` +
			`}`,
	}, {
		// Issue #172
		in: `
		package testenv
		env_:: [NAME=_]: [VALUE=_]
		env_:: foo: "bar"
			`,
		out: "env_.*: alias not allowed in list:\n    test:3:20\n<0>{}",
	}, {
		// Issue #276
		in: `
			a:     int=<100
			`,
		out: "alias \"int\" not allowed as value:\n    test:2:11\n<0>{}",
	}}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			ctx, root, err := compileFileWithErrors(t, tc.in)
			buf := &bytes.Buffer{}
			if err != nil {
				errors.Print(buf, err, nil)
			}
			buf.WriteString(debugStr(ctx, root))
			got := buf.String()
			if got != tc.out {
				t.Errorf("output differs:\ngot  %q\nwant %q", got, tc.out)
			}
		})
	}
}

func TestEmit(t *testing.T) {
	testCases := []struct {
		in  string
		out string
		rw  rewriteMode
	}{{
		in: `"\(hello), \(world)!"` + `
		hello: "Hello"
		world: "World"
		`,
		out: `""+<0>.hello+", "+<0>.world+"!"`,
		rw:  evalRaw,
	}, {
		in: `"\(hello), \(world)!"` + `
		hello: "Hello"
		world: "World"
		`,
		out: `"Hello, World!"`,
		rw:  evalPartial,
	}, {
		// Ambiguous disjunction must cary over to emit value.
		in: `baz

		baz: {
			a: 8000 | 7080
			a: 7080 | int
		}`,
		out: `<0>{a: (8000 | 7080)}`,
		rw:  evalFull,
	}}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			ctx, root := compileFile(t, tc.in)
			v := testResolve(ctx, root.emit, tc.rw)
			if got := debugStr(ctx, v); got != tc.out {
				t.Errorf("output differs:\ngot  %q\nwant %q", got, tc.out)
			}
		})
	}
}

func TestEval(t *testing.T) {
	testCases := []struct {
		in   string
		expr string
		out  string
	}{{
		in: `
			hello: "Hello"
			world: "World"
			`,
		expr: `"\(hello), \(world)!"`,
		out:  `"Hello, World!"`,
	}, {
		in: `
			a: { b: 2, c: 3 }
			z: 1
			`,
		expr: `a.b + a.c + z`,
		out:  `6`,
	}, {
		in: `
			a: { b: 2, c: 3 }
			`,
		expr: `{ d: a.b + a.c }`,
		out:  `<0>{d: 5}`,
	}, {
		in: `
			a: "Hello World!"
			`,
		expr: `strings.ToUpper(a)`,
		out:  `"HELLO WORLD!"`,
	}, {
		in: `
			a: 0x8
			b: 0x1`,
		expr: `bits.Or(a, b)`, // package shorthand
		out:  `9`,
	}, {
		in: `
			a: 0x8
			b: 0x1`,
		expr: `math.Or(a, b)`,
		out:  `_|_(<0>.Or:undefined field "Or")`,
	}, {
		in:   `a: 0x8`,
		expr: `mathematics.Abs(a)`,
		out:  `_|_(reference "mathematics" not found)`,
	}}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			ctx, inst, errs := compileInstance(t, tc.in)
			if errs != nil {
				t.Fatal(errs)
			}
			expr, err := parser.ParseExpr("<test>", tc.expr)
			if err != nil {
				t.Fatal(err)
			}
			evaluated := evalExpr(ctx, inst.eval(ctx), expr)
			v := testResolve(ctx, evaluated, evalFull)
			if got := debugStr(ctx, v); got != tc.out {
				t.Errorf("output differs:\ngot  %q\nwant %q", got, tc.out)
			}
		})
	}
}

func TestResolution(t *testing.T) {
	testCases := []struct {
		name string
		in   string
		err  string
	}{{
		name: "package name identifier should not resolve to anything",
		in: `package time

		import "time"

		a: time.Time
		`,
	}, {
		name: "duplicate_imports.cue",
		in: `
		import "time"
		import time "math"

		t: time.Time
		`,
		err: "time redeclared as imported package name",
	}, {
		name: "unused_import",
		in: `
			import "time"
			`,
		err: `imported and not used: "time"`,
	}, {
		name: "nonexisting import package",
		in:   `import "doesnotexist"`,
		err:  `package "doesnotexist" not found`,
	}, {
		name: "duplicate with different name okay",
		in: `
		import "time"
		import time2 "time"

		a: time.Time
		b: time2.Time
		`,
	}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var r Runtime
			_, err := r.Compile(tc.name, tc.in)
			got := err == nil
			want := tc.err == ""
			if got != want {
				t.Fatalf("got %v; want %v", err, tc.err)
			}
			if err != nil {
				if s := err.Error(); !strings.Contains(s, tc.err) {
					t.Errorf("got %v; want %v", err, tc.err)
				}
			}
		})
	}
}

func TestShadowing(t *testing.T) {
	spec := ast.NewImport(nil, "list")
	testCases := []struct {
		file *ast.File
		want string
	}{{
		file: &ast.File{Decls: []ast.Decl{
			&ast.ImportDecl{Specs: []*ast.ImportSpec{spec}},
			&ast.Field{
				Label: mustParseExpr(`list`).(*ast.Ident),
				Value: ast.NewCall(
					ast.NewSel(&ast.Ident{Name: "list", Node: spec}, "Min")),
			},
		}},
		want: "import listx \"list\", list: listx.Min()",
	}}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			var r Runtime
			inst, err := r.CompileFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}

			ctx := r.index().newContext()

			n, _ := export(ctx, inst, inst.rootStruct, options{
				raw: true,
			})
			got := internal.DebugStr(n)
			assert.Equal(t, got, tc.want)
		})
	}
}

func mustParseExpr(expr string) ast.Expr {
	ex, err := parser.ParseExpr("cue", expr)
	if err != nil {
		panic(err)
	}
	return ex
}
