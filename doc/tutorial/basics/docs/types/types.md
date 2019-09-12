+++
title = "Basic Types"
weight = 2230
exec = "cue eval types.cue"
+++

CUE defines the following basic types

```
null bool string bytes int float
```
in addition to the error type mentioned in the previous section.

CUE does not distinguish between types and values.
A field value can be a type (using one of the above names), a concrete value,
or, in case of composite types (lists and structs), anything in between.

In the example, `point` defines an arbitrary point, while `xaxis` and `yaxis`
define the points on the respective lines.
We say that `point`, `xaxis`, and `yaxis` are abstract points, as these
points are underspecified.
Such abstract values cannot be represented as JSON,
which requires all values to be concrete.

The only concrete point is `origin`.
The `origin` is defined to be both on the x-axis and y-axis, which means it
must be at `0, 0`.
Here we see constraints in action:
`origin` evalutes to `0, 0`, even though we did not specify its coordinates
explicitly.


