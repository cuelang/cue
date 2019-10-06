+++
title = "References and Scopes"
description = ""
weight = 2010
layout = "tutorial"
+++
A reference refers to the value of the field defined within nearest
enclosing scope.

If no field matches the reference within the file, it may match a top-level
field defined in any other file of the same package.

If there is still no match, it may match a predefined value.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>scopes.cue</i>
<p>
{{< highlight go >}}
v: 1
a: {
    v: 2
    b: v // matches the inner v
}
a: {
    c: v // matches the top-level v
}
b: v

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval scopes.cue</i>
<p>
{{< highlight go >}}
v: 1
a: {
    v: 2
    b: 2
    c: 1
}
b: 1
{{< /highlight >}}
</div>
</section>