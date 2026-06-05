package commands

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

func LogEntityIds(l log.Logger, entities entity.Entities) {
	logEntityIds(l.InfoLog, entities)
}

// LogEntityErrorIds lists entity ids at error severity (e.g. uninstall abort diagnostics).
func LogEntityErrorIds(l log.Logger, entities entity.Entities) {
	logEntityIds(l.ErrorLog, entities)
}

func DebugLogEntityIds(l log.Logger, entities entity.Entities) {
	logEntityIds(l.DebugLog, entities)
}

func logEntityIds(logFn func(log.LogId, string, ...any), entities entity.Entities) {
	sorted, err := entities.Sort(entity.NewIdFieldOrder(types.DirectionAscending))
	if err != nil {
		ids := entities.IdSet.UnsortedList()
		slices.SortFunc(ids, func(a, b types.Id) int {
			return cmp.Compare(string(a), string(b))
		})
		for _, id := range ids {
			logFn(logIdCommands, " * {entity}", log.String("entity", string(id)))
		}
		return
	}
	for _, e := range sorted.Items {
		id, err := e.Id()
		if err != nil {
			continue
		}
		logFn(logIdCommands, " * {entity}", log.String("entity", string(id)))
	}
}

func markAsSelected(l log.Logger, msg string) func(entity.Entities, entity.Entities, error) (entity.Entities, error) {
	return func(all entity.Entities, selected entity.Entities, err error) (entity.Entities, error) {
		l.DebugLog(logIdCommands, "{msg} matched {count} entities", log.String("msg", msg), log.Int("count", selected.Len()))
		return all, err
	}
}

// check for argocd tracking id
func MarkAsSelectedArgoCdManagedResources(
	l log.Logger,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	appIds sets.Set[types.AppId],
) (entity.Entities, error) {
	return markAsSelected(l, "ArgoCD managed resources")(entities.Select(func(e entity.Entity) (bool, error) {
		u, ok := e.Unstructured(key)
		if !ok {
			return false, nil
		}
		annotations := u.GetAnnotations()

		if len(annotations) == 0 {
			return false, nil
		}

		trackingId, ok := annotations["argocd.argoproj.io/tracking-id"]
		if !ok {
			return false, nil
		}

		app, _, ok := strings.Cut(trackingId, ":")
		if !ok {
			return false, nil
		}

		if !appIds.Has(types.AppId(app)) {
			return false, nil
		}

		group, err := e.Group()
		if err != nil {
			return false, nil
		}

		kind, err := e.Kind()
		if err != nil {
			return false, nil
		}

		name, err := e.Name()
		if err != nil {
			return false, nil
		}
		ns, err := e.Namespace()
		if err != nil {
			ans, err := e.AppNamespace()
			if err != nil {
				return false, nil
			}
			ns = types.Namespace(ans)
		}

		trackingIdExpected := fmt.Sprintf("%s:%s/%s:%s/%s", app, group, kind, ns, name)

		return trackingId == trackingIdExpected, nil
	}))
}

func MarkAsSelectedPartOf(
	l log.Logger,
	entities entity.Entities,
	groupKey types.EntityKeyUnstructured,
	keys []types.EntityKeyUnstructured,
) (entity.Entities, error) {
	partOfLabels, err := entities.GroupByLabel(groupKey, types.LabelAppPartOf)
	if err != nil {
		return entity.Entities{}, err
	}

	delete(partOfLabels, types.LabelValue("")) // remove entities without part-of label

	partOfs := slices.Sorted(maps.Keys(partOfLabels))
	for _, partOf := range partOfs {
		l.DebugLog(logIdCommands, "found part-of label '{label}'", log.String("label", string(partOf)))
	}
	partOfSet := sets.New(partOfs...)

	return markAsSelected(l, "part-of label")(entities.Select(func(e entity.Entity) (bool, error) {
		for _, key := range keys {
			u, ok := e.Unstructured(key)
			if !ok {
				return false, nil
			}

			labels := u.GetLabels()
			if len(labels) == 0 {
				return false, nil
			}

			partOf, ok := labels[string(types.LabelAppPartOf)]
			if !ok {
				return false, nil
			}

			if partOfSet.Has(types.LabelValue(partOf)) {
				return true, nil
			}
		}

		return false, nil
	}))
}

// MarkAsSelectedByUninstallPredicates marks entities matching uninstall and backup ref predicates.
// rendered must be the selected-app template render used to collect those predicates (same as
// RenderClusterSelectedApps(cluster, HelmNetworkModeOffline, "", appIds, KeyTemplateEntity)); it is
// required so CEL inventory helpers (managedNamespaces(), templateEntities(), …) match predicate collection.
func MarkAsSelectedByUninstallPredicates(
	cluster *hydra.Cluster,
	entities entity.Entities,
	appIds sets.Set[types.AppId],
	rendered entity.Entities,
) (entity.Entities, error) {
	l := cluster.L()
	env, err := cel.NewEnvWithEntityInventory(rendered)
	if err != nil {
		return entity.Entities{}, err
	}

	hydra.WarnDuplicateBackupUninstallTags(cluster, appIds, types.HelmNetworkModeOffline, rendered)

	predicateContexts, err := hydra.HydraAppUninstallPredicateContexts(cluster, appIds, types.HelmNetworkModeOffline, rendered)
	if err != nil {
		return entity.Entities{}, err
	}

	backupPredicateContexts, err := hydra.HydraAppBackupPredicateContexts(cluster, appIds, types.HelmNetworkModeOffline, rendered)
	if err != nil {
		return entity.Entities{}, err
	}
	predicateContexts = append(predicateContexts, backupPredicateContexts...)

	l.DebugLog(logIdCommands, "found {count} uninstall predicates (incl. backup)", log.Int("count", len(predicateContexts)))
	var predicateBar log.Progress
	var predicateTask log.ProgressTask
	if log.TerminalProgressUI() && len(predicateContexts) > 0 {
		predicateBar, err = l.NewProgress("uninstall · predicate checks (CEL)", len(predicateContexts))
		if err != nil {
			return entity.Entities{}, err
		}
		predicateTask = predicateBar.NewTask("")
		defer func() {
			if predicateBar != nil {
				_ = predicateBar.Close()
			}
		}()
	}
	for idx, predicateCtx := range predicateContexts {
		if predicateTask != nil {
			predicateTask.SetDetail(k8s.TruncateFooterDetail(predicateCtx.Summary()))
		}
		predicate := strings.TrimSpace(predicateCtx.Predicate)
		if predicate == "" {
			return entity.Entities{}, fmt.Errorf("empty uninstall selection predicate from %s", predicateCtx.Summary())
		}
		programs, err := env.CompilePredicate("clusterEntity != null", types.CelPredicate(predicate))
		if err != nil {
			return entity.Entities{}, fmt.Errorf("uninstall selection predicate compile failed for %s expression=%q: %w", predicateCtx.Summary(), predicate, err)
		}

		var matched entity.Entities
		entities, matched, err = programs.Select(entities)
		if err != nil {
			return entity.Entities{}, err
		}

		l.DebugLog(logIdCommands, "uninstall selection predicate: {cel} matched {count} entities ({context})",
			log.String("cel", predicate), log.Int("count", matched.Len()), log.String("context", predicateCtx.Summary()))
		DebugLogEntityIds(l, matched)
		if predicateBar != nil {
			predicateBar.Advance(idx+1, len(predicateContexts))
		}
	}
	if predicateBar != nil {
		_ = predicateBar.Close()
		predicateBar = nil
		l.Info(logIdCommands, "uninstall · predicate checks (CEL): completed {count} predicate(s)",
			log.Int("count", len(predicateContexts)))
	}

	return entities, nil
}

// separateEntitiesByForcePredicates classifies leftovers using uninstall-force predicates.
// rendered must be the same selected-app template render used to collect forcePredicates
// (same contract as HydraAppUninstallForcePredicates) so CEL inventory matches predicate compilation.
func separateEntitiesByForcePredicates(
	leftovers entity.Entities,
	forcePredicates []string,
	rendered entity.Entities,
) (forceLeftovers, untrackedLeftovers entity.Entities, err error) {
	if len(forcePredicates) == 0 || leftovers.Len() == 0 {
		empty, err := entity.NewEntities(nil)
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}
		return empty, leftovers, nil
	}

	env, err := cel.NewEnvWithEntityInventory(rendered)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}

	matchedIds := sets.New[types.Id]()
	for _, predicate := range forcePredicates {
		programs, err := env.CompilePredicate(types.CelPredicate(predicate))
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}

		_, matched, err := programs.Select(leftovers)
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}

		for _, e := range matched.Items {
			id, idErr := e.Id()
			if idErr != nil {
				return entity.Entities{}, entity.Entities{}, idErr
			}
			matchedIds.Insert(id)
		}
	}

	var forceItems []entity.Entity
	var untrackedItems []entity.Entity
	for _, e := range leftovers.Items {
		id, idErr := e.Id()
		if idErr != nil {
			return entity.Entities{}, entity.Entities{}, idErr
		}
		if matchedIds.Has(id) {
			forceItems = append(forceItems, e)
		} else {
			untrackedItems = append(untrackedItems, e)
		}
	}

	forceLeftovers, err = entity.NewEntities(forceItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	untrackedLeftovers, err = entity.NewEntities(untrackedItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}

	return forceLeftovers, untrackedLeftovers, nil
}

func SeparateUninstallForceLeftovers(
	cluster *hydra.Cluster,
	leftovers entity.Entities,
	appIds sets.Set[types.AppId],
) (forceLeftovers, untrackedLeftovers entity.Entities, err error) {
	rendered, err := RenderClusterSelectedApps(cluster, types.HelmNetworkModeOffline, "", appIds, types.KeyTemplateEntity,
		WithSkipFoundDefinitionsInfoLog())
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	forcePredicates, err := hydra.HydraAppUninstallForcePredicates(cluster, appIds, types.HelmNetworkModeOffline, rendered)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	return separateEntitiesByForcePredicates(leftovers, forcePredicates, rendered)
}

// MarkAsSelectedBySafeForUninstallationPredicates marks entities matching uninstall-safe predicates.
// renderedAllApps must be the template render of all cluster apps (same as
// RenderClusterSelectedApps(cluster, HelmNetworkModeOffline, "", allAppIds, KeyTemplateEntity)) so
// CEL inventory and predicate collection use the same entity set.
// safeNamespaces is the intersection of leftover uninstall namespaces with namespaces where every
// stakeholder app is in the selected uninstall set (see NamespacesAllowingUninstallSafe).
func MarkAsSelectedBySafeForUninstallationPredicates(
	cluster *hydra.Cluster,
	entities entity.Entities,
	allAppIds sets.Set[types.AppId],
	safeNamespaces sets.Set[types.Namespace],
	renderedAllApps entity.Entities,
) (entity.Entities, error) {
	l := cluster.L()
	rendered := renderedAllApps

	env, err := cel.NewEnvWithEntityInventoryOverlay(rendered, entities, cel.SetSupport("namespaces", safeNamespaces))
	if err != nil {
		return entity.Entities{}, err
	}

	safePredicates, err := hydra.HydraAppUninstallSafePredicates(cluster, allAppIds, types.HelmNetworkModeOffline, rendered)
	if err != nil {
		return entity.Entities{}, err
	}

	l.DebugLog(logIdCommands, "found {count} uninstall-safe predicates", log.Int("count", len(safePredicates)))
	for _, predicate := range safePredicates {
		programs, err := env.CompilePredicate(
			"clusterEntity != null",
			"ns in namespaces",
			types.CelPredicate(predicate),
		)
		if err != nil {
			return entity.Entities{}, err
		}

		var matched entity.Entities
		entities, matched, err = programs.Select(entities)
		if err != nil {
			return entity.Entities{}, err
		}

		l.DebugLog(logIdCommands, "uninstall-safe selection predicate: {cel} matched {count} entities",
			log.String("cel", predicate), log.Int("count", matched.Len()))
		DebugLogEntityIds(l, matched)
	}

	return entities, nil
}

func EntityIdsMatchingSafeForUninstallationPredicates(
	cluster *hydra.Cluster,
	entities entity.Entities,
	allAppIds sets.Set[types.AppId],
	safeNamespaces sets.Set[types.Namespace],
	renderedAllApps entity.Entities,
) (sets.Set[types.Id], error) {
	if entities.Len() == 0 || safeNamespaces.Len() == 0 {
		return sets.New[types.Id](), nil
	}

	safePredicates, err := hydra.HydraAppUninstallSafePredicates(cluster, allAppIds, types.HelmNetworkModeOffline, renderedAllApps)
	if err != nil {
		return nil, err
	}
	if len(safePredicates) == 0 {
		return sets.New[types.Id](), nil
	}

	env, err := cel.NewEnvWithEntityInventoryOverlay(renderedAllApps, entities, cel.SetSupport("namespaces", safeNamespaces))
	if err != nil {
		return nil, err
	}

	return matchedClusterEntityIdsSafeStyle(env, entities, safePredicates)
}
