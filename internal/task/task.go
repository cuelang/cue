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

// Package task provides a registry for tasks to be used by commands.
package task

import (
	"context"
	"io"
	"sync"

	"cuelang.org/go/cue"
)

// A Context provides context for running a task.
type Context struct {
	Context context.Context
	Stdout  io.Writer
	Stderr  io.Writer
}

// A RunnerBuilder creates a Runner.
type RunnerBuilder func(v cue.Value) (Runner, error)

// RunnerFunc defines a function as Runner
type RunnerFunc func(ctx *Context, v cue.Value) (results interface{}, err error)

// Run implements the Runner interface
func (r RunnerFunc) Run(ctx *Context, v cue.Value) (results interface{}, err error) {
	return r(ctx, v)
}

// A Runner defines a command type.
type Runner interface {
	// Init is called with the original configuration before any task is run.
	// As a result, the configuration may be incomplete, but allows some
	// validation before tasks are kicked off.
	// Init(v cue.Value)

	// Runner runs given the current value and returns a new value which is to
	// be unified with the original result.
	Run(ctx *Context, v cue.Value) (results interface{}, err error)
}

// Register registers a task for cue commands.
func Register(key string, f RunnerBuilder) {
	runners.Store(key, f)
}

// Lookup returns the RunnerBuilder for a key.
func Lookup(key string) RunnerBuilder {
	v, ok := runners.Load(key)
	if !ok {
		return nil
	}
	return v.(RunnerBuilder)
}

var runners sync.Map
