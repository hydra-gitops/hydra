package commands

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	hcel "hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"hydra-gitops.org/hydra/hydra-go/core/references"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

type Inventory struct {
	mu                          sync.RWMutex
	cluster                     *hydra.Cluster
	networkMode                 types.HelmNetworkMode
	bootstrap                   types.Bootstrap
	templates                   entity.Entities
	live                        entity.Entities
	entitiesByID                map[types.Id]InventoryEntity
	scopeInfo                   types.ScopeInfoMap
	kubernetesMinor             int
	includeCloneMaterialization bool
	liveOnly                    bool
	showProgress                bool
	dynamicClient               dynamic.Interface
	watchCtx                    context.Context
	watchCancel                 context.CancelFunc
	watchWG                     sync.WaitGroup
	refsTemplate                []types.Ref
	refsCluster                 []types.Ref
	refsMerged                  []types.Ref
	refsErr                     error
	refsReady                   bool
}

type inventoryOption func(*inventoryBuildOptions)

type inventoryBuildOptions struct {
	scopeInfo                   types.ScopeInfoMap
	kubernetesMinor             int
	watchCtx                    context.Context
	dynamicClient               dynamic.Interface
	includeCloneMaterialization bool
	showProgress                bool
	parallel                    int
}

// InventoryEntity is the unified per-id view of the cluster command inventory.
//
// It keeps template and live state for the same normalized id in one record so command logic can
// progressively move away from separate live/template collections without breaking existing readers.
type InventoryEntity struct {
	ID          types.Id
	Template    entity.Entity
	HasTemplate bool
	Live        entity.Entity
	HasLive     bool

	AssignedApp       types.AppId
	HasAssignedApp    bool
	AssignedPreset    string
	HasAssignedPreset bool

	MatchedPresetIDs          []string
	MatchedPresetRules        []string
	MatchedByPresetIDs        bool
	MatchedByPresetCEL        bool
	MatchedByBuiltinRefParser bool
	MatchedByAppRefParser     bool

	WorkloadScoped bool
}

func (e InventoryEntity) PresenceStatus() string {
	return ResourceInventoryPresenceStatus(e.HasTemplate, e.HasLive)
}

func (e InventoryEntity) HasPresetMatch() bool {
	return len(e.MatchedPresetIDs) > 0
}

func cloneInventoryEntity(item InventoryEntity) InventoryEntity {
	out := item
	out.MatchedPresetIDs = slices.Clone(item.MatchedPresetIDs)
	out.MatchedPresetRules = slices.Clone(item.MatchedPresetRules)
	return out
}

func cloneAssignmentMap(in map[types.Id]types.AppId) map[types.Id]types.AppId {
	if in == nil {
		return nil
	}
	out := make(map[types.Id]types.AppId, len(in))
	for id, app := range in {
		out[id] = app
	}
	return out
}

func buildInventoryEntitiesByID(templates, live entity.Entities) map[types.Id]InventoryEntity {
	entitiesByID := make(map[types.Id]InventoryEntity, templates.Len()+live.Len())
	for _, item := range templates.Items {
		id, err := item.Id()
		if err != nil {
			continue
		}
		entry := entitiesByID[id]
		entry.ID = id
		entry.Template = item
		entry.HasTemplate = true
		entitiesByID[id] = entry
	}
	for _, item := range live.Items {
		id, err := item.Id()
		if err != nil {
			continue
		}
		entry := entitiesByID[id]
		entry.ID = id
		entry.Live = item
		entry.HasLive = true
		entitiesByID[id] = entry
	}
	return entitiesByID
}

func withInventoryScopeInfo(scopeInfo types.ScopeInfoMap) inventoryOption {
	return func(o *inventoryBuildOptions) {
		o.scopeInfo = scopeInfo
	}
}

func withInventoryKubernetesMinor(minor int) inventoryOption {
	return func(o *inventoryBuildOptions) {
		o.kubernetesMinor = minor
	}
}

func withInventoryWatch(ctx context.Context) inventoryOption {
	return func(o *inventoryBuildOptions) {
		o.watchCtx = ctx
	}
}

func withInventoryDynamicClient(dynamicClient dynamic.Interface) inventoryOption {
	return func(o *inventoryBuildOptions) {
		o.dynamicClient = dynamicClient
	}
}

func withInventoryShowProgress(showProgress bool) inventoryOption {
	return func(o *inventoryBuildOptions) {
		o.showProgress = showProgress
	}
}

func withInventoryParallel(parallel int) inventoryOption {
	return func(o *inventoryBuildOptions) {
		o.parallel = parallel
	}
}

func newInventoryFromEntities(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	bootstrap types.Bootstrap,
	templates entity.Entities,
	live entity.Entities,
	opts ...inventoryOption,
) (*Inventory, error) {
	cfg := inventoryBuildOptions{includeCloneMaterialization: true}
	for _, opt := range opts {
		opt(&cfg)
	}
	scopeInfo := cfg.scopeInfo
	if len(scopeInfo) == 0 {
		scopeInfo = inferScopeInfoFromEntities(templates, live)
	}
	minor := cfg.kubernetesMinor
	if minor <= 0 {
		minor = 99
	}
	return newInventory(cluster, networkMode, bootstrap, templates, live, scopeInfo, minor, false, cfg)
}

func newInventory(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	bootstrap types.Bootstrap,
	templates entity.Entities,
	live entity.Entities,
	scopeInfo types.ScopeInfoMap,
	kubernetesMinor int,
	liveOnly bool,
	cfg inventoryBuildOptions,
) (*Inventory, error) {
	classifiedTemplates, classifiedLive, err := classifyInventoryEntities(templates, live, kubernetesMinor, liveOnly)
	if err != nil {
		return nil, err
	}
	inv := &Inventory{
		cluster:                     cluster,
		networkMode:                 networkMode,
		bootstrap:                   bootstrap,
		templates:                   classifiedTemplates,
		live:                        classifiedLive,
		entitiesByID:                buildInventoryEntitiesByID(classifiedTemplates, classifiedLive),
		scopeInfo:                   mergeInferredScopeInfo(scopeInfo, templates, live),
		kubernetesMinor:             kubernetesMinor,
		includeCloneMaterialization: cfg.includeCloneMaterialization,
		liveOnly:                    liveOnly,
		showProgress:                cfg.showProgress,
		dynamicClient:               cfg.dynamicClient,
	}
	if cfg.watchCtx != nil {
		if err := inv.startWatch(cfg.watchCtx); err != nil {
			return nil, err
		}
	}
	return inv, nil
}

func (inv *Inventory) Close() {
	inv.mu.Lock()
	cancel := inv.watchCancel
	inv.watchCancel = nil
	inv.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	inv.watchWG.Wait()
}

// TemplateEntities returns the template-side entity view used by the resource model.
func (inv *Inventory) TemplateEntities() entity.Entities {
	if inv == nil {
		empty, _ := entity.NewEntities(nil)
		return empty
	}
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return materializeUnifiedTemplateEntities(inv.entitiesByID)
}

// LiveEntities returns the live-side entity view used by the resource model.
func (inv *Inventory) LiveEntities() entity.Entities {
	if inv == nil {
		empty, _ := entity.NewEntities(nil)
		return empty
	}
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return materializeUnifiedLiveEntities(inv.entitiesByID)
}

func (inv *Inventory) UnifiedEntity(id types.Id) (InventoryEntity, bool) {
	if inv == nil {
		return InventoryEntity{}, false
	}
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	item, ok := inv.entitiesByID[id]
	return cloneInventoryEntity(item), ok
}

func (inv *Inventory) UnifiedEntities() []InventoryEntity {
	if inv == nil {
		return nil
	}
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	out := make([]InventoryEntity, 0, len(inv.entitiesByID))
	for _, item := range inv.entitiesByID {
		out = append(out, cloneInventoryEntity(item))
	}
	slices.SortFunc(out, func(a, b InventoryEntity) int {
		return slices.Compare([]rune(a.ID), []rune(b.ID))
	})
	return out
}

func (inv *Inventory) LiveFiltered(predicate hcel.Predicate) (entity.Entities, error) {
	inv.mu.RLock()
	items := slices.Clone(inv.live.Items)
	inv.mu.RUnlock()
	return filterLiveEntitiesByPredicate(items, predicate)
}

// LiveEntitiesFiltered filters the live-side unified entity view with the provided predicate.
func (inv *Inventory) LiveEntitiesFiltered(predicate hcel.Predicate) (entity.Entities, error) {
	if inv == nil {
		return entity.NewEntities(nil)
	}
	return filterLiveEntitiesByPredicate(inv.LiveEntities().Items, predicate)
}

func filterLiveEntitiesByPredicate(items []entity.Entity, predicate hcel.Predicate) (entity.Entities, error) {
	filtered := make([]entity.Entity, 0, len(items))
	for _, e := range items {
		ok, err := predicate.EvalBool(e, types.MissingKeysError)
		if err != nil {
			return entity.Entities{}, err
		}
		if ok {
			filtered = append(filtered, e)
		}
	}
	return entity.NewEntities(filtered)
}

func materializeUnifiedTemplateEntities(itemsByID map[types.Id]InventoryEntity) entity.Entities {
	items := make([]entity.Entity, 0, len(itemsByID))
	for _, item := range itemsByID {
		if item.HasTemplate {
			items = append(items, item.Template)
		}
	}
	out, err := entity.NewEntities(items)
	if err != nil {
		empty, _ := entity.NewEntities(nil)
		return empty
	}
	return out
}

func materializeUnifiedLiveEntities(itemsByID map[types.Id]InventoryEntity) entity.Entities {
	items := make([]entity.Entity, 0, len(itemsByID))
	for _, item := range itemsByID {
		if item.HasLive {
			items = append(items, item.Live)
		}
	}
	out, err := entity.NewEntities(items)
	if err != nil {
		empty, _ := entity.NewEntities(nil)
		return empty
	}
	return out
}

func (inv *Inventory) RefsTemplate() ([]types.Ref, error) {
	if err := inv.ensureRefs(); err != nil {
		return nil, err
	}
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return slices.Clone(inv.refsTemplate), nil
}

func (inv *Inventory) RefsCluster() ([]types.Ref, error) {
	if err := inv.ensureRefs(); err != nil {
		return nil, err
	}
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return slices.Clone(inv.refsCluster), nil
}

func (inv *Inventory) RefsMerged() ([]types.Ref, error) {
	if err := inv.ensureRefs(); err != nil {
		return nil, err
	}
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return slices.Clone(inv.refsMerged), nil
}

func (inv *Inventory) PresenceStatus(id types.Id) string {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return ResourceInventoryPresenceStatus(inv.templates.IdSet.Has(id), inv.live.IdSet.Has(id))
}

func (inv *Inventory) ensureRefs() error {
	inv.mu.RLock()
	ready := inv.refsReady
	inv.mu.RUnlock()
	if ready {
		inv.mu.RLock()
		err := inv.refsErr
		inv.mu.RUnlock()
		return err
	}
	inv.mu.Lock()
	defer inv.mu.Unlock()
	if inv.refsReady {
		return inv.refsErr
	}
	inv.recomputeRefsLocked()
	return inv.refsErr
}

func (inv *Inventory) recomputeRefsLocked() {
	refsTemplate, refsCluster, refsMerged, err := inv.computeRefsLocked()
	inv.refsTemplate = refsTemplate
	inv.refsCluster = refsCluster
	inv.refsMerged = refsMerged
	inv.refsErr = err
	inv.refsReady = true
}

func (inv *Inventory) computeRefsLocked() ([]types.Ref, []types.Ref, []types.Ref, error) {
	var preferredVersions map[types.GroupKindKey]types.Version
	var sourceParsers []types.RefParser
	if inv.cluster != nil {
		scopeFromCluster := func() (types.ScopeInfoMap, error) {
			if len(inv.scopeInfo) != 0 {
				return inv.scopeInfo, nil
			}
			return ScopeInfoMapFromCluster(inv.cluster, types.KeyClusterEntity, types.CrdModeSilent)
		}
		var err error
		preferredVersions, err = inv.cluster.PreferredVersions(scopeFromCluster)
		if err != nil {
			return nil, nil, nil, err
		}
		appIds := inventoryAppIds(inv.templates)
		if appIds.Len() > 0 {
			sourceParsers, err = hydra.HydraAppRefParsers(inv.cluster, appIds, inv.networkMode, inv.templates)
			if err != nil {
				return nil, nil, nil, err
			}
			seenSourceCM := sets.New[types.Id]()
			sourceCM2, err := hydra.RefParsersFromHydraConfigMaps(inv.live, types.KeyClusterEntity, seenSourceCM, appIds, appIds)
			if err != nil {
				return nil, nil, nil, err
			}
			sourceParsers = append(sourceParsers, sourceCM2...)
		}
	}

	var withClones entity.Entities
	if inv.includeCloneMaterialization && inv.cluster != nil && inv.templates.Len() > 0 {
		appIds := inventoryAppIds(inv.templates)
		extraCloneRules, err := hydra.CloneRulesFromHydraConfigMaps(inv.live, types.KeyClusterEntity, nil, appIds, appIds)
		if err != nil {
			return nil, nil, nil, err
		}
		withClones, _, err = MaterializeHydraClonesForApply(inv.cluster.L(), inv.cluster, appIds, inv.templates, types.KeyTemplateEntity, inv.bootstrap, inv.networkMode, extraCloneRules)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// When both template and live inventories exist, pass live as clusterInventoryOverlay for the
	// template Refs pass so CEL clusterEntities() sees live objects (merged graph / uninstall-safe
	// workloadRegardingEvent edges from template workloads).
	clusterOverlayForTemplateRefs := entity.Entities{}
	if inv.templates.Len() > 0 && inv.live.Len() > 0 {
		clusterOverlayForTemplateRefs = inv.live
	}

	var refsTemplate []types.Ref
	var refsCluster []types.Ref
	var err error
	if inv.templates.Len() > 0 {
		l := log.Default()
		if inv.cluster != nil {
			l = inv.cluster.L()
		}
		refsProgress, closeProgress := inv.newRefsProgress(l, "inventory refs · templates")
		defer closeProgress()
		refsTemplate, err = references.RefsWithProgress(l, inv.templates, types.KeyTemplateEntity, nil, withClones, clusterOverlayForTemplateRefs, preferredVersions, refsProgress, sourceParsers)
		if err != nil {
			return nil, nil, nil, err
		}
		refsTemplate = references.AnnotateRefsWithSource(refsTemplate, types.RefSourceTemplate)
	}
	if inv.live.Len() > 0 {
		l := log.Default()
		if inv.cluster != nil {
			l = inv.cluster.L()
		}
		refsProgress, closeProgress := inv.newRefsProgress(l, "inventory refs · live")
		defer closeProgress()
		refsCluster, err = references.RefsWithProgress(l, inv.live, types.KeyClusterEntity, nil, entity.Entities{}, entity.Entities{}, preferredVersions, refsProgress, sourceParsers)
		if err != nil {
			return nil, nil, nil, err
		}
		refsCluster = references.AnnotateRefsWithSource(refsCluster, types.RefSourceCluster)
	}

	var refsMerged []types.Ref
	switch {
	case len(refsTemplate) > 0 && len(refsCluster) > 0:
		refsMerged = references.MergeRefLists(refsTemplate, refsCluster)
		refsMerged = references.CanonicalizeOwnerRefTargetsToClusterIDs(refsMerged, inv.live)
		refsMerged = references.MergeRefLists(refsMerged)
		refsMerged, err = augmentRefsWithExplicitKeyAttributes(inv.templates, refsMerged)
		if err != nil {
			return nil, nil, nil, err
		}
		refsMerged = references.EnsureRefsHaveOriginSource(refsMerged, types.RefSourceTemplate)
	case len(refsTemplate) > 0:
		refsMerged = refsTemplate
	case len(refsCluster) > 0:
		refsMerged = references.CanonicalizeOwnerRefTargetsToClusterIDs(refsCluster, inv.live)
		refsMerged = references.MergeRefLists(refsMerged)
	default:
		refsMerged = nil
	}
	return refsTemplate, refsCluster, refsMerged, nil
}

func (inv *Inventory) newRefsProgress(l log.Logger, label string) (references.RefsProgress, func()) {
	if inv == nil || !inv.showProgress {
		return nil, func() {}
	}
	bar, err := l.NewProgress(label, 1)
	if err != nil || bar == nil {
		progress := func(done int, total int, detail string) {
			l.DebugLog(logIdCommands, "{label}: {detail}",
				log.String("label", label),
				log.String("detail", detail),
				log.Int("done", done),
				log.Int("total", total))
		}
		return progress, func() {}
	}
	task := bar.NewTask("")
	progress := func(done int, total int, detail string) {
		if total <= 0 {
			total = 1
		}
		if task != nil {
			task.SetDetail(k8s.TruncateFooterDetail(detail))
		}
		bar.Advance(done, total)
	}
	closeProgress := func() {
		_ = bar.Close()
	}
	return progress, closeProgress
}

func (inv *Inventory) startWatch(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)
	inv.watchCtx = ctx
	inv.watchCancel = cancel
	if inv.dynamicClient == nil {
		if inv.cluster == nil {
			cancel()
			return fmt.Errorf("watch requires cluster or dynamic client")
		}
		restConfig, err := RestConfigForHydra(inv.cluster)
		if err != nil {
			cancel()
			return err
		}
		dynamicClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			cancel()
			return err
		}
		inv.dynamicClient = dynamicClient
	}
	for _, target := range inventoryWatchTargets(inv.templates, inv.live, inv.scopeInfo) {
		inv.watchWG.Add(1)
		go inv.runWatch(ctx, target)
	}
	return nil
}

type inventoryWatchTarget struct {
	gvr        types.GVR
	namespaced bool
}

func (inv *Inventory) runWatch(ctx context.Context, target inventoryWatchTarget) {
	defer inv.watchWG.Done()
	resourceClient := inv.dynamicClient.Resource(target.gvr.K8s())
	watcher, err := resourceClient.Watch(ctx, metav1.ListOptions{})
	if err != nil {
		if inv.cluster != nil {
			inv.cluster.L().Warn(logIdCommands, "inventory watch failed for {gvr}: {err}", log.String("gvr", target.gvr.String()), log.Err(err))
		}
		return
	}
	defer watcher.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-watcher.ResultChan():
			if !ok {
				return
			}
			inv.applyWatchEvent(target, string(evt.Type), evt.Object)
		}
	}
}

func (inv *Inventory) applyWatchEvent(target inventoryWatchTarget, eventType string, obj runtime.Object) {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok || u == nil {
		return
	}
	inv.mu.Lock()
	defer inv.mu.Unlock()
	current := slices.Clone(inv.live.Items)
	liveMap := make(map[types.Id]entity.Entity, len(current))
	for _, item := range current {
		id, err := item.Id()
		if err != nil {
			continue
		}
		liveMap[id] = item
	}

	switch eventType {
	case "DELETED":
		id, err := inventoryIDForObject(*u, target.gvr.Resource, target.namespaced)
		if err == nil {
			delete(liveMap, id)
		}
	default:
		built, err := inventoryEntityFromUnstructured(*u, target.gvr.Resource, target.namespaced, types.KeyClusterEntity)
		if err != nil {
			return
		}
		built, err = classifySingleLiveEntity(built, inv.templates.IdSet, inv.kubernetesMinor, inv.liveOnly)
		if err != nil {
			return
		}
		id, err := built.Id()
		if err != nil {
			return
		}
		liveMap[id] = built
	}

	updated := make([]entity.Entity, 0, len(liveMap))
	for _, item := range liveMap {
		updated = append(updated, item)
	}
	live, err := entity.NewEntities(updated)
	if err != nil {
		return
	}
	inv.live = live
	inv.entitiesByID = buildInventoryEntitiesByID(inv.templates, inv.live)
	if inv.refsReady {
		inv.recomputeRefsLocked()
	} else {
		inv.refsReady = false
		inv.refsErr = nil
		inv.refsTemplate = nil
		inv.refsCluster = nil
		inv.refsMerged = nil
	}
}

func inventoryEntityFromUnstructured(
	u unstructured.Unstructured,
	resource types.Resource,
	namespaced bool,
	key types.EntityKeyUnstructured,
) (entity.Entity, error) {
	gvk := types.NewGVKFromK8s(u.GroupVersionKind())
	builder := entity.NewEntityBuilder().
		WithGVK(gvk).
		WithResource(resource).
		WithKind(types.Kind(u.GetKind())).
		WithName(types.Name(u.GetName())).
		WithNamespaced(types.Namespaced(namespaced)).
		WithUnstructured(key, u)
	if ns := u.GetNamespace(); ns != "" {
		builder = builder.WithNamespace(types.Namespace(ns))
	}
	return builder.Build()
}

func inventoryIDForObject(u unstructured.Unstructured, resource types.Resource, namespaced bool) (types.Id, error) {
	e, err := inventoryEntityFromUnstructured(u, resource, namespaced, types.KeyClusterEntity)
	if err != nil {
		return "", err
	}
	return e.Id()
}

func inventoryWatchTargets(templates, live entity.Entities, scopeInfo types.ScopeInfoMap) []inventoryWatchTarget {
	seen := sets.New[string]()
	targets := make([]inventoryWatchTarget, 0)
	appendEntity := func(e entity.Entity) {
		gvr, err := e.GVR()
		if err != nil {
			return
		}
		key := gvr.String()
		if seen.Has(key) {
			return
		}
		namespaced, err := e.Namespaced()
		if err != nil {
			namespaced = false
		}
		seen.Insert(key)
		targets = append(targets, inventoryWatchTarget{gvr: gvr, namespaced: bool(namespaced)})
	}
	for _, e := range templates.Items {
		appendEntity(e)
	}
	for _, e := range live.Items {
		appendEntity(e)
	}
	for _, info := range scopeInfo {
		gvr := types.NewGVR(info.Group, info.Version, info.Resource)
		key := gvr.String()
		if seen.Has(key) {
			continue
		}
		seen.Insert(key)
		targets = append(targets, inventoryWatchTarget{gvr: gvr, namespaced: bool(info.Namespaced)})
	}
	return targets
}

func inventoryAppIds(templates entity.Entities) sets.Set[types.AppId] {
	out := sets.New[types.AppId]()
	for _, item := range templates.Items {
		appIDs, err := item.AppIds()
		if err != nil {
			if appID, appErr := item.AppId(); appErr == nil && appID != "" {
				out.Insert(appID)
			}
			continue
		}
		for _, appID := range appIDs {
			out.Insert(appID)
		}
	}
	return out
}

func inferScopeInfoFromEntities(entities ...entity.Entities) types.ScopeInfoMap {
	result := types.ScopeInfoMap{}
	for _, ents := range entities {
		for _, item := range ents.Items {
			gvk, err := item.GVKString()
			if err != nil {
				continue
			}
			resource, err := item.Resource()
			if err != nil {
				continue
			}
			group, err := item.Group()
			if err != nil {
				continue
			}
			version, err := item.Version()
			if err != nil {
				continue
			}
			kind, err := item.Kind()
			if err != nil {
				continue
			}
			namespaced, err := item.Namespaced()
			if err != nil {
				continue
			}
			result[gvk] = types.ScopeInfo{Group: group, Version: version, Resource: resource, Kind: kind, Namespaced: namespaced}
		}
	}
	return result
}

func mergeInferredScopeInfo(scopeInfo types.ScopeInfoMap, entities ...entity.Entities) types.ScopeInfoMap {
	result := types.ScopeInfoMap{}
	for key, value := range scopeInfo {
		result[key] = value
	}
	for key, value := range inferScopeInfoFromEntities(entities...) {
		if _, ok := result[key]; ok {
			continue
		}
		result[key] = value
	}
	return result
}

func classifyInventoryEntities(
	templates entity.Entities,
	live entity.Entities,
	kubernetesMinor int,
	liveOnly bool,
) (entity.Entities, entity.Entities, error) {
	templateIDs := templates.IdSet
	builtInIDs := sets.New[types.Id]()
	if !liveOnly {
		builtInIDs = KubernetesBuiltinExpectedIDSet(kubernetesMinor, nil)
	}
	classifiedTemplates := make([]entity.Entity, 0, len(templates.Items))
	classifiedTemplates = append(classifiedTemplates, templates.Items...)
	classifiedLive := make([]entity.Entity, 0, len(live.Items))
	for _, item := range live.Items {
		classified, err := classifySingleLiveEntity(item, templateIDs, kubernetesMinor, liveOnly)
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}
		if !liveOnly {
			id, err := classified.Id()
			if err == nil && builtInIDs.Has(id) {
				classified, err = classified.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
					return b.WithBuiltIn()
				})
				if err != nil {
					return entity.Entities{}, entity.Entities{}, err
				}
			}
		}
		classifiedLive = append(classifiedLive, classified)
	}
	outTemplates, err := entity.NewEntities(classifiedTemplates)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	outLive, err := entity.NewEntities(classifiedLive)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	return outTemplates, outLive, nil
}

func classifySingleLiveEntity(
	item entity.Entity,
	templateIDs sets.Set[types.Id],
	kubernetesMinor int,
	liveOnly bool,
) (entity.Entity, error) {
	id, err := item.Id()
	if err != nil {
		return item, err
	}
	return item.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
		b = b.WithoutAppOwned().WithoutBuiltIn()
		if templateIDs.Has(id) || hasTrackedLiveApp(item, types.KeyClusterEntity) {
			b = b.WithAppOwned()
		}
		if !liveOnly && KubernetesBuiltinExpectedIDSet(kubernetesMinor, nil).Has(id) {
			b = b.WithBuiltIn()
		}
		return b
	})
}

func hasTrackedLiveApp(e entity.Entity, key types.EntityKeyUnstructured) bool {
	u, ok := e.Unstructured(key)
	if !ok {
		return false
	}
	trackingID := u.GetAnnotations()["argocd.argoproj.io/tracking-id"]
	return trackingID != ""
}
