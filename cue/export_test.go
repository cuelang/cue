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
	"log"
	"strings"
	"testing"

	"cuelang.org/go/cue/format"
)

func TestExport(t *testing.T) {
	testCases := []struct {
		raw     bool
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
		in: `{ a: { b: 2.0, s: "abc" }, b: a.b, c: a.c, d: a["d"], e: a.t[2:3] }`,
		out: unindent(`
			{
				a: {
					b: 2.0
					s: "abc"
				}
				b: 2.0
				c: _|_ // undefined field "c"
				d: _|_ // undefined field "d"
				e: _|_ // undefined field "t"
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
				a: 5*[int] & [1, 2, ...int]
				b: (>=2 & <=5)*[int] & [1, 2, ...int]
				c: (<=5 & >=3)*[int] & [1, 2, ...int]
				d: [1, 2, ...int]
				e: [1, 2, ...int]
				f: [1, 2, ...]
			}`),
	}, {
		in: `{
			a: >=0*[int]
			a: [...int]
		}`,
		out: unindent(`
			{
				a: [...int]
			}`),
	}, {
		raw: true,
		in:  `{ a: { b: [] }, c: a.b, d: a["b"] }`,
		out: unindent(`
			{
				a b: []
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
		raw: true,
		in:  `{ a: [1, 2], b: { "\(k)": v for k, v in a if a > 1 } }`,
		out: unindent(`
			{
				a: [1, 2]
				b: {
					"\(k)": v for k, v in a if a > 1
				}
			}`),
	}, {
		raw: true,
		in:  `{ a: [1, 2], b: [ v for k, v in a ] }`,
		out: unindent(`
			{
				a: [1, 2]
				b: [ v for k, v in a ]
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

			buf := &bytes.Buffer{}
			err := format.Node(buf, export(ctx, v.eval(ctx)))
			if err != nil {
				log.Fatal(err)
			}
			if got := buf.String(); got != tc.out {
				t.Errorf("\ngot  %v;\nwant %v", got, tc.out)
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
