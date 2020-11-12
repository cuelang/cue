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

package dep_test

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/internal"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/debug"
	"cuelang.org/go/internal/core/dep"
	"cuelang.org/go/internal/core/eval"
	"cuelang.org/go/internal/core/runtime"
	"cuelang.org/go/internal/cuetxtar"
)

var update = flag.Bool("update", false, "update the test files")

func TestVisit(t *testing.T) {
	test := cuetxtar.TxTarTest{
		Root:   "./testdata",
		Name:   "dependencies",
		Update: *update,
	}

	test.Run(t, func(t *cuetxtar.Test) {
		a := t.ValidInstances()

		inst := cue.Build(a)[0].Value()
		if inst.Err() != nil {
			t.Fatal(inst.Err())
		}

		rx, nx := internal.CoreValue(inst)
		ctxt := eval.NewContext(rx.(*runtime.Runtime), nx.(*adt.Vertex))

		testCases := []struct {
			name string
			root string
			fn   func(*adt.OpContext, *adt.Vertex, dep.VisitFunc)
		}{{
			name: "field",
			root: "a.b",
			fn:   dep.Visit,
		}, {
			name: "all",
			root: "a",
			fn:   dep.VisitAll,
		}}

		for _, tc := range testCases {
			v := inst.LookupPath(cue.ParsePath(tc.root))

			_, nx = internal.CoreValue(v)
			n := nx.(*adt.Vertex)
			w := t.Writer(tc.name)

			t.Run(tc.name, func(sub *testing.T) {
				tc.fn(ctxt, n, func(d dep.Dependency) bool {
					str := cue.MakeValue(ctxt, d.Node).Path().String()
					if i := d.Import(); i != nil {
						path := i.ImportPath.StringValue(ctxt)
						str = fmt.Sprintf("%q.%s", path, str)
					}
					fmt.Fprintln(w, str)
					return true
				})
			})
		}
	})
}

// DO NOT REMOVE: for Testing purposes.
func TestX(t *testing.T) {
	// a and a.b are the fields for which to determine the references.
	in := `
	`

	if strings.TrimSpace(in) == "" {
		t.Skip()
	}

	rt := cue.Runtime{}
	inst, err := rt.Compile("", in)
	if err != nil {
		t.Fatal(err)
	}

	v := inst.Lookup("a")

	rx, nx := internal.CoreValue(v)
	r := rx.(*runtime.Runtime)
	n := nx.(*adt.Vertex)

	ctxt := eval.NewContext(r, n)

	for _, c := range n.Conjuncts {
		str := debug.NodeString(ctxt, c.Expr(), nil)
		t.Log(str)
	}

	deps := []string{}

	dep.VisitAll(ctxt, n, func(d dep.Dependency) bool {
		str := cue.MakeValue(ctxt, d.Node).Path().String()
		if i := d.Import(); i != nil {
			path := i.ImportPath.StringValue(ctxt)
			str = fmt.Sprintf("%q.%s", path, str)
		}
		deps = append(deps, str)
		return true
	})

	t.Error(deps)
}
