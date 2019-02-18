// Copyright 2018 The CUE Authors
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

package cue

import (
	"fmt"
	"math/big"
	"testing"

	"cuelang.org/go/cue/ast"
	"github.com/cockroachdb/apd"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestUnquote(t *testing.T) {
	testCases := []struct {
		in, out string
		err     error
	}{
		{`"Hello"`, "Hello", nil},
		{`'Hello'`, "Hello", nil},
		{`'Hellø'`, "Hellø", nil},
		{`"""` + "\n\t\tHello\n\t\t" + `"""`, "Hello", nil},
		{"'''\n\t\tHello\n\t\t'''", "Hello", nil},
		{"'''\n\t\tHello\n\n\t\t'''", "Hello\n", nil},
		{"'''\n\n\t\tHello\n\t\t'''", "\nHello", nil},
		{"'''\n\n\n\n\t\t'''", "\n\n", nil},
		{"'''\n\t\t'''", "", nil},
		{`"""` + "\n\raaa\n\rbbb\n\r" + `"""`, "aaa\nbbb", nil},
		{`'\a\b\f\n\r\t\v\'\\\/'`, "\a\b\f\n\r\t\v'\\/", nil},
		{`"\a\b\f\n\r\t\v\"\\\/"`, "\a\b\f\n\r\t\v\"\\/", nil},
		{`#"The sequence "\U0001F604" renders as \#U0001F604."#`,
			`The sequence "\U0001F604" renders as 😄.`,
			nil},
		{`"  \U00010FfF"`, "  \U00010fff", nil},
		{`"\u0061 "`, "a ", nil},
		{`'\x61\x55'`, "\x61\x55", nil},
		{`'\061\055'`, "\061\055", nil},
		{`'\377 '`, "\377 ", nil},
		{"'e\u0300\\n'", "e\u0300\n", nil},
		{`'\06\055'`, "", errSyntax},
		{`'\0'`, "", errSyntax},
		{`"\06\055"`, "", errSyntax},    // too short
		{`'\777 '`, "", errSyntax},      // overflow
		{`'\U012301'`, "", errSyntax},   // too short
		{`'\U0123012G'`, "", errSyntax}, // invalid digit G
		{`"\x04"`, "", errSyntax},       // not allowed in strings
		{`'\U01230123'`, "", errSyntax}, // too large

		{`"\\"`, "\\", nil},
		{`"\'"`, "", errSyntax},
		{`"\q"`, "", errSyntax},
		{"'\n'", "", errSyntax},
		{"'---\n---'", "", errSyntax},
		{"'''\r'''", "", errMissingNewline},

		{`#"Hello"#`, "Hello", nil},
		{`#"Hello\v"#`, "Hello\\v", nil},
		{`#"Hello\#v\r"#`, "Hello\v\\r", nil},
		{`##"Hello\##v\r"##`, "Hello\v\\r", nil},
		{`##"Hello\##v"##`, "Hello\v", nil},
		{"#'''\n\t\tHello\\#v\n\t\t'''#", "Hello\v", nil},
		{"##'''\n\t\tHello\\#v\n\t\t'''##", "Hello\\#v", nil},
		{`#"""` + "\n\t\t\\#r\n\t\t" + `"""#`, "\r", nil},
		{`#""#`, "", nil},
		{`#"This is a "dog""#`, `This is a "dog"`, nil},
		{"#\"\"\"\n\"\n\"\"\"#", `"`, nil},
		{"#\"\"\"\n\"\"\"\n\"\"\"#", `"""`, nil},
		{"#\"\"\"\n\na\n\n\"\"\"#", "\na\n", nil},
		// Gobble extra \r
		{"#\"\"\"\n\ra\n\r\"\"\"#", `a`, nil},
		{"#\"\"\"\n\r\n\ra\n\r\n\r\"\"\"#", "\na\n", nil},
		// Make sure this works for Windows.
		{"#\"\"\"\r\n\r\na\r\n\r\n\"\"\"#", "\na\n", nil},
		{"#\"\"\"\r\n \r\n a\r\n \r\n \"\"\"#", "\na\n", nil},
		{"#\"\"\"\r\na\r\n\"\"\"#", `a`, nil},
		{"#\"\"\"\r\n\ra\r\n\r\"\"\"#", `a`, nil},
		{`####"   \"####`, `   \`, nil},

		{"```", "", errSyntax},
		{"Hello", "", errSyntax},
		{`"Hello`, "", errUnmatchedQuote},
		{`"""Hello"""`, "", errMissingNewline},
		{"'''\n  Hello\n   '''", "", errInvalidWhitespace},
		{"'''\n   a\n  b\n   '''", "", errInvalidWhitespace},
		{`"Hello""`, "", errSyntax},
		{`#"Hello"`, "", errUnmatchedQuote},
		{`#"Hello'#`, "", errUnmatchedQuote},
		{`#"""#`, "", errMissingNewline},

		// TODO: should these be legal?
		{`#"""#`, "", errMissingNewline},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d/%s", i, tc.in), func(t *testing.T) {
			if got, err := Unquote(tc.in); err != tc.err {
				t.Errorf("error: got %q; want %q", err, tc.err)
			} else if got != tc.out {
				t.Errorf("value: got %q; want %q", got, tc.out)
			}
		})
	}
}

func TestInterpolation(t *testing.T) {
	testCases := []struct {
		quotes string
		in     string
		out    string
		err    error
	}{
		{`""`, `foo\(`, "foo", nil},
		{`"""` + "\n" + `"""`, `foo`, "", errUnmatchedQuote},
		{`#""#`, `foo\#(`, "foo", nil},
		{`#""#`, `foo\(`, "", errUnmatchedQuote},
		{`""`, `foo\(bar`, "", errSyntax},
		{`""`, ``, "", errUnmatchedQuote},
		{`#""#`, `"`, "", errUnmatchedQuote},
		{`#""#`, `\`, "", errUnmatchedQuote},
		{`##""##`, `\'`, "", errUnmatchedQuote},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d/%s/%s", i, tc.quotes, tc.in), func(t *testing.T) {
			info, _, _, _ := ParseQuotes(tc.quotes, tc.quotes)
			if got, err := info.Unquote(tc.in); err != tc.err {
				t.Errorf("error: got %q; want %q", err, tc.err)
			} else if got != tc.out {
				t.Errorf("value: got %q; want %q", got, tc.out)
			}
		})
	}
}

func TestIsDouble(t *testing.T) {
	testCases := []struct {
		quotes string
		double bool
	}{
		{`""`, true},
		{`"""` + "\n" + `"""`, true},
		{`#""#`, true},
		{`''`, false},
		{`'''` + "\n" + `'''`, false},
		{`#''#`, false},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d/%s", i, tc.quotes), func(t *testing.T) {
			info, _, _, err := ParseQuotes(tc.quotes, tc.quotes)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.IsDouble(); got != tc.double {
				t.Errorf("got %v; want %v", got, tc.double)
			}
		})
	}
}

var defIntBase = newNumBase(&ast.BasicLit{}, newNumInfo(numKind, 0, 10, false))
var defRatBase = newNumBase(&ast.BasicLit{}, newNumInfo(floatKind, 0, 10, false))

func mkInt(a int64) *numLit {
	x := &numLit{numBase: defIntBase}
	x.v.SetInt64(a)
	return x
}
func mkIntString(a string) *numLit {
	x := &numLit{numBase: defIntBase}
	x.v.SetString(a)
	return x
}
func mkFloat(a string) *numLit {
	x := &numLit{numBase: defRatBase}
	x.v.SetString(a)
	return x
}
func mkBigInt(a int64) (v apd.Decimal) { v.SetInt64(a); return }

func mkBigFloat(a string) (v apd.Decimal) { v.SetString(a); return }

var diffOpts = []cmp.Option{
	cmp.Comparer(func(x, y big.Rat) bool {
		return x.String() == y.String()
	}),
	cmp.Comparer(func(x, y big.Int) bool {
		return x.String() == y.String()
	}),
	cmp.AllowUnexported(
		nullLit{},
		boolLit{},
		stringLit{},
		bytesLit{},
		numLit{},
		numBase{},
		numInfo{},
	),
	cmpopts.IgnoreUnexported(
		bottom{},
		baseValue{},
		baseValue{},
	),
}

var (
	nullSentinel  = &nullLit{}
	trueSentinel  = &boolLit{b: true}
	falseSentinel = &boolLit{b: false}
)

func TestLiterals(t *testing.T) {
	mkMul := func(x int64, m multiplier, base int) *numLit {
		return &numLit{
			newNumBase(&ast.BasicLit{}, newNumInfo(numKind, m, base, false)),
			mkBigInt(x),
		}
	}
	hk := &numLit{
		newNumBase(&ast.BasicLit{}, newNumInfo(numKind, 0, 10, true)),
		mkBigInt(100000),
	}
	testCases := []struct {
		lit  string
		node value
	}{
		{"0", mkInt(0)},
		{"null", nullSentinel},
		{"true", trueSentinel},
		{"false", falseSentinel},
		{"fls", &bottom{}},
		{`"foo"`, &stringLit{str: "foo"}},
		{`"\"foo\""`, &stringLit{str: `"foo"`}},
		{`"foo\u0032"`, &stringLit{str: `foo2`}},
		{`"foo\U00000033"`, &stringLit{str: `foo3`}},
		{`"foo\U0001f499"`, &stringLit{str: `foo💙`}},
		{`"\a\b\f\n\r\t\v"`, &stringLit{str: "\a\b\f\n\r\t\v"}},
		{`"""
		"""`, &stringLit{str: ""}},
		{`"""
			abc
			"""`, &stringLit{str: "abc"}},
		{`"""
			abc
			def
			"""`, &stringLit{str: "abc\ndef"}},
		{`"""
			abc
				def
			"""`, &stringLit{str: "abc\n\tdef"}},
		{`'\xff'`, &bytesLit{b: []byte("\xff")}},
		{"1", mkInt(1)},
		{"100_000", hk},
		{"1.", mkFloat("1")},
		{"0.0", mkFloat("0.0")},
		{".0", mkFloat(".0")},
		{"1K", mkMul(1000, mulK, 10)},
		{"1Mi", mkMul(1024*1024, mulMi, 10)},
		{"1.5Mi", mkMul((1024+512)*1024, mulMi, 10)},
		{"1.3Mi", &bottom{}}, // Cannot be accurately represented.
		{"1.3G", mkMul(1300000000, mulG, 10)},
		{"1.3e+20", mkFloat("1.3e+20")},
		{"1.3e20", mkFloat("1.3e+20")},
		{"1.3e-5", mkFloat("1.3e-5")},
		{"0x1234", mkMul(0x1234, 0, 16)},
		{"0xABCD", mkMul(0xABCD, 0, 16)},
		{"0b11001000", mkMul(0xc8, 0, 2)},
		{"0b1", mkMul(1, 0, 2)},
		{"0o755", mkMul(0755, 0, 8)},
	}
	p := litParser{
		ctx: &context{Context: &apd.BaseContext},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d/%+q", i, tc.lit), func(t *testing.T) {
			got := p.parse(&ast.BasicLit{Value: tc.lit})
			if !cmp.Equal(got, tc.node, diffOpts...) {
				t.Error(cmp.Diff(got, tc.node, diffOpts...))
				t.Errorf("%#v, %#v\n", got, tc.node)
			}
		})
	}
}

func TestLiteralErrors(t *testing.T) {
	testCases := []struct {
		lit string
	}{
		{`"foo\u"`},
		{`"foo\u003"`},
		{`"foo\U1234567"`},
		{`"foo\U12345678"`},
		{`"foo\Ug"`},
		{`"\xff"`},
		// not allowed in string literal, only binary
		{`"foo\x00"`},
		{`0x`},
		{`0o`},
		{`0_`},
		{``},
		{`"`},
		{`"a`},
		// wrong indentation
		{`"""
			abc
		def
			"""`},
		// non-matching quotes
		{`"""
			abc
			'''`},
		{`"""
			abc
			"`},
		{`"abc \( foo "`},
	}
	p := litParser{
		ctx: &context{Context: &apd.BaseContext},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%+q", tc.lit), func(t *testing.T) {
			got := p.parse(&ast.BasicLit{Value: tc.lit})
			if _, ok := got.(*bottom); !ok {
				t.Fatalf("expected error but found none")
			}
		})
	}
}
