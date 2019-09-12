+++
title = "References and Scopes"
weight = 2410
exec = "cue eval scopes.cue"
+++

A reference refers to the value of the field defined within nearest
enclosing scope.

If no field matches the reference within the file, it may match a top-level
field defined in any other file of the same package.

If there is still no match, it may match a predefined value.

