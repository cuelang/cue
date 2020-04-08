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
	cmd := &cobra.Command{
		Use:   "cmd <name> [inputs]",
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

Commands are defined at the top-level of the configuration
(from cuelang.org/go/pkg/tool/tool.cue):

	command: [Name]: Command
	
	Command :: {
		// Tasks specifies the things to run to complete a command. Tasks are
		// typically underspecified and completed by the particular internal
		// handler that is running them. Tasks can be a single task, or a full
		// hierarchy of tasks.
		//
		// Tasks that depend on the output of other tasks are run after such tasks.
		// Use $after if a task needs to run after another task but does not
		// otherwise depend on its output.
		Tasks
	
		//
		// Example:
		//     mycmd [-n] names
		$usage?: string
	
		// short is short description of what the command does.
		$short?: string

		// long is a longer description that spans multiple lines and
		// likely contain examples of usage of the command.
		$long?: string
	}

	// Tasks defines a hierarchy of tasks. A command completes if all
	// tasks have run to completion.
	Tasks: Task | {
		[name=Name]: Tasks
	}

	// Name defines a valid task or command name.
	Name :: =~#"^\PL([-](\PL|\PN))*$"#

	// A Task defines a step in the execution of a command.
	Task: {
		$type: "tool.Task" // legacy field 'kind' still supported for now.

		// kind indicates the operation to run. It must be of the form
		// packagePath.Operation.
		$id: =~#"\."#

		// $after can be used to specify a task is run after another one, when
		// it does not otherwise refer to an output of that task.
		$after?: Task | [...Task]
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
	who: *"World" | string @tag(who)

	// Say hello!
	command: hello: {
		print: exec.Run & {
			cmd: "echo Hello \(who)! Welcome to \(city)."
		}
	}
	EOF

	$ cue cmd hello
	Hello World! Welcome to Amsterdam.

	$ cue cmd -t who=Jan hello
	Hello Jan! Welcome to Amsterdam.


An example using pipes:

	package foo

	import (
		"tool/cli"
		"tool/exec"
		"tool/file"
	)

	city: "Amsterdam"

	// Say hello!
	command: hello: {
		// save transcript to this file
		var: file: *"out.txt" | string @tag(file)

		ask: cli.Ask & {
			prompt:   "What is your name?"
			response: string
		}

		// starts after ask
		echo: exec.Run & {
			cmd:    ["echo", "Hello", ask.response + "!"]
			stdout: string // capture stdout
		}

		// starts after echo
		file.Append & {
			filename: var.file
			contents: echo.stdout
		}

		// also starts after echo
		print: cli.Print & {
			contents: echo.stdout
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

	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringArrayP(string(flagInject), "t", nil,
		"set the value of a tagged field")

	return cmd
}
