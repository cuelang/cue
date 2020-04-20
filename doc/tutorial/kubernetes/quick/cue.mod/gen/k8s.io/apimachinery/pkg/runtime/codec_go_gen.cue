// Code generated by cue get go. DO NOT EDIT.

//cue:generate cue get go k8s.io/apimachinery/pkg/runtime

package runtime

// NoopEncoder converts an Decoder to a Serializer or Codec for code that expects them but only uses decoding.
NoopEncoder :: {
	"Decoder": Decoder
}

// NoopDecoder converts an Encoder to a Serializer or Codec for code that expects them but only uses encoding.
NoopDecoder :: {
	"Encoder": Encoder
}
