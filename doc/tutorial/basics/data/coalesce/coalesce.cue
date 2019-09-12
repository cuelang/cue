list: [ "Cat", "Mouse", "Dog" ]

a: *list[0] | "None"
b: *list[5] | "None"

n: [null]
v: *n[0]&string | "default"