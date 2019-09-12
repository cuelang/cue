+++
title = "Bottom / Error"
weight = 2000
exec = "cue eval -i bottom.cue"
+++

Specifying duplicate fields with conflicting values results in an error
or bottom.
_Bottom_ is a special value in CUE, denoted `_|_`, that indicates an
error such as conflicting values.
Any error in CUE results in `_|_`.
Logically all errors are equal, although errors may be associated with
metadata such as an error message.

Note that an error is different from `null`: `null` is a valid value,
whereas `_|_` is not.

