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

package ctxio

import (
	"context"
	"io"
	"os"
)

type contextKey int

const (
	stdinKey contextKey = iota
	stdoutKey
	stderrKey
)

func WithStdin(ctx context.Context, stdin io.Reader) context.Context {
	return context.WithValue(ctx, stdinKey, stdin)
}

func Stdin(ctx context.Context) io.Reader {
	if stdin, ok := ctx.Value(stdinKey).(io.Reader); ok {
		return stdin
	}
	return os.Stdin
}

func WithStdout(ctx context.Context, stdout io.Writer) context.Context {
	return context.WithValue(ctx, stdoutKey, stdout)
}

func Stdout(ctx context.Context) io.Writer {
	if stdout, ok := ctx.Value(stdoutKey).(io.Writer); ok {
		return stdout
	}
	return os.Stdout
}

func WithStderr(ctx context.Context, stderr io.Writer) context.Context {
	return context.WithValue(ctx, stderrKey, stderr)
}

func Stderr(ctx context.Context) io.Writer {
	if stderr, ok := ctx.Value(stderrKey).(io.Writer); ok {
		return stderr
	}
	return os.Stderr
}
