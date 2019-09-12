+++
title = "Bounds"
weight = 2000
inputs = ["bounds.cue"]
exec = "cue eval -i bounds.cue"
+++

Bounds define a lower bound, upper bound, or inequality for a certain value.
They work on numbers, strings, bytes, and and null.

The bound is defined for all values for which the corresponding comparison
operation is define.
For instance `>5.0` allows all floating point values greater than `5.0`,
whereas `<0` allows all negative numbers (int or float).

