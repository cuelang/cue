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
	"io/ioutil"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"github.com/spf13/cobra"
)

// NewCommand initialize the root command with custom commands defined
// in target CUE file
func NewCommand() *cobra.Command {
	var err error
	root := &cobra.Command{
		Use:   "cue-cmd",
		Short: "cue-cmd execute custom command defined in target file",
		Long: `cue-cmd execute custom command defined in target file.

Commands are defined in CUE as follows:

	command deploy: {
		cmd:   "kubectl"
		args:  [ "-f", "deploy" ]
		in:    json.Encode($) // encode the emitted configuration.
	}

Add a shebang to a CUE file pointing to cue-cmd could make it executable.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	defer func(root *cobra.Command) {
		if err != nil {
			root.RunE = func(cmd *cobra.Command, args []string) error {
				return err
			}
		}
	}(root)
	if len(os.Args) == 1 {
		return root
	}

	p := os.Args[1]
	if p == "-h" || p == "--help" {
		return root
	}

	root.SetArgs(os.Args[2:])

	var inst *cue.Instance
	inst, err = buildFromFile(p)
	if err != nil {
		return root
	}
	commands := inst.Lookup("command")
	if !commands.Exists() {
		return root
	}
	i, err := commands.Fields()
	if err != nil {
		err = errors.Newf(token.NoPos, "could not create command definitions: %v", err)
		return root
	}
	for i.Next() {
		_, err = addCustom(root, "command", i.Label(), inst)
		if err != nil {
			return root
		}
	}
	return root
}

func buildFromFile(filename string) (*cue.Instance, error) {
	ti := build.NewContext(build.ParseFile(shebangParser)).NewInstance("", nil)

	err := ti.AddFile(filename, nil)
	if err != nil {
		return nil, err
	}
	inst := cue.Build([]*build.Instance{ti})[0]
	return inst, inst.Err
}

func shebangParser(filename string, src interface{}) (*ast.File, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if len(content) > 2 && content[0] == '#' && content[1] == '!' {
		if i := bytes.Index(content, []byte("\n")); i > 0 {
			return parser.ParseFile(filename, content[i+1:])
		}
	}
	return parser.ParseFile(filename, content)
}
