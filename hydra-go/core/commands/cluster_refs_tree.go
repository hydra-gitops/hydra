package commands

import (
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/references"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ClusterTreeGraph holds refs and entities from one full cluster-tree pipeline run
// (render all apps, list cluster inventory, build reference graph). Use LoadClusterTreeGraph
// once and reuse CandidateIds, EnsureStartId, and Refs for picker plus TUI without reloading.
type ClusterTreeGraph struct {
	Refs         []types.Ref
	TemplateEnts entity.Entities
	ClusterEnts  entity.Entities
}

// clusterTreeGraphConfig holds optional inputs for [LoadClusterTreeGraph] and the central
// inspect ref-graph helper [buildInspectRefGraph].
type clusterTreeGraphConfig struct {
	clusterInventory *entity.Entities
	// renderedTemplateEntities lets internal callers pass pre-rendered + api-version-normalized
	// templates so the pipeline does not call [RenderClusterSelectedApps]+[NormalizeApiVersions]
	// again.
	renderedTemplateEntities *entity.Entities
	// skipFoundDefinitionsInfoLog avoids duplicate "found definitions of …" when templates were
	// already rendered in the same command (e.g. [RenderCluster] before [LoadClusterTreeGraph]).
	skipFoundDefinitionsInfoLog bool
	// includeTemplateRefs / includeClusterRefs let callers ask for only one side of the merged
	// graph. Both default true via [LoadClusterTreeGraph]; [LoadInspectRefGraph] callers set them
	// explicitly through [InspectRefGraphParams].
	includeTemplateRefs         bool
	includeClusterRefs          bool
	includeCloneMaterialization bool
}

// ClusterTreeGraphOption configures [LoadClusterTreeGraph].
type ClusterTreeGraphOption func(*clusterTreeGraphConfig)

// WithClusterInventory supplies live cluster entities from a prior [ListClusterAll] call so the
// tree pipeline does not list the cluster again (avoids duplicate INFO logs and redundant API work).
func WithClusterInventory(ents entity.Entities) ClusterTreeGraphOption {
	return func(c *clusterTreeGraphConfig) {
		inv := ents
		c.clusterInventory = &inv
	}
}

// WithSkipFoundDefinitionsInfoLogInClusterTree suppresses the INFO log "found definitions of …"
// for the template render inside the cluster-tree pipeline. Use when the same workflow already
// logged it (e.g. [RenderCluster] or [RenderClusterSelectedApps] for all apps).
func WithSkipFoundDefinitionsInfoLogInClusterTree() ClusterTreeGraphOption {
	return func(c *clusterTreeGraphConfig) {
		c.skipFoundDefinitionsInfoLog = true
	}
}

// LoadClusterTreeGraph runs the full cluster tree pipeline once.
func LoadClusterTreeGraph(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	bootstrap types.Bootstrap,
	opts ...ClusterTreeGraphOption,
) (*ClusterTreeGraph, error) {
	var cfg clusterTreeGraphConfig
	for _, o := range opts {
		o(&cfg)
	}
	params := InspectRefGraphParams{
		Cluster:                     cluster,
		NetworkMode:                 networkMode,
		Bootstrap:                   bootstrap,
		ClusterInventory:            cfg.clusterInventory,
		IncludeTemplateRefs:         true,
		IncludeClusterRefs:          true,
		IncludeCloneMaterialization: true,
		SkipFoundDefinitionsInfoLog: cfg.skipFoundDefinitionsInfoLog,
	}
	g, err := LoadInspectRefGraph(params)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return &ClusterTreeGraph{}, nil
	}
	return &ClusterTreeGraph{
		Refs:         g.Refs,
		TemplateEnts: g.TemplateEnts,
		ClusterEnts:  g.ClusterEnts,
	}, nil
}

// InspectRefGraphParams configures [LoadInspectRefGraph]: an explicit, central API for the
// cluster-inspect reference graph. Callers (cluster inspect, cluster scale, cluster scale status,
// cluster apply scale-up phase, cluster review, cluster uninstall) set these fields explicitly so
// the merged-graph pipeline does not have to guess which optional sources to include.
type InspectRefGraphParams struct {
	Cluster     *hydra.Cluster
	NetworkMode types.HelmNetworkMode
	Bootstrap   types.Bootstrap
	Inventory   *Inventory

	// RenderedTemplateEntities lets callers pass already-rendered, already api-version-normalized
	// template entities so the pipeline does not call [RenderClusterSelectedApps]+[NormalizeApiVersions]
	// again. Leave nil to render inside.
	RenderedTemplateEntities *entity.Entities
	// ClusterInventory lets callers pass an already-listed cluster inventory (from [ListClusterAll]).
	// Leave nil to list inside.
	ClusterInventory *entity.Entities

	// IncludeTemplateRefs builds refs from the rendered template entities (the "refsSync" set used
	// by scale topological ordering and apply-time ref validation).
	IncludeTemplateRefs bool
	// IncludeClusterRefs builds refs from live cluster inventory and merges them with the template
	// refs into the merged inspect graph (the "refsFull"/inspect set).
	IncludeClusterRefs bool
	// IncludeCloneMaterialization adds hydra clone-rule materialized entities as additional ref
	// sources for the template side (matches cluster inspect / apply / uninstall semantics). Set to
	// false only for callers that purposely want a leaner graph.
	IncludeCloneMaterialization bool

	// SkipFoundDefinitionsInfoLog suppresses the duplicate "found definitions of …" INFO log when
	// the same workflow already rendered all apps before calling this helper.
	SkipFoundDefinitionsInfoLog bool
}

// InspectRefGraph is the result of [LoadInspectRefGraph].
type InspectRefGraph struct {
	// RefsTemplate is the template-only ref set (annotated with [types.RefSourceTemplate]). Use
	// this as the "refsSync" input to ScaleUp/ScaleDownWorkloads / cluster-apply ref validation.
	// Empty if IncludeTemplateRefs was false.
	RefsTemplate []types.Ref
	// RefsCluster is the cluster-only ref set (annotated with [types.RefSourceCluster]). Empty if
	// IncludeClusterRefs was false.
	RefsCluster []types.Ref
	// Refs is the merged + canonicalized + augmented ref graph (template ⊕ cluster). When only one
	// side is included, Refs equals that side's annotated list (still canonicalized for cluster-only
	// inputs). This matches what cluster inspect / cluster review currently consume.
	Refs []types.Ref
	// TemplateEnts is the rendered + api-version-normalized template inventory used for the graph.
	TemplateEnts entity.Entities
	// ClusterEnts is the live cluster inventory used for the graph.
	ClusterEnts entity.Entities
}

// LoadInspectRefGraph is the central entry point for the merged cluster-inspect reference graph.
// It runs the same pipeline as [LoadClusterTreeGraph] (render apps, list cluster, build template +
// cluster refs, merge, canonicalize, augment) but with explicit parameters instead of options so
// every caller (inspect, scale, scale status, apply, review, uninstall) sets the same flags
// consistently and can reuse already-rendered templates / already-listed inventory.
func LoadInspectRefGraph(params InspectRefGraphParams) (*InspectRefGraph, error) {
	if params.Inventory != nil {
		var refsTemplate []types.Ref
		var refsCluster []types.Ref
		var refs []types.Ref
		var err error
		if params.IncludeTemplateRefs {
			refsTemplate, err = params.Inventory.RefsTemplate()
			if err != nil {
				return nil, err
			}
		}
		if params.IncludeClusterRefs {
			refsCluster, err = params.Inventory.RefsCluster()
			if err != nil {
				return nil, err
			}
		}
		switch {
		case params.IncludeTemplateRefs && params.IncludeClusterRefs:
			refs, err = params.Inventory.RefsMerged()
			if err != nil {
				return nil, err
			}
		case params.IncludeTemplateRefs:
			refs = refsTemplate
		case params.IncludeClusterRefs:
			refs = refsCluster
		}
		return &InspectRefGraph{
			RefsTemplate: refsTemplate,
			RefsCluster:  refsCluster,
			Refs:         refs,
			TemplateEnts: params.Inventory.TemplateEntities(),
			ClusterEnts:  params.Inventory.LiveEntities(),
		}, nil
	}

	cfg := &clusterTreeGraphConfig{
		clusterInventory:            params.ClusterInventory,
		skipFoundDefinitionsInfoLog: params.SkipFoundDefinitionsInfoLog,
	}
	if params.RenderedTemplateEntities != nil {
		ents := *params.RenderedTemplateEntities
		cfg.renderedTemplateEntities = &ents
	}
	cfg.includeTemplateRefs = params.IncludeTemplateRefs
	cfg.includeClusterRefs = params.IncludeClusterRefs
	cfg.includeCloneMaterialization = params.IncludeCloneMaterialization
	refsTemplate, refsCluster, refs, templateEnts, clusterEnts, err :=
		buildInspectRefGraph(params.Cluster, params.NetworkMode, params.Bootstrap, cfg)
	if err != nil {
		return nil, err
	}
	return &InspectRefGraph{
		RefsTemplate: refsTemplate,
		RefsCluster:  refsCluster,
		Refs:         refs,
		TemplateEnts: templateEnts,
		ClusterEnts:  clusterEnts,
	}, nil
}

// CandidateIds returns sorted resource ids from templates, the ref graph, and live cluster inventory.
func (g *ClusterTreeGraph) CandidateIds() []types.Id {
	known := knownResourceIds(g.TemplateEnts, g.Refs)
	for _, e := range g.ClusterEnts.Items {
		eid, err := e.Id()
		if err != nil {
			continue
		}
		known.Insert(eid)
	}
	return sortIdSet(known)
}

// EnsureStartId returns an error if start is not a known id for this graph.
func (g *ClusterTreeGraph) EnsureStartId(start types.Id) error {
	return ensureResourceIdKnownWithClusterEntities(g.TemplateEnts, g.ClusterEnts, g.Refs, start)
}

// ResourceInventoryPresenceStatus maps template vs live id-set membership to four English labels.
func ResourceInventoryPresenceStatus(inTemplate, inCluster bool) string {
	switch {
	case inTemplate && inCluster:
		return "ok"
	case inTemplate && !inCluster:
		return "template only"
	case !inTemplate && inCluster:
		return "cluster only"
	default:
		return "neither"
	}
}

// PickerRowStatus returns ok | template only | cluster only | neither for id picker rows: template
// inventory vs live cluster inventory (same classification as the reference list Status column).
func (g *ClusterTreeGraph) PickerRowStatus(id types.Id) string {
	inT := g.TemplateEnts.IdSet != nil && g.TemplateEnts.IdSet.Has(id)
	inC := g.ClusterEnts.IdSet != nil && g.ClusterEnts.IdSet.Has(id)
	return ResourceInventoryPresenceStatus(inT, inC)
}

// PickerRowStatuses returns a status label for every id in ids (typically CandidateIds()).
func (g *ClusterTreeGraph) PickerRowStatuses(ids []types.Id) map[types.Id]string {
	m := make(map[types.Id]string, len(ids))
	for _, id := range ids {
		m[id] = g.PickerRowStatus(id)
	}
	return m
}

// LocalTreeGraph holds refs and entities from one template-only pipeline run
// (clusterRefsAllFromTemplates). Load once before the TUI loop with LoadLocalTreeGraph
// and reuse CandidateIds, EnsureStartId, and Refs without re-rendering.
type LocalTreeGraph struct {
	Refs     []types.Ref
	Entities entity.Entities
}

// LoadLocalTreeGraph runs the template-only pipeline once and returns a reusable graph.
func LoadLocalTreeGraph(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
) (*LocalTreeGraph, error) {
	refs, ents, err := clusterRefsAllFromTemplates(cluster, networkMode)
	if err != nil {
		return nil, err
	}
	return &LocalTreeGraph{
		Refs:     refs,
		Entities: ents,
	}, nil
}

// CandidateIds returns sorted resource ids from template entities and the ref graph.
func (g *LocalTreeGraph) CandidateIds() []types.Id {
	return sortIdSet(knownResourceIds(g.Entities, g.Refs))
}

// EnsureStartId returns an error if start is not a known id for this graph.
func (g *LocalTreeGraph) EnsureStartId(start types.Id) error {
	return ensureResourceIdKnown(g.Entities, g.Refs, start)
}

// ClusterRefsAllLocalTree returns every reference edge from rendered templates of all
// effectively enabled apps (same extraction pipeline as hydra local refs, without id filtering).
func ClusterRefsAllLocalTree(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
) ([]types.Ref, error) {
	refs, _, err := clusterRefsAllFromTemplates(cluster, networkMode)
	return refs, err
}

// ClusterRefsAllClusterTree returns every reference edge using the same ref-parser and CEL
// setup as hydra gitops review: rendered templates for all enabled apps as sources,
// merged with live cluster entities for ConfigMap parsers, clone materialization, and
// managed-namespace CEL options.
func ClusterRefsAllClusterTree(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	bootstrap types.Bootstrap,
) ([]types.Ref, error) {
	g, err := LoadClusterTreeGraph(cluster, networkMode, bootstrap)
	if err != nil {
		return nil, err
	}
	return g.Refs, nil
}

// ClusterInventoryRefs returns reference edges produced only from live cluster inventory
// (for example PersistentVolumeClaim.spec.volumeName → PersistentVolume), using the same
// ref-parser pipeline as [LoadClusterTreeGraph]. Template entities are only used for
// [hydra.HydraAppRefParsers] and API version normalization.
func ClusterInventoryRefs(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	renderedAllApps entity.Entities,
	clusterEntities entity.Entities,
) ([]types.Ref, error) {
	return ClusterInventoryRefsWithProgress(cluster, networkMode, renderedAllApps, clusterEntities, nil)
}

func ClusterInventoryRefsWithProgress(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	renderedAllApps entity.Entities,
	clusterEntities entity.Entities,
	progress references.RefsProgress,
) ([]types.Ref, error) {
	l := cluster.L()
	appIds, err := cluster.AppIds(networkMode)
	if err != nil {
		return nil, err
	}
	cluster.ResetPreferredVersionsCache()
	scopeFromCluster := func() (types.ScopeInfoMap, error) {
		return ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, types.CrdModeSilent)
	}
	preferredVersions, err := cluster.PreferredVersions(scopeFromCluster)
	if err != nil {
		return nil, err
	}
	renderedEntities, err := NormalizeApiVersions(l, renderedAllApps, types.KeyTemplateEntity, cluster, scopeFromCluster)
	if err != nil {
		return nil, err
	}
	sourceParsers, err := hydra.HydraAppRefParsers(cluster, appIds, networkMode, renderedEntities)
	if err != nil {
		return nil, err
	}
	seenSourceCM := sets.New[types.Id]()
	sourceCM2, err := hydra.RefParsersFromHydraConfigMaps(clusterEntities, types.KeyClusterEntity, seenSourceCM, appIds, appIds)
	if err != nil {
		return nil, err
	}
	sourceParsers = append(sourceParsers, sourceCM2...)

	refsCluster, err := references.RefsWithProgress(l, clusterEntities, types.KeyClusterEntity, nil, entity.Entities{}, entity.Entities{}, preferredVersions, progress, sourceParsers)
	if err != nil {
		return nil, err
	}
	return refsCluster, nil
}

// TreeRefsLocal loads the full template-only ref graph and ensures start is a known id.
func TreeRefsLocal(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	start types.Id,
) ([]types.Ref, error) {
	refs, ents, err := clusterRefsAllFromTemplates(cluster, networkMode)
	if err != nil {
		return nil, err
	}
	if err := ensureResourceIdKnown(ents, refs, start); err != nil {
		return nil, err
	}
	return refs, nil
}

// TreeRefsCluster loads the full ref graph (templates plus live cluster inputs) and ensures
// start is a known id.
func TreeRefsCluster(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	bootstrap types.Bootstrap,
	start types.Id,
) ([]types.Ref, error) {
	g, err := LoadClusterTreeGraph(cluster, networkMode, bootstrap)
	if err != nil {
		return nil, err
	}
	if err := g.EnsureStartId(start); err != nil {
		return nil, err
	}
	return g.Refs, nil
}

func clusterRefsAllClusterTreeWithEntities(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	bootstrap types.Bootstrap,
	cfg *clusterTreeGraphConfig,
) ([]types.Ref, entity.Entities, entity.Entities, error) {
	if cfg == nil {
		cfg = &clusterTreeGraphConfig{}
	}
	// Existing internal callers expect the merged graph that includes both sides plus clone
	// materialization (matches [LoadClusterTreeGraph] semantics). Set defaults if not set.
	if !cfg.includeTemplateRefs && !cfg.includeClusterRefs {
		cfg.includeTemplateRefs = true
		cfg.includeClusterRefs = true
		cfg.includeCloneMaterialization = true
	}
	_, _, refs, templateEnts, clusterEnts, err := buildInspectRefGraph(cluster, networkMode, bootstrap, cfg)
	if err != nil {
		return nil, entity.Entities{}, entity.Entities{}, err
	}
	return refs, templateEnts, clusterEnts, nil
}

// buildInspectRefGraph is the central pipeline that powers [LoadClusterTreeGraph] and
// [LoadInspectRefGraph]. It renders templates (or reuses cfg.renderedTemplateEntities), lists the
// cluster inventory (or reuses cfg.clusterInventory), and builds the requested ref sets:
//   - refsTemplate: built when cfg.includeTemplateRefs is true (annotated [types.RefSourceTemplate])
//   - refsCluster:  built when cfg.includeClusterRefs is true  (annotated [types.RefSourceCluster])
//   - refs:         merged + canonicalized + augmented (template ⊕ cluster). Equals the single
//     side annotated set when only one is requested.
func buildInspectRefGraph(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	bootstrap types.Bootstrap,
	cfg *clusterTreeGraphConfig,
) ([]types.Ref, []types.Ref, []types.Ref, entity.Entities, entity.Entities, error) {
	if cfg == nil {
		cfg = &clusterTreeGraphConfig{}
	}
	l := cluster.L()
	appIds, err := cluster.AppIds(networkMode)
	if err != nil {
		return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
	}
	if len(appIds) == 0 {
		return nil, nil, nil, entity.Entities{}, entity.Entities{}, nil
	}

	cluster.ResetPreferredVersionsCache()
	scopeFromCluster := func() (types.ScopeInfoMap, error) {
		return ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, types.CrdModeSilent)
	}
	preferredVersions, err := cluster.PreferredVersions(scopeFromCluster)
	if err != nil {
		return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
	}

	var renderedEntities entity.Entities
	if cfg.renderedTemplateEntities != nil {
		renderedEntities = *cfg.renderedTemplateEntities
	} else {
		var renderOpts []RenderClusterSelectedAppsOption
		if cfg.skipFoundDefinitionsInfoLog {
			renderOpts = append(renderOpts, WithSkipFoundDefinitionsInfoLog())
		}
		renderedEntities, err = RenderClusterSelectedApps(cluster, networkMode, "", appIds, types.KeyTemplateEntity, renderOpts...)
		if err != nil {
			return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
		}
		// Align template apiVersions with cluster discovery (same as RenderCluster) so ref merge
		// keys match live entity ids — e.g. Strimzi Kafka manifests often use v1beta2 while the
		// apiserver stores v1, which would otherwise duplicate edges as "missing" / "cluster only".
		renderedEntities, err = NormalizeApiVersions(l, renderedEntities, types.KeyTemplateEntity, cluster, scopeFromCluster)
		if err != nil {
			return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
		}
	}

	var clusterEntities entity.Entities
	if cfg.clusterInventory != nil {
		clusterEntities = *cfg.clusterInventory
	} else {
		clusterEntities, err = ListClusterAll(cluster, types.KeyClusterEntity, false, 0)
		if err != nil {
			return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
		}
		model, modelErr := BuildResourceModel(ResourceModelInput{
			Cluster:         cluster,
			ClusterEntities: &clusterEntities,
			NetworkMode:     networkMode,
			Bootstrap:       bootstrap,
		}, false)
		if modelErr != nil {
			return nil, nil, nil, entity.Entities{}, entity.Entities{}, modelErr
		}
		clusterEntities = model.ClusterEntities()
	}

	sourceParsers, err := hydra.HydraAppRefParsers(cluster, appIds, networkMode, renderedEntities)
	if err != nil {
		return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
	}
	seenSourceCM := sets.New[types.Id]()
	sourceCM2, err := hydra.RefParsersFromHydraConfigMaps(clusterEntities, types.KeyClusterEntity, seenSourceCM, appIds, appIds)
	if err != nil {
		return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
	}
	sourceParsers = append(sourceParsers, sourceCM2...)

	var withClones entity.Entities
	if cfg.includeCloneMaterialization {
		extraCloneRules, err := hydra.CloneRulesFromHydraConfigMaps(clusterEntities, types.KeyClusterEntity, nil, appIds, appIds)
		if err != nil {
			return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
		}
		withClones, _, err = MaterializeHydraClonesForApply(
			cluster.L(), cluster, appIds, renderedEntities, types.KeyTemplateEntity, bootstrap, networkMode, extraCloneRules)
		if err != nil {
			return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
		}
	}

	// Live inventory overlay during the template Refs pass feeds CEL clusterEntities() from embedded
	// parsers (e.g. workloadRegardingEvent, StatefulSet PVC picks) so merged inspect/uninstall can
	// link template-only workloads to live cluster objects.
	clusterOverlayForTemplateRefs := entity.Entities{}
	if cfg.includeTemplateRefs && cfg.includeClusterRefs && clusterEntities.Len() > 0 {
		clusterOverlayForTemplateRefs = clusterEntities
	}

	var refsTemplate []types.Ref
	if cfg.includeTemplateRefs {
		refsTemplate, err = references.Refs(l, renderedEntities, types.KeyTemplateEntity, nil, withClones, clusterOverlayForTemplateRefs, preferredVersions, sourceParsers)
		if err != nil {
			return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
		}
		refsTemplate = references.AnnotateRefsWithSource(refsTemplate, types.RefSourceTemplate)
	}

	var refsCluster []types.Ref
	if cfg.includeClusterRefs {
		refsCluster, err = references.Refs(l, clusterEntities, types.KeyClusterEntity, nil, entity.Entities{}, entity.Entities{}, preferredVersions, sourceParsers)
		if err != nil {
			return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
		}
		refsCluster = references.AnnotateRefsWithSource(refsCluster, types.RefSourceCluster)
	}

	var refs []types.Ref
	switch {
	case cfg.includeTemplateRefs && cfg.includeClusterRefs:
		refs = references.MergeRefLists(refsTemplate, refsCluster)
		refs = references.CanonicalizeOwnerRefTargetsToClusterIDs(refs, clusterEntities)
		refs = references.MergeRefLists(refs)
		refs, err = augmentRefsWithExplicitKeyAttributes(renderedEntities, refs)
		if err != nil {
			return nil, nil, nil, entity.Entities{}, entity.Entities{}, err
		}
		refs = references.EnsureRefsHaveOriginSource(refs, types.RefSourceTemplate)
	case cfg.includeTemplateRefs:
		refs = refsTemplate
	case cfg.includeClusterRefs:
		refs = references.CanonicalizeOwnerRefTargetsToClusterIDs(refsCluster, clusterEntities)
		refs = references.MergeRefLists(refs)
	}

	return refsTemplate, refsCluster, refs, renderedEntities, clusterEntities, nil
}
