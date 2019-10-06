+++
title = "Basic Types"
description = ""
weight = 2030
layout = "tutorial"
+++
CUE defines the following basic types

```
null bool string bytes int float
```
in addition to the error type mentioned in the previous section.

CUE does not distinguish between types and values.
A field value can be a type (using one of the above names), a concrete value,
or, in case of composite types (lists and structs), anything in between.

In the example, `point` defines an arbitrary point, while `xaxis` and `yaxis`
define the points on the respective lines.
We say that `point`, `xaxis`, and `yaxis` are abstract points, as these
points are underspecified.
Such abstract values cannot be represented as JSON,
which requires all values to be concrete.

The only concrete point is `origin`.
The `origin` is defined to be both on the x-axis and y-axis, which means it
must be at `0, 0`.
Here we see constraints in action:
`origin` evalutes to `0, 0`, even though we did not specify its coordinates
explicitly.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>types.cue</i>
<p>
{{< highlight go >}}
point: {
    x: number
    y: number
}

xaxis: point
xaxis x: 0

yaxis: point
yaxis y: 0

origin: xaxis & yaxis

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval types.cue</i>
<p>
{{< highlight go >}}
point: {
    x: number
    y: number
}
xaxis: {
    x: 0
    y: number
}
yaxis: {
    x: number
    y: 0
}
origin: {
    x: 0
    y: 0
}
{{< /highlight >}}
</div>
</section>