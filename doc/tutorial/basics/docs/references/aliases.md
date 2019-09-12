+++
title = "Aliases"
weight = 2430
inputs = ["alias.cue"]
exec = "cue eval alias.cue"
+++

An alias defines a local macro.

A typical use case is to provide access to a shadowed field.

Aliases are not members of a struct. They can be referred to only within the
struct, and they do not appear in the output.

