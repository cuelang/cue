+++
title = "Validation"
description = ""
weight = 2032
layout = "tutorial"
+++
Constraints specify what values are allowed.
To CUE they are just values like anything else,
but conceptually they can be explained as something in between types and
concrete values.

Constraints can be used to validate values of concrete instances.
They can be applied to CUE data, or directly to YAML or JSON.

But constraints can also reduce boilerplate.
If a constraints defines a concrete value, there is no need
to specify it in values to which this constraint applies.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>schema.cue</i>
<p>
{{< highlight go >}}
Language :: {
	tag:  string
	name: =~"^\\p{Lu}" // Must start with an uppercase letter.
}
languages: [...Language]

{{< /highlight >}}
<br>
<i>data.yaml</i>
<p>
{{< highlight go >}}
languages:
  - tag: en
    name: English
  - tag: nl
    name: dutch
  - tag: no
    name: Norwegian

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue vet schema.cue data.yaml</i>
<p>
{{< highlight go >}}
languages.2.tag: conflicting values string and false (mismatched types string and bool):
    ./data.yaml:6:11
    ./schema.cue:2:8
languages.1.name: invalid value "dutch" (does not match =~"^\\p{Lu}"):
    ./schema.cue:3:8
    ./data.yaml:5:12
{{< /highlight >}}
</div>
</section>