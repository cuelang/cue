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

package adt_test

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/txtar"

	"cuelang.org/go/cue"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/debug"
	"cuelang.org/go/internal/core/eval"
	"cuelang.org/go/internal/core/validate"
	"cuelang.org/go/internal/cuetxtar"
	_ "cuelang.org/go/pkg"
)

var (
	update = flag.Bool("update", false, "update the test files")
	todo   = flag.Bool("todo", false, "run tests marked with #todo-compile")
)

func TestEval(t *testing.T) {
	test := cuetxtar.TxTarTest{
		Root:   "../../../cue/testdata",
		Name:   "eval",
		Update: *update,
		Skip:   alwaysSkip,
		ToDo:   needFix,
	}

	if *todo {
		test.ToDo = nil
	}

	r := cue.NewRuntime()

	test.Run(t, func(t *cuetxtar.Test) {
		a := t.ValidInstances()

		v, err := r.Build(a[0])
		if err != nil {
			t.WriteErrors(err)
			return
		}

		e := eval.New(r)
		ctx := e.NewContext(v)
		v.Finalize(ctx)

		t.Log(e.Stats())

		if b := validate.Validate(ctx, v, &validate.Config{
			AllErrors: true,
		}); b != nil {
			fmt.Fprintln(t, "Errors:")
			t.WriteErrors(b.Err)
			fmt.Fprintln(t, "")
			fmt.Fprintln(t, "Result:")
		}

		if v == nil {
			return
		}

		debug.WriteNode(t, r, v, &debug.Config{Cwd: t.Dir})
		fmt.Fprintln(t)
	})
}

var alwaysSkip = map[string]string{
	"compile/erralias": "compile error",
}

var needFix = map[string]string{
	"DIR/NAME": "reason",
}

// TestX is for debugging. Do not delete.
func TestX(t *testing.T) {
	in := `
-- cue.mod/module.cue --
module: "example.com"

-- in.cue --
// import (
//     "encoding/json"
// )

// jv: json.Valid
// jv: "3"

// #list12: {
// 	tail: #list12 | *null
// 	if tail != null {
// 	}
// }

// a: b - 100
// b: a + 100

// c: [c[1], c[0]]

// #list: {
// 	tail: #list | *null
// }

// // circularFor: {
// //     #list: {
// //         tail: #list | *null
// //         for x in tail != null {
// //         }
// //     }
// // }

// // // // // Print a bit more sensible error message than "empty disjunction" here.
// // // // // Issue #465
// // userError: {
// //     a: string | *_|_
// //     if a != "" {
// //     }
// // }

// z1: z2 + 1
// z2: z3 + 2
// z3: z1 - 3
// z3: 8

// ref: {
// 	a: {
// 		x: y + "?"
// 		y: x + "!"
// 	}
// 	a: x: "hey"
// }

// disCycle: {
// 	a: b & {x: 1} | {y: 1}
// 	b: {x: 2} | a & {z: 2}
// }

// b3: =~"[a-z]{4}"
// b3: "foo"

// condition: *true | false
// conditional: {
//     if condition {
//         a: 3
//     }
// }

// x: y + 100
// y: x - 100
// x: 200

// cell3: a:  0 | 1
// cell3: a:  != cell3.b

// cell3: b:  0 | 1
// cell3: b:  != cell3.a

// cell3: a:  0
// cell3: b:  _

// a0: X
// a1: a0 * 2
// Y: a1

// b0: Y
// b1: b0 / 2
// X: b1

// X: 5.0

// res: [ for x in a for y in x {y & {d: "b"}}]
// res: [ a.b.c & {d: "b"}]

// a: b: [C=string]: {d: string, s: "a" + d}
// a: b: c: d: string

// a: { b: 2, c: int }

// incomplete:  {
// 	if a.d {
// 		2
// 	}
// }

// incomplete:  {
//     list: [1, 2, 3]
// 	for x in list if a.d {
// 		x
// 	}
// }

// a: { x: 10 }
// b: {
// 	for k, v in a {
// 		"\(k)": v
// 	}
// 	x: int
// 	if x > 3 {
// 		k: 20
// 	}
// }

// #nonEmptyRange: {
//     min: *1 | int
//     min: <max
//     max: >min
// }

a1: *0 | 1
a1: a3 - a2
a2: *0 | 1
a2: a3 - a1
a3: 1
	`
	t.Skip()

	if strings.HasSuffix(strings.TrimSpace(in), ".cue --") {
		t.Skip()
	}

	a := txtar.Parse([]byte(in))
	instance := cuetxtar.Load(a, "/tmp/test")[0]
	if instance.Err != nil {
		t.Fatal(instance.Err)
	}

	r := cue.NewRuntime()

	v, err := r.Build(instance)
	if err != nil {
		t.Fatal(err)
	}

	// t.Error(debug.NodeString(r, v, nil))
	// eval.Debug = true

	adt.Verbosity = 1
	e := eval.New(r)
	ctx := e.NewContext(v)
	v.Finalize(ctx)
	adt.Verbosity = 0

	t.Error(debug.NodeString(r, v, nil))

	t.Log(ctx.Stats())
}
