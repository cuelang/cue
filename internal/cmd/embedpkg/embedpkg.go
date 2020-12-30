// Copyright 2021 CUE Authors
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

// embedpkg accepts a [packages] argument (see 'go help packages') and creates
// a map[string][]byte for each package argument, a map that represents the
// GoFiles for that package. The principal use case here is the embedding
// of the cuelang.org/go/cmd/cue/cmd/interfaces used by cue get go.
package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"path/filepath"
	"text/template"

	"golang.org/x/tools/go/packages"
)

const genTmpl = `// Code generated by internal/cmd/embedpkg. DO NOT EDIT.

package cmd

// {{ .Basename }}Files is the result of embedding GoFiles from the
// {{ .PkgPath }} package.
var {{ .Basename }}Files = map[string][]byte {
{{- range $fn, $content := .GoFiles }}
	"{{ $fn }}": {{ printf "%#v" $content }},
{{- end }}
}
`

func main() {
	log.SetFlags(0)
	flag.Parse()

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles |
			packages.NeedCompiledGoFiles | packages.NeedModule,
	}
	pkgs, err := packages.Load(cfg, flag.Args()...)
	if err != nil {
		log.Fatal(err)
	}

	// parse template
	tmpl, err := template.New("embedpkg").Parse(genTmpl)
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range pkgs {
		if packages.PrintErrors(pkgs) > 0 {
			// The errors will already have been printed
			log.Fatalln("error loading packages")
		}
		files := map[string][]byte{}
		for _, fn := range p.GoFiles {
			// Because of https://github.com/golang/go/issues/38445 we don't have p.Dir
			content, err := ioutil.ReadFile(fn)
			if err != nil {
				log.Fatal(err)
			}
			relFile, err := filepath.Rel(p.Module.Dir, fn)
			if err != nil {
				log.Fatal(err)
			}
			files[relFile] = content
		}
		data := struct {
			Basename string
			PkgPath  string
			GoFiles  map[string][]byte
		}{
			Basename: p.Name,
			PkgPath:  p.PkgPath,
			GoFiles:  files,
		}
		var b bytes.Buffer
		err = tmpl.Execute(&b, data)
		if err != nil {
			log.Fatal(err)
		}
		err = ioutil.WriteFile(filepath.Join(p.Name+"_gen.go"), b.Bytes(), 0666)
		if err != nil {
			log.Fatal(err)
		}
	}
}
