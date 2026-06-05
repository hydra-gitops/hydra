package cmd

import "github.com/spf13/cobra"

func newClusterOnlyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cluster",
		Short: "Commands for cluster-only workflows",
		Long: `Cluster-only workflows will operate directly on Hydra ConfigMaps and
live Kubernetes state without requiring local Hydra definitions.

This command surface is reserved, but not implemented yet. Use 'hydra gitops'
for the current local-plus-cluster workflows.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}
