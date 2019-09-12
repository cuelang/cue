+++
title = "Lists"
weight = 2280
exec = "cue eval -i lists.cue"
+++

Lists define arbitrary sequences of CUE values.
A list can be closed or open ended.
Open-ended lists may have some predefined elements, but may have
additional, possibly typed elements.

In the example we define `IP` to be a list of `4` elements of type `uint8`, which
is a predeclared value of `>=0 & <=255`.
`PrivateIP` defines the IP ranges defined for private use.
Note that as it is already defined to be an `IP`, the length of the list
is already fixed at `4` and we do not have to specify a value for all elements.
Also note that instead of writing `...uint8`, we could have written `...`
as the type constraint is already already implied by `IP`.

The output contains a valid private IP address (`myIP`)
and an invalid one (`yourIP`).

