// Copyright 2020 CUE Authors
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

package experiment

import "os"

// This file contains feature flags used to enable experimental features of CUE.

func isEnabled(s string) bool {
	return s == "1"
}

var (
	expAll bool = isEnabled(os.Getenv("CUE_X_ALL"))

	FlexibleConstraints bool = getenv("CUE_X_FLEXIBLE_CONSTRAINTS")
)

func EnableAll() {
	FlexibleConstraints = true
}

func getenv(s string) bool {
	env := os.Getenv(s)
	return expAll || isEnabled(env)
}
