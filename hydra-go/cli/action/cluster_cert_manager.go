package action

import (
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// ClusterCertManagerFlags contains common configuration options for cert-manager operations.
type ClusterCertManagerFlags struct {
	flags.ContextFlag
	flags.ClusterFlag
	flags.ColorFlag
	flags.DryRunFlag
	flags.NoClusterFlag
	flags.ForceBackupRestoreFlag
	flags.NoCacheFlag
}

func (f *ClusterCertManagerFlags) Flags() flags.Flags {
	return f
}

// ClusterCertManagerRestore restores cert-manager resources to the cluster.
func ClusterCertManagerRestore(f ClusterCertManagerFlags) error {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	if f.NoCluster {
		l.Info(logIdAction, "no-cluster mode: skipping cluster operations")
		return nil
	}

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       hydra.RESTClientLimits{},
		ClusterName:  f.Cluster,
	})
	if err != nil {
		return err
	}

	appId, err := certManagerChildAppId(cluster, types.HelmNetworkModeOnline)
	if err != nil {
		return err
	}

	results, err := commands.BackupRestoreWithOptions(
		cluster,
		[]types.AppId{appId},
		types.HelmNetworkModeOnline,
		types.KubernetesVersion(""),
		f.ForceBackupRestore,
		f.Color,
		f.DryRun,
		commands.BackupRestoreOptions{},
	)
	if err != nil {
		return err
	}

	commands.PrintBackupResults(l, results, f.Color)

	if commands.HasConflicts(results) {
		return log.CreateError(errors.ErrAborted,
			"cert-manager restore aborted: some secrets would be overwritten; pass --force-backup-restore to overwrite cluster secrets from backup",
			log.String("app", string(appId)))
	}

	return nil
}

func certManagerChildAppId(cluster *hydra.Cluster, networkMode types.HelmNetworkMode) (types.AppId, error) {
	appIds, err := cluster.AppIds(networkMode)
	if err != nil {
		return "", err
	}

	var matches []types.AppId
	for id := range appIds {
		child, err := id.ChildAppName()
		if err != nil {
			return "", err
		}
		if child != nil && *child == types.ChildAppName("cert-manager") {
			matches = append(matches, id)
		}
	}

	switch len(matches) {
	case 0:
		example := string(cluster.ClusterName) + ".cluster-infra.cert-manager"
		return "", log.CreateError(errors.ErrHydraConfigError,
			"no child app named cert-manager found for cluster {cluster}; use hydra gitops backup restore with a full app id (e.g. {example})",
			log.String("cluster", string(cluster.ClusterName)),
			log.String("example", example))
	case 1:
		return matches[0], nil
	default:
		var b strings.Builder
		for i, id := range matches {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(string(id))
		}
		return "", log.CreateError(errors.ErrHydraConfigError,
			"multiple cert-manager child apps found for cluster {cluster}: {apps}; use hydra gitops backup restore with one app id",
			log.String("cluster", string(cluster.ClusterName)),
			log.String("apps", b.String()))
	}
}
