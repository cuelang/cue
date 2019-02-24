# Resolution of Marked Expressions

## Introduction 

This is a proof that the resolution algorithm for expressions with marks
preserves the common algebraic properties of mark-free expressions,
except for distribution over `|`.

Let `R` be the mark-free resolution function, that is, the function that maps
mark-free expressions to their values. Expressions under `R` have the usual
desirable algebraic properties:
```
R(a | b) = R(b | a)               // | is commutative
R(a | (b | c)) = R((a | b) | c)   // | is associative
R(a & b) = R(b & a)               // & is commutative
R(a & (b & c)) = R((a & b) & c)   // & is associative
```
and similarly for other operators (e.g. addition is commutative and associative).

Define a function `R*` that resolves expressions that may contain marks as
follows:

1. If exactly one expression of a disjunct is unmarked, remove it. Apply `R` to
   the resulting expression, ignoring marks. If the result is not bottom, return
   it.
2. Otherwise, return the result of applying `R` to the expression, ignoring
   marks.

I will show that `R*` has the same algebraic properties as `R`, except
that operators do not distribute over `|`.

Since `R*` consists of two applications of `R` on transformed versions of the
input expression, we have to show that each transformation preserves the
properties of `R`.

Clearly the transformation in step 2 preserves `R`'s properties, since it
doesn't do anything to the expression except remove marks.

It remains to show that the transformation of step 1, dropping unmarked
disjuncts, preserves the properties of `R`. To show that, I must define step 1
more precisely. I will define a function `D` (for _drop_) on expressions using
pseudocode.

## Definitions of `D` and `R*`

I assume every expression has two fields: `node`, the expression's AST, and
`marked`, a boolean describing whether the expression is marked. (In CUE syntax,
only elements of a disjunction can be marked, but `D` must propagate mark
information up the expression tree, so we must give every expression a mark
bit.)

I will call any expression that does not have an operator a _leaf expression_.
`D` on leaf expressions is the identity:
```
D(e), isLeaf(e) = e
```

`D` on unary expressions preserves mark information but otherwise does nothing:
```
D(op e) = UnopExpr{op: op, node: D(e).node, marked: D(e).marked}
```

`D` behaves similarly on all binary expressions except `|`, propagating the mark
bit if either operand is marked:

```
D(a op b), op != "|" = BinopExpr{
                op: op, 
                lhs: D(a).node, 
                rhs: D(b).node, 
                marked: D(a).marked or D(b).marked,
            }
```
The interesting case is the `|` operator. `D` drops a single unmarked disjunct,
but preserves the mark bit:

```
D(a | b) = 
    if D(a).marked and not D(b).marked
        D(a)
    else if D(b).marked and not D(a).marked
        D(b)
    else
        BinopExpr{
            op: "|",
            lhs: D(a).node,
            rhs: D(b).node,
            marked: D(a).marked or D(b).marked,
        }
```
Note that the "else" clause, when both disjuncts are either marked or unmarked,
is the same as for the other operators.

If we assume that `R` uses only the `node` field of an expression and ignores
the `marked` field, we can now define `R*` precisely:

```
R*(e) = 
    if R(D(e)) != _|_
       R(D(e))
    else
       R(e)
```

## Example
As an example, I compute `R*((*1 | int) & 2)`. First, compute `D`:
```
  D((*1 | int) & 2)
= D(*1 | int) & D(2)
= D(*1) & D(2)
= *1 & 2
```
In the second line, I applied the first `if` clause of the disjunction rule for
`D` to drop the unmarked disjunct `int`.

Continuing with the evaluation of `R*`, we ignore the mark on `1` and compute

```
  R(D(e))
= R(1 & 2)
= _|_
```
So we use the `else` clause of `R*` by calling `R` on the input expression
without marks:

```
  R((1 | int) & 2)
= 2
```

## Proof for `D`

Now I prove that `D` preserves commutativity and associativity.

For all operators except `|`, `D` doesn't affect the form of the expression, so
it doesn't affect the algebra. So I only have to prove properties that involve
`|`.

### Commutativity of `|`

I want to show that `R(D(a | b)) = R(D(b | a))` regardless of how `a` and `b` are
marked.

First consider the cases where neither or both operands are marked. These cases
are handled by the final `else` of the definition of `D(a | b)`, which preserves
the expression. So in these cases,
```
D(a | b).node = a | b
D(b | a).node = b | a
```
and since `R` ignores marks and `|` is commutative under `R`, we have
`R(a | b) = R (b | a)`.

Now consider the case where `D(a).marked = true` and `D(b).marked = false`,
which for convenience I'll write as `D(*a | b)`. In this case `b` is dropped
whether it is the first or second operand, by the first two `if` clauses of the
definition of `D(a | b)`. So
```
D(*a | b) = D(b | *a) = *a
```
and since the resulting expression is the same, the result of applying `R` is as
well.

The `D(a | *b)` case is handled by the same argument.

### Associativity of `|`

I need to show that
```
R(D(a | (b | c))) = R(D((a | b) | c))
```
for all combinations of marks on `a`, `b` and `c`.

As above, if none of the operands are marked or all are marked, the expression
isn't altered, so the above equality holds by the associativity of `|` under
`R`.

Now I consider the remaining cases, where some but not all subexpressions are
marked. In all these cases, the value of `D` is identical regardless of
grouping.


#### Only `a` is marked

We begin with `D(*a | (b | c))`.
Since neither `b` nor `c` is marked, `D(b | c).marked` is false.
So from the first clause of the definition,
`D(*a | (b | c)) = *a`.

Considering `D((*a | b) | c)`: the subexpression `D(*a | b)` is `*a`, so the
whole expression is `D(*a | c)` = `*a`.

So `D(*a | (b | c)) = D((*a | b) | c) = *a`.

#### Only `b` is marked

I present this and the following cases purely symbolically.

```
  D(a | (*b | c))
= D(*b | c)
= *b

  D((a | *b) | c)
= D(a | *b)
= *b
```

The case where only `c` is marked is similar.

#### `a` and `b` are marked

```
  D(*a | (*b | c))
= D(*a) | D(*b | c)
= *a | *b

  D((*a | *b) | c)
= D(*a | *b)
= *a | *b
```

The case where `a` and `c` are marked is similar.

#### `b` and `c` are marked

```
  D(a | (*b | *c))
= D(*b | *c)
= *b | *c

  D((a | *b) | *c)
= D(a | *b) | D(*c)
= D(*b) | D(*c)
= *b | *c
```

### Failure of Distribution over `|`

Operators don't distribute over `|` in the presence of marks. The counterexample is:

```
  D(*a op (*b | c))
= D(*a) op D(*b | c)
= *a op *b

  D((*a op *b) | (*a op c))
= D(*a op *b) | D(*a op c)
= (*a op *b) | (*a op c)
```


