package native

import (
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/eval"
	"cuelang.org/go/internal/core/runtime"
)

// Register registry custom packages
func Register(packages ...Package) {
	for i := range packages {
		p := packages[i]

		if p == nil {
			continue
		}

		ip := newInternalPackage(packages[i])

		if ip == nil {
			continue
		}

		importPath := p.ImportPath()

		runtime.RegisterBuiltin(importPath, func(r adt.Runtime) (*adt.Vertex, errors.Error) {
			ctx := eval.NewContext(r, nil)
			return ip.MustCompile(ctx, importPath), nil
		})

	}
}
