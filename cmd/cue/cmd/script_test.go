package cmd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/rogpeppe/testscript"
)

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:           "testdata/script",
		UpdateScripts: *update,
	})
}

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"cue": main,
	}))
}

func main() int {
	err := Main(context.Background(), os.Args[1:])
	if err != nil {
		if err != ErrPrintedError {
			fmt.Println(err)
		}
		return 1
	}
	return 0
}
