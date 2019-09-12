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

package ast

import (
	"strconv"
	"unicode"
	"unicode/utf8"

	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
)

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch >= utf8.RuneSelf && unicode.IsLetter(ch)
}

func isDigit(ch rune) bool {
	// TODO(mpvl): Is this correct?
	return '0' <= ch && ch <= '9' || ch >= utf8.RuneSelf && unicode.IsDigit(ch)
}

// QuoteIdent quotes an identifier, if needed, and reports
// an error if the identifier is invalid.
func QuoteIdent(ident string) (string, error) {
	if ident != "" && ident[0] == '`' {
		if _, err := strconv.Unquote(ident); err != nil {
			return "", errors.Newf(token.NoPos, "invalid quoted identifier %q", ident)
		}
		return ident, nil
	}

	// TODO: consider quoting keywords
	// switch ident {
	// case "for", "in", "if", "let", "true", "false", "null":
	// 	goto escape
	// }

	for _, r := range ident {
		if isLetter(r) || isDigit(r) || r == '_' {
			continue
		}
		if r == '-' {
			goto escape
		}
		return "", errors.Newf(token.NoPos, "invalid character '%s' in identifier", string(r))
	}

	return ident, nil

escape:
	return "`" + ident + "`", nil
}

// ParseIdent unquotes a possibly quoted identifier and validates
// if the result is valid.
func ParseIdent(n *Ident) (string, error) {
	ident := n.Name
	if ident == "" {
		return "", errors.Newf(n.Pos(), "empty identifier")
	}
	quoted := false
	if ident[0] == '`' {
		u, err := strconv.Unquote(ident)
		if err != nil {
			return "", errors.Newf(n.Pos(), "invalid quoted identifier")
		}
		ident = u
		quoted = true
	}

	for _, r := range ident {
		if isLetter(r) || isDigit(r) || r == '_' {
			continue
		}
		if r == '-' && quoted {
			continue
		}
		return "", errors.Newf(n.Pos(), "invalid character '%s' in identifier", string(r))
	}

	return ident, nil
}

// LabelName reports the name of a label, whether it is an identifier
// (it binds a value to a scope), and whether it is valid.
// Keywords that are allowed in label positions are interpreted accordingly.
//
// Examples:
//
//     Label   Result
//     foo     "foo"  true   nil
//     `a-b`   "a-b"  true   nil
//     true    true   true   nil
//     "foo"   "foo"  false  nil
//     `a-b    ""     false  invalid identifier
//     "foo    ""     false  invalid string
//     "\(x)"  ""     false  errors.Is(err, ErrIsExpression)
//     <A>     "A"    false  errors.Is(err, ErrIsExpression)
//
func LabelName(l Label) (name string, isIdent bool, err error) {
	switch n := l.(type) {
	case *Ident:
		str, err := ParseIdent(n)
		if err != nil {
			return "", false, err
		}
		return str, true, nil

	case *BasicLit:
		switch n.Kind {
		case token.STRING:
			// Use strconv to only allow double-quoted, single-line strings.
			str, err := strconv.Unquote(n.Value)
			if err != nil {
				err = errors.Newf(l.Pos(), "invalid")
			}
			return str, false, err

		case token.NULL, token.TRUE, token.FALSE:
			return n.Value, true, nil

			// TODO: allow numbers to be fields?
		}
	}
	// This includes interpolation and template labels.
	return "", false, errors.Wrapf(ErrIsExpression, l.Pos(),
		"label is interpolation or template")
}

// ErrIsExpression reports whether a label is an expression.
// This error is never returned directly. Use errors.Is or xerrors.Is.
var ErrIsExpression = errors.New("not a concrete label")
