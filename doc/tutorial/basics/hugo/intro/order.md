+++
title = "Order is irrelevant"
description = ""
weight = 2030
layout = "tutorial"
+++
CUE's basic operations are defined in a way that the order in which
you combine two configurations is irrelevant to the outcome.

This is crucial property of CUE
that makes it easy for humans _and_ machines to reason over values and
makes advanced tooling and automation possible.

If you can think of an example where order matters, it is not valid CUE.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>order.cue</i>
<p>
{{< highlight go >}}
a: {x: 1, y: int}
a: {x: int, y: 2}

b: {x: int, y: 2}
b: {x: 1, y: int}

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval -i order.cue</i>
<p>
{{< highlight go >}}
a: {
    x: 1
    y: 2
}
b: {
    x: 1
    y: 2
}
{{< /highlight >}}
</div>
</section>