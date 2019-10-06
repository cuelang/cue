+++
title = "Hidden Fields"
description = ""
draft = true
weight = 2099
layout = "tutorial"
+++
A non-quoted field name that starts with an underscore (`_`) is not
emitted from the output.
To include fields in the configuration that start with an underscore
put them in quotes.

Quoted and non-quoted fields share the same namespace unless they start
with an underscore.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>hidden.cue</i>
<p>
{{< highlight go >}}
"_foo": 2
_foo:   3
foo:    4

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue export hidden.cue</i>
<p>
{{< highlight go >}}
{
    "_foo": 2,
    "foo": 4
}
{{< /highlight >}}
</div>
</section>