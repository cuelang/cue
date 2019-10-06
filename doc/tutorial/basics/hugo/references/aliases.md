+++
title = "Aliases"
description = ""
weight = 2030
layout = "tutorial"
+++
An alias defines a local macro.

A typical use case is to provide access to a shadowed field.

Aliases are not members of a struct. They can be referred to only within the
struct, and they do not appear in the output.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>alias.cue</i>
<p>
{{< highlight go >}}
A = a  // A is an alias for a
a: {
    d: 3
}
b: {
    a: {
        // A provides access to the outer "a" which would
        // otherwise be hidden by the inner one.
        c: A.d
    }
}

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval alias.cue</i>
<p>
{{< highlight go >}}
a: {
    d: 3
}
b: {
    a: {
        c: 3
    }
}
{{< /highlight >}}
</div>
</section>