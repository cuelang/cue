+++
title = "Null Coalescing"
weight = 2680
exec = "cue eval coalesce.cue"
+++

<!-- jba: the terms here are confusing. "Null coalescing" is actually not
  that, but then there is something called "actual null coalescing."
  
  Just say that because _|_ | X evaluates to X, you can use disjunction
  to represent fallback values.
  
  And then you can use that to effectively type-check with a default value.
-->

With null coalescing we really mean error, or bottom, coalescing.
The defaults mechanism for disjunctions can also be
used to provide fallback values in case an expression evaluates to bottom.

In the example the fallback values are specified
for `a` and `b` in case the list index is out of bounds.

To do actual null coalescing one can unify a result with the desired type
to force an error.
In that case the default will be used if either the lookup fails or
the result is not of the desired type.

