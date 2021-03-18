package native

import (
	"encoding/json"
	"fmt"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/token"
)

func init() {
	Register(custom{})
}

type custom struct {
}

func (custom) ImportPath() string {
	return "extension/custom"
}

func (custom) StringConst() string {
	return "STRING"
}

func (custom) CustomFormat(v map[string]interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func Test(t *testing.T) {
	test := func(pkg, expr string) []*bimport {
		return []*bimport{{"",
			[]string{fmt.Sprintf("import %q\n(%s)", pkg, expr)},
		}}
	}

	testCases := []struct {
		instances []*bimport
		emit      string
	}{
		{
			test("extension/custom", "custom.CustomFormat({ a: 1 })"),
			`"eyJhIjoxfQ=="`,
		},
		{
			test("extension/custom", "custom.StringConst"),
			`"STRING"`,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			insts := cue.Build(makeInstances(tc.instances))
			if err := insts[0].Err; err != nil {
				t.Fatal(err)
			}
			v := insts[0].Value()
			got := fmt.Sprintf("%+v", v)
			if got != tc.emit {
				t.Errorf("\n got: %s\nwant: %s", got, tc.emit)
			}
		})
	}
}

type bimport struct {
	path  string // "" means top-level
	files []string
}

type builder struct {
	ctxt    *build.Context
	imports map[string]*bimport
}

func (b *builder) load(pos token.Pos, path string) *build.Instance {
	bi := b.imports[path]
	if bi == nil {
		return nil
	}
	return b.build(bi)
}

func makeInstances(insts []*bimport) (instances []*build.Instance) {
	b := builder{
		ctxt:    build.NewContext(),
		imports: map[string]*bimport{},
	}
	for _, bi := range insts {
		if bi.path != "" {
			b.imports[bi.path] = bi
		}
	}
	for _, bi := range insts {
		if bi.path == "" {
			instances = append(instances, b.build(bi))
		}
	}
	return
}

func (b *builder) build(bi *bimport) *build.Instance {
	path := bi.path
	if path == "" {
		path = "dir"
	}
	p := b.ctxt.NewInstance(path, b.load)
	for i, f := range bi.files {
		_ = p.AddFile(fmt.Sprintf("file%d.cue", i), f)
	}
	_ = p.Complete()
	return p
}
