package v1

// This file adds manual constraints on types.

EndpointAddress: {
	Byte = #"([01]?\d?\d|2[0-4]\d|25[0-5])"#
	ip: =~#"^(\#(Byte)\.){3}\#(Byte)$"#

	ip: !~#"^127\.0\.0"# & !~#"^169\.254\."# & !~#"^224\."#
}
