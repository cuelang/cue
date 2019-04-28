// Copyright 2019 The CUE Authors
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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue/format"
	"cuelang.org/go/internal/protobuf"
	"github.com/spf13/cobra"
)

// File extensions
// Default     Legacy           Description
// .proto                       Protocol Description
// .textproto  .textpb .pbtxt   Text protocol buffer

// Modes:
// - inline: Generate files along the source directories.
// - get:    Generate files within package directories.
// Right now, online inline mode is supported.

var getProtoCmd = &cobra.Command{
	Use:   "proto",
	Short: "convert proto definitions to CUE",
	Long: `
`,
	RunE: runGetProto,
}

func init() {
	getCmd.AddCommand(getProtoCmd)

	protoPaths = getProtoCmd.Flags().StringArrayP("proto_path", "I", nil, "paths in which to search for imports")

	includeImports = getProtoCmd.Flags().Bool("include_imports", false, "make descriptor self-contained")
}

var (
	protoPaths     *[]string
	includeImports *bool
)

func runGetProto(cmd *cobra.Command, args []string) (err error) {
	hasPkg := len(args) == 0
	hasProto := false
	for _, f := range args {
		if strings.HasSuffix(f, ".proto") {
			hasProto = true
		} else {
			hasPkg = true
		}
	}

	if hasPkg && hasProto {
		return fmt.Errorf("mix of package and proto files")
	}

	if hasPkg {
		return processPackages(cmd, args)
	} else {
		return processFiles(cmd, args)
	}

}

func processPackages(cmd *cobra.Command, args []string) error {
	for _, inst := range buildFromArgs(cmd, args) {
		f, err := ioutil.ReadDir(inst.Dir)
		if err != nil {
			return err
		}
		files := []string{}
		for _, f := range f {
			if strings.HasSuffix(f.Name(), ".proto") {
				files = append(files, filepath.Join(inst.Dir, f.Name()))
			}
		}
		processFiles(cmd, files)
	}
	return nil
}

func processFiles(cmd *cobra.Command, files []string) error {
	for _, f := range files {
		err := parseProtoDef(&protobuf.Config{
			Filename: f,
			Paths:    *protoPaths,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func parseProtoDef(c *protobuf.Config) (err error) {
	file, err := protobuf.ParseDefinitions(c)
	if err != nil {
		return err
	}

	name := c.Filename
	name = name[:len(name)-len(".proto")]
	name += "_proto_gen.cue"

	w, err := os.Create(name)
	if err != nil {
		return err
	}
	defer func() {
		e := w.Close()
		if err != nil {
			err = e
		}
	}()

	return format.Node(w, file)
}
