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
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal"
	"cuelang.org/go/internal/cli"
)

// Disallow
// - block comments
// - old-style field comprehensions
// - space separator syntax
const syntaxVersion = -1000 + 13

var defaultConfig = &load.Config{
	Context: build.NewContext(
		build.ParseFile(func(name string, src interface{}) (*ast.File, error) {
			return parser.ParseFile(name, src,
				parser.FromVersion(syntaxVersion),
				parser.ParseComments,
			)
		})),
}

var runtime = &cue.Runtime{}

var inTest = false

func mustParseFlags(t *testing.T, cmd *cobra.Command, flags ...string) {
	if err := cmd.ParseFlags(flags); err != nil {
		t.Fatal(err)
	}
}

func exitIfErr(cmd *Command, inst *cue.Instance, err error, fatal bool) {
	exitOnErr(cmd, err, fatal)
}

func getLang() language.Tag {
	loc := os.Getenv("LC_ALL")
	if loc == "" {
		loc = os.Getenv("LANG")
	}
	loc = strings.Split(loc, ".")[0]
	return language.Make(loc)
}

func exitOnErr(cmd *Command, err error, fatal bool) {
	if err == nil {
		return
	}

	// Link x/text as our localizer.
	p := message.NewPrinter(getLang())
	format := func(w io.Writer, format string, args ...interface{}) {
		p.Fprintf(w, format, args...)
	}

	cwd, _ := os.Getwd()

	w := &bytes.Buffer{}
	errors.Print(w, err, &errors.Config{
		Format:  format,
		Cwd:     cwd,
		ToSlash: inTest,
	})

	b := w.Bytes()
	_, _ = cmd.Stderr().Write(b)
	if fatal {
		exit()
	}
}

func buildFromArgs(cmd *Command, args []string) []*cue.Instance {
	binst := loadFromArgs(cmd, args, defaultConfig)
	if binst == nil {
		return nil
	}
	decorateInstances(cmd, binst)
	return buildInstances(cmd, binst)
}

// A tag binds an identifier to a field to allow passing
// command-line values.
//
// A tag is of the form
//     @tag(name,[type[,shorthand{"|"shorthand}]])
//
// Both the name and type are mandatory. Tags are set using the -t option on
// the command line. -t name=value will parse value for the type defined for
// name and set the field for which this tag was defined to this value.
// A tag may be associated with multiple fields.
//
// Tags also allow shorthands. If a shorthand bar is declared for a tag with
// name foo, then -t bar is identical to -t foo=bar.
//
// It is a deliberate choice to not allow alternative values to be associated
// with shorthands. Doing so would create a powerful mechanism that would assign
// different values to different fields based on the same shorthand, duplicating
// functionality that is already available in CUE.
type tag struct {
	key        string
	kind       cue.Kind
	shorthands []string

	field *ast.Field
}

func parseTag(pos token.Pos, body string) (t tag, err errors.Error) {
	t.kind = cue.StringKind

	a := internal.ParseAttrBody(pos, body)

	t.key, _ = a.String(0)
	if !ast.IsValidIdent(t.key) {
		return t, errors.Newf(pos, "invalid identifier %q", t.key)
	}

	if s, ok, _ := a.Lookup(1, "type"); ok {
		switch s {
		case "string":
		case "int":
			t.kind = cue.IntKind
		case "number":
			t.kind = cue.NumberKind
		case "bool":
			t.kind = cue.BoolKind
		default:
			return t, errors.Newf(pos, "invalid type %q", s)
		}
	}

	if s, ok, _ := a.Lookup(1, "short"); ok {
		for _, s := range strings.Split(s, "|") {
			if !ast.IsValidIdent(t.key) {
				return t, errors.Newf(pos, "invalid identifier %q", s)
			}
			t.shorthands = append(t.shorthands, s)
		}
	}

	return t, nil
}

func (t *tag) inject(value string) errors.Error {
	e, err := cli.ParseValue(token.NoPos, t.key, value, t.kind)
	if err != nil {
		return err
	}
	t.field.Value = ast.NewBinExpr(token.AND, t.field.Value, e)
	return nil
}

// findTags defines which fields may be associated with tags.
//
// TODO: should we limit the depth at which tags may occur?
func findTags(b *build.Instance) (tags []tag, errs errors.Error) {
	for _, f := range b.Files {
		ast.Walk(f, func(n ast.Node) bool {
			if b.Err != nil {
				return false
			}

			switch x := n.(type) {
			case *ast.StructLit, *ast.File:
				return true

			case *ast.Field:
				// TODO: allow optional fields?
				_, _, err := ast.LabelName(x.Label)
				if err != nil || x.Optional != token.NoPos {
					return false
				}

				for _, a := range x.Attrs {
					key, body := a.Split()
					if key != "tag" {
						continue
					}
					t, err := parseTag(a.Pos(), body)
					if err != nil {
						errs = errors.Append(errs, err)
						continue
					}
					t.field = x
					tags = append(tags, t)
				}
				return true
			}
			return false
		}, nil)
	}
	return tags, errs
}

func injectTags(tags []string, b []*build.Instance) errors.Error {
	var a []tag
	for _, p := range b {
		// fmt.Println("XXXX", p.Files)
		x, err := findTags(p)
		if err != nil {
			return err
		}
		a = append(a, x...)
	}

	// Parses command line args
	for _, s := range tags {
		p := strings.Index(s, "=")
		found := false
		if p > 0 { // key-value
			for _, t := range a {
				if t.key == s[:p] {
					found = true
					if err := t.inject(s[p+1:]); err != nil {
						return err
					}
				}
			}
			if !found {
				return errors.Newf(token.NoPos, "no tag for %q", s[:p])
			}
		} else { // shorthand
			for _, t := range a {
				for _, sh := range t.shorthands {
					if sh == s {
						found = true
						if err := t.inject(s); err != nil {
							return err
						}
					}
				}
			}
			if !found {
				return errors.Newf(token.NoPos, "no shorthand for %q", s)
			}
		}
	}
	return nil
}

func loadFromArgs(cmd *Command, args []string, cfg *load.Config) []*build.Instance {
	binst := load.Instances(args, cfg)
	if len(binst) == 0 {
		return nil
	}

	return binst
}

func decorateInstances(cmd *Command, a []*build.Instance) {
	tags := flagTags.StringArray(cmd)
	if len(tags) == 0 {
		return
	}
	exitOnErr(cmd, injectTags(tags, a), true)
}

func buildInstances(cmd *Command, binst []*build.Instance) []*cue.Instance {
	// TODO:
	// If there are no files and User is true, then use those?
	// Always use all files in user mode?
	instances := cue.Build(binst)
	for _, inst := range instances {
		// TODO: consider merging errors of multiple files, but ensure
		// duplicates are removed.
		exitIfErr(cmd, inst, inst.Err, true)
	}

	decorateInstances(cmd, binst)

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

func buildToolInstances(cmd *Command, binst []*build.Instance) ([]*cue.Instance, error) {
	instances := cue.Build(binst)
	for _, inst := range instances {
		if inst.Err != nil {
			return nil, inst.Err
		}
	}

	// TODO check errors after the fact in case of ignore.
	for _, inst := range instances {
		if err := inst.Value().Validate(); err != nil {
			return nil, err
		}
	}
	return instances, nil
}

func buildTools(cmd *Command, args []string) (*cue.Instance, error) {
	binst := loadFromArgs(cmd, args, &load.Config{Tools: true})
	if len(binst) == 0 {
		return nil, nil
	}
	included := map[string]bool{}

	ti := binst[0].Context().NewInstance(binst[0].Root, nil)
	for _, inst := range binst {
		for _, f := range inst.ToolCUEFiles {
			if file := inst.Abs(f); !included[file] {
				_ = ti.AddFile(file, nil)
				included[file] = true
			}
		}
	}
	decorateInstances(cmd, append(binst, ti))

	insts, err := buildToolInstances(cmd, binst)
	if err != nil {
		return nil, err
	}

	inst := cue.Merge(insts...).Build(ti)
	return inst, inst.Err
}
