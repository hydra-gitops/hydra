package action

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type ClusterSyncStatusFlags struct {
	flags.ClusterRESTClientFlags
	flags.ContextFlag
	flags.ColorFlag
	flags.NoCacheFlag
	AppIds []string
}

var _ flags.WithContextFlag = (*ClusterSyncStatusFlags)(nil)
var _ flags.WithColorFlag = (*ClusterSyncStatusFlags)(nil)

func (f *ClusterSyncStatusFlags) Flags() flags.Flags {
	return f
}

type ClusterSyncSetFlags struct {
	flags.ClusterRESTClientFlags
	flags.ContextFlag
	flags.ColorFlag
	flags.DryRunFlag
	flags.NoClusterFlag
	flags.NoCacheFlag
	AppIds []string
}

var _ flags.WithContextFlag = (*ClusterSyncSetFlags)(nil)
var _ flags.WithColorFlag = (*ClusterSyncSetFlags)(nil)
var _ flags.WithDryRunFlag = (*ClusterSyncSetFlags)(nil)

func (f *ClusterSyncSetFlags) Flags() flags.Flags {
	return f
}

func ClusterSyncStatus(f ClusterSyncStatusFlags) error {
	l := log.Default()
	l.DebugLog(logIdAction, "listing AppProject sync (appIds: {appIds})",
		log.String("appIds", fmt.Sprintf("%v", f.AppIds)))

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes),
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		ClusterName:  types.InCluster,
	})
	if err != nil {
		return err
	}

	resolvedAppIds, err := commands.ResolveAppNames(cluster, f.AppIds)
	if err != nil {
		return err
	}

	return commands.SyncStatus(cluster, f.Color, resolvedAppIds)
}

func ClusterSyncAuto(f ClusterSyncSetFlags) error {
	return clusterSyncSet(f, "allow", true)
}

func ClusterSyncManual(f ClusterSyncSetFlags) error {
	return clusterSyncSet(f, "deny", true)
}

func ClusterSyncPrevent(f ClusterSyncSetFlags) error {
	return clusterSyncSet(f, "deny", false)
}

func clusterSyncSet(f ClusterSyncSetFlags, kind string, manualSync bool) error {
	l := log.Default()

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes),
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		ClusterName:  types.InCluster,
	})
	if err != nil {
		return err
	}

	resolvedAppIds, err := commands.ResolveAppNames(cluster, f.AppIds)
	if err != nil {
		return err
	}

	if f.NoCluster {
		l.Info(logIdAction, "no-cluster mode: resolved {count} application(s) — skipping cluster operations",
			log.Int("count", len(resolvedAppIds)))
		return nil
	}

	for _, appId := range resolvedAppIds {
		err := commands.SyncSet(cluster, appId, kind, manualSync, f.DryRun)
		if err != nil {
			return err
		}
	}

	l.Info(logIdAction, "successfully processed {count} application(s)",
		log.Int("count", len(resolvedAppIds)))
	return nil
}
