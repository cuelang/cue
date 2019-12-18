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
	"bytes"
	"encoding/json"
	"fmt"
	goast "go/ast"
	"io"
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
	"unicode"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
	"github.com/cockroachdb/apd/v2"
)

// Kind determines the underlying type of a Value.
type Kind int

const BottomKind Kind = 0

const (
	// NullKind indicates a null value.
	NullKind Kind = 1 << iota

	// BoolKind indicates a boolean value.
	BoolKind

	// IntKind represents an integral number.
	IntKind

	// FloatKind represents a decimal float point number that cannot be
	// converted to an integer. The underlying number may still be integral,
	// but resulting from an operation that enforces the float type.
	FloatKind

	// StringKind indicates any kind of string.
	StringKind

	// BytesKind is a blob of data.
	BytesKind

	// StructKind is a kev-value map.
	StructKind

	// ListKind indicates a list of values.
	ListKind

	nextKind

	// _numberKind is used as a implementation detail inside
	// Kind.String to indicate NumberKind.
	_numberKind

	// NumberKind represents any kind of number.
	NumberKind = IntKind | FloatKind
)

// String returns the representation of the Kind as
// a CUE expression. For example:
//
//	(IntKind|ListKind).String()
//
// will return:
//
//	(int|[...])
func (k Kind) String() string {
	if k == BottomKind {
		return "_|_"
	}
	if (k & NumberKind) == NumberKind {
		k = (k &^ NumberKind) | _numberKind
	}
	var buf strings.Builder
	multiple := bits.OnesCount(uint(k)) > 1
	if multiple {
		buf.WriteByte('(')
	}
	for count := 0; ; count++ {
		n := bits.TrailingZeros(uint(k))
		if n == bits.UintSize {
			break
		}
		bit := Kind(1 << uint(n))
		k &^= bit
		s, ok := kindStrs[bit]
		if !ok {
			s = fmt.Sprintf("bad(%d)", n)
		}
		if count > 0 {
			buf.WriteByte('|')
		}
		buf.WriteString(s)
	}
	if multiple {
		buf.WriteByte(')')
	}
	return buf.String()
}

var kindStrs = map[Kind]string{
	NullKind:    "null",
	BoolKind:    "bool",
	IntKind:     "int",
	FloatKind:   "float",
	StringKind:  "string",
	BytesKind:   "bytes",
	StructKind:  "{...}",
	ListKind:    "[...]",
	_numberKind: "number",
}

// An structValue represents a JSON object.
//
// TODO: remove
type structValue struct {
	ctx  *context
	path *valueData
	obj  *structLit
	arcs arcs
}

// Len reports the number of fields in this struct.
func (o *structValue) Len() int {
	if o.obj == nil {
		return 0
	}
	return len(o.arcs)
}

// At reports the key and value of the ith field, i < o.Len().
func (o *structValue) At(i int) (key string, v Value) {
	a := o.arcs[i]
	v = newChildValue(o, i)
	return o.ctx.labelStr(a.feature), v
}

// Lookup reports the field for the given key. The returned Value is invalid
// if it does not exist.
func (o *structValue) Lookup(key string) Value {
	f := o.ctx.strLabel(key)
	i := 0
	len := o.Len()
	for ; i < len; i++ {
		if o.arcs[i].feature == f {
			break
		}
	}
	if i == len {
		// TODO: better message.
		ctx := o.ctx
		x := ctx.mkErr(o.obj, codeNotExist, "value %q not found", key)
		v := x.evalPartial(ctx)
		return Value{ctx.index, &valueData{o.path, 0, arc{cache: v, v: x}}}
	}
	return newChildValue(o, i)
}

// MarshalJSON returns a valid JSON encoding or reports an error if any of the
// fields is invalid.
func (o *structValue) marshalJSON() (b []byte, err errors.Error) {
	b = append(b, '{')
	n := o.Len()
	for i := 0; i < n; i++ {
		k, v := o.At(i)
		s, err := json.Marshal(k)
		if err != nil {
			return nil, unwrapJSONError(err)
		}
		b = append(b, s...)
		b = append(b, ':')
		bb, err := json.Marshal(v)
		if err != nil {
			return nil, unwrapJSONError(err)
		}
		b = append(b, bb...)
		if i < n-1 {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return b, nil
}

var _ errors.Error = &marshalError{}

type marshalError struct {
	err errors.Error
	b   *bottom
}

func toMarshalErr(v Value, b *bottom) error {
	return &marshalError{v.toErr(b), b}
}

func marshalErrf(v Value, src source, code errCode, msg string, args ...interface{}) error {
	arguments := append([]interface{}{code, msg}, args...)
	b := v.idx.mkErr(src, arguments...)
	return toMarshalErr(v, b)
}

func (e *marshalError) Error() string {
	path := e.Path()
	if len(path) == 0 {
		return fmt.Sprintf("cue: marshal error: %v", e.err)
	}
	p := strings.Join(path, ".")
	return fmt.Sprintf("cue: marshal error at path %s: %v", p, e.err)
}

func (e *marshalError) Path() []string               { return e.err.Path() }
func (e *marshalError) Msg() (string, []interface{}) { return e.err.Msg() }
func (e *marshalError) Position() token.Pos          { return e.err.Position() }
func (e *marshalError) InputPositions() []token.Pos {
	return e.err.InputPositions()
}

func unwrapJSONError(err error) errors.Error {
	switch x := err.(type) {
	case *json.MarshalerError:
		return unwrapJSONError(x.Err)
	case *marshalError:
		return x
	case errors.Error:
		return &marshalError{x, nil}
	default:
		return &marshalError{errors.Wrapf(err, token.NoPos, "json error"), nil}
	}
}

// An Iterator iterates over values.
//
type Iterator struct {
	val  Value
	ctx  *context
	iter iterAtter
	len  int
	p    int
	cur  Value
	f    label
}

// Next advances the iterator to the next value and reports whether there was
// any. It must be called before the first call to Value or Key.
func (i *Iterator) Next() bool {
	if i.p >= i.len {
		i.cur = Value{}
		return false
	}
	arc := i.iter.iterAt(i.ctx, i.p)
	i.cur = i.val.makeChild(i.ctx, uint32(i.p), arc)
	i.f = arc.feature
	i.p++
	return true
}

// Value returns the current value in the list. It will panic if Next advanced
// past the last entry.
func (i *Iterator) Value() Value {
	return i.cur
}

// Label reports the label of the value if i iterates over struct fields and
// "" otherwise.
func (i *Iterator) Label() string {
	if i.f == 0 {
		return ""
	}
	return i.ctx.labelStr(i.f)
}

// IsHidden reports if a field is hidden from the data model.
func (i *Iterator) IsHidden() bool {
	return i.f&hidden != 0
}

// IsOptional reports if a field is optional.
func (i *Iterator) IsOptional() bool {
	return i.cur.path.arc.optional
}

// marshalJSON iterates over the list and generates JSON output. HasNext
// will return false after this operation.
func marshalList(l *Iterator) (b []byte, err errors.Error) {
	b = append(b, '[')
	if l.Next() {
		for i := 0; ; i++ {
			x, err := json.Marshal(l.Value())
			if err != nil {
				return nil, unwrapJSONError(err)
			}
			b = append(b, x...)
			if !l.Next() {
				break
			}
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return b, nil
}

func (v Value) getNum(k kind) (*numLit, errors.Error) {
	v, _ = v.Default()
	if err := v.checkKind(v.ctx(), k); err != nil {
		return nil, v.toErr(err)
	}
	n, _ := v.path.cache.(*numLit)
	return n, nil
}

// MantExp breaks x into its mantissa and exponent components and returns the
// exponent. If a non-nil mant argument is provided its value is set to the
// mantissa of x. The components satisfy x == mant × 10**exp. It returns an
// error if v is not a number.
//
// The components are not normalized. For instance, 2.00 is represented mant ==
// 200 and exp == -2. Calling MantExp with a nil argument is an efficient way to
// get the exponent of the receiver.
func (v Value) MantExp(mant *big.Int) (exp int, err error) {
	n, err := v.getNum(numKind)
	if err != nil {
		return 0, err
	}
	if n.v.Form != 0 {
		return 0, ErrInfinite
	}
	if mant != nil {
		mant.Set(&n.v.Coeff)
		if n.v.Negative {
			mant.Neg(mant)
		}
	}
	return int(n.v.Exponent), nil
}

// AppendInt appends the string representation of x in the given base to buf and
// returns the extended buffer, or an error if the underlying number was not
// an integer.
func (v Value) AppendInt(buf []byte, base int) ([]byte, error) {
	i, err := v.Int(nil)
	if err != nil {
		return nil, err
	}
	return i.Append(buf, base), nil
}

// AppendFloat appends to buf the string form of the floating-point number x.
// It returns an error if v is not a number.
func (v Value) AppendFloat(buf []byte, fmt byte, prec int) ([]byte, error) {
	n, err := v.getNum(numKind)
	if err != nil {
		return nil, err
	}
	ctx := apd.BaseContext
	nd := int(apd.NumDigits(&n.v.Coeff)) + int(n.v.Exponent)
	if n.v.Form == apd.Infinite {
		if n.v.Negative {
			buf = append(buf, '-')
		}
		return append(buf, string('∞')...), nil
	}
	if fmt == 'f' && nd > 0 {
		ctx.Precision = uint32(nd + prec)
	} else {
		ctx.Precision = uint32(prec)
	}
	var d apd.Decimal
	ctx.Round(&d, &n.v)
	return d.Append(buf, fmt), nil
}

var (
	// ErrBelow indicates that a value was rounded down in a conversion.
	ErrBelow = errors.New("value was rounded down")

	// ErrAbove indicates that a value was rounded up in a conversion.
	ErrAbove = errors.New("value was rounded up")

	// ErrInfinite indicates that a value is infinite.
	ErrInfinite = errors.New("infinite")
)

// Int converts the underlying integral number to an big.Int. It reports an
// error if the underlying value is not an integer type. If a non-nil *Int
// argument z is provided, Int stores the result in z instead of allocating a
// new Int.
func (v Value) Int(z *big.Int) (*big.Int, error) {
	n, err := v.getNum(intKind)
	if err != nil {
		return nil, err
	}
	if z == nil {
		z = &big.Int{}
	}
	if n.v.Exponent != 0 {
		panic("cue: exponent should always be nil for integer types")
	}
	z.Set(&n.v.Coeff)
	if n.v.Negative {
		z.Neg(z)
	}
	return z, nil
}

// Int64 converts the underlying integral number to int64. It reports an
// error if the underlying value is not an integer type or cannot be represented
// as an int64. The result is (math.MinInt64, ErrAbove) for x < math.MinInt64,
// and (math.MaxInt64, ErrBelow) for x > math.MaxInt64.
func (v Value) Int64() (int64, error) {
	n, err := v.getNum(intKind)
	if err != nil {
		return 0, err
	}
	if !n.v.Coeff.IsInt64() {
		if n.v.Negative {
			return math.MinInt64, ErrAbove
		}
		return math.MaxInt64, ErrBelow
	}
	i := n.v.Coeff.Int64()
	if n.v.Negative {
		i = -i
	}
	return i, nil
}

// Uint64 converts the underlying integral number to uint64. It reports an
// error if the underlying value is not an integer type or cannot be represented
// as a uint64. The result is (0, ErrAbove) for x < 0, and
// (math.MaxUint64, ErrBelow) for x > math.MaxUint64.
func (v Value) Uint64() (uint64, error) {
	n, err := v.getNum(intKind)
	if err != nil {
		return 0, err
	}
	if n.v.Negative {
		return 0, ErrAbove
	}
	if !n.v.Coeff.IsUint64() {
		return math.MaxUint64, ErrBelow
	}
	i := n.v.Coeff.Uint64()
	return i, nil
}

// trimZeros trims 0's for better JSON respresentations.
func trimZeros(s string) string {
	n1 := len(s)
	s2 := strings.TrimRight(s, "0")
	n2 := len(s2)
	if p := strings.IndexByte(s2, '.'); p != -1 {
		if p == n2-1 {
			return s[:len(s2)+1]
		}
		return s2
	}
	if n1-n2 <= 4 {
		return s
	}
	return fmt.Sprint(s2, "e+", n1-n2)
}

var (
	smallestPosFloat64 *apd.Decimal
	smallestNegFloat64 *apd.Decimal
	maxPosFloat64      *apd.Decimal
	maxNegFloat64      *apd.Decimal
)

func init() {
	const (
		// math.SmallestNonzeroFloat64: 1 / 2**(1023 - 1 + 52)
		smallest = "4.940656458412465441765687928682213723651e-324"
		// math.MaxFloat64: 2**1023 * (2**53 - 1) / 2**52
		max = "1.797693134862315708145274237317043567981e+308"
	)
	ctx := apd.BaseContext
	ctx.Precision = 40

	var err error
	smallestPosFloat64, _, err = ctx.NewFromString(smallest)
	if err != nil {
		panic(err)
	}
	smallestNegFloat64, _, err = ctx.NewFromString("-" + smallest)
	if err != nil {
		panic(err)
	}
	maxPosFloat64, _, err = ctx.NewFromString(max)
	if err != nil {
		panic(err)
	}
	maxNegFloat64, _, err = ctx.NewFromString("-" + max)
	if err != nil {
		panic(err)
	}
}

// Float64 returns the float64 value nearest to x. It reports an error if v is
// not a number. If x is too small to be represented by a float64 (|x| <
// math.SmallestNonzeroFloat64), the result is (0, ErrBelow) or (-0, ErrAbove),
// respectively, depending on the sign of x. If x is too large to be represented
// by a float64 (|x| > math.MaxFloat64), the result is (+Inf, ErrAbove) or
// (-Inf, ErrBelow), depending on the sign of x.
func (v Value) Float64() (float64, error) {
	n, err := v.getNum(numKind)
	if err != nil {
		return 0, err
	}
	if n.v.Negative {
		if n.v.Cmp(smallestNegFloat64) == 1 {
			return -0, ErrAbove
		}
		if n.v.Cmp(maxNegFloat64) == -1 {
			return math.Inf(-1), ErrBelow
		}
	} else {
		if n.v.Cmp(smallestPosFloat64) == -1 {
			return 0, ErrBelow
		}
		if n.v.Cmp(maxPosFloat64) == 1 {
			return math.Inf(1), ErrAbove
		}
	}
	f, _ := n.v.Float64()
	return f, nil
}

type valueData struct {
	parent *valueData
	index  uint32
	arc
}

// path returns the path of the value.
func (v *valueData) appendPath(a []string, idx *index) ([]string, kind) {
	var k kind
	if v.parent != nil {
		a, k = v.parent.appendPath(a, idx)
	}
	switch k {
	case listKind:
		a = append(a, strconv.FormatInt(int64(v.index), 10))
	case structKind:
		f := idx.labelStr(v.arc.feature)
		if !isIdent(f) && !isNumber(f) {
			f = quote(f, '"')
		}
		a = append(a, f)
	}
	return a, v.arc.cache.kind()
}

var validIdent = []*unicode.RangeTable{unicode.L, unicode.N}

func isIdent(s string) bool {
	valid := []*unicode.RangeTable{unicode.Letter}
	for _, r := range s {
		if !unicode.In(r, valid...) && r != '_' {
			return false
		}
		valid = validIdent
	}
	return true
}

func isNumber(s string) bool {
	for _, r := range s {
		if r < '0' || '9' < r {
			return false
		}
	}
	return true
}

// Value holds any value, which may be a Boolean, Error, List, Null, Number,
// Struct, or String.
type Value struct {
	idx  *index
	path *valueData
}

func newValueRoot(ctx *context, x value) Value {
	v := x.evalPartial(ctx)
	return Value{ctx.index, &valueData{nil, 0, arc{cache: v, v: x}}}
}

func newChildValue(obj *structValue, i int) Value {
	a := obj.arcs[i]
	for j, b := range obj.obj.arcs {
		if b.feature == a.feature {
			a = obj.obj.iterAt(obj.ctx, j)
			// TODO: adding more technical debt here. The evaluator should be
			// rewritten.
			x := obj.obj
			ctx := obj.ctx
			if x.optionals != nil {
				name := ctx.labelStr(x.arcs[i].feature)
				arg := &stringLit{x.baseValue, name, nil}

				val, _ := x.optionals.constraint(ctx, arg)
				if val != nil {
					a.v = mkBin(ctx, x.Pos(), opUnify, a.v, val)
				}
			}
			break
		}
	}

	return Value{obj.ctx.index, &valueData{obj.path, uint32(i), a}}
}

func remakeValue(base Value, v value) Value {
	path := *base.path
	path.v = v
	path.cache = v.evalPartial(base.ctx())
	return Value{base.idx, &path}
}

func (v Value) ctx() *context {
	return v.idx.newContext()
}

func (v Value) makeChild(ctx *context, i uint32, a arc) Value {
	return Value{v.idx, &valueData{v.path, i, a}}
}

func (v Value) eval(ctx *context) evaluated {
	if v.path == nil || v.path.cache == nil {
		panic("undefined value")
	}
	return ctx.manifest(v.path.cache)
}

// Eval resolves the references of a value and returns the result.
// This method is not necessary to obtain concrete values.
func (v Value) Eval() Value {
	if v.path == nil {
		return v
	}
	return remakeValue(v, v.path.v.evalPartial(v.ctx()))
}

// Default reports the default value and whether it existed. It returns the
// normal value if there is no default.
func (v Value) Default() (Value, bool) {
	if v.path == nil {
		return v, false
	}
	u := v.path.cache
	if u == nil {
		u = v.path.v.evalPartial(v.ctx())
	}
	x := v.ctx().manifest(u)
	if x != u {
		return remakeValue(v, x), true
	}
	return v, false
}

// Label reports he label used to obtain this value from the enclosing struct.
//
// TODO: get rid of this somehow. Probably by including a FieldInfo struct
// or the like.
func (v Value) Label() (string, bool) {
	if v.path.feature == 0 {
		return "", false
	}
	return v.idx.labelStr(v.path.feature), true
}

// Kind returns the kind of value. It returns BottomKind for atomic values that
// are not concrete. For instance, it will return BottomKind for the bounds
// >=0.
func (v Value) Kind() Kind {
	if v.path == nil {
		return BottomKind
	}
	c := v.path.cache
	if c == nil {
		c = v.path.v.evalPartial(v.ctx())
	}
	k := c.kind()
	if k.isGround() {
		switch {
		case k.isAnyOf(nullKind):
			return NullKind
		case k.isAnyOf(boolKind):
			return BoolKind
		case k&numKind == (intKind):
			return IntKind
		case k&numKind == (floatKind):
			return FloatKind
		case k.isAnyOf(numKind):
			return NumberKind
		case k.isAnyOf(bytesKind):
			return BytesKind
		case k.isAnyOf(stringKind):
			return StringKind
		case k.isAnyOf(structKind):
			return StructKind
		case k.isAnyOf(listKind):
			return ListKind
		}
	}
	return BottomKind
}

// IncompleteKind returns a mask of all kinds that this value may be.
func (v Value) IncompleteKind() Kind {
	if v.path == nil {
		return BottomKind
	}
	var k kind
	x := v.path.v.evalPartial(v.ctx())
	switch x := convertBuiltin(x).(type) {
	case *builtin:
		k = x.representedKind()
	case *customValidator:
		k = x.call.Params[0]
	default:
		k = x.kind()
	}
	vk := BottomKind // Everything is a bottom kind.
	for i := kind(1); i < nonGround; i <<= 1 {
		if k&i != 0 {
			switch i {
			case nullKind:
				vk |= NullKind
			case boolKind:
				vk |= BoolKind
			case intKind:
				vk |= IntKind
			case floatKind:
				vk |= FloatKind
			case stringKind:
				vk |= StringKind
			case bytesKind:
				vk |= BytesKind
			case structKind:
				vk |= StructKind
			case listKind:
				vk |= ListKind
			}
		}
	}
	return vk
}

// MarshalJSON marshalls this value into valid JSON.
func (v Value) MarshalJSON() (b []byte, err error) {
	b, err = v.marshalJSON()
	if err != nil {
		return nil, unwrapJSONError(err)
	}
	return b, nil
}

func (v Value) marshalJSON() (b []byte, err error) {
	v, _ = v.Default()
	if v.path == nil {
		return json.Marshal(nil)
	}
	ctx := v.idx.newContext()
	x := v.eval(ctx)
	// TODO: implement marshalles in value.
	switch k := x.kind(); k {
	case nullKind:
		return json.Marshal(nil)
	case boolKind:
		return json.Marshal(x.(*boolLit).b)
	case intKind, floatKind, numKind:
		return x.(*numLit).v.MarshalText()
	case stringKind:
		return json.Marshal(x.(*stringLit).str)
	case bytesKind:
		return json.Marshal(x.(*bytesLit).b)
	case listKind:
		l := x.(*list)
		i := Iterator{ctx: ctx, val: v, iter: l, len: len(l.elem.arcs)}
		return marshalList(&i)
	case structKind:
		obj, _ := v.structValData(ctx)
		return obj.marshalJSON()
	case bottomKind:
		return nil, toMarshalErr(v, x.(*bottom))
	default:
		if k.hasReferences() {
			return nil, marshalErrf(v, x, codeIncomplete, "value %q contains unresolved references", ctx.str(x))
		}
		if !k.isGround() {
			return nil, marshalErrf(v, x, codeIncomplete, "cannot convert incomplete value %q to JSON", ctx.str(x))
		}
		return nil, marshalErrf(v, x, 0, "cannot convert value %q of type %T to JSON", ctx.str(x), x)
	}
}

// Syntax converts the possibly partially evaluated value into syntax. This
// can use used to print the value with package format.
func (v Value) Syntax(opts ...Option) ast.Node {
	// TODO: the default should ideally be simplified representation that
	// exactly represents the value. The latter can currently only be
	// ensured with Raw().
	if v.path == nil || v.path.cache == nil {
		return nil
	}
	ctx := v.ctx()
	o := getOptions(opts)
	if o.raw {
		n, _ := export(ctx, v.path.v, o)
		return n
	}
	n, _ := export(ctx, v.path.cache, o)
	return n
}

// Decode initializes x with Value v. If x is a struct, it will validate the
// constraints specified in the field tags.
func (v Value) Decode(x interface{}) error {
	// TODO: optimize
	b, err := v.MarshalJSON()
	if err != nil {
		return err
	}
	return json.Unmarshal(b, x)
}

// // EncodeJSON generates JSON for the given value.
// func (v Value) EncodeJSON(w io.Writer, v Value) error {
// 	return nil
// }

// Doc returns all documentation comments associated with the field from which
// the current value originates.
func (v Value) Doc() []*ast.CommentGroup {
	if v.path == nil {
		return nil
	}
	return v.path.docs.appendDocs(nil)
}

// Split returns a list of values from which v originated such that
// the unification of all these values equals v and for all returned values.
// It will also split unchecked unifications (embeddings), so unifying the
// split values may fail if actually unified.
// Source returns a non-nil value.
//
// Deprecated: use Expr.
func (v Value) Split() []Value {
	if v.path == nil {
		return nil
	}
	ctx := v.ctx()
	a := []Value{}
	for _, x := range separate(v.path.v) {
		path := *v.path
		path.cache = x.evalPartial(ctx)
		path.v = x
		a = append(a, Value{v.idx, &path})
	}
	return a
}

func separate(v value) (a []value) {
	c := v.computed()
	if c == nil || (c.op != opUnify && c.op != opUnifyUnchecked) {
		return []value{v}
	}
	if c.x != nil {
		a = append(a, separate(c.x)...)
	}
	if c.y != nil {
		a = append(a, separate(c.y)...)
	}
	return a
}

// Source returns the original node for this value. The return value may not
// be a syntax.Expr. For instance, a struct kind may be represented by a
// struct literal, a field comprehension, or a file. It returns nil for
// computed nodes. Use Split to get all source values that apply to a field.
func (v Value) Source() ast.Node {
	if v.path == nil {
		return nil
	}
	return v.path.v.syntax()
}

// Err returns the error represented by v or nil v is not an error.
func (v Value) Err() error {
	if err := v.checkKind(v.ctx(), bottomKind); err != nil {
		return v.toErr(err)
	}
	return nil
}

// Pos returns position information.
func (v Value) Pos() token.Pos {
	if v.path == nil || v.Source() == nil {
		return token.NoPos
	}
	pos := v.Source().Pos()
	return pos
}

// IsConcrete reports whether the current value is a concrete scalar value
// (not relying on default values), a terminal error, a list, or a struct.
// It does not verify that values of lists or structs are concrete themselves.
// To check whether there is a concrete default, use v.Default().IsConcrete().
func (v Value) IsConcrete() bool {
	if v.path == nil {
		return false // any is neither concrete, not a list or struct.
	}
	x := v.path.v.evalPartial(v.ctx())

	// Errors marked as incomplete are treated as not complete.
	if isIncomplete(x) {
		return false
	}
	// Other errors are considered ground.
	return x.kind().isConcrete()
}

// Deprecated: IsIncomplete
//
// It indicates that the value cannot be fully evaluated due to
// insufficient information.
func (v Value) IsIncomplete() bool {
	// TODO: remove
	x := v.eval(v.ctx())
	if !x.kind().isConcrete() {
		return true
	}
	return isIncomplete(x)
}

// Exists reports whether this value existed in the configuration.
func (v Value) Exists() bool {
	if v.path == nil {
		return false
	}
	return exists(v.eval(v.ctx()))
}

func (v Value) checkKind(ctx *context, want kind) *bottom {
	if v.path == nil {
		return errNotExists
	}
	// TODO: use checkKind
	x := v.eval(ctx)
	if b, ok := x.(*bottom); ok {
		return b
	}
	got := x.kind()
	if want != bottomKind {
		if got&want&concreteKind == bottomKind {
			return ctx.mkErr(x, "cannot use value %v (type %s) as %s",
				v.ctx().str(x), got, want)
		}
		if !got.isGround() {
			return ctx.mkErr(x, codeIncomplete,
				"non-concrete value %v", got)
		}
	}
	return nil
}

func makeInt(v Value, x int64) Value {
	return remakeValue(v, newInt(v.path.v.base(), base10).setInt64(x))
}

// Len returns the number of items of the underlying value.
// For lists it reports the capacity of the list. For structs it indicates the
// number of fields, for bytes the number of bytes.
func (v Value) Len() Value {
	if v.path != nil {
		switch x := v.path.v.evalPartial(v.ctx()).(type) {
		case *list:
			return remakeValue(v, x.len.evalPartial(v.ctx()))
		case *bytesLit:
			return makeInt(v, int64(x.len()))
		case *stringLit:
			return makeInt(v, int64(x.len()))
		}
	}
	const msg = "len not supported for type %v"
	return remakeValue(v, v.ctx().mkErr(v.path.v, msg, v.Kind()))
}

// Elem returns the value of undefined element types of lists and structs.
func (v Value) Elem() (Value, bool) {
	ctx := v.ctx()
	switch x := v.path.v.(type) {
	case *structLit:
		t, _ := x.optionals.constraint(ctx, nil)
		if t == nil {
			break
		}
		return newValueRoot(ctx, t), true
	case *list:
		return newValueRoot(ctx, x.typ), true
	}
	return Value{}, false
}

// List creates an iterator over the values of a list or reports an error if
// v is not a list.
func (v Value) List() (Iterator, error) {
	v, _ = v.Default()
	ctx := v.ctx()
	if err := v.checkKind(ctx, listKind); err != nil {
		return Iterator{ctx: ctx}, v.toErr(err)
	}
	l := v.eval(ctx).(*list)
	return Iterator{ctx: ctx, val: v, iter: l, len: len(l.elem.arcs)}, nil
}

// Null reports an error if v is not null.
func (v Value) Null() error {
	v, _ = v.Default()
	if err := v.checkKind(v.ctx(), nullKind); err != nil {
		return v.toErr(err)
	}
	return nil
}

// // IsNull reports whether v is null.
// func (v Value) IsNull() bool {
// 	return v.Null() == nil
// }

// Bool returns the bool value of v or false and an error if v is not a boolean.
func (v Value) Bool() (bool, error) {
	v, _ = v.Default()
	ctx := v.ctx()
	if err := v.checkKind(ctx, boolKind); err != nil {
		return false, v.toErr(err)
	}
	return v.eval(ctx).(*boolLit).b, nil
}

// String returns the string value if v is a string or an error otherwise.
func (v Value) String() (string, error) {
	v, _ = v.Default()
	ctx := v.ctx()
	if err := v.checkKind(ctx, stringKind); err != nil {
		return "", v.toErr(err)
	}
	return v.eval(ctx).(*stringLit).str, nil
}

// Bytes returns a byte slice if v represents a list of bytes or an error
// otherwise.
func (v Value) Bytes() ([]byte, error) {
	v, _ = v.Default()
	ctx := v.ctx()
	switch x := v.eval(ctx).(type) {
	case *bytesLit:
		return append([]byte(nil), x.b...), nil
	case *stringLit:
		return []byte(x.str), nil
	}
	return nil, v.toErr(v.checkKind(ctx, bytesKind|stringKind))
}

// Reader returns a new Reader if v is a string or bytes type and an error
// otherwise.
func (v Value) Reader() (io.Reader, error) {
	v, _ = v.Default()
	ctx := v.ctx()
	switch x := v.eval(ctx).(type) {
	case *bytesLit:
		return bytes.NewReader(x.b), nil
	case *stringLit:
		return strings.NewReader(x.str), nil
	}
	return nil, v.toErr(v.checkKind(ctx, stringKind|bytesKind))
}

// TODO: distinguish between optional, hidden, etc. Probably the best approach
// is to mark options in context and have a single function for creating
// a structVal.

// structVal returns an structVal or an error if v is not a struct.
func (v Value) structValData(ctx *context) (structValue, *bottom) {
	return v.structValOpts(ctx, options{
		omitHidden:      true,
		omitDefinitions: true,
		omitOptional:    true,
	})
}

func (v Value) structValFull(ctx *context) (structValue, *bottom) {
	return v.structValOpts(ctx, options{})
}

// structVal returns an structVal or an error if v is not a struct.
func (v Value) structValOpts(ctx *context, o options) (structValue, *bottom) {
	v, _ = v.Default() // TODO: remove?
	if err := v.checkKind(ctx, structKind); err != nil {
		return structValue{}, err
	}
	obj := v.eval(ctx).(*structLit)

	// TODO: This is expansion appropriate?
	// TODO: return an incomplete error if there are still expansions remaining.
	obj, err := obj.expandFields(ctx) // expand comprehensions
	if err != nil {
		return structValue{}, err
	}

	// check if any fields can be omitted
	needFilter := false
	if o.omitHidden || o.omitOptional || o.omitDefinitions {
		f := label(0)
		for _, a := range obj.arcs {
			f |= a.feature
			if a.optional && o.omitOptional {
				needFilter = true
				break
			}
			if a.definition && (o.omitDefinitions || o.concrete) {
				needFilter = true
				break
			}
		}
		needFilter = needFilter || f&hidden != 0
	}

	if needFilter {
		arcs := make([]arc, len(obj.arcs))
		k := 0
		for _, a := range obj.arcs {
			if a.definition && (o.omitDefinitions || o.concrete) {
				continue
			}
			if a.feature&hidden != 0 && o.omitHidden {
				continue
			}
			if o.omitOptional && a.optional {
				continue
			}
			arcs[k] = a
			k++
		}
		arcs = arcs[:k]
		return structValue{ctx, v.path, obj, arcs}, nil
	}
	return structValue{ctx, v.path, obj, obj.arcs}, nil
}

// Struct returns the underlying struct of a value or an error if the value
// is not a struct.
func (v Value) Struct(opts ...Option) (*Struct, error) {
	ctx := v.ctx()
	if err := v.checkKind(ctx, structKind); err != nil {
		return nil, v.toErr(err)
	}
	obj := v.eval(ctx).(*structLit)

	// TODO: This is expansion appropriate?
	obj, err := obj.expandFields(ctx)
	if err != nil {
		return nil, v.toErr(err)
	}

	return &Struct{v, obj}, nil
}

// Struct represents a CUE struct value.
type Struct struct {
	v Value
	s *structLit
}

// FieldInfo contains information about a struct field.
type FieldInfo struct {
	Name  string
	Pos   int
	Value Value

	IsDefinition bool
	IsOptional   bool
	IsHidden     bool
}

func (s *Struct) Len() int {
	return len(s.s.arcs)
}

// field reports information about the ith field, i < o.Len().
func (s *Struct) Field(i int) FieldInfo {
	ctx := s.v.ctx()
	a := s.s.arcs[i]
	a.cache = s.s.at(ctx, i)

	// TODO: adding more technical debt here. The evaluator should be
	// rewritten.
	x := s.s
	if x.optionals != nil {
		name := ctx.labelStr(x.arcs[i].feature)
		arg := &stringLit{x.baseValue, name, nil}

		val, _ := x.optionals.constraint(ctx, arg)
		if val != nil {
			a.v = mkBin(ctx, x.Pos(), opUnify, a.v, val)
		}
	}

	v := Value{ctx.index, &valueData{s.v.path, uint32(i), a}}
	str := ctx.labelStr(a.feature)
	return FieldInfo{str, i, v, a.definition, a.optional, a.feature&hidden != 0}
}

func (s *Struct) FieldByName(name string) (FieldInfo, error) {
	f := s.v.ctx().strLabel(name)
	for i, a := range s.s.arcs {
		if a.feature == f {
			return s.Field(i), nil
		}
	}
	return FieldInfo{}, errNotFound
}

// Fields creates an iterator over the Struct's fields.
func (s *Struct) Fields(opts ...Option) Iterator {
	iter, _ := s.v.Fields(opts...)
	return iter
}

// Fields creates an iterator over v's fields if v is a struct or an error
// otherwise.
func (v Value) Fields(opts ...Option) (Iterator, error) {
	o := options{omitDefinitions: true, omitHidden: true, omitOptional: true}
	o.updateOptions(opts)
	ctx := v.ctx()
	obj, err := v.structValOpts(ctx, o)
	if err != nil {
		return Iterator{ctx: ctx}, v.toErr(err)
	}
	n := &structLit{
		obj.obj.baseValue,   // baseValue
		obj.obj.emit,        // emit
		obj.obj.optionals,   // template
		obj.obj.closeStatus, // closeStatus
		nil,                 // comprehensions
		obj.arcs,            // arcs
		nil,                 // attributes
	}
	return Iterator{ctx: ctx, val: v, iter: n, len: len(n.arcs)}, nil
}

// Lookup reports the value at a path starting from v.
// The empty path returns v itself.
//
// The Exists() method can be used to verify if the returned value existed.
// Lookup cannot be used to look up hidden or optional fields or definitions.
func (v Value) Lookup(path ...string) Value {
	ctx := v.ctx()
	for _, k := range path {
		obj, err := v.structValData(ctx)
		if err != nil {
			// TODO: return a Value at the same location and a new error?
			return newValueRoot(ctx, err)
		}
		v = obj.Lookup(k)
	}
	return v
}

var errNotFound = errors.Newf(token.NoPos, "field not found")

// LookupField reports information about a field of v.
func (v Value) LookupField(path string) (FieldInfo, error) {
	s, err := v.Struct()
	if err != nil {
		// TODO: return a Value at the same location and a new error?
		return FieldInfo{}, err
	}
	f, err := s.FieldByName(path)
	if err != nil {
		return f, err
	}
	if f.IsHidden || f.IsDefinition && !goast.IsExported(path) {
		return f, errNotFound
	}
	return f, err
}

// Template returns a function that represents the template definition for a
// struct in a configuration file. It returns nil if v is not a struct kind or
// if there is no template associated with the struct.
//
// The returned function returns the value that would be unified with field
// given its name.
func (v Value) Template() func(label string) Value {
	ctx := v.ctx()
	x, ok := v.path.cache.(*structLit)
	if !ok || x.optionals.isEmpty() {
		return nil
	}

	return func(label string) (v Value) {
		arg := &stringLit{x.baseValue, label, nil}

		if v, _ := x.optionals.constraint(ctx, arg); v != nil {
			return newValueRoot(ctx, v)
		}
		return v
	}
}

// Subsumes reports whether w is an instance of v.
//
// Value v and w must be obtained from the same build.
// TODO: remove this requirement.
func (v Value) Subsumes(w Value) bool {
	ctx := v.ctx()
	return subsumes(ctx, v.eval(ctx), w.eval(ctx), subChoose)
}

// Unify reports the greatest lower bound of v and w.
//
// Value v and w must be obtained from the same build.
// TODO: remove this requirement.
func (v Value) Unify(w Value) Value {
	ctx := v.ctx()
	if v.path == nil {
		return w
	}
	if w.path == nil {
		return v
	}
	a := v.path.v
	b := w.path.v
	src := binSrc(token.NoPos, opUnify, a, b)
	val := mkBin(ctx, src.Pos(), opUnify, a, b)
	u := newValueRoot(ctx, val)
	if err := u.Validate(); err != nil {
		u = newValueRoot(ctx, ctx.mkErr(src, err))
	}
	return u
}

// Equals reports whether two values are equal, ignoring optional fields.
// The result is undefined for incomplete values.
func (v Value) Equals(other Value) bool {
	if v.path == nil || other.path == nil {
		return false
	}
	x := v.path.val()
	y := other.path.val()
	return equals(v.ctx(), x, y)
}

// Format prints a debug version of a value.
func (v Value) Format(state fmt.State, verb rune) {
	ctx := v.ctx()
	if v.path == nil {
		fmt.Fprint(state, "<nil>")
		return
	}
	_, _ = io.WriteString(state, ctx.str(v.path.cache))
}

// Reference returns the instance and path referred to by this value such that
// inst.Lookup(path) resolves to the same value, or no path if this value is not
// a reference. If a reference contains index selection (foo[bar]), it will
// only return a reference if the index resolves to a concrete value.
func (v Value) Reference() (inst *Instance, path []string) {
	// TODO: don't include references to hidden fields.
	if v.path == nil {
		return nil, nil
	}
	ctx := v.ctx()
	var x value
	var feature string
	switch sel := v.path.v.(type) {
	case *selectorExpr:
		x = sel.x
		feature = ctx.labelStr(sel.feature)

	case *indexExpr:
		e := sel.index.evalPartial(ctx)
		s, ok := e.(*stringLit)
		if !ok {
			return nil, nil
		}
		x = sel.x
		feature = s.str

	default:
		return nil, nil
	}
	imp, a := mkPath(ctx, v.path, x, feature, 0)
	return imp, a
}

func mkPath(c *context, up *valueData, x value, feature string, d int) (imp *Instance, a []string) {
	switch x := x.(type) {
	case *selectorExpr:
		imp, a = mkPath(c, up, x.x, c.labelStr(x.feature), d+1)
		if imp == nil {
			return nil, nil
		}

	case *indexExpr:
		e := x.index.evalPartial(c)
		s, ok := e.(*stringLit)
		if !ok {
			return nil, nil
		}
		imp, a = mkPath(c, up, x.x, s.str, d+1)
		if imp == nil {
			return nil, nil
		}

	case *nodeRef:
		// the parent must exist.
		for ; up != nil && up.cache != x.node.(value); up = up.parent {
		}
		var v value
		v, a = mkFromRoot(c, up, d+2)
		if v == nil {
			v = x.node
		}
		imp = c.getImportFromNode(v)
	default:
		return nil, nil
	}
	return imp, append(a, feature)
}

func mkFromRoot(c *context, up *valueData, d int) (root value, a []string) {
	if up == nil {
		return nil, make([]string, 0, d)
	}
	root, a = mkFromRoot(c, up.parent, d+1)
	if up.parent != nil {
		a = append(a, c.labelStr(up.feature))
	} else {
		root = up.v
	}
	return root, a
}

// References reports all references used to evaluate this value. It does not
// report references for sub fields if v is a struct.
//
// Deprecated: can be implemented in terms of Reference and Expr.
func (v Value) References() [][]string {
	// TODO: the pathFinder algorithm is quite broken. Using Reference and Expr
	// will cast a much more accurate net on referenced values.
	ctx := v.ctx()
	pf := pathFinder{up: v.path}
	raw := v.path.v
	if raw == nil {
		return nil
	}
	rewrite(ctx, raw, pf.find)
	return pf.paths
}

type pathFinder struct {
	paths [][]string
	stack []label
	up    *valueData
}

func (p *pathFinder) find(ctx *context, v value) (value, bool) {
	switch x := v.(type) {
	case *selectorExpr:
		i := len(p.stack)
		p.stack = append(p.stack, x.feature)
		rewrite(ctx, x.x, p.find)
		p.stack = p.stack[:i]
		return v, false

	case *nodeRef:
		i := len(p.stack)
		up := p.up
		for ; up != nil && up.cache != x.node.(value); up = up.parent {
		}
		for ; up != nil && up.feature > 0; up = up.parent {
			p.stack = append(p.stack, up.feature)
		}
		path := make([]string, len(p.stack))
		for i, v := range p.stack {
			path[len(path)-1-i] = ctx.labelStr(v)
		}
		p.paths = append(p.paths, path)
		p.stack = p.stack[:i]
		return v, false

	case *structLit:
		// If the stack is empty, we do not descend, as we are not evaluating
		// sub fields.
		if len(p.stack) == 0 {
			return v, false
		}

		stack := p.stack
		p.stack = nil
		for _, a := range x.arcs {
			rewrite(ctx, a.v, p.find)
		}
		p.stack = stack
		return v, false
	}
	return v, true
}

type options struct {
	concrete        bool // enforce that values are concrete
	raw             bool // show original values
	hasHidden       bool
	omitHidden      bool
	omitDefinitions bool
	omitOptional    bool
	omitAttrs       bool
	disallowCycles  bool // implied by concrete
}

// An Option defines modes of evaluation.
type Option option

type option func(p *options)

// Used in Iter, Validate, Subsume?, Fields() Syntax, Export

// TODO: could also be used for subsumption.

// Concrete ensures that all values are concrete.
//
// For Validate this means it returns an error if this is not the case.
// In other cases a non-concrete value will be replaced with an error.
func Concrete(concrete bool) Option {
	return func(p *options) {
		if concrete {
			p.concrete = true
			if !p.hasHidden {
				p.omitHidden = true
				p.omitDefinitions = true
			}
		}
	}
}

// DisallowCycles forces validation in the precense of cycles, even if
// non-concrete values are allowed. This is implied by Concrete(true).
func DisallowCycles(disallow bool) Option {
	return func(p *options) { p.disallowCycles = disallow }
}

// Raw tells Syntax to generate the value as is without any simplifications.
func Raw() Option {
	return func(p *options) { p.raw = true }
}

// All indicates that all fields and values should be included in processing
// even if they can be elided or omitted.
func All() Option {
	return func(p *options) {
		p.omitAttrs = false
		p.omitHidden = false
		p.omitDefinitions = false
		p.omitOptional = false
	}
}

// Definitions indicates whether definitions should be included.
//
// Definitions may still be included for certain functions if they are referred
// to by other other values.
func Definitions(include bool) Option {
	return func(p *options) {
		p.hasHidden = true
		p.omitDefinitions = !include
	}
}

// Hidden indicates that definitions and hidden fields should be included.
//
// Deprecated: Hidden fields are deprecated.
func Hidden(include bool) Option {
	return func(p *options) {
		p.hasHidden = true
		p.omitHidden = !include
		p.omitDefinitions = !include
	}
}

// Optional indicates that optional fields should be included.
func Optional(include bool) Option {
	return func(p *options) { p.omitOptional = !include }
}

// Attributes indicates that attributes should be included.
func Attributes(include bool) Option {
	return func(p *options) { p.omitAttrs = !include }
}

func getOptions(opts []Option) (o options) {
	o.updateOptions(opts)
	return
}

func (o *options) updateOptions(opts []Option) {
	for _, fn := range opts {
		fn(o)
	}
}

// Validate reports any errors, recursively. The returned error may represent
// more than one error, retrievable with errors.Errors, if more than one
// exists.
func (v Value) Validate(opts ...Option) error {
	x := validator{}
	o := options{}
	o.updateOptions(opts)
	// Logically, errors are always permitted in logical fields, so we
	// force-disable them.
	// TODO: consider whether we should honor the option to allow checking
	// optional fields.
	o.omitOptional = true
	x.walk(v, o)
	return errors.Sanitize(x.errs)
}

type validator struct {
	errs  errors.Error
	depth int
}

func (x *validator) before(v Value, o options) bool {
	if err := v.checkKind(v.ctx(), bottomKind); err != nil {
		if !o.concrete && isIncomplete(err) {
			if o.disallowCycles && err.code == codeCycle {
				x.errs = errors.Append(x.errs, v.toErr(err))
			}
			return false
		}
		x.errs = errors.Append(x.errs, v.toErr(err))
		if len(errors.Errors(x.errs)) > 50 {
			return false // mostly to avoid some hypothetical cycle issue
		}
	}
	if o.concrete {
		if err := isGroundRecursive(v.ctx(), v.eval(v.ctx())); err != nil {
			x.errs = errors.Append(x.errs, v.toErr(err))
		}
	}
	return true
}

func (x *validator) walk(v Value, opts options) {
	// TODO(#42): we can get rid of the arbitrary evaluation depth once CUE has
	// proper structural cycle detection. See Issue #42. Currently errors
	// occuring at a depth > 20 will not be detected.
	if x.depth > 20 {
		return
	}
	ctx := v.ctx()
	switch v.Kind() {
	case StructKind:
		if !x.before(v, opts) {
			return
		}
		x.depth++
		obj, err := v.structValOpts(ctx, opts)
		if err != nil {
			x.errs = errors.Append(x.errs, v.toErr(err))
		}
		for i := 0; i < obj.Len(); i++ {
			_, v := obj.At(i)
			opts := opts
			if obj.arcs[i].definition {
				opts.concrete = false
			}
			x.walk(v, opts)
		}
		x.depth--

	case ListKind:
		if !x.before(v, opts) {
			return
		}
		x.depth++
		list, _ := v.List()
		for list.Next() {
			x.walk(list.Value(), opts)
		}
		x.depth--

	default:
		x.before(v, opts)
	}
}

func isGroundRecursive(ctx *context, v value) *bottom {
	switch x := v.(type) {
	case *list:
		for i := 0; i < len(x.elem.arcs); i++ {
			v := ctx.manifest(x.at(ctx, i))
			if err := isGroundRecursive(ctx, v); err != nil {
				return err
			}
		}
	default:
		if !x.kind().isGround() {
			return ctx.mkErr(v, "incomplete value (%v)", ctx.str(v))
		}
	}
	return nil
}

// Walk descends into all values of v, calling f. If f returns false, Walk
// will not descent further. It only visits values that are part of the data
// model, so this excludes optional fields, hidden fields, and definitions.
func (v Value) Walk(before func(Value) bool, after func(Value)) {
	ctx := v.ctx()
	switch v.Kind() {
	case StructKind:
		if before != nil && !before(v) {
			return
		}
		obj, _ := v.structValData(ctx)
		for i := 0; i < obj.Len(); i++ {
			_, v := obj.At(i)
			v.Walk(before, after)
		}
	case ListKind:
		if before != nil && !before(v) {
			return
		}
		list, _ := v.List()
		for list.Next() {
			list.Value().Walk(before, after)
		}
	default:
		if before != nil {
			before(v)
		}
	}
	if after != nil {
		after(v)
	}
}

// Attribute returns the attribute data for the given key.
// The returned attribute will return an error for any of its methods if there
// is no attribute for the requested key.
func (v Value) Attribute(key string) Attribute {
	const msgNotExist = "attribute %q does not exist"
	// look up the attributes
	if v.path == nil || v.path.attrs == nil {
		return Attribute{err: errors.Newf(token.NoPos, msgNotExist, key)}
	}
	for _, a := range v.path.attrs.attr {
		if a.key() != key {
			continue
		}
		at := Attribute{}
		if err := parseAttrBody(v.ctx(), nil, a.body(), &at.attr); err != nil {
			return Attribute{err: v.toErr(err)}
		}
		return at
	}
	return Attribute{err: errors.Newf(token.NoPos, msgNotExist, key)}
}

// An Attribute contains meta data about a field.
type Attribute struct {
	attr parsedAttr
	err  error
}

// Err returns the error associated with this Attribute or nil if this
// attribute is valid.
func (a *Attribute) Err() error {
	return a.err
}

func (a *Attribute) hasPos(p int) error {
	if a.err != nil {
		return a.err
	}
	if p >= len(a.attr.fields) {
		return fmt.Errorf("field does not exist")
	}
	return nil
}

// String reports the possibly empty string value at the given position or
// an error the attribute is invalid or if the position does not exist.
func (a *Attribute) String(pos int) (string, error) {
	if err := a.hasPos(pos); err != nil {
		return "", err
	}
	return a.attr.fields[pos].text(), nil
}

// Int reports the integer at the given position or an error if the attribute is
// invalid, the position does not exist, or the value at the given position is
// not an integer.
func (a *Attribute) Int(pos int) (int64, error) {
	if err := a.hasPos(pos); err != nil {
		return 0, err
	}
	// TODO: use CUE's literal parser once it exists, allowing any of CUE's
	// number types.
	return strconv.ParseInt(a.attr.fields[pos].text(), 10, 64)
}

// Flag reports whether an entry with the given name exists at position pos or
// onwards or an error if the attribute is invalid or if the first pos-1 entries
// are not defined.
func (a *Attribute) Flag(pos int, key string) (bool, error) {
	if err := a.hasPos(pos - 1); err != nil {
		return false, err
	}
	for _, kv := range a.attr.fields[pos:] {
		if kv.text() == key {
			return true, nil
		}
	}
	return false, nil
}

// Lookup searches for an entry of the form key=value from position pos onwards
// and reports the value if found. It reports an error if the attribute is
// invalid or if the first pos-1 entries are not defined.
func (a *Attribute) Lookup(pos int, key string) (val string, found bool, err error) {
	if err := a.hasPos(pos - 1); err != nil {
		return "", false, err
	}
	for _, kv := range a.attr.fields[pos:] {
		if kv.key() == key {
			return kv.value(), true, nil
		}
	}
	return "", false, nil
}

// Expr reports the operation of the underlying expression and the values it
// operates on.
//
// For unary expressions, it returns the single value of the expression.
//
// For binary expressions it returns first the left and right value, in that
// order. For associative operations however, (for instance '&' and '|'), it may
// return more than two values, where the operation is to be applied in
// sequence.
//
// For selector and index expressions it returns the subject and then the index.
// For selectors, the index is the string value of the identifier.
//
// For interpolations it returns a sequence of values to be concatenated, some
// of which will be literal strings and some unevaluated expressions.
//
// A builtin call expression returns the value of the builtin followed by the
// args of the call.
func (v Value) Expr() (Op, []Value) {
	// TODO: return v if this is complete? Yes for now
	if v.path == nil {
		return NoOp, nil
	}
	// TODO: replace appends with []Value{}. For not leave.
	a := []Value{}
	op := NoOp
	switch x := v.path.v.(type) {
	case *binaryExpr:
		a = append(a, remakeValue(v, x.left))
		a = append(a, remakeValue(v, x.right))
		op = opToOp[x.op]
	case *unaryExpr:
		a = append(a, remakeValue(v, x.x))
		op = opToOp[x.op]
	case *bound:
		a = append(a, remakeValue(v, x.value))
		op = opToOp[x.op]
	case *unification:
		// pre-expanded unification
		for _, conjunct := range x.values {
			a = append(a, remakeValue(v, conjunct))
		}
		op = AndOp
	case *disjunction:
		// Filter defaults that are subsumed by another value.
		count := 0
	outer:
		for _, disjunct := range x.values {
			if disjunct.marked {
				for _, n := range x.values {
					if !n.marked && subsumes(v.ctx(), n.val, disjunct.val, 0) {
						continue outer
					}
				}
			}
			count++
			a = append(a, remakeValue(v, disjunct.val))
		}
		if count > 1 {
			op = OrOp
		}
	case *interpolation:
		for _, p := range x.parts {
			a = append(a, remakeValue(v, p))
		}
		op = InterpolationOp
	case *selectorExpr:
		a = append(a, remakeValue(v, x.x))
		a = append(a, remakeValue(v, &stringLit{
			x.baseValue,
			v.ctx().labelStr(x.feature),
			nil,
		}))
		op = SelectorOp
	case *indexExpr:
		a = append(a, remakeValue(v, x.x))
		a = append(a, remakeValue(v, x.index))
		op = IndexOp
	case *sliceExpr:
		a = append(a, remakeValue(v, x.x))
		a = append(a, remakeValue(v, x.lo))
		a = append(a, remakeValue(v, x.hi))
		op = SliceOp
	case *callExpr:
		a = append(a, remakeValue(v, x.x))
		for _, arg := range x.args {
			a = append(a, remakeValue(v, arg))
		}
		op = CallOp
	case *customValidator:
		a = append(a, remakeValue(v, x.call))
		for _, arg := range x.args {
			a = append(a, remakeValue(v, arg))
		}
		op = CallOp
	default:
		a = append(a, v)
	}
	return op, a
}
