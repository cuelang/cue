+++
title = "Order is Irrelevant"
weight = 2240
exec = "cue eval -i unification.cue"
+++

As mentioned before, values of duplicates fields are combined.
This process is called unification.
Unification can also be written explicitly with the `&` operator.

There is always a single unique result, possibly bottom,
for unifying any two CUE values.

Unification is commutative, associative, and idempotent.
In other words, order doesn't matter and unifying a given set of values
in any order always gives the same result.

