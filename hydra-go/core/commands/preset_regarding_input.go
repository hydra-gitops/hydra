package commands

import (
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/references"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/workloadclosure"
)

// WorkloadClosureMatchInputFromInventory builds refs and indexes over refInventory (typically the
// full live cluster list) for preset workload-closure matching and app ref-ownership closure.
func WorkloadClosureMatchInputFromInventory(
	l log.Logger,
	refInventory entity.Entities,
	preferredVersions map[types.GroupKindKey]types.Version,
) (workloadclosure.MatchInput, error) {
	if refInventory.Len() == 0 {
		return workloadclosure.EmptyMatchInput(types.KeyClusterEntity), nil
	}
	l.DebugLog(logIdCommands, "workload closure: extract refs from {count} inventory entities",
		log.Int("count", refInventory.Len()))
	refs, err := references.Refs(l, refInventory, types.KeyClusterEntity, nil, entity.Entities{}, entity.Entities{}, preferredVersions, nil)
	if err != nil {
		return workloadclosure.MatchInput{}, err
	}
	l.DebugLog(logIdCommands, "workload closure: build match input",
		log.Int("refs", len(refs)),
		log.Int("entities", refInventory.Len()))
	return workloadclosure.NewMatchInput(refs, refInventory, types.KeyClusterEntity)
}

// WorkloadClosureMatchInputFromMergedInventory builds workload-closure input from an already
// computed merged inventory ref graph. renderedTemplates are available as ref sources for
// template-only workload anchors, while UID owner resolution still comes from refInventory.
func WorkloadClosureMatchInputFromMergedInventory(
	mergedRefs []types.Ref,
	refInventory entity.Entities,
	renderedTemplates entity.Entities,
) (workloadclosure.MatchInput, error) {
	if len(mergedRefs) == 0 && refInventory.Len() == 0 && renderedTemplates.Len() == 0 {
		return workloadclosure.EmptyMatchInput(types.KeyClusterEntity), nil
	}
	return workloadclosure.NewMatchInputWithExtraEntities(
		mergedRefs,
		refInventory,
		renderedTemplates,
		types.KeyClusterEntity,
	)
}
