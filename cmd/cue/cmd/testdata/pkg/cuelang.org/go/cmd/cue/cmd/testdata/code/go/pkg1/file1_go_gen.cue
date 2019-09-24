// Code generated by cue get go. DO NOT EDIT.

//cue:generate cue get go cuelang.org/go/cmd/cue/cmd/testdata/code/go/pkg1

package pkg1

import (
	p2 "cuelang.org/go/cmd/cue/cmd/testdata/code/go/pkg2:pkgtwo"
	"time"
)

// Foozer foozes a jaman.
Foozer :: {
	Int:    int
	String: string

	Inline

	NoInline:   NoInline
	CustomJSON: CustomJSON
	CustomYAML: null | CustomYAML @go(,*CustomYAML)
	AnyJSON:    _                 @go(,json.Marshaler)
	AnyText:    string            @go(,encoding.TextMarshaler)
	bar?:       int               @go(Bar)

	// Time is mapped to CUE's internal type.
	Time:   time.Time
	Barzer: p2.Barzer
	Map: {<_>: null | CustomJSON} @go(,map[string]*CustomJSON)
	Slice1: [...int] @go(,[]int)
	Slice2: [...] @go(,[]interface{})
	Slice3: null | [...] @go(,*[]json.Unmarshaler)
	Array1: 5 * [int]    @go(,[5]int)
	Array2: 5 * [_]      @go(,[5]interface{})
	Array3: null | 5*[_] @go(,*[5]json.Marshaler)
	Intf:   Interface    @protobuf(2,varint,name=intf)
	Intf2:  _            @go(,interface{})
	Intf3: {
		Interface: Interface
	} @go(,struct{Interface})
	Intf4: _ @go(,"interface{Foo()}")

	// Even though this struct as a type implements MarshalJSON, it is known
	// that it is really only implemented by the embedded field.
	Embed: {
		CustomJSON: CustomJSON
	} @go(,struct{CustomJSON})
}

// Level gives an indication of the extent of stuff.
Level :: int // enumLevel

enumLevel ::
	Unknown |
	Low |
	Medium |
	High

Unknown :: Level & 0
Low ::     Level & 1

// Medium is neither High nor Low
Medium :: Level & 2
High ::   Level & 3

CustomJSON :: _

CustomYAML :: {
}

Inline :: {
	Kind: string
}

NoInline :: {
	Kind: string
}

Interface :: _
