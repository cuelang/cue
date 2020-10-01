package test

#Test: {
	// doc comment
	@protobuf((yoyo.foo)=true) // line comment
	@protobuf((yoyo.bar)=false)
	test?: int32 @protobuf(1)
}
