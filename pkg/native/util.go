package native

import (
	"cuelang.org/go/internal/core/runtime"
)

func IsBuiltinPackage(importPath string) bool {
	return runtime.SharedRuntime.IsBuiltinPackage(importPath)
}
