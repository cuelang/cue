// Code generated by cue get go. DO NOT EDIT.

//cue:generate cue get go cuelang.org/go/cmd/cue/cmd/testdata/code/go/pkg2

// Package pkgtwo does other stuff.
package pkgtwo

import t "time"

// A Barzer barzes.
#Barzer: {
	a:     int @go(A) @protobuf(2,varint,)
	T:     t.Time
	B?:    null | int    @go(,*big.Int)
	C:     int           @go(,big.Int)
	F:     string        @go(,big.Float) @xml(,attr)
	G?:    null | string @go(,*big.Float)
	S:     string
	"x-y": bool @go(XY)
	Err:   _    @go(,error)
}

#Perm: 0o755

#Few: 3

#Couple: int & 2
