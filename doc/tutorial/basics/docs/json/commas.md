+++
title = "Commas are Optional after Fields"
weight = 2000
exec = "cue export commas.cue"
+++

Commas are optional at the end of fields.
This is also true for the last field.
The convention is to omit them.

<!-- Side Note -->
_CUE borrows a trick from Go to achieve this: the formal grammar still
requires commas, but the scanner inserts commas according to a small set
of simple rules._

