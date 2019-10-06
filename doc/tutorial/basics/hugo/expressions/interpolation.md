+++
title = "Interpolation"
description = ""
weight = 2020
layout = "tutorial"
+++
String and bytes literals support interpolation.

Any valid CUE expression may be used inside the escaped parentheses.
Interpolation may also be used in multiline string and byte literals.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>interpolation.cue</i>
<p>
{{< highlight go >}}
"You are \( cost - budget ) dollars over budget!"

cost:   102
budget: 88

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval interpolation.cue</i>
<p>
{{< highlight go >}}
"You are 14 dollars over budget!"
{{< /highlight >}}
</div>
</section>