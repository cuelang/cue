+++
title = "Cycles in Fields"
weight = 2485
exec = "cue eval cycleref.cue"
+++

Also, we know that unifying a field with itself will result in the same value.
Thus if we have a cycle between some fields, all we need to do is ignore
the cycle and unify their values once to achieve the same result as
merging them ad infinitum.

