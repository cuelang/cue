+++
title = "String Literals"
description = ""
weight = 2060
layout = "tutorial"
+++
CUE strings allow a richer set of escape sequences than JSON.

CUE also supports multi-line strings, enclosed by a pair of triple quotes `"""`.
The opening quote must be followed by a newline.
The closing quote must also be on a newline.
The whitespace directly preceding the closing quote must match the preceding
whitespace on all other lines and is removed from these lines.

Strings may also contain [interpolations](interpolation.md).


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>stringlit.cue</i>
<p>
{{< highlight go >}}
// 21-bit unicode characters
a: "\U0001F60E" // 😎

// multiline strings
b: """
    Hello
    World!
    """

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue export stringlit.cue</i>
<p>
{{< highlight go >}}
{
    "a": "😎",
    "b": "Hello\nWorld!"
}
{{< /highlight >}}
</div>
</section>