[TOC](Readme.md) [Prev](selectors.md) [Next](emit.md)

_References and Visibility_

# Aliases

An alias defines a local macro.

A typical use case is to provide access to a shadowed field.

Alias are not members of a struct. They can be referred to, but they do not
appear in the output.

<!-- CUE editor -->
```
A = a  // A is an alias for a
a: {
    d: 3
}
b: {
    a: {
        // A provides access to the outer "a" which would
        // otherwise be hidden by the inner one.
        c: A.d
    }
}
```

<!-- result -->
```
a: {
    d: 3
}
b: {
    a: {
        c: 3
    }
}
```
