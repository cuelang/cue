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
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// TODO: generate long description from documentation.

func newCmdCmd(c *Command) *cobra.Command {
	return &cobra.Command{
		Use:   "cmd <name> [-x] [instances]",
		Short: "run a user-defined shell command",
		Long: `cmd executes defined the named command for each of the named instances.

Commands define actions on instances. For example, they may specify
how to upload a configuration to Kubernetes. Commands are defined
directly in tool files, which are regular CUE files within the same
package with a filename ending in _tool.cue. These are typically
defined at the top of the module root so that they apply to all
instances.

Each command consists of one or more tasks. A task may load or write
a file, consult a user on the command line, fetch a web page, and
so on. Each task has inputs and outputs. Outputs are typically are
filled out by the task implementation as the task completes.

Inputs of tasks my refer to outputs of other tasks. The cue tool does
a static analysis of the configuration and only starts tasks that are
fully specified. Upon completion of each task, cue rewrites the instance,
filling in the completed task, and reevaluates which other tasks can
now start, and so on until all tasks have completed.

Commands are defined at the top-level of the configuration:

	command: [Name=string]: { // from tool.Command
		// usage gives a short usage pattern of the command.
		// Example:
		//    fmt [-n] [-x] [packages]
		usage?: Name | string

		// short gives a brief on-line description of the command.
		// Example:
		//    reformat package sources
		short?: string

		// long gives a detailed description of the command, including a
		// description of flags usage and examples.
		long?: string

		// A task defines a single action to be run as part of this command.
		// Each task can have inputs and outputs, depending on the type
		// task. The outputs are initially unspecified, but are filled out
		// by the tooling
		task: [string]: { // from "tool".Task
			// supported fields depend on type
		}
	}

Available tasks can be found in the package documentation at

	https://godoc.org/cuelang.org/go/pkg/tool

More on tasks can be found in the tasks topic.

Examples:

A simple file using command line execution:

	$ cat <<EOF > hello_tool.cue
	package foo

	import "tool/exec"

	city: "Amsterdam"

	// Say hello!
	command: hello: {
		// whom to say hello to
		var: who: *"World" | string

		task: print: exec.Run & {
			cmd: "echo Hello \(var.who)! Welcome to \(city)."
		}
	}
	EOF

	$ cue cmd hello
	Hello World! Welcome to Amsterdam.

	$ cue cmd hello -who you  # Setting arguments is not supported yet by cue
	Hello you! Welcome to Amsterdam.


An example using pipes:

	package foo

	import "tool/exec"

	city: "Amsterdam"

	// Say hello!
	command: hello: {
		var: file: "out.txt" | string // save transcript to this file

		task: ask: cli.Ask & {
			prompt:   "What is your name?"
			response: string
		}

		// starts after ask
		task: echo: exec.Run & {
			cmd:    ["echo", "Hello", task.ask.response + "!"]
			stdout: string // capture stdout
		}

		// starts after echo
		task: write: file.Append & {
			filename: var.file
			contents: task.echo.stdout
		}

		// also starts after echo
		task: print: cli.Print & {
			contents: task.echo.stdout
		}
	}

`,
		RunE: mkRunE(c, func(cmd *Command, args []string) error {
			w := cmd.Stderr()
			if len(args) == 0 {
				fmt.Fprintln(w, "cmd must be run as one of its subcommands")
			} else {
				const msg = `cmd must be run as one of its subcommands: unknown subcommand %q
Ensure commands are defined in a "_tool.cue" file.
`
				fmt.Fprintf(w, msg, args[0])
			}
			fmt.Fprintln(w, "Run 'cue help cmd' for known subcommands.")
			os.Exit(1) // TODO: get rid of this
			return nil
		}),
	}
}
