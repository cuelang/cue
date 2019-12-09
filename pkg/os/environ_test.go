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

package os

import (
	"os"
	"testing"
)

func TestGetenv(t *testing.T) {
	expectedTestval := "es_environ_testvalue"
	os.Setenv("OS_ENVIRON_TEST", expectedTestval)
	defer os.Unsetenv("OS_ENVIRON_TEST")

	if envval := Getenv("OS_ENVIRON_TEST"); envval != expectedTestval {
		t.Errorf("os.Getenv: %q != %q", envval, expectedTestval)
	}

	os.Unsetenv("OS_ENVIRON_UNSET")
	if envval := Getenv("OS_ENVIRON_UNSET"); envval != "" {
		t.Errorf("os.Getenv: unset OS_ENVIRON_UNSET should be empty, but is %q", envval)
	}
}
