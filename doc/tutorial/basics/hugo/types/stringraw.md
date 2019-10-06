+++
title = "\"Raw\" Strings"
description = ""
weight = 2065
layout = "tutorial"
+++
CUE does not support raw strings in the strictest sense.
Instead it allows modifying the escape delimiter by requiring
an arbitrary number of hash `#` signs after the backslash by
enclosing a string literal in an equal number of hash signs on either end.

This works for normal and interpolated strings.
Quotes do not have to be escaped in such strings.


<a id="td-block-padding" class="td-offset-anchor"></a>
<section class="row td-box td-box--white td-box--gradient td-box--height-auto">
<div class="col-lg-6 mr-0">
<i>stringraw.cue</i>
<p>
{{< highlight go >}}
msg1: #"The sequence "\U0001F604" renders as \#U0001F604."#

msg2: ##"""
    A regular expression can conveniently be written as:

        #"\d{3}"#

    This construct works for bytes, strings and their
    multi-line variants.
    """##

{{< /highlight >}}
<br>
</div>

<div class="col-lg-6 ml-0"><i>$ cue eval stringraw.cue</i>
<p>
{{< highlight go >}}
msg1: "The sequence \"\\U0001F604\" renders as 😄."
msg2: """
        A regular expression can conveniently be written as:
        
            #\"\\d{3}\"#
        
        This construct works for bytes, strings and their
        multi-line variants.
        """
{{< /highlight >}}
</div>
</section>