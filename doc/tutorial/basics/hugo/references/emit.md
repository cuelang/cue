+++
title = "Emit Values"
description = ""
weight = 2050
layout = "tutorial"
+++
By default all top-level fields are emitted when evaluating a configuration.
Embedding a value at top-level will cause that value to be emitted instead.

Emit values allow CUE configurations, like JSON,
to define any type, instead of just structs, while keeping the common case
of defining structs light.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>emit.cue</i>
<p>
{{< highlight go >}}
"Hello \(who)!"

who: "world"

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval emit.cue</i>
<p>
{{< highlight go >}}
"Hello world!"
{{< /highlight >}}
</div>
</section>