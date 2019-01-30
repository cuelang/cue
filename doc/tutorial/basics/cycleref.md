[TOC](Readme.md) [Prev](cycle.md) _Next_

_Cycles_

# Cycles in fields

Also, we know that merging a field with itself will result in the same value.
Thus if we have a cycle between some fields, all we need to do is ignore
the cycle and add merge their values once to achieve the same result as
merging them ad infinitum.

<!-- CUE editor -->
```
labels: selectors
labels: {app: "foo"}

selectors: labels
selectors: {name: "bar"}
```

<!-- result -->
```
labels:    {app: "foo", name: "bar"}
selectors: {app: "foo", name: "bar"}
```
