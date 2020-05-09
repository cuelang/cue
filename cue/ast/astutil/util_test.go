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

package astutil

import (
	"path"
	"testing"

	"cuelang.org/go/cue/ast"
)

func TestImportInfo(t *testing.T) {
	testCases := []struct {
		want string
		name string
		path string
	}{
		{"bar", "", "a.b/bar"},
		{"foo", "foo", "a.b/bar"},
		{"foo", "", "a.b/bar:foo"},
		{"strings", "", "strings"},
	}
	for _, tc := range testCases {
		t.Run(path.Join(tc.name, tc.path), func(t *testing.T) {
			var ident *ast.Ident
			if tc.name != "" {
				ident = ast.NewIdent(tc.name)
			}
			got, id := ImportInfo(&ast.ImportSpec{
				Name: ident,
				Path: ast.NewString(tc.path),
			})
			if got != tc.want || id != tc.path {
				t.Errorf("got %s %q; want %s %q", got, id, tc.want, tc.path)
			}
		})
	}
}
