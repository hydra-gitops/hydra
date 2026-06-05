package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterTemplateCommandRegisters(t *testing.T) {
	cmd := NewClusterCommand(ClusterCommandParams{})

	tpl := childCommandWithUse(cmd, "template")
	require.NotNil(t, tpl, "expected hydra gitops template to exist")
	assert.Contains(t, tpl.Use, "template")

	assert.NotNil(t, tpl.Flags().Lookup("hydra-context"))
	assert.NotNil(t, tpl.Flags().Lookup("crd-mode"))
	assert.NotNil(t, tpl.Flags().Lookup("helm-network-mode"))
}
