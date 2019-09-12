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

package basics

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/internal/cuetest"
	"github.com/kylelemons/godebug/diff"
)

var update = flag.Bool("update", false, "update test data")

func TestTutorial(t *testing.T) {
	// t.Skip()

	err := filepath.Walk("data", func(path string, f os.FileInfo, err error) error {
		if !f.IsDir() || path == "data" {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			simulateFile(t, path)
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func simulateFile(t *testing.T, path string) {
	file := filepath.Join(path, "cmd.sh")
	b, err := ioutil.ReadFile(file)
	if err != nil {
		t.Skipf("failed to open file %q: %v", path, err)
	}
	command := string(bytes.TrimSpace(b))

	out := &bytes.Buffer{}
	cuetest.Run(t, path, command, &cuetest.Config{Stdout: out})

	file = filepath.Join(path, "out.txt")
	if *update {
		_ = ioutil.WriteFile(file, out.Bytes(), 0644)
		return
	}

	b, err = ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to open file %q: %v", path, err)
	}
	want := strings.TrimSpace(string(b))
	got := strings.TrimSpace(out.String())

	if got != want {
		t.Errorf("files differ:\n%s", diff.Diff(got, want))
	}
}
