+++
title = "Regular expressions"
description = ""
weight = 2070
layout = "tutorial"
+++
The `=~` and `!~` operators can be used to check against regular expressions.

The expression `a =~ b` is true if `a` matches `b`, while
`a !~ b` is true if `a` does _not_ match `b`.

Just as with comparison operators, these operators may be used
as unary versions to define a set of strings.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>regexp.cue</i>
<p>
{{< highlight go >}}
a: "foo bar" =~ "foo [a-z]{3}"
b: "maze" !~ "^[a-z]{3}$"

c: =~"^[a-z]{3}$" // any string with lowercase ASCII of length 3

d: c
d: "foo"

e: c
e: "foo bar"

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval -i regexp.cue</i>
<p>
{{< highlight go >}}
a: true
b: true
c: =~"^[a-z]{3}$"
d: "foo"
e: _|_ // invalid value "foo bar" (does not match =~"^[a-z]{3}$")
{{< /highlight >}}
</div>
</section>