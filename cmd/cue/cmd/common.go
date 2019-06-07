// Copyright 2018 The CUE Authors
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

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"github.com/spf13/cobra"
)

var runtime = &cue.Runtime{}

// printHeader is a hacky and unprincipled way to sanatize the package path.
func printHeader(w io.Writer, cwd, dir string) {
	if cwd != "" {
		if dir == cwd {
			return
		}
		dir = strings.Replace(dir, cwd, ".", 1)
	}
	fmt.Fprintf(w, "--- %s\n", dir)
}

func exitIfErr(cmd *cobra.Command, inst *cue.Instance, err error, fatal bool) {
	exitOnErr(cmd, inst.Dir, err, fatal)
}

func exitOnErr(cmd *cobra.Command, file string, err error, fatal bool) {
	if err == nil {
		return
	}
	cwd := "////"
	if p, _ := os.Getwd(); p != "" {
		cwd = p
	}

	w := &bytes.Buffer{}
	printHeader(w, cwd, file)
	errors.Print(w, err)

	// TODO: do something more principled than this.
	b := w.Bytes()
	b = bytes.ReplaceAll(b, []byte(cwd), []byte("."))
	cmd.OutOrStderr().Write(b)
	if fatal {
		exit()
	}
}

func buildFromArgs(cmd *cobra.Command, args []string) []*cue.Instance {
	binst := loadFromArgs(cmd, args)
	if binst == nil {
		return nil
	}
	return buildInstances(cmd, binst)
}

var (
	config = &load.Config{
		Context: build.NewContext(build.ParseOptions(parser.ParseComments)),
	}
)

func loadFromArgs(cmd *cobra.Command, args []string) []*build.Instance {
	log.SetOutput(cmd.OutOrStderr())
	binst := load.Instances(args, config)
	if len(binst) == 0 {
		return nil
	}
	return binst
}

func buildInstances(cmd *cobra.Command, binst []*build.Instance) []*cue.Instance {
	instances := cue.Build(binst)
	for _, inst := range instances {
		// TODO: consider merging errors of multiple files, but ensure
		// duplicates are removed.
		exitIfErr(cmd, inst, inst.Err, true)
	}

	if flagIgnore.Bool(cmd) {
		return instances
	}

	// TODO check errors after the fact in case of ignore.
	for _, inst := range instances {
		// TODO: consider merging errors of multiple files, but ensure
		// duplicates are removed.
		exitIfErr(cmd, inst, inst.Value().Validate(), !flagIgnore.Bool(cmd))
	}
	return instances
}

func buildTools(cmd *cobra.Command, args []string) *cue.Instance {
	binst := loadFromArgs(cmd, args)
	if len(binst) == 0 {
		return nil
	}

	included := map[string]bool{}

	ti := binst[0].Context().NewInstance(binst[0].Root, nil)
	for _, inst := range binst {
		for _, f := range inst.ToolCUEFiles {
			if file := inst.Abs(f); !included[file] {
				ti.AddFile(file, nil)
				included[file] = true
			}
		}
	}

	inst := cue.Merge(buildInstances(cmd, binst)...).Build(ti)
	exitIfErr(cmd, inst, inst.Err, true)
	return inst
}
