+++
title = "Duplicates"
description = ""
weight = 2035
layout = "tutorial"
+++
Constraints specify what values are allowed.
To CUE they are just values like anything else,
but conceptually they can be explained as something in between types and
concrete values.

Constraints can be used to validate values of concrete instances.
They can be applied to CUE data, or directly to YAML or JSON.

But constraints can also reduce boilerplate.
If a constraints defines a concrete value, there is no need
to specify it in values to which this constraint applies.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>dup.cue</i>
<p>
{{< highlight go >}}
a: 4
a: 4

s: {
    x: 1
}
s: {
    y: 2
}

l: [ 1, 2 ]
l: [ 1, 2 ]

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval dup.cue</i>
<p>
{{< highlight go >}}
a: 4
s: {
    x: 1
    y: 2
}
l: [1, 2]
{{< /highlight >}}
</div>
</section>