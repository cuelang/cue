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

package cue

import (
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/compile"
)

// This file contains query-related code.

// getScopePrefix finds the Vertex that exists in v for the longest prefix of p.
//
// It is used to make the parent scopes visible when resolving expressions.
func getScopePrefix(v Value, p Path) *adt.Vertex {
	for _, sel := range p.Selectors() {
		w := v.LookupPath(MakePath(sel))
		if !w.Exists() {
			break
		}
		v = w
	}
	return v.v
}

func errFn(pos token.Pos, msg string, args ...interface{}) {}

// resolveExpr binds unresolved expressions to values in the expression or v.
func resolveExpr(ctx *context, v *adt.Vertex, x ast.Expr) adt.Value {
	cfg := &compile.Config{Scope: v}

	astutil.ResolveExpr(x, errFn)

	c, err := compile.Expr(cfg, ctx.opCtx, pkgID(), x)
	if err != nil {
		return &adt.Bottom{Err: err}
	}
	return adt.Resolve(ctx.opCtx, c)
}
