+++
title = "Default Values"
description = ""
weight = 2055
layout = "tutorial"
+++
Elements of a disjunction may be marked as preferred.
If there is only one mark, or the users constraints a field enough such that
only one mark remains, that value is the default value.

In the example, `replicas` defaults to `1`.
In the case of `protocol`, however, there are multiple definitions with
different, mutually incompatible defaults.
In that case, both `"tcp"` and `"udp"` are preferred and one must explicitly
specify either `"tcp"` or `"udp"` as if no marks were given.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>defaults.cue</i>
<p>
{{< highlight go >}}
// any positive number, 1 is the default
replicas: uint | *1

// the default value is ambiguous
protocol: *"tcp" | "udp"
protocol: *"udp" | "tcp"

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval defaults.cue</i>
<p>
{{< highlight go >}}
replicas: 1
protocol: "tcp" | "udp" | *_|_
{{< /highlight >}}
</div>
</section>