+++
title = "Cycles in Fields"
description = ""
weight = 2085
layout = "tutorial"
+++
Also, we know that unifying a field with itself will result in the same value.
Thus if we have a cycle between some fields, all we need to do is ignore
the cycle and unify their values once to achieve the same result as
merging them ad infinitum.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>cycleref.cue</i>
<p>
{{< highlight go >}}
labels: selectors
labels: {app: "foo"}

selectors: labels
selectors: {name: "bar"}

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval cycleref.cue</i>
<p>
{{< highlight go >}}
labels: {
    name: "bar"
    app:  "foo"
}
selectors: {
    name: "bar"
    app:  "foo"
}
{{< /highlight >}}
</div>
</section>