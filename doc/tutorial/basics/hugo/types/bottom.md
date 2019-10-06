+++
title = "Bottom / Error"
description = ""
weight = 2020
layout = "tutorial"
+++
Specifying duplicate fields with conflicting values results in an error
or bottom.
_Bottom_ is a special value in CUE, denoted `_|_`, that indicates an
error such as conflicting values.
Any error in CUE results in `_|_`.
Logically all errors are equal, although errors may be associated with
metadata such as an error message.

Note that an error is different from `null`: `null` is a valid value,
whereas `_|_` is not.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>bottom.cue</i>
<p>
{{< highlight go >}}
a: 4
a: 5

l: [ 1, 2 ]
l: [ 1, 3 ]

list: [0, 1, 2]
val: list[3]

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval -i bottom.cue</i>
<p>
{{< highlight go >}}
list: [0, 1, 2]
a: _|_ // conflicting values 4 and 5
l: [1, _|_, // conflicting values 2 and 3
]
val: _|_ // index 3 out of bounds
{{< /highlight >}}
</div>
</section>