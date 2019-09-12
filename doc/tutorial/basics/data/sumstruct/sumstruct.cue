// floor defines the specs of a floor in some house.
floor: {
    level:   int  // the level on which this floor resides
    hasExit: bool // is there a door to exit the house?
}

// constraints on the possible values of floor.
floor: {
    level: 0 | 1
    hasExit: true
} | {
    level: -1 | 2 | 3
    hasExit: false
}