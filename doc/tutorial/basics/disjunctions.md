[TOC](Readme.md) [Prev](unification.md) [Next](disjstruct.md)

_Types and Values_

# Disjunctions

Disjunctions, or sum types, define a new type that is one of several things.

In the example, `conn` defines a `protocol` field that must be one of two
values: `"tcp"` or `"upd"`.
It is an error for a concrete `conn`
to define anything else than these two values.

<!-- CUE editor -->
```
conn: {
    address:  string
    port:     int
    protocol: "tcp" | "udp"
}

lossy: {
    address:  "1.2.3.4"
    port:     8888
    protocol: "udp"
}
```