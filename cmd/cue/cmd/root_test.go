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

package cmd

import "testing"

func TestHelp(t *testing.T) {
	cmd, err := New([]string{"help"})
	if err != nil || cmd == nil {
		t.Error("help command failed unexpectedly")
	}

	cmd, err = New([]string{"--help"})
	if err != nil || cmd == nil {
		t.Error("help command failed unexpectedly")
	}

	cmd, err = New([]string{"-h"})
	if err != nil || cmd == nil {
		t.Error("help command failed unexpectedly")
	}

	cmd, err = New([]string{"help", "cmd"})
	if err != nil || cmd == nil {
		t.Error("help command failed unexpectedly")
	}

	cmd, err = New([]string{"cmd", "--help"})
	if err != nil || cmd == nil {
		t.Error("help command failed unexpectedly")
	}

	cmd, err = New([]string{"cmd", "-h"})
	if err != nil || cmd == nil {
		t.Error("help command failed unexpectedly")
	}

	cmd, err = New([]string{"help", "eval"})
	if err != nil || cmd == nil {
		t.Error("help command failed unexpectedly")
	}

	cmd, err = New([]string{"eval", "--help"})
	if err != nil || cmd == nil {
		t.Error("help command failed unexpectedly")
	}
}
