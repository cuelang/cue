// Code generated by go generate. DO NOT EDIT.

//go:generate rm pkg.go
//go:generate go run ../../gen/gen.go

package hmac

import (
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/pkg/internal"
)

func init() {
	internal.Register("crypto/hmac", pkg)
}

var _ = adt.TopKind // in case the adt package isn't used

var pkg = &internal.Package{
	Native: []*internal.Builtin{{
		Name:  "MD5",
		Const: "\"MD5\"",
	}, {
		Name:  "SHA1",
		Const: "\"SHA1\"",
	}, {
		Name:  "SHA224",
		Const: "\"SHA224\"",
	}, {
		Name:  "SHA256",
		Const: "\"SHA256\"",
	}, {
		Name:  "SHA384",
		Const: "\"SHA384\"",
	}, {
		Name:  "SHA512",
		Const: "\"SHA512\"",
	}, {
		Name:  "SHA512_224",
		Const: "\"SHA512_224\"",
	}, {
		Name:  "SHA512_256",
		Const: "\"SHA512_256\"",
	}, {
		Name: "Sign",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.BytesKind | adt.StringKind},
			{Kind: adt.BytesKind | adt.StringKind},
		},
		Result: adt.BytesKind | adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			hashName, data, key := c.String(0), c.Bytes(1), c.Bytes(2)
			if c.Do() {
				c.Ret, c.Err = Sign(hashName, data, key)
			}
		},
	}},
}
