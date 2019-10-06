+++
title = "Field Comprehensions"
description = ""
weight = 2050
layout = "tutorial"
+++
CUE also supports comprehensions for fields.

One cannot refer to generated fields with references.
Instead, one must use indexing.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>fieldcomp.cue</i>
<p>
{{< highlight go >}}
import "strings"

a: [ "Barcelona", "Shanghai", "Munich" ]

{
    for k, v in a {
        "\( strings.ToLower(v) )": {
            pos:     k + 1
            name:    v
            nameLen: len(v)
        }
    }
}

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval fieldcomp.cue</i>
<p>
{{< highlight go >}}
barcelona: {
    name:    "Barcelona"
    pos:     1
    nameLen: 9
}
shanghai: {
    name:    "Shanghai"
    pos:     2
    nameLen: 8
}
munich: {
    name:    "Munich"
    pos:     3
    nameLen: 6
}
{{< /highlight >}}
</div>
</section>