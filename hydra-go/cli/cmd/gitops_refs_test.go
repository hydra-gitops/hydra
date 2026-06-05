package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hydra gitops refs: same CLI shape as hydra local refs (<cluster> + resource id), but the ref
// graph is expected to come from the cluster refs pipeline (rendered templates merged with live
// cluster edges where applicable — same universe as hydra gitops inspect / cluster review),
// not the template-only graph used by hydra local refs.

func TestClusterCommandContainsRefsSubcommand(t *testing.T) {
	cmd := NewClusterCommand(ClusterCommandParams{})

	refsCmd := childCommandWithUsePrefix(cmd, "refs ")
	require.NotNil(t, refsCmd, "expected hydra gitops refs to exist")
	assert.Equal(t, "refs <cluster> <id>", refsCmd.Use)
}

func TestClusterRefsCommandRegistersExpectedFlags(t *testing.T) {
	cmd := NewClusterCommand(ClusterCommandParams{})

	refsCmd := childCommandWithUsePrefix(cmd, "refs ")
	require.NotNil(t, refsCmd, "expected hydra gitops refs to exist")

	assert.NotNil(t, refsCmd.Flags().Lookup("hydra-context"))
	assert.NotNil(t, refsCmd.Flags().Lookup("helm-network-mode"))
	assert.NotNil(t, refsCmd.Flags().Lookup("color"))
	assert.NotNil(t, refsCmd.Flags().Lookup("bootstrap"))
}
