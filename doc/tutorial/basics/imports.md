[TOC](Readme.md) [Prev](packages.md) [Next](operators.md)

_Modules, Packages, and Instances_

# Imports

A CUE file may import definitions from builtin or user-defined packages.
A CUE file does not need to be part of a package to use imports.

The example here shows the use of builtin packages.

This code groups the imports into a parenthesized, "factored" import statement.

You can also write multiple import statements, like:

```
import "encoding/json"
import "math"
```

But it is good style to use the factored import statement.

<!-- CUE editor -->
_imports.cue:_
```
import (
	"encoding/json"
	"math"
)

data: json.Marshal({ a: math.Sqrt(7) })
```

<!-- result -->
`$ cue eval imports.cue`
```
data: "{\"a\":2.6457513110645907}"
```

The following example shows the use of user-defined packages.

To be able to use user-defined packages a directory layout is asumed by the cue tool.
Your project root needs to be marked by cue.mod file (which is allowed to be empty).
Alongside your cue.mod file a pkg directory needs to be present.

You can create the directory layout as follows:

```sh
touch cue.mod
mkdir -p pkg/cuelang.or/example
```

The first path component is used as a identifier
to distinguish core packages from non-core packages.
Non-core packages need to start with a domain name.
In our example this would be `cuelang.org`.

_pkg/cuelang.org/example/example.cue:_
```
package example

foo: 100
```

_a.cue:_
```
package a

import "cuelang.org/example"

bar: foo
```

`$ cue eval a.cue`
```
bar: {
    foo: 100
}
```
