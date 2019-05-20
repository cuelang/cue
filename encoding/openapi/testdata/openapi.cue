package openapi

// MyMessage is my message.
MyMessage: {
	port?: Port & {} @protobuf(1)

	foo: Bar & >10 & <1000 & int32 @protobuf(2)

	bar: [...string] @protobuf(3)
}

MyMessage: {
	// Field a.
	a: 1
} | {
	b: string //2: crash
}

YourMessage: ({a: number} | {b: string} | {b: number}) & {a?: string}

YourMessage2: ({a: number} | {b: number}) &
	({c: number} | {d: number}) &
	({e: number} | {f: number})

Msg2: {b: number} | {a: string}

// Bar: [...number] | *[1, 2, 3]
// Bar: "foo" | "bar" | "baz"
Bar: int32

// // Baz: Port | *{port: 1}

Port: {
	port: int

	obj: []
}
