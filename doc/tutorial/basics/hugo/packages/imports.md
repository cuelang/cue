+++
title = "Imports"
description = ""
weight = 2030
layout = "tutorial"
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


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>imports.cue</i>
<p>
{{< highlight go >}}
import (
	"encoding/json"
	"math"
)

data: json.Marshal({ a: math.Sqrt(7) })

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval imports.cue</i>
<p>
{{< highlight go >}}
data: "{\"a\":2.6457513110645907}"
{{< /highlight >}}
</div>
</section>