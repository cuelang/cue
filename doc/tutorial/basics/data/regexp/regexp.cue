a: "foo bar" =~ "foo [a-z]{3}"
b: "maze" !~ "^[a-z]{3}$"

c: =~"^[a-z]{3}$" // any string with lowercase ASCII of length 3

d: c
d: "foo"

e: c
e: "foo bar"