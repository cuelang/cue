+++
title = "Disjunctions"
description = ""
weight = 2050
layout = "tutorial"
+++
Disjunctions, or sum types, define a new type that is one of several things.

In the example, `conn` defines a `protocol` field that must be one of two
values: `"tcp"` or `"udp"`.
It is an error for a concrete `conn`
to define anything else than these two values.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>disjunctions.cue</i>
<p>
{{< highlight go >}}
conn: {
    address:  string
    port:     int
    protocol: "tcp" | "udp"
}

lossy: conn & {
    address:  "1.2.3.4"
    port:     8888
    protocol: "udp"
}

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval disjunctions.cue</i>
<p>
{{< highlight go >}}
conn: {
    address:  string
    port:     int
    protocol: "tcp" | "udp"
}
lossy: {
    address:  "1.2.3.4"
    port:     8888
    protocol: "udp"
}
{{< /highlight >}}
</div>
</section>