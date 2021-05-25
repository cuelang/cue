// Code generated by go generate. DO NOT EDIT.

//go:generate rm pkg.go
//go:generate go run ../../gen/gen.go

package exec

import (
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/pkg/internal"
)

func init() {
	internal.Register("tool/exec", pkg)
}

var _ = adt.TopKind // in case the adt package isn't used

var pkg = &internal.Package{
	Native: []*internal.Builtin{},
	CUE: `{
	Run: {
		$id: *"tool/exec.Run" | "exec"
		cmd: string | [string, ...string]
		dir: *null | string
		env: {
			[string]: string | [...=~"="]
		}
		stdout:  *null | string | bytes
		stderr:  *null | string | bytes
		stdin:   *null | string | bytes
		success: bool
	}
}`,
}
