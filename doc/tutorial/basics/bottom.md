[TOC](Readme.md) [Prev](duplicates.md) [Next](types.md)

_Types ~~and~~ are Values_

# Bottom

Specifying duplicate fields with conflicting values results in an error.
An error, also called _bottom_ and denoted `_|_`,
is a special CUE value indicating incompatible values.

Any error in CUE results in `_|_`.
Logically all errors are equal, although errors may be associated with
metadata such as an error message.

Note that an error is different from `null`: `null` is a valid JSON value,
whereas `_|_` is not.

<!-- CUE editor -->
```
a: 4
a: 5

l: [ 1, 2 ]
l: [ 1, 3 ]

list: [0, 1, 2]
val: list[3]
```

<!-- result -->
```
a:    _|_
l:    _|_
list: [0, 1, 2]
val:  _|_
```