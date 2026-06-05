package commands

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// ClusterApplyCrdScopeMap merges built-in scope, live cluster discovery, and CustomResourceDefinition
// manifests from the full cluster render catalog. Use this scope map when applying a selected app
// subset so custom resource types can resolve namespaced vs cluster scope from CRDs shipped in other
// apps on the same cluster.
func ClusterApplyCrdScopeMap(
	fullCatalog entity.Entities,
	live types.ScopeInfoMap,
	key types.EntityKeyUnstructured,
) (types.ScopeInfoMap, error) {
	crdScope, err := fullCatalog.ScopeInfoMapFromCrds(key)
	if err != nil {
		return nil, err
	}
	return MergeScopeInfoMaps(DefaultScopeInfoMap(), live, crdScope)
}

// ValidateClusterApplyCrdEligibility checks that every API type in the selected apply manifests is
// admitted by either the live cluster discovery map or a CRD object included in the same selection.
// CRDs present only in non-selected apps' renders must not satisfy this check.
func ValidateClusterApplyCrdEligibility(
	selected entity.Entities,
	live types.ScopeInfoMap,
	key types.EntityKeyUnstructured,
) error {
	definedBySelectedCrds, err := selected.ScopeInfoMapFromCrds(key)
	if err != nil {
		return err
	}
	defaultMap := DefaultScopeInfoMap()
	for _, e := range selected.Items {
		gvk, err := e.GVKString()
		if err != nil {
			return err
		}
		if _, ok := defaultMap[gvk]; ok {
			continue
		}
		if _, ok := live[gvk]; ok {
			continue
		}
		if _, ok := definedBySelectedCrds[gvk]; ok {
			continue
		}
		return log.CreateError(errors.ErrRequiredCrdMissing,
			"CRD for {gvk} is required for cluster apply but is not available on the cluster and is not included in the selected apply manifests",
			log.String("gvk", string(gvk)))
	}
	return nil
}
