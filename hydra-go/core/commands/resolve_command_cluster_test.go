package commands

import (
	"testing"

	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestResolveCommandCluster_RejectsEmptyAppIds(t *testing.T) {
	_, err := ResolveCommandCluster(ResolveCommandClusterOptions{
		Config:       types.NewConfig(types.ColorNo, types.DryRunNo, types.KubernetesConnectionAllowedNo, true),
		HydraContext: types.HydraContext("/does/not/matter"),
		AppIds:       sets.New[types.AppId](),
		Limits:       hydra.RESTClientLimits{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no apps specified")
}

func TestResolveCommandCluster_RejectsAppIdsFromDifferentClusters(t *testing.T) {
	appIds := sets.New[types.AppId](
		types.AppId("prod.apps.api"),
		types.AppId("dev.apps.api"),
	)

	_, err := ResolveCommandCluster(ResolveCommandClusterOptions{
		Config:       types.NewConfig(types.ColorNo, types.DryRunNo, types.KubernetesConnectionAllowedNo, true),
		HydraContext: types.HydraContext("/does/not/matter"),
		AppIds:       appIds,
		Limits:       hydra.RESTClientLimits{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same cluster")
}
