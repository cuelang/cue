+++
title = "Hidden Fields"
weight = 2000
exec = "cue export hidden.cue"
draft = true
+++

A non-quoted field name that starts with an underscore (`_`) is not
emitted from the output.
To include fields in the configuration that start with an underscore
put them in quotes.

Quoted and non-quoted fields share the same namespace unless they start
with an underscore.

