+++
title = "Packages"
description = ""
weight = 2010
layout = "tutorial"
+++
A CUE file is a standalone file by default.
A `package` clause allows a single configuration to be split across multiple
files.

The configuration for a package is defined by the concatenation of all its
files, after stripping the package clauses and not considering imports.

Duplicate definitions are treated analogously to duplicate definitions within
the same file.
The order in which files are loaded is undefined, but any order will result
in the same outcome, given that order does not matter.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>a.cue</i>
<p>
{{< highlight go >}}
package config

foo: 100
bar: int

{{< /highlight >}}
<br>
<i>b.cue</i>
<p>
{{< highlight go >}}
package config

bar: 200

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval a.cue b.cue</i>
<p>
{{< highlight go >}}
foo: 100
bar: 200
{{< /highlight >}}
</div>
</section>