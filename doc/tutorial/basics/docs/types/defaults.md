+++
title = "Default Values"
weight = 2000
exec = "cue eval defaults.cue"
+++

Elements of a disjunction may be marked as preferred.
If there is only one mark, or the users constraints a field enough such that
only one mark remains, that value is the default value.

In the example, `replicas` defaults to `1`.
In the case of `protocol`, however, there are multiple definitions with
different, mutually incompatible defaults.
In that case, both `"tcp"` and `"udp"` are preferred and one must explicitly
specify either `"tcp"` or `"udp"` as if no marks were given.

