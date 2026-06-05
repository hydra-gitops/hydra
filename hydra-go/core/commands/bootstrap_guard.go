package commands

import (
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ValidateClusterApplyBootstrapGuard enforces bootstrap-guard rules for cluster apply when
// enforceBootstrapGuard is true. It must run after render and CRD eligibility checks, and before
// ConvertSopsSecretsToSecrets when SOPS decoding is enabled.
func ValidateClusterApplyBootstrapGuard(
	l log.Logger,
	cluster *hydra.Cluster,
	appIds sets.Set[types.AppId],
	networkMode types.HelmNetworkMode,
	rendered entity.Entities,
	bootstrap types.Bootstrap,
	skipBootstrapGuard bool,
	enforceBootstrapGuard bool,
) error {
	if !enforceBootstrapGuard {
		return nil
	}

	if bootstrap == types.BootstrapYes && skipBootstrapGuard {
		return log.CreateError(errors.ErrBootstrapGuard,
			"--bootstrap and --skip-bootstrap-guard cannot be used together")
	}

	predicates, err := hydra.HydraAppBootstrapGuardPredicates(cluster, appIds, networkMode, rendered)
	if err != nil {
		return err
	}

	if len(predicates) == 0 {
		if skipBootstrapGuard {
			l.Warn(logIdCommands, "--skip-bootstrap-guard was set but no bootstrap-guard ref rules apply to the selected apps; the flag has no effect")
		}
		return nil
	}

	if bootstrap == types.BootstrapYes {
		return nil
	}

	env, err := cel.NewEnvWithEntityInventory(rendered)
	if err != nil {
		return err
	}

	matchedIds, err := matchBootstrapGuardEntities(env, predicates, rendered)
	if err != nil {
		return err
	}

	if skipBootstrapGuard {
		if matchedIds.Len() == 0 {
			l.Warn(logIdCommands, "--skip-bootstrap-guard was set but the rendered apply set contains no bootstrap-guard resources; the flag has no effect")
		}
		return nil
	}

	if matchedIds.Len() == 0 {
		return nil
	}

	ids := matchedIds.UnsortedList()
	slices.SortFunc(ids, func(a, b types.Id) int {
		return strings.Compare(string(a), string(b))
	})
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = string(id)
	}
	return log.CreateError(errors.ErrBootstrapGuard,
		"apply selection includes bootstrap-guard resources; use --bootstrap for initial cluster setup or --skip-bootstrap-guard if the sops-secrets-operator is already running: {ids}",
		log.String("ids", strings.Join(idStrs, ", ")))
}

func matchBootstrapGuardEntities(
	env cel.Env,
	predicates []string,
	rendered entity.Entities,
) (sets.Set[types.Id], error) {
	matchedIds := sets.New[types.Id]()
	for _, predicate := range predicates {
		programs, err := env.CompilePredicate(
			"templateEntity != null",
			types.CelPredicate(predicate),
		)
		if err != nil {
			return nil, err
		}
		_, matched, err := programs.Select(rendered)
		if err != nil {
			return nil, err
		}
		for _, e := range matched.Items {
			id, err := e.Id()
			if err != nil {
				return nil, err
			}
			matchedIds.Insert(id)
		}
	}
	return matchedIds, nil
}
