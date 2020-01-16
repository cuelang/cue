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

package flag

// A Value are all possible values allowed in flags.
Value :: Scalar | Array

// Scalar are all possible non-composite flag values.
Scalar :: bool | number | string

// Array is an array of scalar values.
Array :: [...Scalar]

// Name indicates a valid flag name.
Name :: =~"[a-z]([_a-z1-9]*[a-z1-9])?"

// Set defines a set of command line flags, the values of which will be set
// at run time. The doc comment of the flag is presented to the user in help.
//
// To define a shorthand, define the shorthand as a new flag referring to
// the flag of which it is a shorthand.
Set :: {
	$id: "tool/flag.Set"

	[Name]: Value
}

// TODO:
// // Help prints the help text for the given command line flags.
// Help :: {
//  $id: "tool/flag.Help"

//  set:  Set
//  text: string
// }
