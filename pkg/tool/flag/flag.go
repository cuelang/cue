// Copyright 2019 CUE Authors
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

package flag

//go:generate go run gen.go

import (
	"cuelang.org/go/cue"
	// cuerrors is set to avoid including it in cue/builtins.go when generating.
	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/internal/task"
)

func init() {
	task.Register("tool/flag.Set", newSetCmd)
	// task.Register("tool/flag.Print", newPrintCmd)
}

type setCmd struct{}

func newSetCmd(v cue.Value) (task.Runner, error) {
	return &setCmd{}, nil
}

func (c *setCmd) Run(ctx *task.Context, v cue.Value) (res interface{}, err error) {
	m := map[string]interface{}{}
	for iter, _ := v.Fields(); iter.Next(); {
		name := iter.Label()
		f := ctx.Flags.Lookup(name)
		if f == nil {
			return nil, cueerrors.Newf(iter.Value().Pos(), "undefined flag %q", name)
		}
		if f.Changed {
			m[name] = f.Value
		}
	}
	return m, nil
}
