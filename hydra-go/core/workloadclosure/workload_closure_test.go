package workloadclosure

import (
	"testing"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"github.com/stretchr/testify/require"
)

func TestEmptyMatchInput_NoRefsOrEntities(t *testing.T) {
	in := EmptyMatchInput(types.KeyClusterEntity)
	require.Empty(t, in.Refs)
	require.Empty(t, in.EntityByID)
	require.Empty(t, in.UIDMap)
	require.Empty(t, in.Index)
	require.Equal(t, types.KeyClusterEntity, in.Key)
}
