// Code generated by go generate. DO NOT EDIT.

//go:generate rm pkg.go
//go:generate go run ../gen/gen.go

package math

import (
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/pkg/internal"
)

func init() {
	internal.Register("math", pkg)
}

var _ = adt.TopKind // in case the adt package isn't used

var pkg = &internal.Package{
	Native: []*internal.Builtin{{
		Name:  "MaxExp",
		Const: "2147483647",
	}, {
		Name:  "MinExp",
		Const: "-2147483648",
	}, {
		Name:  "MaxPrec",
		Const: "4294967295",
	}, {
		Name:  "ToNearestEven",
		Const: "0",
	}, {
		Name:  "ToNearestAway",
		Const: "1",
	}, {
		Name:  "ToZero",
		Const: "2",
	}, {
		Name:  "AwayFromZero",
		Const: "3",
	}, {
		Name:  "ToNegativeInf",
		Const: "4",
	}, {
		Name:  "ToPositiveInf",
		Const: "5",
	}, {
		Name:  "Below",
		Const: "-1",
	}, {
		Name:  "Exact",
		Const: "0",
	}, {
		Name:  "Above",
		Const: "1",
	}, {
		Name:   "Jacobi",
		Params: []adt.Kind{adt.IntKind, adt.IntKind},
		Result: adt.IntKind,
		Func: func(c *internal.CallCtxt) {
			x, y := c.BigInt(0), c.BigInt(1)
			if c.Do() {
				c.Ret = Jacobi(x, y)
			}
		},
	}, {
		Name:  "MaxBase",
		Const: "62",
	}, {
		Name:   "Floor",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.IntKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Floor(x)
			}
		},
	}, {
		Name:   "Ceil",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.IntKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Ceil(x)
			}
		},
	}, {
		Name:   "Trunc",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.IntKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Trunc(x)
			}
		},
	}, {
		Name:   "Round",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.IntKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Round(x)
			}
		},
	}, {
		Name:   "RoundToEven",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.IntKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = RoundToEven(x)
			}
		},
	}, {
		Name:   "MultipleOf",
		Params: []adt.Kind{adt.NumKind, adt.NumKind},
		Result: adt.BoolKind,
		Func: func(c *internal.CallCtxt) {
			x, y := c.Decimal(0), c.Decimal(1)
			if c.Do() {
				c.Ret, c.Err = MultipleOf(x, y)
			}
		},
	}, {
		Name:   "Abs",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Abs(x)
			}
		},
	}, {
		Name:   "Acosh",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Acosh(x)
			}
		},
	}, {
		Name:   "Asin",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Asin(x)
			}
		},
	}, {
		Name:   "Acos",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Acos(x)
			}
		},
	}, {
		Name:   "Asinh",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Asinh(x)
			}
		},
	}, {
		Name:   "Atan",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Atan(x)
			}
		},
	}, {
		Name:   "Atan2",
		Params: []adt.Kind{adt.NumKind, adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			y, x := c.Float64(0), c.Float64(1)
			if c.Do() {
				c.Ret = Atan2(y, x)
			}
		},
	}, {
		Name:   "Atanh",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Atanh(x)
			}
		},
	}, {
		Name:   "Cbrt",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Cbrt(x)
			}
		},
	}, {
		Name:  "E",
		Const: "2.71828182845904523536028747135266249775724709369995957496696763",
	}, {
		Name:  "Pi",
		Const: "3.14159265358979323846264338327950288419716939937510582097494459",
	}, {
		Name:  "Phi",
		Const: "1.61803398874989484820458683436563811772030917980576286213544861",
	}, {
		Name:  "Sqrt2",
		Const: "1.41421356237309504880168872420969807856967187537694807317667974",
	}, {
		Name:  "SqrtE",
		Const: "1.64872127070012814684865078781416357165377610071014801157507931",
	}, {
		Name:  "SqrtPi",
		Const: "1.77245385090551602729816748334114518279754945612238712821380779",
	}, {
		Name:  "SqrtPhi",
		Const: "1.27201964951406896425242246173749149171560804184009624861664038",
	}, {
		Name:  "Ln2",
		Const: "0.693147180559945309417232121458176568075500134360255254120680009",
	}, {
		Name:  "Log2E",
		Const: "1.442695040888963407359924681001892137426645954152985934135449408",
	}, {
		Name:  "Ln10",
		Const: "2.3025850929940456840179914546843642076011014886287729760333278",
	}, {
		Name:  "Log10E",
		Const: "0.43429448190325182765112891891660508229439700580366656611445378",
	}, {
		Name:   "Copysign",
		Params: []adt.Kind{adt.NumKind, adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x, y := c.Decimal(0), c.Decimal(1)
			if c.Do() {
				c.Ret = Copysign(x, y)
			}
		},
	}, {
		Name:   "Dim",
		Params: []adt.Kind{adt.NumKind, adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x, y := c.Decimal(0), c.Decimal(1)
			if c.Do() {
				c.Ret, c.Err = Dim(x, y)
			}
		},
	}, {
		Name:   "Erf",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Erf(x)
			}
		},
	}, {
		Name:   "Erfc",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Erfc(x)
			}
		},
	}, {
		Name:   "Erfinv",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Erfinv(x)
			}
		},
	}, {
		Name:   "Erfcinv",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Erfcinv(x)
			}
		},
	}, {
		Name:   "Exp",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Exp(x)
			}
		},
	}, {
		Name:   "Exp2",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Exp2(x)
			}
		},
	}, {
		Name:   "Expm1",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Expm1(x)
			}
		},
	}, {
		Name:   "Gamma",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Gamma(x)
			}
		},
	}, {
		Name:   "Hypot",
		Params: []adt.Kind{adt.NumKind, adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			p, q := c.Float64(0), c.Float64(1)
			if c.Do() {
				c.Ret = Hypot(p, q)
			}
		},
	}, {
		Name:   "J0",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = J0(x)
			}
		},
	}, {
		Name:   "Y0",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Y0(x)
			}
		},
	}, {
		Name:   "J1",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = J1(x)
			}
		},
	}, {
		Name:   "Y1",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Y1(x)
			}
		},
	}, {
		Name:   "Jn",
		Params: []adt.Kind{adt.IntKind, adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			n, x := c.Int(0), c.Float64(1)
			if c.Do() {
				c.Ret = Jn(n, x)
			}
		},
	}, {
		Name:   "Yn",
		Params: []adt.Kind{adt.IntKind, adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			n, x := c.Int(0), c.Float64(1)
			if c.Do() {
				c.Ret = Yn(n, x)
			}
		},
	}, {
		Name:   "Ldexp",
		Params: []adt.Kind{adt.NumKind, adt.IntKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			frac, exp := c.Float64(0), c.Int(1)
			if c.Do() {
				c.Ret = Ldexp(frac, exp)
			}
		},
	}, {
		Name:   "Log",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Log(x)
			}
		},
	}, {
		Name:   "Log10",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Log10(x)
			}
		},
	}, {
		Name:   "Log2",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret, c.Err = Log2(x)
			}
		},
	}, {
		Name:   "Log1p",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Log1p(x)
			}
		},
	}, {
		Name:   "Logb",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Logb(x)
			}
		},
	}, {
		Name:   "Ilogb",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.IntKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Ilogb(x)
			}
		},
	}, {
		Name:   "Mod",
		Params: []adt.Kind{adt.NumKind, adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x, y := c.Float64(0), c.Float64(1)
			if c.Do() {
				c.Ret = Mod(x, y)
			}
		},
	}, {
		Name:   "Pow",
		Params: []adt.Kind{adt.NumKind, adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x, y := c.Decimal(0), c.Decimal(1)
			if c.Do() {
				c.Ret, c.Err = Pow(x, y)
			}
		},
	}, {
		Name:   "Pow10",
		Params: []adt.Kind{adt.IntKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			n := c.Int32(0)
			if c.Do() {
				c.Ret = Pow10(n)
			}
		},
	}, {
		Name:   "Remainder",
		Params: []adt.Kind{adt.NumKind, adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x, y := c.Float64(0), c.Float64(1)
			if c.Do() {
				c.Ret = Remainder(x, y)
			}
		},
	}, {
		Name:   "Signbit",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.BoolKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Decimal(0)
			if c.Do() {
				c.Ret = Signbit(x)
			}
		},
	}, {
		Name:   "Cos",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Cos(x)
			}
		},
	}, {
		Name:   "Sin",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Sin(x)
			}
		},
	}, {
		Name:   "Sinh",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Sinh(x)
			}
		},
	}, {
		Name:   "Cosh",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Cosh(x)
			}
		},
	}, {
		Name:   "Sqrt",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Sqrt(x)
			}
		},
	}, {
		Name:   "Tan",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Tan(x)
			}
		},
	}, {
		Name:   "Tanh",
		Params: []adt.Kind{adt.NumKind},
		Result: adt.NumKind,
		Func: func(c *internal.CallCtxt) {
			x := c.Float64(0)
			if c.Do() {
				c.Ret = Tanh(x)
			}
		},
	}},
}
