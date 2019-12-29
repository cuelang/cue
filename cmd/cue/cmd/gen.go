package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newGenCmd(c *Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gen <language> [packages]",
		Short: "generate language specific code useful for embedding cue",
		Long:  "",
		RunE: mkRunE(c, func(cmd *Command, args []string) error {
			stderr := cmd.Stderr()
			if len(args) == 0 {
				fmt.Fprintln(stderr, "get must be run as one of its subcommands")
			} else {
				fmt.Fprintf(stderr, "get must be run as one of its subcommands: unknown subcommand %q\n", args[0])
			}
			fmt.Fprintln(stderr, "Run 'cue help get' for known subcommands.")
			os.Exit(1) // TODO: get rid of this
			return nil
		}),
	}
	cmd.AddCommand(newGenGoCmd(c))
	return cmd
}
