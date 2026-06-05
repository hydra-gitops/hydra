package commands

import (
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// RenderClusterEachAppSeparate renders each app in appIds independently and returns a map
// keyed by app id. Used for uninstall ownership validation and per-app uninstall-force
// classification (same render contract as [RenderClusterSelectedApps] for a single app).
func RenderClusterEachAppSeparate(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
	appIds sets.Set[types.AppId],
	key types.EntityKeyUnstructured,
	opts ...RenderClusterSelectedAppsOption,
) (map[types.AppId]entity.Entities, error) {
	out := make(map[types.AppId]entity.Entities, appIds.Len())
	for appId := range appIds {
		ents, err := RenderClusterSelectedApps(
			cluster, networkMode, kubernetesVersion, sets.New(appId), key, opts...)
		if err != nil {
			return nil, err
		}
		out[appId] = ents
	}
	return out, nil
}
