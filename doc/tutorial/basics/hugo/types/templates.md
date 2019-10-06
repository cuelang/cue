+++
title = "Templates"
description = ""
weight = 2090
layout = "tutorial"
+++
<!-- jba: this is not in the spec, aside from the TemplateLabel grammar rule. -->

One of CUE's most powerful features is templates.
A template defines a value to be unified with each field of a struct.

The template's identifier (in angular brackets) is bound to name of each
of its sibling fields and is visible within the template value
that is unified with each of the siblings.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>templates.cue</i>
<p>
{{< highlight go >}}
// The following struct is unified with all elements in job.
// The name of each element is bound to Name and visible in the struct.
job <Name>: {
    name:     Name
    replicas: uint | *1
    command:  string
}

job list command: "ls"

job nginx: {
    command:  "nginx"
    replicas: 2
}

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval templates.cue</i>
<p>
{{< highlight go >}}
job: {
    list: {
        name:     "list"
        replicas: 1
        command:  "ls"
    }
    nginx: {
        name:     "nginx"
        replicas: 2
        command:  "nginx"
    }
}
{{< /highlight >}}
</div>
</section>