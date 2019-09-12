+++
title = "Templates"
weight = 2000
exec = "cue eval templates.cue"
+++

<!-- jba: this is not in the spec, aside from the TemplateLabel grammar rule. -->

One of CUE's most powerful features is templates.
A template defines a value to be unified with each field of a struct.

The template's identifier (in angular brackets) is bound to name of each
of its sibling fields and is visible within the template value
that is unified with each of the siblings.

