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

package literal

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Form defines how to quote a string or bytes literal.
type Form struct {
	quote       byte
	multiline   bool
	auto        bool
	exact       bool
	asciiOnly   bool
	graphicOnly bool
	indent      string
}

// TODO:
// - Fixed or max level of escape modifiers (#""#).
// - Option to fall back to bytes if value cannot be represented as string.
//   E.g. ExactString.
// - QuoteExact that fails with an error if a string cannot be represented
//   without loss.
// - Handle auto-breaking for long lines (Swift-style, \-terminated lines).
//   This is not supported yet in CUE, but may, and should be considred as
//   a possibility in API design.
// - Other possible convenience forms: Blob (auto-break bytes), String (bytes
//   or string), Label.

// WithTabIndent returns a new Form with indentation set to the given number
// of tabs. The result will be a multiline string.
func (f Form) WithTabIndent(n int) Form {
	f.indent = tabs(n)
	f.multiline = true
	return f
}

const tabIndent = "\t\t\t\t\t\t\t\t\t\t\t\t"

func tabs(n int) string {
	if n < len(tabIndent) {
		return tabIndent[:n]
	}
	return strings.Repeat("\t", n)
}

// WithOptionalIndent is like WithTabIndent, but only returns a multiline
// strings if it doesn't contain any newline characters.
func (f Form) WithOptionalTabIndent(tabs int) Form {
	if tabs < len(tabIndent) {
		f.indent = tabIndent[:tabs]
	} else {
		f.indent = strings.Repeat("\t", tabs)
	}
	f.auto = true
	return f
}

// WithASCIIOnly ensures the quoted strings consists solely of valid ASCII
// characters.
func (f Form) WithASCIIOnly() Form {
	f.asciiOnly = true
	return f
}

// WithGraphicOnly ensures the quoted strings consists solely of printable
// characters.
func (f Form) WithGraphicOnly() Form {
	f.graphicOnly = true
	return f
}

var (
	// String defines the format of a CUE string. Conversions may be lossy.
	String Form = stringForm

	// TODO: ExactString: quotes to bytes type if the string cannot be
	// represented without loss of accuracy.

	// Label is like Text, but optimized for labels.
	Label Form = stringForm

	// Bytes defines the format of bytes literal.
	Bytes Form = bytesForm

	stringForm = Form{quote: '"'}
	bytesForm  = Form{quote: '\'', exact: true}
)

// Quote returns CUE string literal representing s. The returned string uses CUE
// escape sequences (\t, \n, \u00FF, \u0100) for control characters and
// non-printable characters as defined by strconv.IsPrint.
//
// It reports an error if the string cannot be converted to the desired form.
func (f Form) Quote(s string) string {
	return string(f.Append(make([]byte, 0, 3*len(s)/2), s))
}

const (
	lowerhex = "0123456789abcdef"
)

// Append appends a quoted string to a buffer.
func (f Form) Append(buf []byte, s string) []byte {
	if f.auto && strings.ContainsRune(s, '\n') {
		f.multiline = true
	}

	// Often called with big strings, so preallocate. If there's quoting,
	// this is conservative but still helps a lot.
	if cap(buf)-len(buf) < len(s) {
		nBuf := make([]byte, len(buf), len(buf)+1+len(s)+1)
		copy(nBuf, buf)
		buf = nBuf
	}
	buf = append(buf, f.quote)
	if f.multiline {
		buf = append(buf, f.quote, f.quote, '\n')
		if s == "" {
			buf = append(buf, f.indent...)
			buf = append(buf, f.quote, f.quote, f.quote)
			return buf
		}
		if len(s) > 0 && s[0] != '\n' {
			buf = append(buf, f.indent...)
		}
	}

	buf = f.appendEscaped(buf, s)

	if f.multiline {
		buf = append(buf, '\n')
		buf = append(buf, f.indent...)
		buf = append(buf, f.quote, f.quote, f.quote)
	} else {
		buf = append(buf, f.quote)
	}

	return buf
}

// AppendEscaped appends an escaped string to a buffer, without adding quotes.
// It does not include the last indentation.
func (f Form) AppendEscaped(buf []byte, s string) []byte {
	if f.auto && strings.ContainsRune(s, '\n') {
		f.multiline = true
	}

	// Often called with big strings, so preallocate. If there's quoting,
	// this is conservative but still helps a lot.
	if cap(buf)-len(buf) < len(s) {
		nBuf := make([]byte, len(buf), len(buf)+1+len(s)+1)
		copy(nBuf, buf)
		buf = nBuf
	}

	buf = f.appendEscaped(buf, s)

	return buf
}

func (f Form) appendEscaped(buf []byte, s string) []byte {
	for width := 0; len(s) > 0; s = s[width:] {
		r := rune(s[0])
		width = 1
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRuneInString(s)
		}
		if f.exact && width == 1 && r == utf8.RuneError {
			buf = append(buf, `\x`...)
			buf = append(buf, lowerhex[s[0]>>4])
			buf = append(buf, lowerhex[s[0]&0xF])
			continue
		}
		if f.multiline && r == '\n' {
			buf = append(buf, '\n')
			if len(s) > 1 && s[1] != '\n' {
				buf = append(buf, f.indent...)
			}
			continue
		}
		buf = f.appendEscapedRune(buf, r)
	}
	return buf
}

func (f *Form) appendEscapedRune(buf []byte, r rune) []byte {
	var runeTmp [utf8.UTFMax]byte
	if (!f.multiline && r == rune(f.quote)) || r == '\\' { // always backslashed
		buf = append(buf, '\\')
		buf = append(buf, byte(r))
		return buf
	}
	if f.asciiOnly {
		if r < utf8.RuneSelf && strconv.IsPrint(r) {
			buf = append(buf, byte(r))
			return buf
		}
	} else if strconv.IsPrint(r) || f.graphicOnly && isInGraphicList(r) {
		n := utf8.EncodeRune(runeTmp[:], r)
		buf = append(buf, runeTmp[:n]...)
		return buf
	}
	switch r {
	case '\a':
		buf = append(buf, `\a`...)
	case '\b':
		buf = append(buf, `\b`...)
	case '\f':
		buf = append(buf, `\f`...)
	case '\n':
		buf = append(buf, `\n`...)
	case '\r':
		buf = append(buf, `\r`...)
	case '\t':
		buf = append(buf, `\t`...)
	case '\v':
		buf = append(buf, `\v`...)
	default:
		switch {
		case r < ' ' && f.exact:
			buf = append(buf, `\x`...)
			buf = append(buf, lowerhex[byte(r)>>4])
			buf = append(buf, lowerhex[byte(r)&0xF])
		case r > utf8.MaxRune:
			r = 0xFFFD
			fallthrough
		case r < 0x10000:
			buf = append(buf, `\u`...)
			for s := 12; s >= 0; s -= 4 {
				buf = append(buf, lowerhex[r>>uint(s)&0xF])
			}
		default:
			buf = append(buf, `\U`...)
			for s := 28; s >= 0; s -= 4 {
				buf = append(buf, lowerhex[r>>uint(s)&0xF])
			}
		}
	}
	return buf
}

// isInGraphicList reports whether the rune is in the isGraphic list. This separation
// from IsGraphic allows quoteWith to avoid two calls to IsPrint.
// Should be called only if IsPrint fails.
func isInGraphicList(r rune) bool {
	// We know r must fit in 16 bits - see makeisprint.go.
	if r > 0xFFFF {
		return false
	}
	rr := uint16(r)
	i := bsearch16(isGraphic, rr)
	return i < len(isGraphic) && rr == isGraphic[i]
}

// bsearch16 returns the smallest i such that a[i] >= x.
// If there is no such i, bsearch16 returns len(a).
func bsearch16(a []uint16, x uint16) int {
	i, j := 0, len(a)
	for i < j {
		h := i + (j-i)/2
		if a[h] < x {
			i = h + 1
		} else {
			j = h
		}
	}
	return i
}

// isGraphic lists the graphic runes not matched by IsPrint.
var isGraphic = []uint16{
	0x00a0,
	0x1680,
	0x2000,
	0x2001,
	0x2002,
	0x2003,
	0x2004,
	0x2005,
	0x2006,
	0x2007,
	0x2008,
	0x2009,
	0x200a,
	0x202f,
	0x205f,
	0x3000,
}
