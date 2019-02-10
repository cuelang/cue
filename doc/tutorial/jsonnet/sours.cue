
// Instead of a function with parameters, write a struct
// with fields that aren't concrete.

_Sour: {
  _spirit: string,
  ingredients: [
    { kind: _spirit, qty: 2 },
    { kind: "Egg white", qty: 1 },
    { kind: "Lemon Juice", qty: 1 },
    { kind: "Simple Syrup", qty: 1 },
  ],
  garnish: *"Lemon twist" | string
  served: "Straight Up",
}

"Whiskey Sour": _Sour & {_spirit: "Bulleit Bourbon", garnish: "Orange bitters"}
"Pisco Sour": _Sour & {_spirit: "Machu Pisco", garnish: "Angostura bitters"}
"Basic Sour": _Sour & {_spirit: "gin"}

