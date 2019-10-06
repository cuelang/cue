+++
title = "Disjunctions of Structs"
description = ""
weight = 2055
layout = "tutorial"
+++
Disjunctions work for any type.

In this example we see that a `floor` of some specific house
has an exit on level 0 and 1, but not on any other floor.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>sumstruct.cue</i>
<p>
{{< highlight go >}}
// floor defines the specs of a floor in some house.
floor: {
    level:   int  // the level on which this floor resides
    hasExit: bool // is there a door to exit the house?
}

// constraints on the possible values of floor.
floor: {
    level: 0 | 1
    hasExit: true
} | {
    level: -1 | 2 | 3
    hasExit: false
}
{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"></div>
</section>