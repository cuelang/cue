package reprodata

A: {
  foo: "goo" @hof()
} @myattr()

ex1: attrs(A)
ex2: attrs(A.foo)
