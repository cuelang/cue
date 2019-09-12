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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/internal/cuetest"
)

func TestTutorial(t *testing.T) {
	// t.Skip()

	err := filepath.Walk(".", func(path string, f os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".md") && !strings.Contains(path, "/") {
			t.Run(path, func(t *testing.T) { simulateFile(t, path) })
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func simulateFile(t *testing.T, path string) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to open file %q: %v", path, err)
	}

	if path == "Readme.md" {
		return
	}
	body := ""
	frontMatter := &bytes.Buffer{}
	fmt.Fprintln(frontMatter, "+++")
	var (
		baseName = path[:len(path)-len(".md")]
		docsDir  string
	)

	defer func() {
		if len(body) == 0 {
			return
		}
		fmt.Fprintln(frontMatter, "+++")
		frontMatter.WriteString(body)

		err = ioutil.WriteFile(
			filepath.Join(docsDir, filepath.Base(path)),
			frontMatter.Bytes(), 0644)

	}()

	dir, err := ioutil.TempDir("", "tutbasics")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dir)

	c := cuetest.NewChunker(t, b)

	c.Find("\n")
	c.Next("_", "_")
	section := c.Text()
	sub := strings.ToLower(strings.Fields(section)[0])
	sub = strings.TrimRight(sub, ",")
	dataDir := filepath.Join("data", baseName)
	_ = os.MkdirAll(dataDir, 0755)
	docsDir = filepath.Join("docs", sub)
	_ = os.MkdirAll(docsDir, 0755)

	ioutil.WriteFile(filepath.Join(docsDir, "_index.md"), []byte(
		fmt.Sprintf(`+++
title = %q
weight = 2000
description = ""
+++`, section),
	), 0644)

	c.Next("# ", "\n")
	fmt.Fprintf(frontMatter, "title = %q\n", c.Text())
	fmt.Fprintf(frontMatter, "weight = 2000\n") // TODO adjust manually.

	inputs := []string{}
	// collect files
	for i := 0; c.Find("<!-- CUE editor -->"); i++ {
		if i == 0 {
			body = c.Text()
		}
		if !c.Next("_", "_") {
			continue
		}
		filename := strings.TrimRight(c.Text(), ":")
		inputs = append(inputs, filename)

		if !c.Next("```", "```") {
			t.Fatalf("No body for filename %q in file %q", filename, path)
		}
		b := bytes.TrimSpace(c.Bytes())

		ioutil.WriteFile(filepath.Join(dir, filename), b, 0644)
		err := ioutil.WriteFile(filepath.Join(dataDir, filename), b, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	key := filepath.Base(baseName)

	if len(inputs) != 1 || inputs[0] != key+".cue" {
		a := strings.Replace(fmt.Sprintf("%q", inputs), " ", ",", -1)
		fmt.Fprintf(frontMatter, "inputs = %s\n", a)
	}

	if !c.Find("<!-- result -->") {
		return
	}

	if !c.Next("`$ ", "`") {
		t.Fatalf("No command for result section in file %q", path)
	}
	command := c.Text()
	fmt.Fprintf(frontMatter, "exec = %q\n", command)
	err = ioutil.WriteFile(filepath.Join(dataDir, "cmd.sh"), []byte(command), 0755)
	if err != nil {
		t.Fatal(err)
	}

	if !c.Next("```", "```") {
		return
	}
	gold := c.Text()
	if p := strings.Index(gold, "\n"); p > 0 {
		gold = gold[p+1:]
	}

	cuetest.Run(t, dir, command, &cuetest.Config{Golden: gold})

	gold = strings.TrimSpace(gold) + "\n"
	err = ioutil.WriteFile(filepath.Join(dataDir, "out.txt"), []byte(gold), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
