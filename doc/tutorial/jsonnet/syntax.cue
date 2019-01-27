/* A C-style comment. */
// A single-line comment.

// Can omit top-level braces.
cocktails: {
    // Ingredient quantities are in fl oz.
    "Tom Collins": {  // use double quotes for strings; single quotes are bytes
      ingredients: [
        { kind: "Farmer's Gin", qty: 1.5 },
        { kind: "Lemon", qty: 1 },
        { kind:" Simple Syrup", qty: 0.5 },
        { kind: "Soda", qty: 2 },
        { kind: "Angostura", qty: "dash" },
      ] // can omit commas between object keys and values (but not list elements)
      garnish: "Maraschino Cherry"
      served: "Tall"
      description: """
        The Tom Collins is essentially gin and
        lemonade.  The bitters add complexity.

      """
    }
    Manhattan: {
      ingredients: [
        { kind: "Rye", qty: 2.5 },
        { kind: "Sweet Red Vermouth", qty: 1 },
        { kind: "Angostura", qty: "dash" },
      ],
      garnish: "Maraschino Cherry"
      served: "Straight Up"
      description: "A clear \\ red drink." // no verbatim strings
    }
}

