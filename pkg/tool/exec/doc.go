// Code generated by cue get go. DO NOT EDIT.

// Package exec defines tasks for running commands.
//
// These are the supported tasks:
//
//     // Run executes the given shell command.
//     Run: {
//     	kind: "tool/exec.Run"
//
//     	// cmd is the command to run.
//     	cmd: string | [string, ...string]
//
//     	// install is an optional command to install the binaries needed
//     	// to run the command.
//     	install?: string | [string, ...string]
//
//     	// env defines the environment variables to use for this system.
//     	env <Key>: string
//
//     	// stdout captures the output from stdout if it is of type bytes or string.
//     	// The default value of null indicates it is redirected to the stdout of the
//     	// current process.
//     	stdout: *null | string | bytes
//
//     	// stderr is like stdout, but for errors.
//     	stderr: *null | string | bytes
//
//     	// stdin specifies the input for the process.
//     	stdin?: string | bytes
//
//     	// success is set to true when the process terminates with with a zero exit
//     	// code or false otherwise. The user can explicitly specify the value
//     	// force a fatal error if the desired success code is not reached.
//     	success: bool
//     }
//
//     // Env collects the environment variables of the current process.
//     Env: {
//     	kind: "tool/exec.Env"
//
//     	env <Name>: string | number
//     }
//
package exec
