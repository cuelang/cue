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

package exec

//go:generate go run gen.go

import (
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/xerrors"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/internal/task"
)

func init() {
	task.Register("tool/exec.Run", newExecCmd)

	// For backwards compatibility.
	task.Register("exec", newExecCmd)
}

type execCmd struct{}

func newExecCmd(v cue.Value) (task.Runner, error) {
	return &execCmd{}, nil
}

func (c *execCmd) Run(ctx *task.Context) (res interface{}, err error) {
	// TODO: set environment variables, if defined.
	var bin string
	var args []string
	doc := ""
	v := ctx.Lookup("cmd")
	if ctx.Err != nil {
		return nil, ctx.Err
	}
	switch v.Kind() {
	case cue.StringKind:
		str := ctx.String("cmd")
		doc = str
		list := strings.Fields(str)
		bin = list[0]
		args = append(args, list[1:]...)

	case cue.ListKind:
		list, _ := v.List()
		if !list.Next() {
			return cue.Value{}, errors.New("empty command list")
		}
		bin, err = list.Value().String()
		if err != nil {
			return cue.Value{}, err
		}
		doc += bin
		for list.Next() {
			str, err := list.Value().String()
			if err != nil {
				return cue.Value{}, err
			}
			args = append(args, str)
			doc += " " + str
		}
	}

	if bin == "" {
		return cue.Value{}, errors.New("empty command")
	}

	cmd := exec.CommandContext(ctx.Context, bin, args...)

	stream := func(name string) (stream cue.Value, ok bool) {
		c := ctx.Obj.Lookup(name)
		// Although the schema defines a default versions, older implementations
		// may not use it yet.
		if !c.Exists() {
			return
		}
		if err := c.Null(); ctx.Err != nil || err == nil {
			return
		}
		return c, true
	}

	if v, ok := stream("stdin"); ok {
		if !v.IsConcrete() {
			cmd.Stdin = ctx.Stdin
		} else if cmd.Stdin, err = v.Reader(); err != nil {
			return nil, fmt.Errorf("cue: %v", err)
		}
	}
	_, captureOut := stream("stdout")
	if !captureOut {
		cmd.Stdout = ctx.Stdout
	}
	_, captureErr := stream("stderr")
	if !captureErr {
		cmd.Stderr = ctx.Stderr
	}

	update := map[string]interface{}{}
	if captureOut {
		var stdout []byte
		stdout, err = cmd.Output()
		update["stdout"] = string(stdout)
	} else {
		err = cmd.Run()
	}
	update["success"] = err == nil
	if err != nil {
		if exit := (*exec.ExitError)(nil); xerrors.As(err, &exit) && captureErr {
			update["stderr"] = string(exit.Stderr)
		} else {
			update = nil
		}
		err = fmt.Errorf("command %q failed: %v", doc, err)
	}
	return update, err
}
