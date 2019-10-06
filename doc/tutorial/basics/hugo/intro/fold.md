+++
title = "Folding of Single-Field Structs"
description = ""
weight = 2040
layout = "tutorial"
+++
In JSON, one defines nested values, one value at a time.
Another way to look at this is that a JSON configuration is a set of
path-value pairs.

In CUE one defines a set of paths to which to apply
a concrete value or constraint all at once.
Because or CUE's order independence, values get merged

This example shows some path-value pairs, as well as
a constraint that is applied to those to validate them.
<!--
This also gives a handy shorthand for writing structs with single
members.
-->


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>fold.cue</i>
<p>
{{< highlight go >}}
// path-value pairs
outer middle1 inner: 3
outer middle2 inner: 7

// collection-constraint pair
outer <Any> inner: int

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue export fold.cue</i>
<p>
{{< highlight go >}}
{
    "outer": {
        "middle1": {
            "inner": 3
        },
        "middle2": {
            "inner": 7
        }
    }
}
{{< /highlight >}}
</div>
</section>