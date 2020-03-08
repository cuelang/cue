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

package encoding

import (
	"testing"

	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/parser"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		name string
		form build.Form
		in   string
		ok   bool
	}{{
		form: "data",
		in: `
		// Foo
		a: 2
		"b-b": 3
		s: -2
		a: +2
		`,
		ok: true,
	}, {
		form: "graph",
		in: `
		X=3
		a: X
		"b-b": 3
		s: a
		`,
		ok: true,
	},

		{form: "data", in: `import "foo" `},
		{form: "data", in: `a: a`},
		{form: "data", in: `a: 1 + 3`},
		{form: "data", in: `a: 1 + 3`},
		{form: "data", in: `a :: 1`},
		{form: "data", in: `a: <1`},
		{form: "data", in: `a: !true`},
		{form: "data", in: `a: 1 | 2`},
		{form: "data", in: `a: 1 | *2`},
		{form: "data", in: `X=3, a: X`},
		{form: "data", in: `2+2`},
		{form: "data", in: `"\(3)"`},
		{form: "data", in: `for x in [2] { a: 2 }`},
		{form: "data", in: `a: len([])`},
		{form: "data", in: `a: [...]`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile("", tc.in, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			d := Decoder{}
			d.validate(f, &build.File{
				Filename: "foo.cue",
				Encoding: build.CUE,
				Form:     tc.form,
			})
			ok := d.err == nil
			if ok != tc.ok {
				t.Errorf("got %v; want %v", ok, tc.ok)
			}
		})
	}
}
