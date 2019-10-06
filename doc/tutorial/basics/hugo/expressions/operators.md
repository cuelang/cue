+++
title = "Operators"
description = ""
weight = 2010
layout = "tutorial"
+++
CUE supports many common arithmetic and boolean operators.

The operators for division and remainder are different for `int` and `float`.
For `float` CUE supports the `/` and `%`  operators with the usual meaning.
For `int` CUE supports both Euclidean division (`div` and `mod`)
and truncated division (`quo` and `rem`).


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>op.cue</i>
<p>
{{< highlight go >}}
a: 3 / 2   // type float
b: 3 div 2 // type int: Euclidean division

c: 3 * "blah"
d: 3 * [1, 2, 3]

e: 8 < 10

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval -i op.cue</i>
<p>
{{< highlight go >}}
a: 1.5
b: 1
c: "blahblahblah"
d: [1, 2, 3, 1, 2, 3, 1, 2, 3]
e: true
{{< /highlight >}}
</div>
</section>