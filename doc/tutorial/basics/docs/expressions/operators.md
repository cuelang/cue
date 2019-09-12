+++
title = "Operators"
weight = 2000
inputs = ["op.cue"]
exec = "cue eval -i op.cue"
+++

CUE supports many common arithmetic and boolean operators.

The operators for division and remainder are different for `int` and `float`.
For `float` CUE supports the `/` and `%`  operators with the usual meaning.
For `int` CUE supports both Euclidean division (`div` and `mod`)
and truncated division (`quo` and `rem`).

