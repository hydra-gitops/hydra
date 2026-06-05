package cmd

import (
	hflags "hydra-gitops.org/hydra/hydra-go/cli/flags"
	"github.com/spf13/cobra"
)

func mergeAndValidateClusterREST(cmd *cobra.Command, dst *hflags.ClusterRESTClientFlags) error {
	if err := hflags.MergeClusterRESTFromCmd(cmd, dst); err != nil {
		return err
	}
	return hflags.ValidateClusterRESTClientFlags(dst)
}
