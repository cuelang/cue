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

package eval

import (
	"flag"
	"fmt"
	"testing"

	"cuelang.org/go/cue/internal/compile"
	"cuelang.org/go/cue/internal/debug"
	"cuelang.org/go/cue/internal/runtime"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/internal/cuetxtar"
	"cuelang.org/go/pkg/strings"
)

var (
	update = flag.Bool("update", false, "update the test files")
	todo   = flag.Bool("todo", false, "run tests marked with #todo-compile")
)

func TestEval(t *testing.T) {
	test := cuetxtar.TxTarTest{
		Root:   "../../testdata",
		Name:   "eval",
		Update: *update,
		Skip:   alwaysSkip,
		ToDo:   needFix,
	}

	if *todo {
		test.ToDo = nil
	}

	r := runtime.New()

	test.Run(t, func(t *cuetxtar.Test) {
		a := t.ValidInstances()

		v, err := compile.Files(nil, r, a[0].Files...)
		if err != nil {
			t.Fatal(err)
		}

		e := evaluator{
			r:     r,
			index: r,
		}

		err = e.Eval(v)
		t.WriteErrors(err)

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
	"fulleval/048_dont_pass_incomplete_values_to_builtins": "import",
	"fulleval/050_json_Marshaling_detects_incomplete":      "import",
	"fulleval/051_detectIncompleteYAML":                    "import",
	"fulleval/052_detectIncompleteJSON":                    "import",
	"fulleval/056_issue314":                                "import",
	"resolve/013_custom_validators":                        "import",

	"export/027": "cycle",
	"export/028": "cycle",
	"export/030": "cycle",

	"export/020":                  "builtin",
	"resolve/034_closing_structs": "builtin",
	"resolve/048_builtins":        "builtin",

	"fulleval/027_len_of_incomplete_types": "builtin",

	"fulleval/032_or_builtin_should_not_fail_on_non-concrete_empty_list": "builtin",

	"fulleval/049_alias_reuse_in_nested_scope": "builtin",
	"fulleval/053_issue312":                    "builtin",
}

// TestX is for debugging. Do not delete.
func TestX(t *testing.T) {
	in := `

	`

	if strings.TrimSpace(in) == "" {
		t.Skip()
	}

	file, err := parser.ParseFile("TestX", in)
	if err != nil {
		t.Fatal(err)
	}
	r := runtime.New()

	v, err := compile.Files(nil, r, file)
	if err != nil {
		t.Fatal(err)
	}

	e := evaluator{
		r:     r,
		index: r,
	}

	err = e.Eval(v)
	if err != nil {
		t.Fatal(err)
	}

	t.Error(debug.NodeString(r, v, nil))
}
