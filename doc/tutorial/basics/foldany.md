[TOC](Readme.md) [Prev](fold.md) [Next](comments.md)

_JSON Sugar and other Goodness_

# Folding all Fields

This also works if a struct has more than one member.

In general, any JSON configuration can be expressed as a collection of
path-leaf pairs without using any curly braces.

<!-- CUE editor -->
```
outer middle1 inner: 3
outer middle2 inner: 7
```

<!-- JSON result -->
```json
"outer": {
    "middle1": {
        "inner": 3
    },
    "middle2": {
        "inner": 7
    }
}
```