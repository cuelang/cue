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

package path

import (
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/pkg/internal"
)

func init() {
	internal.Register("path", pkg)
}

var _ = adt.TopKind // in case the adt package isn't used

var (
	unixStr    = &adt.String{Str: "unix"}    // default for most funcs
	windowsStr = &adt.String{Str: "windows"} // default for VolumeName
)

var pkg = &internal.Package{
	Native: []*internal.Builtin{{
		Name: "Split",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.ListKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = Split(path, OS(os))
			}
		},
	}, {
		Name: "SplitList",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind},
		},
		Result: adt.ListKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = SplitList(path, OS(os))
			}
		},
	}, {
		Name: "Join",
		Params: []internal.Param{
			{Kind: adt.ListKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			list, os := c.StringList(0), c.String(1)
			if c.Do() {
				c.Ret = Join(list, OS(os))
			}
		},
	}, {
		Name: "Match",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.BoolKind,
		Func: func(c *internal.CallCtxt) {
			pattern, name, os := c.String(0), c.String(1), c.String(2)
			if c.Do() {
				c.Ret, c.Err = Match(pattern, name, OS(os))
			}
		},
	}, {
		Name: "Clean",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = Clean(path, OS(os))
			}
		},
	}, {
		Name: "ToSlash",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = ToSlash(path, OS(os))
			}
		},
	}, {
		Name: "FromSlash",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = FromSlash(path, OS(os))
			}
		},
	}, {
		Name: "Ext",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = Ext(path, OS(os))
			}
		},
	}, {
		Name: "Resolve",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			dir, sub, os := c.String(0), c.String(1), c.String(2)
			if c.Do() {
				c.Ret = Resolve(dir, sub, OS(os))
			}
		},
	}, {
		Name: "Rel",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			base, target, os := c.String(0), c.String(1), c.String(2)
			if c.Do() {
				c.Ret, c.Err = Rel(base, target, OS(os))
			}
		},
	}, {
		Name: "Base",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = Base(path, OS(os))
			}
		},
	}, {
		Name: "Dir",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = Dir(path, OS(os))
			}
		},
	}, {
		Name: "IsAbs",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: unixStr},
		},
		Result: adt.BoolKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = IsAbs(path, OS(os))
			}
		},
	}, {
		Name: "VolumeName",
		Params: []internal.Param{
			{Kind: adt.StringKind},
			{Kind: adt.StringKind, Default: windowsStr},
		},
		Result: adt.StringKind,
		Func: func(c *internal.CallCtxt) {
			path, os := c.String(0), c.String(1)
			if c.Do() {
				c.Ret = VolumeName(path, OS(os))
			}
		},
	}},
}
