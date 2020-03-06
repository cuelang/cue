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

package openapi_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kylelemons/godebug/diff"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/encoding/openapi"
)

var update *bool = flag.Bool("update", false, "update the test output")

func TestParseDefinitions(t *testing.T) {
	info := *(*openapi.OrderedMap)(ast.NewStruct(
		"title", ast.NewString("test"),
		"version", ast.NewString("v1"),
	))
	defaultConfig := &openapi.Config{}
	resolveRefs := &openapi.Config{Info: info, ExpandReferences: true}

	testCases := []struct {
		in, out string
		config  *openapi.Config
	}{{
		"structural.cue",
		"structural.json",
		resolveRefs,
	}, {
		"simple.cue",
		"simple.json",
		resolveRefs,
	}, {
		"simple.cue",
		"simple-filter.json",
		&openapi.Config{Info: info, FieldFilter: "min.*|max.*"},
	}, {
		"array.cue",
		"array.json",
		defaultConfig,
	}, {
		"struct.cue",
		"struct.json",
		defaultConfig,
	}, {
		"strings.cue",
		"strings.json",
		defaultConfig,
	}, {
		"nums.cue",
		"nums.json",
		defaultConfig,
	}, {
		"builtins.cue",
		"builtins.json",
		defaultConfig,
	}, {
		"oneof.cue",
		"oneof.json",
		defaultConfig,
	}, {
		"oneof.cue",
		"oneof-resolve.json",
		resolveRefs,
	}, {
		"openapi.cue",
		"openapi.json",
		defaultConfig,
	}, {
		"openapi.cue",
		"openapi-norefs.json",
		resolveRefs,
	}, {
		"oneof.cue",
		"oneof-funcs.json",
		&openapi.Config{
			Info: info,
			ReferenceFunc: func(inst *cue.Instance, path []string) string {
				return strings.ToUpper(strings.Join(path, "_"))
			},
			DescriptionFunc: func(v cue.Value) string {
				return "Randomly picked description from a set of size one."
			},
		},
	}, {
		"refs.cue",
		"refs.json",
		&openapi.Config{
			Info: info,
			ReferenceFunc: func(inst *cue.Instance, path []string) string {
				switch {
				case strings.HasPrefix(path[0], "Excluded"):
					return ""
				}
				return strings.Join(path, ".")
			},
		},
	}, {
		"issue131.cue",
		"issue131.json",
		&openapi.Config{Info: info, SelfContained: true},
	}}
	for _, tc := range testCases {
		t.Run(tc.out, func(t *testing.T) {
			filename := filepath.FromSlash(tc.in)

			inst := cue.Build(load.Instances([]string{filename}, &load.Config{
				Dir: "./testdata",
			}))[0]
			if inst.Err != nil {
				t.Fatal(errors.Details(inst.Err, nil))
			}

			b, err := openapi.Gen(inst, tc.config)
			if err != nil {
				t.Fatal(err)
			}
			var out = &bytes.Buffer{}
			_ = json.Indent(out, b, "", "   ")

			wantFile := filepath.Join("testdata", tc.out)
			if *update {
				_ = ioutil.WriteFile(wantFile, out.Bytes(), 0644)
				return
			}

			b, err = ioutil.ReadFile(wantFile)
			if err != nil {
				t.Fatal(err)
			}

			if d := diff.Diff(out.String(), string(b)); d != "" {
				t.Errorf("files differ:\n%v", d)
			}
		})
	}
}

// This is for debugging purposes. Do not remove.
func TestX(t *testing.T) {
	t.Skip()

	var r cue.Runtime
	inst, err := r.Compile("test", `
	AnyField: "any value"
	`)
	if err != nil {
		t.Fatal(err)
	}

	b, err := openapi.Gen(inst, &openapi.Config{
		ExpandReferences: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var out = &bytes.Buffer{}
	_ = json.Indent(out, b, "", "   ")
	t.Error(out.String())
}
