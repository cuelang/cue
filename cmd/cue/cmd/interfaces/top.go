// Copyright 2019 CUE Authors
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

package interfaces

import (
	"encoding/json"
)

type (
	jsonMarshaler   = json.Marshaler
	jsonUnmarshaler = json.Unmarshaler

	yamlMarshal   interface{ MarshalYAML() (interface{}, error) }
	yamlUnmarshal interface {
		UnmarshalYAML(func(interface{}) error) error
	}
)

// Suppress incorrect linter errors (types are used).
var (
	_ = jsonMarshaler(nil)
	_ = jsonUnmarshaler(nil)
	_ = yamlMarshal(nil)
	_ = yamlUnmarshal(nil)
)
