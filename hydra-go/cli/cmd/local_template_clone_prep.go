package cmd

import (
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/cli/action"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// localTemplatePatchedCloneEntities computes materialized clone entities appended after
// per-app template output by hydra local template (patched diff against the full-cluster render).
func localTemplatePatchedCloneEntities(
	l log.Logger,
	cluster *hydra.Cluster,
	selectedAppIds sets.Set[types.AppId],
	f *action.TemplateFlags,
) (entity.Entities, error) {
	var empty entity.Entities

	allAppIds, err := cluster.AppIds(f.HelmNetworkMode)
	if err != nil {
		return empty, err
	}
	fullRender, err := commands.RenderClusterSelectedApps(cluster, f.HelmNetworkMode, "", allAppIds, types.KeyTemplateEntity)
	if err != nil {
		return empty, err
	}
	rules, err := hydra.HydraAppCloneRules(cluster, allAppIds, f.HelmNetworkMode, fullRender)
	if err != nil {
		return empty, err
	}
	if len(rules) == 0 {
		if f.Bootstrap == types.BootstrapYes {
			if err := commands.ValidateBootstrapTemplateClones(f.Bootstrap, nil, 0); err != nil {
				return empty, err
			}
		}
		return empty, nil
	}

	withClones, bootCount, err := commands.MaterializeHydraClonesForApply(
		l, cluster, allAppIds, fullRender, types.KeyTemplateEntity, f.Bootstrap, f.HelmNetworkMode, nil)
	if err != nil {
		return empty, err
	}
	if err := commands.ValidateBootstrapTemplateClones(f.Bootstrap, rules, bootCount); err != nil {
		return empty, err
	}
	added, err := commands.DiffEntities(fullRender, withClones)
	if err != nil {
		return empty, err
	}
	partitionRender, err := commands.RenderClusterSelectedApps(
		cluster, f.HelmNetworkMode, f.KubernetesVersion, selectedAppIds, types.KeyTemplateEntity)
	if err != nil {
		return empty, err
	}
	patchedAdded, err := commands.ApplyTemplatePatchesUsingPartitionRender(
		cluster, selectedAppIds, f.HelmNetworkMode, partitionRender, fullRender, added, types.KeyTemplateEntity)
	if err != nil {
		return empty, err
	}
	return patchedAdded, nil
}
