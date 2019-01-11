[TOC](Readme.md) [Prev](fieldcomp.md) [Next](coalesce.md)

_Expressions_

# Conditional Fields

Field comprehensions can also be used to
add a single field conditionally.

Converting the resulting configuration to JSON results in an error
as `justification` is required yet no concrete value is given.


<!-- CUE editor -->
```
price: float

// Require a justification if price is too high
justification: string if price > 100

price: 200
```

<!-- result -->
```
price:         200
justification: string
```