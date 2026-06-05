package cmd

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/base/buildinfo"
	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the hydra version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(buildinfo.CLIString())
			return nil
		},
	}
}
