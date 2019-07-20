// Copyright 2019 The CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package base64 implements base64 encoding as specified by RFC 4648.
package base64

import "encoding/base64"

// EncodedLen returns the length in bytes of the base64 encoding
// of an input buffer of length n.
func EncodedLen(n int) int {
	return base64.StdEncoding.EncodedLen(n)
}

// DecodedLen returns the maximum length in bytes of the decoded data
// corresponding to n bytes of base64-encoded data.
func DecodedLen(x int) int {
	return base64.StdEncoding.DecodedLen(x)
}

// Encode returns the base64 encoding of src.
func Encode(src []byte) string {
	return base64.StdEncoding.EncodeToString(src)
}

// Decode returns the bytes represented by the base64 string s.
func Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
