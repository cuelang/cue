+++
title = "Numbers"
weight = 2260
exec = "cue eval -i numbers.cue"
+++

CUE defines two kinds of numbers.
Integers, denoted `int`, are whole, or integral, numbers.
Floats, denoted `float`, are decimal floating point numbers.

An integer literal (e.g. `4`) can be of either type, but defaults to `int`.
A floating point literal (e.g. `4.0`) is only compatible with `float`.

In the example, the result of `b` is a `float` and cannot be
used as an `int` without conversion.

