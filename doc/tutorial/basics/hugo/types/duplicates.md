+++
title = "Duplicate Fields"
description = ""
weight = 2020
layout = "tutorial"
+++
CUE allows duplicated field definitions as long as they don't conflict.

For values of basic types this means they must be equal.

For structs, fields are merged and duplicated fields are handled recursively.

For lists, all elements must match accordingly
([we discuss open-ended lists later](lists.md).)


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