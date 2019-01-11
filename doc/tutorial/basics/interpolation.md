[TOC](Readme.md) [Prev](operators.md) [Next](interpolfield.md)

_Expressions_

# Interpolation

String and bytes literals support interpolation.

Any valid CUE expression may be used inside the escaped parentheses.
Interpolation may also be used in multiline string and byte literals.

<!-- CUE editor -->
```
"You are \( cost - budget ) dollars over budget!"

cost:   102
budget: 88
```

<!-- result -->
```
"You are 14 dollars over budget!"
```