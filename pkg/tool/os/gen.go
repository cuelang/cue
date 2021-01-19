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

// +build ignore

package main

// TODO: remove when we have a cuedoc server. Until then,
// piggyback on pkg.go.dev.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
)

const msg = `// Code generated by cue get go. DO NOT EDIT.

// Package os defines tasks for retrieving os-related information.
//
// CUE definitions:
//     %s
package os
`

func main() {
	f, _ := os.Create("doc.go")
	defer f.Close()
	b, _ := ioutil.ReadFile("os.cue")
	i := bytes.Index(b, []byte("package os"))
	b = b[i+len("package os")+1:]
	b = bytes.ReplaceAll(b, []byte("\n"), []byte("\n//     "))
	b = bytes.ReplaceAll(b, []byte("\t"), []byte("    "))
	fmt.Fprintf(f, msg, string(b))
}
