+++
title = "Duplicate Fields"
weight = 2000
inputs = ["dup.cue"]
exec = "cue eval dup.cue"
+++

CUE allows duplicated field definitions as long as they don't conflict.

For values of basic types this means they must be equal.

For structs, fields are merged and duplicated fields are handled recursively.

For lists, all elements must match accordingly
([we discuss open-ended lists later](lists.md).)

