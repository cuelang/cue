[TOC](Readme.md) [Prev](interpolation.md) [Next](listcomp.md)

_Expressions_

# Interpolation of Field Names

String interpolations may also be used in field names.

One cannot refer to generated fields with references.

<!-- CUE editor -->
_genfield.cue:_
```
sandwich: {
    type:            "Cheese"
    "has\(type)":    true
    hasButter:       true
    butterAndCheese: hasButter && hasCheese
}
```

<!-- result -->
`$ cue eval -i genfield.cue`
```
sandwich: {
    type:            "Cheese"
    hasButter:       true
    butterAndCheese: _|_ /* reference "hasCheese" not found */
    hasCheese:       true
}
```