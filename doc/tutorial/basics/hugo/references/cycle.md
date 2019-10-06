+++
title = "Reference Cycles"
description = ""
weight = 2080
layout = "tutorial"
+++
CUE can handle many types of cycles just fine.
Because all values are final, a field with a concrete value of, say `200`,
can only be valid if it is that value.
So if it is unified with another expression, we can delay the evaluation of
this until later.

By postponing that evaluation, we can often break cycles.
This is very useful for template writers that may not know what fields
a user will want to fill out.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>cycle.cue</i>
<p>
{{< highlight go >}}
// CUE knows how to resolve the following:
x: 200
x: y + 100
y: x - 100

// If a cycle is not broken, CUE will just report it.
a: b + 100
b: a - 100

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval -i -c cycle.cue</i>
<p>
{{< highlight go >}}
x: 200
y: 100
a: _|_ // cycle detected
b: _|_ // cycle detected
{{< /highlight >}}
</div>
</section>