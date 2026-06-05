package action

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type ClusterBackupCreateFlags struct {
	flags.ClusterRESTClientFlags
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.DryRunFlag
	flags.NoClusterFlag
	flags.PredicatesFlag
	flags.ExcludeAppFlag
	flags.NoCacheFlag
	AppIdPatterns []types.AppIdPattern
}

func (f *ClusterBackupCreateFlags) Flags() flags.Flags {
	return f
}

type ClusterBackupRestoreFlags struct {
	flags.ClusterRESTClientFlags
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.DryRunFlag
	flags.NoClusterFlag
	flags.KubernetesVersionFlag
	flags.ForceBackupRestoreFlag
	flags.BackupRestoreCreateNamespacesFlag
	flags.PredicatesFlag
	flags.ExcludeAppFlag
	flags.NoCacheFlag
	AppIdPatterns []types.AppIdPattern
}

func (f *ClusterBackupRestoreFlags) Flags() flags.Flags {
	return f
}

// ClusterBackupCreate creates backups for secrets matched by backup predicates.
func ClusterBackupCreate(f ClusterBackupCreateFlags) error {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	appIds, err := commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, false)
	if err != nil {
		return err
	}

	if len(appIds) == 0 {
		return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for backup")
	}

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		AppIds:       appIds,
	})
	if err != nil {
		return err
	}

	if f.NoCluster {
		l.Info(logIdAction, "no-cluster mode: skipping cluster operations")
		return nil
	}

	appIdSlice := make([]types.AppId, 0, len(appIds))
	for appId := range appIds {
		appIdSlice = append(appIdSlice, appId)
	}

	results, err := commands.BackupCreateWithOptions(cluster, appIdSlice, f.HelmNetworkMode, f.Color, f.DryRun, commands.BackupCreateOptions{
		SecretPredicates: f.Predicates,
	})
	if err != nil {
		return err
	}

	commands.PrintBackupResults(l, results, f.Color)
	return nil
}

type ClusterBackupListFlags struct {
	flags.ClusterRESTClientFlags
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.KubernetesVersionFlag
	flags.ExcludeAppFlag
	AppIdPatterns []types.AppIdPattern
}

func (f *ClusterBackupListFlags) Flags() flags.Flags {
	return f
}

// ClusterBackupList lists backup SopsSecrets found in rendered manifests.
func ClusterBackupList(f ClusterBackupListFlags) error {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedNo)

	appIds, err := commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, false)
	if err != nil {
		return err
	}

	if len(appIds) == 0 {
		return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for backup list")
	}

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		AppIds:       appIds,
	})
	if err != nil {
		return err
	}

	appIdSlice := make([]types.AppId, 0, len(appIds))
	for appId := range appIds {
		appIdSlice = append(appIdSlice, appId)
	}

	results, err := commands.BackupList(cluster, appIdSlice, f.HelmNetworkMode, f.KubernetesVersion)
	if err != nil {
		return err
	}

	commands.PrintBackupResults(l, results, f.Color)
	return nil
}

type ClusterBackupDiffFlags struct {
	flags.ClusterRESTClientFlags
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.NoClusterFlag
	flags.KubernetesVersionFlag
	flags.ExcludeAppFlag
	flags.NoCacheFlag
	AppIdPatterns []types.AppIdPattern
}

func (f *ClusterBackupDiffFlags) Flags() flags.Flags {
	return f
}

// ClusterBackupDiff compares backup secrets with the current cluster state.
func ClusterBackupDiff(f ClusterBackupDiffFlags) error {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	appIds, err := commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, false)
	if err != nil {
		return err
	}

	if len(appIds) == 0 {
		return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for backup diff")
	}

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		AppIds:       appIds,
	})
	if err != nil {
		return err
	}

	if f.NoCluster {
		l.Info(logIdAction, "no-cluster mode: skipping cluster operations")
		return nil
	}

	appIdSlice := make([]types.AppId, 0, len(appIds))
	for appId := range appIds {
		appIdSlice = append(appIdSlice, appId)
	}

	results, err := commands.BackupDiff(cluster, appIdSlice, f.HelmNetworkMode, f.KubernetesVersion, f.Color)
	if err != nil {
		return err
	}

	commands.PrintBackupResults(l, results, f.Color)
	return nil
}

// ClusterBackupRestore restores secrets from backup SopsSecret files.
func ClusterBackupRestore(f ClusterBackupRestoreFlags) error {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	appIds, err := commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, false)
	if err != nil {
		return err
	}

	if len(appIds) == 0 {
		return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for backup restore")
	}

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		AppIds:       appIds,
	})
	if err != nil {
		return err
	}

	if f.NoCluster {
		l.Info(logIdAction, "no-cluster mode: skipping cluster operations")
		return nil
	}

	appIdSlice := make([]types.AppId, 0, len(appIds))
	for appId := range appIds {
		appIdSlice = append(appIdSlice, appId)
	}

	results, err := commands.BackupRestoreWithOptions(cluster, appIdSlice, f.HelmNetworkMode, f.KubernetesVersion, f.ForceBackupRestore, f.Color, f.DryRun, commands.BackupRestoreOptions{
		SecretPredicates:        f.Predicates,
		CreateMissingNamespaces: f.CreateNamespaces,
	})
	if err != nil {
		return err
	}

	commands.PrintBackupResults(l, results, f.Color)

	if commands.HasConflicts(results) {
		return log.CreateError(errors.ErrAborted, "backup restore aborted: some secrets would be overwritten, use --force-backup-restore to proceed")
	}

	return nil
}
