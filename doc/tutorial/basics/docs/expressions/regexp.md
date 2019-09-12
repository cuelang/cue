+++
title = "Regular expressions"
weight = 2000
exec = "cue eval -i regexp.cue"
+++

The `=~` and `!~` operators can be used to check against regular expressions.

The expression `a =~ b` is true if `a` matches `b`, while
`a !~ b` is true if `a` does _not_ match `b`.

Just as with comparison operators, these operators may be used
as unary versions to define a set of strings.


