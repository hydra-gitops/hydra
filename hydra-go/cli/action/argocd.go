package action

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	corehydra "hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type ArgocdStatusFlags struct {
	flags.ContextFlag
	flags.ColorFlag
	flags.ExcludeAppFlag
	flags.NoCacheFlag
	AppIds []string
}

var _ flags.WithContextFlag = (*ArgocdStatusFlags)(nil)
var _ flags.WithColorFlag = (*ArgocdStatusFlags)(nil)
var _ flags.WithExcludeAppFlag = (*ArgocdStatusFlags)(nil)

func (f *ArgocdStatusFlags) Flags() flags.Flags {
	return f
}

type ArgocdSyncSetFlags struct {
	flags.ContextFlag
	flags.ColorFlag
	flags.DryRunFlag
	flags.ExcludeAppFlag
	flags.NoCacheFlag
	AppIds []string
}

var _ flags.WithContextFlag = (*ArgocdSyncSetFlags)(nil)
var _ flags.WithColorFlag = (*ArgocdSyncSetFlags)(nil)
var _ flags.WithDryRunFlag = (*ArgocdSyncSetFlags)(nil)
var _ flags.WithExcludeAppFlag = (*ArgocdSyncSetFlags)(nil)

func (f *ArgocdSyncSetFlags) Flags() flags.Flags {
	return f
}

var argocdResolvePathWithCluster = corehydra.ResolvePathWithCluster

var argocdResolveStatusSelection = func(h corehydra.Hydra, includePatterns []string, excludePatterns []types.AppIdPattern) ([]string, error) {
	allVisibleApps, err := commands.ListArgocdApplicationNames(h)
	if err != nil {
		return nil, err
	}
	return commands.ResolveArgocdStatusSelection(includePatterns, appIdPatternsToStrings(excludePatterns), allVisibleApps)
}

var argocdResolveSyncTargets = func(h corehydra.Hydra, includePatterns []string, excludePatterns []types.AppIdPattern) ([]string, error) {
	allVisibleApps, err := commands.ListArgocdApplicationNames(h)
	if err != nil {
		return nil, err
	}
	return commands.ResolveArgocdSyncTargets(includePatterns, appIdPatternsToStrings(excludePatterns), allVisibleApps)
}

var argocdStatusRun = func(cluster *corehydra.Cluster, color types.Color, appIds []string) error {
	return commands.ArgocdStatus(cluster, color, appIds)
}

var argocdSyncSetRun = func(cluster *corehydra.Cluster, appName string, kind string, manualSync bool, dryRun types.DryRun) error {
	return commands.SyncSet(cluster, appName, kind, manualSync, dryRun)
}

func ArgocdStatus(f ArgocdStatusFlags) error {
	l := log.Default()
	l.DebugLog(logIdAction, "listing ArgoCD application status (appIds: {appIds})",
		log.String("appIds", fmt.Sprintf("%v", f.AppIds)))

	cluster, err := argocdResolvePathWithCluster(l, f.HydraContext, types.InCluster, flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes), corehydra.RESTClientLimits{})
	if err != nil {
		return err
	}

	resolvedAppIds, err := argocdResolveStatusSelection(cluster, f.AppIds, f.ExcludeAppPatterns)
	if err != nil {
		return err
	}

	return argocdStatusRun(cluster, f.Color, resolvedAppIds)
}

func ArgocdSyncAuto(f ArgocdSyncSetFlags) error {
	return argocdSyncSet(f, "allow", true)
}

func ArgocdSyncManual(f ArgocdSyncSetFlags) error {
	return argocdSyncSet(f, "deny", true)
}

func ArgocdSyncPrevent(f ArgocdSyncSetFlags) error {
	return argocdSyncSet(f, "deny", false)
}

func argocdSyncSet(f ArgocdSyncSetFlags, kind string, manualSync bool) error {
	l := log.Default()

	cluster, err := argocdResolvePathWithCluster(l, f.HydraContext, types.InCluster, flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes), corehydra.RESTClientLimits{})
	if err != nil {
		return err
	}

	targetAppIds, err := argocdResolveSyncTargets(cluster, f.AppIds, f.ExcludeAppPatterns)
	if err != nil {
		return err
	}

	for _, appId := range targetAppIds {
		if err := argocdSyncSetRun(cluster, appId, kind, manualSync, f.DryRun); err != nil {
			return err
		}
	}

	l.Info(logIdAction, "successfully processed {count} application(s)",
		log.Int("count", len(targetAppIds)))
	return nil
}

func appIdPatternsToStrings(patterns []types.AppIdPattern) []string {
	result := make([]string, len(patterns))
	for i, pattern := range patterns {
		result[i] = string(pattern)
	}
	return result
}
