// Copyright 2019 CUE Authors
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
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"cuelang.org/go/cue"
)

// Common flags
const (
	flagAll      flagName = "all"
	flagDryrun   flagName = "dryrun"
	flagVerbose  flagName = "verbose"
	flagTrace    flagName = "trace"
	flagForce    flagName = "force"
	flagIgnore   flagName = "ignore"
	flagSimplify flagName = "simplify"
	flagPackage  flagName = "package"
	flagDebug    flagName = "debug"

	flagExpression flagName = "expression"
	flagEscape     flagName = "escape"
	flagGlob       flagName = "name"
	flagRecursive  flagName = "recursive"
	flagType       flagName = "type"
	flagList       flagName = "list"
	flagPath       flagName = "path"
)

var flagMedia = stringFlag{
	name: "out",
	text: "output format (json, yaml or text)",
	def:  "json",
}

var flagOut = stringFlag{
	name:  "out",
	short: "o",
	text:  "alternative output or - for stdout",
}

func addGlobalFlags(f *pflag.FlagSet) {
	f.Bool(string(flagDebug), false,
		"give detailed error info")
	f.Bool(string(flagTrace), false,
		"trace computation")
	f.StringP(string(flagPackage), "p", "",
		"CUE package to evaluate")
	f.BoolP(string(flagSimplify), "s", false,
		"simplify output")
	f.BoolP(string(flagIgnore), "i", false,
		"proceed in the presence of errors")
	f.BoolP(string(flagVerbose), "v", false,
		"print information about progress")
}

type flagName string

func (f flagName) Bool(cmd *Command) bool {
	v, _ := cmd.Flags().GetBool(string(f))
	return v
}

func (f flagName) String(cmd *Command) string {
	v, _ := cmd.Flags().GetString(string(f))
	return v
}

func (f flagName) StringArray(cmd *Command) []string {
	v, _ := cmd.Flags().GetStringArray(string(f))
	return v
}

type stringFlag struct {
	name  string
	short string
	text  string
	def   string
}

func (f *stringFlag) Add(cmd *cobra.Command) {
	cmd.Flags().StringP(f.name, f.short, f.def, f.text)
}

func (f *stringFlag) String(cmd *Command) string {
	v, err := cmd.Flags().GetString(f.name)
	if err != nil {
		return f.def
	}
	return v
}

func (c *commandIndex) addFlags(sub *cobra.Command, v cue.Value) {
	id := v.Lookup("$id")
	if !id.Exists() {
		for iter, _ := v.Fields(); iter.Next(); {
			c.addFlags(sub, iter.Value())
		}
		return
	}

	if kind, _ := id.String(); kind != "tool/flag.Set" {
		return
	}

	shorthands := map[string]string{}
	for iter, _ := v.Fields(); iter.Next(); {
		name := iter.Label()
		if utf8.RuneCountInString(name) != 1 {
			continue
		}
		v := iter.Value()
		_, path := v.Reference()
		if len(path) == 0 {
			c.errf(v.Pos(), "shorthand %q must refer to other flag in same section")
			continue
		}
		shorthands[path[len(path)-1]] = name
	}

	for iter, _ := v.Fields(); iter.Next(); {
		name := iter.Label()
		if utf8.RuneCountInString(name) == 1 {
			continue
		}

		v := iter.Value()

		doc := ""
		for _, cg := range v.Doc() {
			str := strings.TrimSpace(cg.Text())
			if str == "" {
				continue
			}
			if cg.Doc || doc == "" {
				doc = str
			}
		}

		def, hasDefault := v.Default()

		parseDef := func(x interface{}) {
			if !hasDefault {
				return
			}
			err := def.Decode(x)
			if err != nil {
				c.errf(def.Pos(), "invalid %T default %q: %v", x, def, err)
			}
		}

		short := shorthands[name]

		switch k := v.IncompleteKind(); k {
		case cue.StringKind:
			var d string
			parseDef(&d)
			sub.Flags().StringP(name, short, d, doc)
		case cue.IntKind:
			var d int64
			parseDef(&d)
			sub.Flags().Int64P(name, short, d, doc)
		case cue.FloatKind, cue.NumberKind:
			var d float64
			parseDef(&d)
			sub.Flags().Float64P(name, short, d, doc)
		case cue.BoolKind:
			var d bool
			parseDef(&d)
			sub.Flags().BoolP(name, short, d, doc)
		case cue.ListKind:
			switch e, _ := v.Elem(); e.IncompleteKind() {
			case cue.StringKind, cue.BytesKind:
				var d []string
				parseDef(&d)
				sub.Flags().StringSliceP(name, short, d, doc)

			case cue.IntKind:
				var d []int
				parseDef(&d)
				sub.Flags().IntSliceP(name, short, d, doc)

			case cue.BoolKind:
				var d []bool
				parseDef(&d)
				sub.Flags().BoolSliceP(name, short, d, doc)

			default:
				c.errf(v.Pos(), "unsupported list type %s", k)
			}
		}
	}
}
