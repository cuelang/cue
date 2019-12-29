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
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"github.com/spf13/cobra"
)

func newFmtCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fmt [-s] [packages]",
		Short: "formats CUE configuration files",
		Long:  `Fmt formats the given files or the files for the given packages in place`,
		RunE:  mkRunE(c, doFmt),
	}
	cmd.Flags().BoolP("list", "l", false, "List, instead of fixing, files that are not formatted correctly")
	return cmd
}

func doFmt(cmd *Command, args []string) error {
	listMode, _ := cmd.Flags().GetBool("list")
	errorCount := 0

	for _, inst := range load.Instances(args, &load.Config{
		Tests: true,
		Tools: true,
	}) {
		if inst.Err != nil {
			exitOnErr(cmd, inst.Err, false)
			continue
		}
		all := []string{}
		all = append(all, inst.CUEFiles...)
		all = append(all, inst.ToolCUEFiles...)
		all = append(all, inst.TestCUEFiles...)

		for _, path := range all {
			fullpath := inst.Abs(path)

			stat, err := os.Stat(fullpath)
			if err != nil {
				return err
			}

			b, err := ioutil.ReadFile(fullpath)
			if err != nil {
				return err
			}

			opts := []format.Option{}
			if flagSimplify.Bool(cmd) {
				opts = append(opts, format.Simplify())
			}

			f, err := parser.ParseFile(fullpath, b, parser.ParseComments)
			if err != nil {
				return err
			}
			n := fix(f)

			b2, err := format.Node(n, opts...)
			if err != nil {
				return err
			}

			if !bytes.Equal(b, b2) {
				errorCount++

				if listMode {
					fmt.Println(path, ": Not formatted correctly")
					continue
				}
			}

			err = ioutil.WriteFile(fullpath, b2, stat.Mode())
			if err != nil {
				return err
			}
		}
	}

	if listMode && errorCount > 0 {
		return errors.New("Not all Cue files are formatted correctly (--list provided)")
	}

	return nil
}
