// Copyright 2018 The CUE Authors
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

package file

// Read reads the contents of a file.
Read: {
	kind: "tool/file.Read"

	// filename names the file to read.
	filename: !=""

	// contents is the read contents. If the contents are constraint to bytes
	// (the default), the file is read as is. If it is constraint to a string,
	// the contents are checked to be valid UTF-8.
	contents: *bytes | string
}

// Append writes contents to the given file.
Append: {
	kind: "tool/file.Append"

	// filename names the file to append.
	filename: !=""

	// permissions defines the permissions to use if the file does not yet exist.
	permissions: int | *0o644

	// contents specifies the bytes to be written.
	contents: bytes | string
}

// Create writes contents to the given file.
Create: {
	kind: "tool/file.Create"

	// filename names the file to write.
	filename: !=""

	// permissions defines the permissions to use if the file does not yet exist.
	permissions: int | *0o644

	// contents specifies the bytes to be written.
	contents: bytes | string
}

Glob: {
	kind: "tool/file.Glob"

	glob: !=""
	files: [...string]
}
