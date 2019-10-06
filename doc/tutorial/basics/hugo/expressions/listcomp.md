+++
title = "List Comprehensions"
description = ""
weight = 2040
layout = "tutorial"
+++
Lists can be created with list comprehensions.

The example shows the use of `for` loops and `if` guards.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>listcomp.cue</i>
<p>
{{< highlight go >}}
[ x*x for x in items if x rem 2 == 0]

items: [ 1, 2, 3, 4, 5, 6, 7, 8, 9 ]

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval listcomp.cue</i>
<p>
{{< highlight go >}}
[4, 16, 36, 64]
{{< /highlight >}}
</div>
</section>