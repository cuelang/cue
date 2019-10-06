+++
title = "Conditional Fields"
description = ""
weight = 2055
layout = "tutorial"
+++
Field comprehensions can also be used to
add a single field conditionally.

Converting the resulting configuration to JSON results in an error
as `justification` is required yet no concrete value is given.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>conditional.cue</i>
<p>
{{< highlight go >}}
price: number

// Require a justification if price is too high
if price > 100 {
    justification: string
}

price: 200

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval conditional.cue</i>
<p>
{{< highlight go >}}
price:         200
justification: string
{{< /highlight >}}
</div>
</section>