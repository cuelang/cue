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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
)

// TODO: commands
//   fix:      rewrite/refactor configuration files
//             -i interactive: open diff and ask to update
//   serve:    like cmd, but for servers
//   get:      convert cue from other languages, like proto and go.
//   gen:      generate files for other languages
//   generate  like go generate (also convert cue to go doc)
//   test      load and fully evaluate test files.
//
// TODO: documentation of concepts
//   tasks     the key element for cmd, serve, and fix

type runFunction func(cmd *Command, args []string) error

func mkRunE(c *Command, f runFunction) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		c.Command = cmd
		err := f(c, args)
		if err != nil {
			exitOnErr(c, err, true)
		}
		return err
	}
}

// newRootCmd creates the base command when called without any subcommands
func newRootCmd() *Command {
	cmd := &cobra.Command{
		Use:   "cue",
		Short: "cue emits configuration files to user-defined commands.",
		Long: `cue evaluates CUE files, an extension of JSON, and sends them
to user-defined commands for processing.

Commands are defined in CUE as follows:

	import "tool/exec"
	command: deploy: {
		exec.Run
		cmd:   "kubectl"
		args:  [ "-f", "deploy" ]
		in:    json.Encode(userValue) // encode the emitted configuration.
	}

cue can also combine the results of http or grpc request with the input
configuration for further processing. For more information on defining commands
run 'cue help cmd' or go to cuelang.org/pkg/cmd.

For more information on writing CUE configuration files see cuelang.org.`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		//	Run: func(cmd *cobra.Command, args []string) { },

		SilenceUsage: true,
	}

	c := &Command{Command: cmd, root: cmd}

	cmdCmd := newCmdCmd(c)
	c.cmd = cmdCmd

	subCommands := []*cobra.Command{
		cmdCmd,
		newEvalCmd(c),
		newExportCmd(c),
		newFmtCmd(c),
		newGetCmd(c),
		newImportCmd(c),
		newModCmd(c),
		newTrimCmd(c),
		newVersionCmd(c),
		newVetCmd(c),

		// Hidden
		newAddCmd(c),
	}

	addGlobalFlags(cmd.PersistentFlags())

	for _, sub := range subCommands {
		cmd.AddCommand(sub)
	}

	return c
}

// MainTest is like Main, runs the cue tool and returns the code for passing to os.Exit.
func MainTest() int {
	inTest = true
	return Main()
}

// Main runs the cue tool and returns the code for passing to os.Exit.
func Main() int {
	err := mainErr(context.Background(), os.Args[1:])
	if err != nil {
		if err != ErrPrintedError {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	return 0
}

func mainErr(ctx context.Context, args []string) error {
	cmd, err := New(args)
	if err != nil {
		return err
	}
	err = cmd.Run(ctx)
	// TODO: remove this ugly hack. Either fix Cobra or use something else.
	stdin = nil
	return err
}

type Command struct {
	// The currently active command.
	*cobra.Command

	root *cobra.Command

	// Subcommands
	cmd *cobra.Command

	hasErr bool
}

type errWriter Command

func (w *errWriter) Write(b []byte) (int, error) {
	c := (*Command)(w)
	c.hasErr = true
	return c.Command.OutOrStderr().Write(b)
}

// Hint: search for uses of OutOrStderr other than the one here to see
// which output does not trigger a non-zero exit code. os.Stderr may never
// be used directly.

// Stderr returns a writer that should be used for error messages.
func (c *Command) Stderr() io.Writer {
	return (*errWriter)(c)
}

// TODO: add something similar for Stdout. The output model of Cobra isn't
// entirely clear, and such a change seems non-trivial.

// Consider overriding these methods from Cobra using OutOrStdErr.
// We don't use them currently, but may be safer to block. Having them
// will encourage their usage, and the naming is inconsistent with other CUE APIs.
// PrintErrf(format string, args ...interface{})
// PrintErrln(args ...interface{})
// PrintErr(args ...interface{})

func (c *Command) SetOutput(w io.Writer) {
	c.root.SetOutput(w)
}

func (c *Command) SetInput(r io.Reader) {
	// TODO: ugly hack. Cobra does not have a way to pass the stdin.
	stdin = r
}

// ErrPrintedError indicates error messages have been printed to stderr.
var ErrPrintedError = errors.New("terminating because of errors")

func (c *Command) Run(ctx context.Context) (err error) {
	// Three categories of commands:
	// - normal
	// - user defined
	// - help
	// For the latter two, we need to use the default loading.
	defer recoverError(&err)

	if err := c.root.Execute(); err != nil {
		return err
	}
	if c.hasErr {
		return ErrPrintedError
	}
	return nil
}

func recoverError(err *error) {
	switch e := recover().(type) {
	case nil:
	case panicError:
		*err = e.Err
	default:
		panic(e)
	}
	// We use panic to escape, instead of os.Exit
}

func New(args []string) (cmd *Command, err error) {
	defer recoverError(&err)

	cmd = newRootCmd()
	rootCmd := cmd.root
	rootCmd.SetArgs(args)
	if len(args) == 0 {
		return cmd, nil
	}

	var sub = map[string]*subSpec{
		"cmd": {commandSection, cmd.cmd},
		// "serve": {"server", nil},
		// "fix":   {"fix", nil},
	}

	name := args[0]
	if name == "help" {
		_, err := findCustomCommands(cmd, sub)
		return cmd, err
	}

	if _, ok := sub[name]; ok {
		if len(args) == 1 {
			_, err := findCustomCommands(cmd, sub)
			return cmd, err
		}
		name = args[1]
	} else if c, _, err := rootCmd.Find(args); err == nil && c != nil {
		return cmd, nil
	}

	cmds, err := findCustomCommands(cmd, sub)
	if err != nil {
		return cmd, err
	}

	if _, ok := cmds.index[name]; !ok {
		switch a := cmds.shorthands[name]; len(a) {
		case 0:
			err = errors.Newf(token.NoPos,
				`command %q is not defined
Ensure commands are defined in a "_tool.cue" file.
Run 'cue help cmd' to show available commands.`,
				name,
			)
			return cmd, err

		case 1:
			rootCmd.AddCommand(a[0].cmd)
		default:
			names := []string{}
			for _, c := range a {
				names = append(names, c.key)
			}
			return cmd, errors.Newf(token.NoPos,
				`multiple commands with name %q (%s) defined, qualify with package name`,
				name, strings.Join(names, ", "))
		}
	}

	return cmd, nil
}

// findCustomCommands uses the following algorithm:
// - find module root (or cwd if non-existing)
// - repeat the following steps for each directory up to the root
// - find all files ending with _tool, per package.
// - detect duplicate definitions. If a command is only defined
//   for a single package, it can be used as is.
//   Otherwise the user will have to type pkg/command.
func findCustomCommands(cmd *Command, sub map[string]*subSpec) (*commandIndex, error) {
	// Get the root directory.
	c := &load.Config{Tools: true}
	err := c.Init()
	if err != nil {
		return nil, err
	}

	cf := &commandIndex{
		cmd:        cmd,
		dir:        c.Dir,
		cwd:        c.Dir,
		root:       c.ModuleRoot,
		pkgDone:    map[string]bool{},
		index:      map[string]*commandInfo{},
		shorthands: map[string][]*commandInfo{},
		specs:      sub,
	}

	cf.find()
	errors.Print(os.Stderr, cf.errs, nil)
	return cf, cf.errs
}

type commandIndex struct {
	cmd   *Command
	tools *build.Instance

	root string
	cwd  string
	dir  string
	file string

	commands   []*commandInfo
	pkgDone    map[string]bool
	index      map[string]*commandInfo
	shorthands map[string][]*commandInfo

	specs map[string]*subSpec

	errs errors.Error
}

type subSpec struct {
	name string
	cmd  *cobra.Command
}

type commandInfo struct {
	key     string
	name    string
	pkgName string
	pos     token.Pos
	cmd     *cobra.Command // the corresponding cobra command
}

func (c *commandIndex) addAllToParent(cmd *Command) {
}

func (c *commandIndex) addToCommand(cmd *Command, info *commandInfo, tools *build.Instance) error {
	return nil
}

func (c *commandIndex) errf(pos token.Pos, format string, args ...interface{}) {
	c.errs = errors.Append(c.errs, errors.Newf(pos, format, args...))
}

func (c *commandIndex) find() {
	for {
		c.searchDir()

		parent := filepath.Dir(c.dir)
		if len(parent) < len(c.root) || len(parent) >= len(c.dir) {
			break
		}
		c.dir = parent
	}
}

func (c *commandIndex) searchDir() {
	typ := commandSection

	m, err := filepath.Glob(filepath.Join(c.dir, "*_tool.cue"))
	if err != nil {
		c.errf(token.NoPos, "cannot read directory: %v", err)
		return
	}

	for _, match := range m {
		c.file = match
		// Load AST only.
		f, err := parser.ParseFile(match, nil, parser.PackageClauseOnly)
		if err != nil {
			c.errs = errors.Append(c.errs, errors.Promote(err, "parse error"))
			return
		}
		pkg := f.PackageName()
		var ti *build.Instance
		if pkg == "" || pkg == "_" {
			// TODO: Backwards compatibilty mode. Consider phasing out.
			pkg = "_"
		}
		if pkg == "_" {
			ti = load.Instances([]string{match}, &load.Config{
				Dir:   c.dir,
				Tools: true,
			})[0]
			_ = ti.AddFile(match, nil)
		} else {
			if c.pkgDone[pkg] {
				continue
			}
			c.pkgDone[pkg] = true
			// Load tool files only and allow dangling references.
			bi := load.Instances(nil, &load.Config{
				Dir:     c.dir,
				Tools:   true,
				Package: pkg,
			})[0]
			if bi.Err != nil {
				c.errs = errors.Append(c.errs, bi.Err)
				return
			}
			ti = bi.Context().NewInstance(bi.Root, nil)
			for _, f := range bi.ToolCUEFiles {
				_ = ti.AddFile(bi.Abs(f), nil)
			}
		}
		if ti.Err != nil {
			c.errs = errors.Append(c.errs, ti.Err)
			return
		}
		c.tools = ti

		// Ignore errors: likely resulting from unresolved errors.
		// TODO: only skip unresolved errors.
		inst := cue.Build([]*build.Instance{ti})[0]
		// if inst.Err != nil {
		// 	c.errs = errors.Append(c.errs, inst.Err)
		// 	return
		// }

		if v := inst.Lookup(typ); v.Exists() {
			c.parseSection(c.specs["cmd"], pkg, v)
		}
	}
}

func (c *commandIndex) parseSection(spec *subSpec, pkgName string, v cue.Value) {
	iter, err := v.Fields()
	if err != nil {
		c.errf(v.Pos(), "%s section must be of type struct", spec.name)
		return
	}

	for iter.Next() {
		name := iter.Label()
		v := iter.Value()
		if name == "" || !isCommandName(name) {
			c.errf(v.Pos(), "invalid command name %q", name)
		}

		c.parseCommand(spec, pkgName, name, v)
	}
}

func (c *commandIndex) parseCommand(spec *subSpec, pkgName, name string, v cue.Value) {
	if _, err := v.Fields(); err != nil {
		c.errs = errors.Append(c.errs, errors.Promote(err, "command definition must be struct"))
		return
	}

	path := ""
	key := name
	if pkgName == "_" {
		rel, err := filepath.Rel(c.cwd, c.file)
		if err != nil {
			c.errf(token.NoPos, "%v", err.Error())
		}
		path = rel
	} else {
		rel, err := filepath.Rel(c.cwd, c.dir)
		if err != nil {
			c.errf(token.NoPos, "%v", err.Error())
		}
		path = fmt.Sprintf("%s:%s", rel, pkgName)
		if !strings.HasPrefix(path, ".") {
			path = "./" + path
		}
		key = fmt.Sprintf("%s/%s", pkgName, name)
	}

	if info, ok := c.index[key]; ok {
		c.errf(v.Pos(), "command %q defined in multiple locations", name)
		c.errf(info.pos, "previous definition here")
	}

	tools := load.Instances([]string{path}, &load.Config{Tools: true})[0]

	// TODO: change the way packages are "hooked" in to tool packages:
	//   Make the underlying packages implicitly available through one of the
	//   following mechanisms:
	//		1) a fixed top-level field
	//		2) a top-level field referencing a specific builtin
	//		3) (preferred) a special import statement: import input ".",
	//		   signifying the "current" package under consideration.
	for _, f := range tools.ToolCUEFiles {
		_ = tools.AddFile(tools.Abs(f), nil)
	}
	sub := &cobra.Command{
		RunE: mkRunE(c.cmd, func(cmd *Command, args []string) error {
			inst, err := buildTools(cmd, pkgName, args, tools)

			exitIfErr(cmd, inst, err, true)
			if err != nil {
				return err
			}

			return doTasks(cmd, commandSection, name, inst)
		}),
	}

	info := &commandInfo{
		key:     key,
		name:    name,
		pkgName: pkgName,
		pos:     v.Pos(),
		cmd:     sub,
	}

	c.commands = append(c.commands, info)
	c.index[key] = info
	c.shorthands[name] = append(c.shorthands[name], info)

	docs := v.Doc()
	if len(docs) > 0 {
		sub.Long = docs[0].Text()
		sub.Short, sub.Long = splitLine(sub.Long)
		if strings.HasPrefix(sub.Long, "Usage:") {
			sub.Use, sub.Long = splitLine(sub.Long[len("Usage:"):])
		}
	}
	sub.Use = lookupString(v, "$usage", sub.Use)
	sub.Long = lookupString(v, "$long", sub.Long)
	sub.Short = lookupString(v, "short", sub.Short)
	prefix := name + " "
	if !strings.HasPrefix(sub.Use, prefix) {
		sub.Use = name
	}

	// Note on Security: we add the alias so Cobra can find the command under
	// this name. However, Cobra doesn't enforce that the alias is unique, so we
	// will still have to verify down the line.
	sub.Aliases = []string{name}

	// TODO: piece out flag section from command definition
	// c.addFlags(sub, v)

	fullName := fmt.Sprintf("%s/%s", pkgName, name)
	if !strings.HasPrefix(sub.Use, prefix) {
		sub.Use = fullName
	} else {
		sub.Use = fullName + sub.Use[len(prefix):]
	}

	// add shorthand
	spec.cmd.AddCommand(sub)
}

func isCommandName(s string) bool {
	return !strings.Contains(s, `/\`) && !strings.Contains(s, ".")
}

type panicError struct {
	Err error
}

func exit() {
	panic(panicError{ErrPrintedError})
}
