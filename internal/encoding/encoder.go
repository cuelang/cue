// Copyright 2020 CUE Authors
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

package encoding

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal/filetypes"
	"cuelang.org/go/pkg/encoding/yaml"
)

// An Encoder converts CUE to various file formats, including CUE itself.
// An Encoder allows
type Encoder struct {
	cfg          *Config
	closer       io.Closer
	interpret    func(*cue.Instance) (*ast.File, error)
	encFile      func(*ast.File) error
	encValue     func(cue.Value) error
	autoSimplify bool
}

func (e Encoder) Close() error {
	if e.closer == nil {
		return nil
	}
	return e.closer.Close()
}

// NewEncoder writes content to the file with the given specification.
func NewEncoder(f *build.File, cfg *Config) (*Encoder, error) {
	w, closer, err := writer(f, cfg)
	if err != nil {
		return nil, err
	}
	e := &Encoder{
		cfg:    cfg,
		closer: closer,
	}

	switch f.Interpretation {
	case "":
	// case build.OpenAPI:
	// 	// TODO: get encoding options
	// 	cfg := openapi.Config{}
	// 	i.interpret = func(inst *cue.Instance) (*ast.File, error) {
	// 		return openapi.Generate(inst, cfg)
	// 	}
	// case build.JSONSchema:
	// 	// TODO: get encoding options
	// 	cfg := openapi.Config{}
	// 	i.interpret = func(inst *cue.Instance) (*ast.File, error) {
	// 		return jsonschmea.Generate(inst, cfg)
	// 	}
	default:
		return nil, fmt.Errorf("unsupported interpretation %q", f.Interpretation)
	}

	switch f.Encoding {
	case build.CUE:
		fi, err := filetypes.FromFile(f, cfg.Mode)
		if err != nil {
			return nil, err
		}

		synOpts := []cue.Option{}
		if !fi.KeepDefaults || !fi.Incomplete {
			synOpts = append(synOpts, cue.Final())
		}

		synOpts = append(synOpts,
			cue.Docs(fi.Docs),
			cue.Attributes(fi.Attributes),
			cue.Optional(fi.Optional),
			cue.Concrete(!fi.Incomplete),
			cue.Definitions(fi.Definitions),
			cue.ResolveReferences(!fi.References),
			cue.DisallowCycles(!fi.Cycles),
		)

		opts := []format.Option{
			format.UseSpaces(4),
			format.TabIndent(false),
		}
		opts = append(opts, cfg.Format...)

		useSep := false
		format := func(name string, n ast.Node) error {
			if name != "" && cfg.Stream {
				// TODO: make this relative to DIR
				fmt.Fprintf(w, "--- %s\n", filepath.Base(name))
			} else if useSep {
				fmt.Println("---")
			}
			useSep = true

			opts := opts
			if e.autoSimplify {
				opts = append(opts, format.Simplify())
			}

			b, err := format.Node(n, opts...)
			if err != nil {
				return err
			}
			_, err = w.Write(b)
			return err
		}
		e.encValue = func(v cue.Value) error {
			return format("", v.Syntax(synOpts...))
		}
		e.encFile = func(f *ast.File) error { return format(f.Filename, f) }

	case build.JSON, build.JSONL:
		// SetEscapeHTML
		d := json.NewEncoder(w)
		d.SetIndent("", "    ")
		d.SetEscapeHTML(cfg.EscapeHTML)
		e.encValue = func(v cue.Value) error {
			err := d.Encode(v)
			if x, ok := err.(*json.MarshalerError); ok {
				err = x.Err
			}
			return err
		}

	case build.YAML:
		streamed := false
		e.encValue = func(v cue.Value) error {
			if streamed {
				fmt.Fprintln(w, "---")
			}
			streamed = true

			str, err := yaml.Marshal(v)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(w, str)
			return err
		}

	case build.Text:
		e.encValue = func(v cue.Value) error {
			str, err := v.String()
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(w, str)
			return err
		}

	default:
		return nil, fmt.Errorf("unsupported encoding %q", f.Encoding)
	}

	return e, nil
}

func (e *Encoder) EncodeFile(f *ast.File) error {
	e.autoSimplify = false
	return e.encodeFile(f, e.interpret)
}

func (e *Encoder) Encode(inst *cue.Instance) error {
	e.autoSimplify = true
	if e.interpret != nil {
		f, err := e.interpret(inst)
		if err != nil {
			return err
		}
		return e.encodeFile(f, nil)
	}
	if e.encValue != nil {
		return e.encValue(inst.Value())
	}
	return e.encFile(valueToFile(inst.Value()))
}

func (e *Encoder) encodeFile(f *ast.File, interpret func(*cue.Instance) (*ast.File, error)) error {
	if interpret == nil && e.encFile != nil {
		return e.encFile(f)
	}
	var r cue.Runtime
	inst, err := r.CompileFile(f)
	if err != nil {
		return err
	}
	if interpret != nil {
		return e.Encode(inst)
	}
	return e.encValue(inst.Value())
}

func writer(f *build.File, cfg *Config) (io.Writer, io.Closer, error) {
	if cfg.Out != nil {
		return cfg.Out, nil, nil
	}
	path := f.Filename
	if path == "-" {
		if cfg.Stdout == nil {
			return os.Stdout, nil, nil
		}
		return cfg.Stdout, nil, nil
	}
	if !cfg.Force {
		if _, err := os.Stat(path); err == nil {
			return nil, nil, errors.Wrapf(os.ErrExist, token.NoPos,
				"error writing %q", path)
		}
	}
	w, err := os.Create(path)
	return w, w, err
}
