package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"cuelang.org/go/cue"
	"golang.org/x/xerrors"
)

type CustomTask struct {
	bin          string
	args         []string
	doc          string
	inputFormat  string
	outputFormat string
}

func NewCustomTask(bin string, args []string, doc string) (*CustomTask, error) {
	return &CustomTask{
		bin:  bin,
		args: args,
		doc:  doc,
	}, nil
}

func (t *CustomTask) Run(ctx *Context, v cue.Value) (results interface{}, err error) {
	cmd := exec.CommandContext(ctx.Context, t.bin, t.args...)

	get := func(name string) (f cue.Value, ok bool) {
		c := v.Lookup(name)
		// Although the schema defines a default versions, older implementations
		// may not use it yet.
		if !c.Exists() {
			return
		}
		if err := c.Null(); err == nil {
			return
		}
		return c, true
	}
	if input, ok := get("input"); ok {
		stdin, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		cmd.Stdin = bytes.NewBuffer(stdin)
	}
	_, captureOut := get("output")
	if !captureOut {
		cmd.Stdout = ctx.Stdout
	}
	_, captureErr := get("error")
	if !captureErr {
		cmd.Stderr = ctx.Stderr
	}

	update := map[string]interface{}{}
	if captureOut {
		var stdout []byte
		stdout, err = cmd.Output()

		if err == nil {
			var output interface{}
			err = json.Unmarshal(stdout, &output)
			update["output"] = output
		} else {
			update["output"] = string(stdout)
		}
	} else {
		err = cmd.Run()
	}
	update["success"] = err == nil
	if err != nil {
		if exit := (*exec.ExitError)(nil); xerrors.As(err, &exit) && captureErr {
			update["error"] = string(exit.Stderr)
		} else {
			update = nil
		}
		err = fmt.Errorf("command %q failed: %v", t.doc, err)
	}
	return update, err
}
