package cmd

import (
	"io/ioutil"
	"path/filepath"

	"cuelang.org/go/encoding/gocode"
	"github.com/spf13/cobra"
)

func newGenGoCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go [packages]",
		Short: "generates go stubs for validation functions and method from a given CUE instance",
		Long:  "",
		RunE:  mkRunE(c, runGenGo),
	}

	cmd.Flags().StringP(string(flagOutputFileName), "n", "cue_gen.go",
		"name of the generated output file")

	return cmd
}

const (
	flagOutputFileName flagName = "name"
)

func runGenGo(cmd *Command, args []string) error {
	instances := buildFromArgs(cmd, args)
	outputFileName := flagOutputFileName.String(cmd)

	for _, inst := range instances {
		// package path
		pkgPath := inst.Dir

		// generate code for instance
		b, err := gocode.Generate(pkgPath, inst, nil)
		exitIfErr(cmd, inst, err, true)

		// write generated file
		goFile := filepath.Join(pkgPath, outputFileName)
		err = ioutil.WriteFile(goFile, b, 0644)
		exitIfErr(cmd, inst, err, true)
	}

	return nil
}
