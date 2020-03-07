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
	"github.com/spf13/cobra"

	"cuelang.org/go/internal/encoding"
	"cuelang.org/go/internal/filetypes"
)

// newDefCmd creates a new eval command
func newDefCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "def",
		Short: "print consolidated definitions",
		Long: `def prints consolidated configuration as a single file.

Printing is skipped if validation fails.

The --expression flag is used to only print parts of a configuration.
`,
		RunE: mkRunE(c, runDef),
	}

	addOutFlags(cmd.Flags(), true)
	addOrphanFlags(cmd.Flags())

	cmd.Flags().StringArrayP(string(flagExpression), "e", nil, "evaluate this expression only")

	cmd.Flags().BoolP(string(flagAttributes), "A", false,
		"display field attributes")

	cmd.Flags().StringArrayP(string(flagTags), "t", nil,
		"set the value of a tagged field")

	// TODO: Option to include comments in output.
	return cmd
}

func runDef(cmd *Command, args []string) error {
	b, err := parseArgs(cmd, args, nil)
	exitOnErr(cmd, err, true)

	b.encConfig.Mode = filetypes.Def

	f, err := b.out("-", filetypes.Def)
	exitOnErr(cmd, err, true)

	e, err := encoding.NewEncoder(f, b.encConfig)
	exitOnErr(cmd, err, true)

	iter := b.instances()
	defer iter.close()
	for i := 0; iter.scan(); i++ {
		if f := iter.file(); f != nil {
			err := e.EncodeFile(f)
			exitOnErr(cmd, err, true)
		} else {
			err := e.Encode(iter.instance())
			exitOnErr(cmd, err, true)
		}
	}
	exitOnErr(cmd, iter.err(), true)
	return nil
}
