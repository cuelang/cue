+++
title = "Disjunctions"
weight = 2000
+++

Disjunctions, or sum types, define a new type that is one of several things.

In the example, `conn` defines a `protocol` field that must be one of two
values: `"tcp"` or `"udp"`.
It is an error for a concrete `conn`
to define anything else than these two values.

