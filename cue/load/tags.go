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

package load

import (
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal"
	"cuelang.org/go/internal/cli"
)

// A tag binds an identifier to a field to allow passing command-line values.
//
// A tag is of the form
//     @tag(<name>,[type=(string|int|number|bool)][,short=<shorthand>+])
//
// The name is mandatory and type defaults to string. Tags are set using the -t
// option on the command line. -t name=value will parse value for the type
// defined for name and set the field for which this tag was defined to this
// value. A tag may be associated with multiple fields.
//
// Tags also allow shorthands. If a shorthand bar is declared for a tag with
// name foo, then -t bar is identical to -t foo=bar.
//
// It is a deliberate choice to not allow other values to be associated with
// shorthands than the shorthand name itself. Doing so would create a powerful
// mechanism that would assign different values to different fields based on the
// same shorthand, duplicating functionality that is already available in CUE.
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

func injectTags(tags []string, l *loader) errors.Error {
	// Parses command line args
	for _, s := range tags {
		p := strings.Index(s, "=")
		found := l.buildTags[s]
		if p > 0 { // key-value
			for _, t := range l.tags {
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
			for _, t := range l.tags {
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
				return errors.Newf(token.NoPos, "tag %q not used in any file", s)
			}
		}
	}
	return nil
}

func shouldBuildFile(f *ast.File, fp *fileProcessor) (bool, errors.Error) {
	tags := fp.c.Tags

	a, errs := getBuildAttr(f)
	if errs != nil {
		return false, errs
	}
	if a == nil {
		return true, nil
	}

	_, body := a.Split()

	expr, err := parser.ParseExpr("", body)
	if err != nil {
		return false, errors.Promote(err, "")
	}

	tagMap := map[string]bool{}
	for _, t := range tags {
		tagMap[t] = !strings.ContainsRune(t, '=')
	}

	c := checker{tags: tagMap, loader: fp.c.loader}
	include := c.shouldInclude(expr)
	if c.err != nil {
		return false, c.err
	}
	return include, nil
}

func getBuildAttr(f *ast.File) (*ast.Attribute, errors.Error) {
	var a *ast.Attribute
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.Attribute:
			key, _ := x.Split()
			if key != "if" {
				continue
			}
			if a != nil {
				err := errors.Newf(d.Pos(), "multiple @if attributes")
				err = errors.Append(err,
					errors.Newf(a.Pos(), "previous declaration here"))
				return nil, err
			}
			a = x

		case *ast.Package:
			break
		}
	}
	return a, nil
}

type checker struct {
	loader *loader
	tags   map[string]bool
	err    errors.Error
}

func (c *checker) shouldInclude(expr ast.Expr) bool {
	switch x := expr.(type) {
	case *ast.Ident:
		c.loader.buildTags[x.Name] = true
		return c.tags[x.Name]

	case *ast.BinaryExpr:
		switch x.Op {
		case token.LAND:
			return c.shouldInclude(x.X) && c.shouldInclude(x.Y)

		case token.LOR:
			return c.shouldInclude(x.X) || c.shouldInclude(x.Y)

		default:
			c.err = errors.Append(c.err, errors.Newf(token.NoPos,
				"invalid operator %v", x.Op))
			return false
		}

	case *ast.UnaryExpr:
		if x.Op != token.NOT {
			c.err = errors.Append(c.err, errors.Newf(token.NoPos,
				"invalid operator %v", x.Op))
		}
		return !c.shouldInclude(x.X)

	default:
		c.err = errors.Append(c.err, errors.Newf(token.NoPos,
			"invalid type %T in build attribute", expr))
		return false
	}
}
