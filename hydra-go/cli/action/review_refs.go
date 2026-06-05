package action

import (
	"fmt"
	"io"
	"os"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/exitcode"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yq"
	"k8s.io/apimachinery/pkg/util/sets"
)

func validateReviewRefsParallel(par int) error {
	if par < 0 {
		return fmt.Errorf("--parallel must not be negative (use 0 for GOMAXPROCS)")
	}
	if par > 64 {
		return fmt.Errorf("--parallel must be at most 64")
	}
	return nil
}

type ReviewRefsFlags struct {
	flags.ClusterRESTClientFlags
	flags.ContextFlag
	flags.ColorFlag
	flags.HelmNetworkModeFlag
	flags.ExcludeAppFlag
	flags.BootstrapFlag
	flags.NoCacheFlag
	flags.ReviewRefsYamlFlag
	flags.ClusterListParallelFlag
	AppIdPatterns []types.AppIdPattern
	// ClusterReviewClusterName is set for `hydra gitops review cluster <name>` (no dots). Empty for
	// `hydra gitops review app ...`.
	ClusterReviewClusterName string
}

var _ flags.WithContextFlag = (*ReviewRefsFlags)(nil)
var _ flags.WithColorFlag = (*ReviewRefsFlags)(nil)
var _ flags.WithBootstrapFlag = (*ReviewRefsFlags)(nil)
var _ flags.WithReviewRefsYamlFlag = (*ReviewRefsFlags)(nil)
var _ flags.WithClusterListParallelFlag = (*ReviewRefsFlags)(nil)

func (f *ReviewRefsFlags) WithBootstrapFlag() *flags.BootstrapFlag {
	return &f.BootstrapFlag
}

func (f *ReviewRefsFlags) WithReviewRefsYamlFlag() *flags.ReviewRefsYamlFlag {
	return &f.ReviewRefsYamlFlag
}

func (f *ReviewRefsFlags) WithClusterListParallelFlag() *flags.ClusterListParallelFlag {
	return &f.ClusterListParallelFlag
}

func (f *ReviewRefsFlags) Flags() flags.Flags {
	return f
}

func ReviewRefs(f ReviewRefsFlags) error {
	if err := validateReviewRefsParallel(f.Parallel); err != nil {
		return err
	}
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedNo)

	appIds, err := commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, false)
	if err != nil {
		return err
	}
	if len(appIds) == 0 {
		return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for review")
	}

	cluster, err := reviewClusterForAppIds(l, f.HydraContext, config, appIds, hydra.RESTClientLimits{})
	if err != nil {
		return err
	}

	var collected []commands.ReviewFinding
	count, err := commands.ReviewRefsCallback(cluster, appIds, f.HelmNetworkMode, f.Bootstrap, f.Parallel, func(ff commands.ReviewFinding) error {
		collected = append(collected, ff)
		return nil
	})
	if err != nil {
		return err
	}
	log.FlushProgressForStdout()
	if err := writeReviewFindingsHumanOrYaml(os.Stdout, f.Color, f.Yaml, collected); err != nil {
		return err
	}

	return finishReviewRefs(l, count)
}

func ClusterReviewRefs(f ReviewRefsFlags) error {
	if err := validateReviewRefsParallel(f.Parallel); err != nil {
		return err
	}
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	var (
		appIds  sets.Set[types.AppId]
		cluster *hydra.Cluster
		err     error
	)
	if f.ClusterReviewClusterName != "" {
		clusterName, nerr := types.NewClusterName(f.ClusterReviewClusterName)
		if nerr != nil {
			return nerr
		}
		appIds, err = commands.ResolveAppIdsInClusterWithExcludes(
			l, f.HydraContext, config, clusterName, f.ExcludeAppPatterns, f.HelmNetworkMode,
			f.ToRESTClientLimits())
		if err != nil {
			return err
		}
		if len(appIds) == 0 {
			return log.CreateError(errors.ErrNoAppsSpecified, "no apps left for review after excludes")
		}
		cluster, err = commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
			Config:       config,
			HydraContext: f.HydraContext,
			Limits:       f.ToRESTClientLimits(),
			ClusterName:  clusterName,
		})
	} else {
		appIds, err = commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, false)
		if err != nil {
			return err
		}
		if len(appIds) == 0 {
			return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for review")
		}
		cluster, err = reviewClusterForAppIds(l, f.HydraContext, config, appIds,
			f.ToRESTClientLimits())
	}
	if err != nil {
		return err
	}

	reportUnassigned := f.ClusterReviewClusterName != ""
	var collected []commands.ReviewFinding
	count, err := commands.ReviewClusterRefsCallback(
		cluster, appIds, f.HelmNetworkMode, f.Bootstrap, reportUnassigned, func(ff commands.ReviewFinding) error {
			collected = append(collected, ff)
			return nil
		}, log.TerminalProgressUI(), f.Parallel)
	if err != nil {
		return err
	}
	log.FlushProgressForStdout()
	if err := writeReviewFindingsHumanOrYaml(os.Stdout, f.Color, f.Yaml, collected); err != nil {
		return err
	}

	return finishReviewRefs(l, count)
}

func writeReviewFindingsHumanOrYaml(w io.Writer, color types.Color, asYaml bool, findings []commands.ReviewFinding) error {
	if asYaml {
		for _, finding := range findings {
			data, err := yq.ToYaml(color, []commands.ReviewFinding{finding})
			if err != nil {
				return err
			}
			if _, err := fmt.Fprint(w, data); err != nil {
				return err
			}
		}
		return nil
	}
	return commands.WriteReviewFindingsGroupedText(w, findings, color)
}

func finishReviewRefs(l log.Logger, findingCount int) error {
	if findingCount == 0 {
		l.Info(logIdAction, "review found no reference issues")
		return nil
	}

	// Log at error level so the count is visible with default logging. Returning exitcode.Error
	// skips root's ErrorLog for the exit message, and Cobra prints failures to SetErr at
	// slog.LevelDebug, which is hidden unless --verbose.
	l.Error(logIdAction, "{count} review finding(s) found", log.Int("count", findingCount))
	return exitcode.Newf(1, "%d review finding(s) found", findingCount)
}

func newReviewFindingStdoutCallback(color types.Color, asYaml bool) commands.ReviewFindingCallback {
	return newReviewFindingWriterCallback(os.Stdout, color, asYaml)
}

func newReviewFindingWriterCallback(w io.Writer, color types.Color, asYaml bool) commands.ReviewFindingCallback {
	return func(finding commands.ReviewFinding) error {
		if asYaml {
			data, err := yq.ToYaml(color, []commands.ReviewFinding{finding})
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(w, data)
			return err
		}
		return commands.WriteReviewFindingsGroupedText(w, []commands.ReviewFinding{finding}, color)
	}
}

func reviewClusterForAppIds(
	l log.Logger,
	hydraContext types.HydraContext,
	config types.Config,
	appIds sets.Set[types.AppId],
	limits hydra.RESTClientLimits,
) (*hydra.Cluster, error) {
	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: hydraContext,
		Limits:       limits,
		AppIds:       appIds,
	})
	if err == nil || !errors.ErrInvalidHydraStructure.MatchesError(err) {
		return cluster, err
	}

	oneAppId, ok := appIds.Clone().PopAny()
	if !ok {
		return nil, log.CreateError(errors.ErrNoAppsSpecified, "no apps specified")
	}

	clusterName, err := oneAppId.ClusterName()
	if err != nil {
		return nil, err
	}

	for appId := range appIds {
		candidateCluster, err := appId.ClusterName()
		if err != nil {
			return nil, err
		}
		if candidateCluster != clusterName {
			return nil, log.CreateError(errors.ErrAppIdsDifferentClusters, "all app ids must belong to the same cluster for review",
				log.String("app1", string(oneAppId)),
				log.String("app2", string(appId)))
		}
	}

	context, err := hydra.NewContext(l, types.ContextPath(hydraContext), config)
	if err != nil {
		return nil, err
	}

	return hydra.NewCluster(context, clusterName, limits)
}
