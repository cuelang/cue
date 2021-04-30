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

package load

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal/diff"
)

type testTagger map[string]ast.Expr

func (tt testTagger) Tag(name string) (ast.Expr, error) {
	return tt[name], nil
}

var tt = testTagger{
	"now":      ast.NewString("2021-04-30 14:22:41.280316 +0000 UTC"),
	"os":       ast.NewString("m1"),
	"cwd":      ast.NewString("home"),
	"username": ast.NewString("cueser"),
	"hostname": ast.NewString("cuebe"),
	"rand":     ast.NewLit(token.INT, "112950970371208119678246559335704039641"),
}

func TestTags(t *testing.T) {
	dir, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(dir)

	testCases := []struct {
		in  string
		out string
		err string
	}{{
		in: `
		rand: int    @tag(foo,var=rand)
		time: string @tag(bar,var=now)
		host: string @tag(bar,var=hostname)
		user: string @tag(bar,var=username)
		cwd:  string @tag(bar,var=cwd)
		`,

		out: `{
			rand: 112950970371208119678246559335704039641
			time: "2021-04-30 14:22:41.280316 +0000 UTC"
			host: "cuebe"
			user: "cueser"
			cwd:  "home"
		}`,
	}, {
		in: `
		time: int @tag(bar,var=now)
		`,
		err: `time: conflicting values int and "2021-04-30 14:22:41.280316 +0000 UTC" (mismatched types int and string)`,
	}, {
		// Auto inject only on marked places
		// TODO: Is this the right thing to do?
		in: `
			u1: string @tag(bar,var=username)
			u2: string @tag(bar)
			`,
		out: `{
			u1: "cueser"
            u2: string // not filled
        }`,
	}, {
		in: `
		u1: string @tag(bar,var=user)
		`,
		err: `tag variable 'user' not found`,
	}}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			cfg := &Config{
				Dir: dir,
				Overlay: map[string]Source{
					filepath.Join(dir, "foo.cue"): FromString(tc.in),
				},
				TagVars: tt,
			}
			b := Instances([]string{"foo.cue"}, cfg)[0]

			c := cuecontext.New()
			got := c.BuildInstance(b)
			switch err := got.Err(); {
			case (err == nil) != (tc.err == ""):
				t.Fatalf("error: got %v; want %v", err, tc.err)

			case err != nil:
				got := err.Error()
				if got != tc.err {
					t.Fatalf("error: got %v; want %v", got, tc.err)
				}

			default:
				want := c.CompileString(tc.out)
				if !got.Equals(want) {
					_, es := diff.Diff(got, want)
					b := &bytes.Buffer{}
					diff.Print(b, es)
					t.Error(b)
				}
			}
		})
	}
}
