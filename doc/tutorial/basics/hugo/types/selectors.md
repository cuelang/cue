+++
title = "Accessing Fields"
description = ""
weight = 2020
layout = "tutorial"
+++
Selectors access fields within a struct using the `.` notation.
This only works if a field name is a valid identifier and it is not computed.
For other cases one can use the indexing notation.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>selectors.cue</i>
<p>
{{< highlight go >}}
a: {
    b: 2
    "c-e": 5
}
v: a.b
w: a["c-e"]

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval selectors.cue</i>
<p>
{{< highlight go >}}
a: {
    b:     2
    "c-e": 5
}
v: 2
w: 5
{{< /highlight >}}
</div>
</section>