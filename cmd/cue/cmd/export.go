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

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/internal/encoding"
	"cuelang.org/go/internal/filetypes"
)

// newExportCmd creates and export command
func newExportCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "output data in a standard format",
		Long: `export evaluates the configuration found in the current
directory and prints the emit value to stdout.

Examples:
Evaluated and emit

	# a single file
	cue export config.cue

	# multiple files: these are combined at the top-level. Order doesn't matter.
	cue export file1.cue foo/file2.cue

	# all files within the "mypkg" package: this includes all files in the
	# current directory and its ancestor directories that are marked with the
	# same package.
	cue export -p cloud

	# the -p flag can be omitted if the directory only contains files for
	# the "mypkg" package.
	cue export

Emit value:
For CUE files, the generated configuration is derived from the top-level
single expression, the emit value. For example, the file

	// config.cue
	arg1: 1
	arg2: "my string"

	{
		a: arg1
		b: arg2
	}

yields the following JSON:

	{
		"a": 1,
		"b", "my string"
	}

In absence of arguments, the current directory is loaded as a package instance.
A package instance for a directory contains all files in the directory and its
ancestor directories, up to the module root, belonging to the same package.
If the package is not explicitly defined by the '-p' flag, it must be uniquely
defined by the files in the current directory.


Formats
The following formats are recognized:

json    output as JSON
		Outputs any CUE value.

text    output as raw text
        The evaluated value must be of type string.
`,

		RunE: mkRunE(c, runExport),
	}
	flagMedia.Add(cmd)
	cmd.Flags().Bool(string(flagEscape), false, "use HTML escaping")

	cmd.Flags().StringArrayP(string(flagExpression), "e", nil, "export this expression only")

	cmd.Flags().StringArrayP(string(flagTags), "t", nil,
		"set the value of a tagged field")

	return cmd
}

func runExport(cmd *Command, args []string) error {
	b, err := parseArgs(cmd, args, nil)
	exitOnErr(cmd, err, true)
	w := cmd.OutOrStdout()

	var exprs []ast.Expr
	for _, e := range flagExpression.StringArray(cmd) {
		expr, err := parser.ParseExpr("<expression flag>", e)
		if err != nil {
			return err
		}
		exprs = append(exprs, expr)
	}

	format := flagMedia.String(cmd) + ":-"
	f, err := filetypes.ParseFile(format, filetypes.Export)
	exitOnErr(cmd, err, true)

	cfg := &encoding.Config{
		Out: w,
	}

	enc, err := encoding.NewEncoder(f, cfg)
	exitOnErr(cmd, err, true)
	defer enc.Close()

	for _, inst := range b.instances() {
		if exprs == nil {
			err = enc.Encode(inst, nil)
			exitIfErr(cmd, inst, err, true)
			continue
		}
		for _, e := range exprs {
			v := inst.Eval(e)
			exitIfErr(cmd, inst, v.Err(), true)
			err = enc.Encode(inst, &v)
			exitIfErr(cmd, inst, err, true)
		}
	}
	return nil
}
