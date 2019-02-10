import "strings"

// Cue doesn't have functions.
// Instead, write a struct with related fields,
// then "call" the function by unifying with arguments.
_add: {
	x: int|float
	y: 10
	out: x + y
}

a: (_add & {x: 3.0}).out

// You can use this technique to construct "functions" that "return"
// other "functions":

_adder: {
	y: int|float
	out: { x: int|float, out: x + y }
}

_add10: (_adder & {y: 10.0}).out

b: (_add10 & {x: 3.0}).out

// built-ins

c: strings.Join(strings.Split("foo/bar", "/"), " ")

d: [ len("hello"),len([1, 2, 3])]
