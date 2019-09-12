+++
title = "Imports"
weight = 2000
exec = "cue eval imports.cue"
+++

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

