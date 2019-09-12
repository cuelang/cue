+++
title = "\"Raw\" Strings"
weight = 2000
exec = "cue eval stringraw.cue"
+++

CUE does not support raw strings in the strictest sense.
Instead it allows modifying the escape delimiter by requiring
an arbitrary number of hash `#` signs after the backslash by
enclosing a string literal in an equal number of hash signs on either end.

This works for normal and interpolated strings.
Quotes do not have to be escaped in such strings.

