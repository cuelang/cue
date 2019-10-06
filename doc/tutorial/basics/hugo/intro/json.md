+++
title = "Quotes are Optional for Field Names"
description = ""
weight = 2010
layout = "tutorial"
+++
JSON objects are called structs in CUE.
An object member is called a field.


Double quotes may be omitted from field names if their name contains no
special characters and does not start with a number:


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>json.cue</i>
<p>
{{< highlight go >}}
one: 1
two: 2

// A field using quotes.
"two-and-a-half": 2.5

list: [
	1,
	2,
	3,
]

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue export json.cue</i>
<p>
{{< highlight go >}}
{
    "list": [
        1,
        2,
        3
    ],
    "one": 1,
    "two": 2,
    "two-and-a-half": 2.5
}
{{< /highlight >}}
</div>
</section>