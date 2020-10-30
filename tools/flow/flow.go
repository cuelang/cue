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

// Package flow provides a low-level workflow manager based on a CUE Instance.
//
// A Task defines an operational unit in a Workflow and corresponds to a struct
// in a CUE instance. This package does not define what a Task looks like in a
// CUE Instance. Instead, the user of this package must supply a TaskFunc that
// creates a Runner for cue.Values that are deemed to be a Task.
//
// The Tasks of a WorkFlow may depend on other tasks. Cyclic dependencies are
// thereby not allowed. A Task A depends on another Task B if A, directly or
// indirectly, has a reference to any field of Task B, including its root.
//
// Example:
//   var inst cue.Instance
//
//   // taskFunc takes a Value v and returns a Runner if v is a Task.
//   w := flow.New(inst, taskFunc, nil)
//
//   err := w.Run(context.Background())
//   if err != nil {
//       ...
//   }
//
package flow

// TODO:
// - Should we allow lists as a shorthand for a sequence of tasks?
// - Should we allow tasks to be a child of another task? Currently, the search
//   for tasks end once a task root is found.

import (
	"context"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/internal"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/convert"
)

var (
	// ErrAbort may be returned by a task to avoid processing dependant tasks.
	// This can be used by control nodes to influence execution.
	ErrAbort = errors.New("abort dependant tasks without failure")

	// ErrSkip is used to the Controller whether to find subtasks within another
	// task.
	ErrSkip = errors.New("do not descend to find more tasks")
)

// A TaskFunc creates a Runner for v if v defines a task or reports nil
// otherwise. It reports an error for illformed tasks.
type TaskFunc func(v cue.Value) (Runner, error)

// A Runner executes a Task.
type Runner interface {
	// Run runs a Task. If any of the tasks it depends on returned an error it
	// is passed to this task. It reports an error upon failure.
	//
	// Any results to be returned can be set by calling Fill on the passed task.
	//
	// TODO: what is a good contract for receiving and passing errors and abort.
	//
	// If for a returned error x errors.Is(x, ErrAbort), all dependant tasks
	// will not be run, without this being an error.
	Run(t *Task, err error) error
}

// RunnerFunc
type RunnerFunc func(t *Task) error

// A Config defines options for interpreting an Instance as a Workflow.
type Config struct {
	// Root limits the search for tasks to be within the path indicated to root.
	// For the cue command, this is set to ["command"]. The default value is
	// for all tasks to be root.
	Root cue.Path

	// Allow references outside of Root to be implied tasks. If a reference is
	// outside of Root, resolution will walk up the tree until a node is
	// identified as a Task. This will activate the task and cause the reference
	// to be a dependency on that task.
	AllowNonRoot bool
}

// A Workflow defines a set of Tasks to be executed.
type Workflow struct {
	cfg    Config
	isTask TaskFunc

	inst      *cue.Instance
	env       *adt.Environment
	conjuncts []adt.Conjunct
	taskCh    chan *Task

	nodes   map[*adt.Vertex]int
	opCtx   *adt.OpContext
	context context.Context
	cancel  context.CancelFunc

	// keys maps task keys to their index. This allows a recreation of the
	// Instance while retaining the original task indices.
	//
	// TODO: do instance updating in place to allow for more efficient
	// processing.
	keys  map[string]int
	tasks []*Task

	errs errors.Error
}

func (c *Workflow) addErr(err error, msg string) {
	c.errs = errors.Append(c.errs, errors.Promote(err, msg))
}

// New creates a Workflow for a given Instance and TaskFunc.
func New(inst *cue.Instance, f TaskFunc, cfg *Config) *Workflow {
	c := &Workflow{
		cfg: *cfg,
	}
	a := make([]string, len(c.cfg.Root))
	copy(a, c.cfg.Root)

	c.initTasks()
	return c

}

// Run runs the tasks of a Workflow until completion.
func (c *Workflow) Run(ctx context.Context) error {
	c.runLoop()
	return c.errs
}

type TaskStatus int

const (
	Pending TaskStatus = iota
	Running
	Aborted // is aborted not just success?
	Failed
	Success
)

// A Task contains the context for a single task execution.
type Task struct {
	v      cue.Value
	c      *Workflow
	r      Runner
	key    string
	index  int
	path   cue.Path
	labels []adt.Feature

	update adt.Expr
	err    errors.Error

	subTasks []int
	deps     map[int]bool
	depTasks []*Task
	status   TaskStatus
	// - incoming dependencies, including errors.
}

func (t *Task) done() bool {
	return t.status > Running
}

func (t *Task) vertex() *adt.Vertex {
	_, x := internal.CoreValue(t.v)
	return x.(*adt.Vertex)
}

func (t *Task) addDep(index int) {
	if index < 0 || index == t.index {
		return
	}
	if t.deps == nil {
		t.deps = map[int]bool{}
	}
	t.deps[index] = true
}

// Fill fills in values of the Controller's configuration for the current task.
// The changes take effect after the task completes.
func (t *Task) Fill(x interface{}) error {
	expr := convert.GoValueToExpr(t.c.opCtx, true, x)
	if t.update == nil {
		t.update = expr
		return nil
	}
	t.update = &adt.BinaryExpr{
		Op: adt.AndOp,
		X:  t.update,
		Y:  expr,
	}
	return nil
}

// Dependencies reports the Tasks t depends on.
func (t *Task) Dependencies() []*Task {
	return nil
}

// Err returns the error of a completed Task.
func (t *Task) Err() error {
	return t.err
}

// Path reports the path of Task within the Instance in which it is defined.
func (t *Task) Path() cue.Path {
	return t.path
}

// TODO: how to implement a shell's equivalent of || and &&?
//
// Or: {
// 	#id: "tool.Or"
// 	[...#Task]
// }
//
// task: tool.Or&{
// 	#id: "tool.Far"
//
// }
//
// // #shouldRun: .dependencies.[].success.{@}
// taskList:
// taskList: tool.Or&[
// 	cli.Print&{
//
// 	},
// 	tool.AbortOnError,
// 	cli.Exec&{
// 		#enable: tool.IgnoreError, // PassError, // OnError // AbortOnError
// 	},
// ]
//
// // Or is a sentinel runner that indicates that this value is a list of tasks
// // that need to be run in sequential order upon the first success.
// var Or Runner
//
// // And return
// var And Runner
//
// task: [
// 	cli.Print&{
//
// 	}
// 	tool.Or,
// 	cli.Exec&{
//
// 	}
// 	too.And,
// ]
