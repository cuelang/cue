+++
title = "Number Literals"
description = ""
weight = 2045
layout = "tutorial"
+++
CUE adds a variety of sugar for writing numbers.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>numlit.cue</i>
<p>
{{< highlight go >}}
[
    1_234,       // 1234
    5M,          // 5_000_000
    1.5Gi,       // 1_610_612_736
    0x1000_0000, // 268_435_456
]

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue export numlit.cue</i>
<p>
{{< highlight go >}}
[
    1234,
    5000000,
    1610612736,
    268435456
]
{{< /highlight >}}
</div>
</section>