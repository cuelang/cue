+++
title = "String Literals"
weight = 2070
exec = "cue export stringlit.cue"
+++

CUE strings allow a richer set of escape sequences than JSON.

CUE also supports multi-line strings, enclosed by a pair of triple quotes `"""`.
The opening quote must be followed by a newline.
The closing quote must also be on a newline.
The whitespace directly preceding the closing quote must match the preceding
whitespace on all other lines and is removed from these lines.

Strings may also contain [interpolations](../../expressions/interpolation).

