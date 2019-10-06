+++
title = "Numbers"
description = ""
weight = 2040
layout = "tutorial"
+++
CUE defines two kinds of numbers.
Integers, denoted `int`, are whole, or integral, numbers.
Floats, denoted `float`, are decimal floating point numbers.

An integer literal (e.g. `4`) can be of either type, but defaults to `int`.
A floating point literal (e.g. `4.0`) is only compatible with `float`.

In the example, the result of `b` is a `float` and cannot be
used as an `int` without conversion.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>numbers.cue</i>
<p>
{{< highlight go >}}
a: int
a: 4 // type int

b: number
b: 4.0 // type float

c: int
c: 4.0

d: 4  // will evaluate to type int (default)

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval -i numbers.cue</i>
<p>
{{< highlight go >}}
a: 4
b: 4.0
c: _|_ // conflicting values int and 4.0 (mismatched types int and float)
d: 4
{{< /highlight >}}
</div>
</section>