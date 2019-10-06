+++
title = "Bytes"
description = ""
weight = 2068
layout = "tutorial"
+++
CUE distinguishes between a `string` and a `bytes` type.
Bytes are converted to base64 when emitting JSON.
Byte literals are defined with single quotes.
The following additional escape sequences are allowed in byte literals:

    \xnn   // arbitrary byte value defined as a 2-digit hexadecimal number
    \nnn   // arbitrary byte value defined as a 3-digit octal number
<!-- jba: this contradicts the spec, which has \nnn (no leading zero) -->


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>bytes.cue</i>
<p>
{{< highlight go >}}
a: '\x03abc'

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue export bytes.cue</i>
<p>
{{< highlight go >}}
{
    "a": "A2FiYw=="
}
{{< /highlight >}}
</div>
</section>