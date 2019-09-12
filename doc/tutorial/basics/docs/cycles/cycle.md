+++
title = "Reference Cycles"
weight = 2000
exec = "cue eval -i -c cycle.cue"
+++

CUE can handle many types of cycles just fine.
Because all values are final, a field with a concrete value of, say `200`,
can only be valid if it is that value.
So if it is unified with another expression, we can delay the evaluation of
this until later.

By postponing that evaluation, we can often break cycles.
This is very useful for template writers that may not know what fields
a user will want to fill out.


