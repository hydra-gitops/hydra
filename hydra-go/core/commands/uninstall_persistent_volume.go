package commands

import (
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// MergePersistentVolumesBoundToUninstallClaims adds cluster PersistentVolumes referenced by
// PersistentVolumeClaim→PersistentVolume ref edges (built-in PVC ref-parser) for every PVC
// already present in uninstalls. This covers cluster-scoped PVs that are otherwise excluded
// from namespace-scoped uninstall selection.
func MergePersistentVolumesBoundToUninstallClaims(
	cluster *hydra.Cluster,
	uninstalls entity.Entities,
	clusterEntities entity.Entities,
	renderedAllApps entity.Entities,
	networkMode types.HelmNetworkMode,
) (entity.Entities, error) {
	refs, err := ClusterInventoryRefs(cluster, networkMode, renderedAllApps, clusterEntities)
	if err != nil {
		return entity.Entities{}, err
	}
	merged, err := mergePersistentVolumesFromInventoryRefs(refs, uninstalls, clusterEntities)
	if err != nil {
		return entity.Entities{}, err
	}
	if merged.Len() > uninstalls.Len() {
		cluster.L().DebugLog(logIdCommands,
			"merged {added} PersistentVolume(s) bound to uninstall PersistentVolumeClaims via refs",
			log.Int("added", merged.Len()-uninstalls.Len()))
	}
	return merged, nil
}

func mergePersistentVolumesFromInventoryRefs(
	refs []types.Ref,
	uninstalls entity.Entities,
	clusterEntities entity.Entities,
) (entity.Entities, error) {
	pvcIds := sets.New[types.Id]()
	for _, e := range uninstalls.Items {
		gvk, err := e.GVKString()
		if err != nil {
			continue
		}
		if gvk != types.KubernetesGvkV1PersistentVolumeClaim {
			continue
		}
		id, err := e.Id()
		if err != nil {
			continue
		}
		pvcIds.Insert(id)
	}
	if pvcIds.Len() == 0 {
		return uninstalls, nil
	}

	pvIds := sets.New[types.Id]()
	for _, ref := range refs {
		if !pvcIds.Has(ref.From) {
			continue
		}
		kind, err := ref.To.Kind()
		if err != nil || kind != types.KubernetesKindPersistentVolume {
			continue
		}
		pvIds.Insert(ref.To)
	}
	if pvIds.Len() == 0 {
		return uninstalls, nil
	}

	_, pvEntities, err := clusterEntities.SelectByIdSet(pvIds)
	if err != nil {
		return entity.Entities{}, err
	}
	if pvEntities.Len() == 0 {
		return uninstalls, nil
	}
	return uninstalls.Merge(pvEntities, types.KeyClusterEntity)
}
