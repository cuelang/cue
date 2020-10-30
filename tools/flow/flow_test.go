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

package flow_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/internal/cuetxtar"
	"cuelang.org/go/tools/flow"
)

var update = flag.Bool("update", false, "update the test files")

// TestTasks tests the logic that determines which nodes are tasks and what are
// their dependencies.
func TestFlow(t *testing.T) {
	test := cuetxtar.TxTarTest{
		Root:   "./testdata",
		Name:   "run",
		Update: *update,
	}

	test.Run(t, func(t *cuetxtar.Test) {
		a := t.ValidInstances()

		inst := cue.Build(a)[0]
		if inst.Err != nil {
			t.Fatal(inst.Err)
		}

		count := 0

		updateFunc := func(c *flow.Controller, task *flow.Task) error {
			str := mermaidGraph(c)
			step := fmt.Sprintf("t%d", count)
			fmt.Fprintln(t.Writer(step), str)

			if task != nil {
				fmt.Fprintln(t.Writer(path.Join(step, "value")), task.Value())
			}

			count++
			return nil
		}

		cfg := &flow.Config{
			Root:       cue.ParsePath("root"),
			UpdateFunc: updateFunc,
		}
		c := flow.New(cfg, inst, taskFunc)

		w := t.Writer("errors")
		if err := c.Run(context.Background()); err != nil {
			cwd, _ := os.Getwd()
			fmt.Fprint(w, "error: ")
			errors.Print(w, err, &errors.Config{Cwd: cwd})
		}
	})
}

func taskFunc(v cue.Value) (flow.Runner, error) {
	if v.Lookup("$id").Err() != nil {
		return nil, nil
	}
	return flow.RunnerFunc(func(t *flow.Task) error {
		str, _ := t.Value().Lookup("val").String()
		t.Fill(map[string]string{"out": str})
		return nil
	}), nil
}

// mermaidGraph generates a mermaid graph of the current state. This can be
// pasted into https://mermaid-js.github.io/mermaid-live-editor/ for
// visualization.
func mermaidGraph(c *flow.Controller) string {
	w := &strings.Builder{}
	fmt.Fprintln(w, "graph TD")
	for i, t := range c.Tasks() {
		fmt.Fprintf(w, "  t%d(\"%s [%s]\")\n", i, t.Path(), t.State())
		for _, t := range t.Dependencies() {
			fmt.Fprintf(w, "  t%d-->t%d\n", i, t.Index())
		}
	}
	return w.String()
}

// DO NOT REMOVE: for testing purposes.
func TestX(t *testing.T) {
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

	c := flow.New(&flow.Config{
		// Root: cue.ParsePath("root"),
	}, inst, taskFunc)

	t.Error(mermaidGraph(c))

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(errors.Details(err, nil))
	}
}
