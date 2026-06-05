package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"hydra-gitops.org/hydra/hydra-go/core/references"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ResourceModelInput struct {
	Cluster     *hydra.Cluster
	NetworkMode types.HelmNetworkMode
	Bootstrap   types.Bootstrap

	TemplateEntities *entity.Entities
	ClusterEntities  *entity.Entities

	PerAppTemplateEntities map[types.AppId]entity.Entities
	AppIds                 sets.Set[types.AppId]
	PredicateAppIds        sets.Set[types.AppId]

	KubernetesMinor int
	ScopeInfo       types.ScopeInfoMap
	Parallel        int
	WatchCtx        context.Context
}

type ResourceModel struct {
	rows       map[types.Id]ResourceModelRow
	orderedIDs []types.Id

	templateEntities entity.Entities
	clusterEntities  entity.Entities
	clusterTopology  *resourceModelClusterTopology

	assignmentMeta ClusterEntityAssignmentMetadata
	hasAssignment  bool

	inventory *Inventory
}

type resourceModelClusterTopology struct {
	ownerRefChildren ownerRefChildIndex
	rootFirstIndexes []int
}

type ResourceModelRow struct {
	ID          types.Id
	Template    entity.Entity
	HasTemplate bool
	Cluster     entity.Entity
	HasCluster  bool

	AssignedApp    types.AppId
	HasAssignedApp bool
	Reasons        []AssignmentReason
	AmbiguousApps  []types.AppId
	Unassigned     bool
}

type DuplicateTemplateResourceError struct {
	Conflicts map[types.Id][]types.AppId
}

const resourceModelAssignmentProgressEvery = 64

func (e DuplicateTemplateResourceError) Error() string {
	ids := make([]string, 0, len(e.Conflicts))
	for id := range e.Conflicts {
		ids = append(ids, string(id))
	}
	slices.Sort(ids)
	var b strings.Builder
	b.WriteString("template resource rendered by more than one app")
	for _, raw := range ids {
		apps := e.Conflicts[types.Id(raw)]
		appNames := make([]string, 0, len(apps))
		for _, app := range apps {
			appNames = append(appNames, string(app))
		}
		slices.Sort(appNames)
		b.WriteString(fmt.Sprintf("\n  - %s: %s", raw, strings.Join(appNames, ", ")))
	}
	return b.String()
}

type resourceModelProgressTracker struct {
	bar     log.Progress
	total   int
	current int
}

type resourceModelProgressStep int

const (
	resourceModelProgressStepTemplateAssignments resourceModelProgressStep = iota + 1
	resourceModelProgressStepLiveRefs
	resourceModelProgressStepRefOwnershipUninstallPredicateLines
	resourceModelProgressStepCELInventoryEnvironments
	resourceModelProgressStepWorkloadClosureMatchInput
	resourceModelProgressStepCompileRefOwnershipPredicates
	resourceModelProgressStepCompileWeakRefOwnershipPredicates
	resourceModelProgressStepTemplateIDAssignment
	resourceModelProgressStepExpandOwnerReferencesPass1
	resourceModelProgressStepStrongTeardownRefs
	resourceModelProgressStepExpandOwnerReferencesPass2
	resourceModelProgressStepWeakTeardownRefs
	resourceModelProgressStepWeakOwnershipPropagation
	resourceModelProgressStepSoleTemplateNamespaceOwnership
	resourceModelProgressStepOwnerNamespacesNamespaceObjects
	resourceModelProgressStepOwnerNamespacesRemainingWorkloads
	resourceModelProgressStepValidateNamespaceOwnership
	resourceModelProgressStepClusterDefaultsPresets
	resourceModelProgressStepAssignmentReady
)

func resourceModelProgressStepLabel(step resourceModelProgressStep) string {
	switch step {
	case resourceModelProgressStepTemplateAssignments:
		return "resource model · template assignments"
	case resourceModelProgressStepLiveRefs:
		return "resource model · live refs"
	case resourceModelProgressStepRefOwnershipUninstallPredicateLines:
		return "resource model · ref ownership uninstall predicate lines"
	case resourceModelProgressStepCELInventoryEnvironments:
		return "resource model · CEL inventory environments"
	case resourceModelProgressStepWorkloadClosureMatchInput:
		return "resource model · workload closure match input"
	case resourceModelProgressStepCompileRefOwnershipPredicates:
		return "resource model · compile ref ownership predicates"
	case resourceModelProgressStepCompileWeakRefOwnershipPredicates:
		return "resource model · compile negative-priority ref ownership predicates"
	case resourceModelProgressStepTemplateIDAssignment:
		return "resource model · template id assignment"
	case resourceModelProgressStepExpandOwnerReferencesPass1:
		return "resource model · expand owner references (pass 1)"
	case resourceModelProgressStepStrongTeardownRefs:
		return "resource model · priority >= 0 teardown refs"
	case resourceModelProgressStepExpandOwnerReferencesPass2:
		return "resource model · expand owner references (pass 2)"
	case resourceModelProgressStepWeakTeardownRefs:
		return "resource model · priority < 0 teardown refs"
	case resourceModelProgressStepWeakOwnershipPropagation:
		return "resource model · negative-priority ownership propagation"
	case resourceModelProgressStepSoleTemplateNamespaceOwnership:
		return "resource model · sole-template namespace ownership"
	case resourceModelProgressStepOwnerNamespacesNamespaceObjects:
		return "resource model · ownerNamespaces Namespace objects"
	case resourceModelProgressStepOwnerNamespacesRemainingWorkloads:
		return "resource model · ownerNamespaces remaining workloads"
	case resourceModelProgressStepValidateNamespaceOwnership:
		return "resource model · validate namespace ownership"
	case resourceModelProgressStepClusterDefaultsPresets:
		return "resource model · cluster-defaults presets"
	case resourceModelProgressStepAssignmentReady:
		return "resource model · assignment ready"
	default:
		return "resource model"
	}
}

func newResourceModelProgressTracker(l log.Logger, enabled bool, total int) (*resourceModelProgressTracker, error) {
	tracker := &resourceModelProgressTracker{total: total}
	if !enabled || total <= 0 {
		return tracker, nil
	}
	bar, err := l.NewProgress("resource model", total)
	if err != nil {
		return nil, err
	}
	if bar != nil {
		tracker.bar = bar
	}
	return tracker, nil
}

func (p *resourceModelProgressTracker) Advance(detail string) {
	_ = detail
	if p == nil || p.total <= 0 {
		return
	}
	p.current++
	if p.bar != nil {
		p.bar.Advance(p.current, p.total)
	}
}

func (p *resourceModelProgressTracker) AdvanceStep(step resourceModelProgressStep, detail string) {
	_ = step
	p.Advance(detail)
}

func (p *resourceModelProgressTracker) SetDetail(detail string) {
	_ = detail
}

func (p *resourceModelProgressTracker) Close() {
	if p == nil {
		return
	}
	if p.bar != nil {
		_ = p.bar.Close()
	}
}

func newResourceModelSubProgress(l log.Logger, enabled bool, label string, total int) (log.Progress, log.ProgressTask, func(), error) {
	if !enabled || total <= 0 {
		return nil, nil, func() {}, nil
	}
	bar, err := l.NewProgress(label, total)
	if err != nil {
		return nil, nil, nil, err
	}
	if bar == nil {
		return nil, nil, func() {}, nil
	}
	task := bar.NewTask("")
	closeProgress := func() {
		if task != nil {
			_ = task.Close()
		}
		_ = bar.Close()
	}
	return bar, task, closeProgress, nil
}

func newResourceModelRefsProgress(l log.Logger, enabled bool, label string) (references.RefsProgress, func(), error) {
	bar, task, closeProgress, err := newResourceModelSubProgress(l, enabled, label, 1)
	if err != nil {
		return nil, nil, err
	}
	if bar == nil {
		return nil, closeProgress, nil
	}
	progress := func(done int, total int, detail string) {
		if total <= 0 {
			total = 1
		}
		if task != nil {
			task.SetDetail(k8s.TruncateFooterDetail(detail))
		}
		bar.Advance(done, total)
	}
	return progress, closeProgress, nil
}

func resourceModelTopBarLastStep(in ResourceModelInput, templateAssignmentTotal int, splitAssignment bool) resourceModelProgressStep {
	if templateAssignmentTotal > 0 {
		if in.ClusterEntities == nil || in.Cluster == nil {
			return resourceModelProgressStepTemplateAssignments
		}
	}
	if in.ClusterEntities != nil && in.Cluster != nil {
		if splitAssignment {
			return resourceModelProgressStepAssignmentReady
		} else {
			return resourceModelProgressStepLiveRefs
		}
	}
	return 0
}

func BuildResourceModel(in ResourceModelInput, showProgress bool) (*ResourceModel, error) {
	if in.TemplateEntities == nil && in.ClusterEntities == nil {
		return nil, fmt.Errorf("resource model requires template entities, cluster entities, or both")
	}
	l := log.Default()
	if in.Cluster != nil {
		l = in.Cluster.L()
	}
	templates := emptyEntities()
	if in.TemplateEntities != nil {
		templates = *in.TemplateEntities
	}
	clusterEntities := emptyEntities()
	if in.ClusterEntities != nil {
		clusterEntities = *in.ClusterEntities
	}
	perApp := clonePerAppEntities(in.PerAppTemplateEntities)
	if in.TemplateEntities != nil && len(perApp) == 0 {
		var err error
		perApp, err = PartitionTemplateEntitiesByPrimaryApp(templates)
		if err != nil {
			return nil, err
		}
	}
	if err := validateUniqueTemplateOwnership(perApp); err != nil {
		return nil, err
	}
	appIds := cloneAppIDSet(in.AppIds)
	if appIds == nil {
		appIds = sets.New[types.AppId]()
		for app := range perApp {
			appIds.Insert(app)
		}
	}
	k8sMinor := in.KubernetesMinor
	if k8sMinor <= 0 && in.Cluster != nil {
		if sm, err := KubernetesServerMinorVersion(in.Cluster); err == nil && sm > 0 {
			k8sMinor = sm
		}
	}
	if k8sMinor <= 0 {
		k8sMinor = 99
	}
	if err := validateUniqueTemplateOwnership(perApp); err != nil {
		return nil, err
	}
	templateAssignmentTotal := 0
	for _, ents := range perApp {
		templateAssignmentTotal += len(ents.Items)
	}
	splitAssignment := showProgress
	progress, err := newResourceModelProgressTracker(l, showProgress, int(resourceModelTopBarLastStep(in, templateAssignmentTotal, splitAssignment)))
	if err != nil {
		return nil, err
	}
	progressClosed := false
	defer func() {
		if !progressClosed {
			progress.Close()
		}
	}()
	inv, err := newInventoryFromEntities(
		in.Cluster,
		in.NetworkMode,
		in.Bootstrap,
		templates,
		clusterEntities,
		withInventoryScopeInfo(in.ScopeInfo),
		withInventoryKubernetesMinor(k8sMinor),
		withInventoryParallel(in.Parallel),
		withInventoryWatch(in.WatchCtx),
		withInventoryShowProgress(showProgress),
	)
	if err != nil {
		return nil, err
	}
	progress.SetDetail("build unified inventory")
	rows := rowsFromInventory(inv)
	model := &ResourceModel{
		rows:             rows,
		orderedIDs:       sortedResourceRowIDs(rows),
		templateEntities: templates,
		clusterEntities:  clusterEntities,
		inventory:        inv,
	}
	templateAssignmentBar, templateAssignmentTask, closeTemplateAssignment, err := newResourceModelSubProgress(
		l,
		showProgress,
		resourceModelProgressStepLabel(resourceModelProgressStepTemplateAssignments),
		templateAssignmentTotal,
	)
	if err != nil {
		return nil, err
	}
	defer closeTemplateAssignment()
	if err := model.applyTemplateAssignments(perApp, templateAssignmentBar, templateAssignmentTask); err != nil {
		return nil, err
	}
	progress.AdvanceStep(resourceModelProgressStepTemplateAssignments, resourceModelProgressStepLabel(resourceModelProgressStepTemplateAssignments))
	if in.ClusterEntities != nil {
		if in.Cluster == nil {
			progress.Close()
			progressClosed = true
			l.Info(logIdCommands, "resource model ready: {rows} rows", log.Int("rows", len(model.rows)))
			return model, nil
		}
		allRendered := templates
		refsProgress, closeRefsProgress, err := newResourceModelRefsProgress(
			l,
			showProgress,
			resourceModelProgressStepLabel(resourceModelProgressStepLiveRefs),
		)
		if err != nil {
			return nil, err
		}
		defer closeRefsProgress()
		var refsLastDetail string
		mirroredRefsProgress := func(done int, total int, detail string) {
			if refsProgress != nil {
				refsProgress(done, total, detail)
			}
			refsLastDetail = detail
		}
		refs, err := ClusterInventoryRefsWithProgress(in.Cluster, in.NetworkMode, allRendered, clusterEntities, mirroredRefsProgress)
		if err != nil {
			return nil, err
		}
		detail := "derive live inventory refs"
		if refsLastDetail != "" {
			detail = refsLastDetail
		}
		progress.AdvanceStep(resourceModelProgressStepLiveRefs, detail)
		assignmentInput := AssignClusterEntitiesToAtMostOneAppByRefsInput{
			Cluster:             in.Cluster,
			ResourceModel:       model,
			ClusterEntities:     clusterEntities,
			AllAppIDs:           appIds,
			WeakAppIDs:          defaultPredicateAppIDs(in.PredicateAppIds, appIds),
			PerAppRendered:      perApp,
			RenderedAllApps:     allRendered,
			MergedInspectRefs:   refs,
			InitialAssignment:   nil,
			Progress:            nil,
			ProgressPrefixSteps: 0,
			ProgressGrandTotal:  0,
			Parallel:            in.Parallel,
			NetworkMode:         in.NetworkMode,
			aggregateProgress:   progress,
			splitProgress:       splitAssignment,
		}
		assignment, meta, _, err := assignClusterEntitiesToAtMostOneAppByRefsImpl(assignmentInput)
		if err != nil {
			return nil, err
		}
		if !assignmentInput.splitProgress {
			progress.AdvanceStep(resourceModelProgressStepAssignmentReady, "resource model · live assignment")
		}
		assignment, meta, err = resolveResourceModelAssignmentConflictDetails(assignmentInput, assignment, meta)
		if err != nil {
			return nil, err
		}
		model.hasAssignment = true
		model.assignmentMeta = cloneClusterEntityAssignmentMetadata(meta)
		model.applyAssignments(assignment, meta)
	}
	progress.Close()
	progressClosed = true
	l.Info(logIdCommands, "resource model ready: {rows} rows", log.Int("rows", len(model.rows)))
	return model, nil
}

func resolveResourceModelAssignmentConflictDetails(
	in AssignClusterEntitiesToAtMostOneAppByRefsInput,
	assignment map[types.Id]types.AppId,
	meta ClusterEntityAssignmentMetadata,
) (map[types.Id]types.AppId, ClusterEntityAssignmentMetadata, error) {
	ambiguousIDs := meta.AmbiguousEntityIDs()
	if len(ambiguousIDs) == 0 || clusterEntityAssignmentHasDetailedReasonsForAll(meta, ambiguousIDs) {
		return assignment, meta, nil
	}
	baseAssignment := cloneAssignmentMap(assignment)
	for _, id := range ambiguousIDs {
		delete(baseAssignment, id)
	}
	detailInput := in
	detailInput.ConflictDetailIDs = ambiguousIDs
	detailInput.FocusEntityIDs = ambiguousIDs
	detailInput.InitialAssignment = baseAssignment
	detailInput.Progress = nil
	detailInput.ProgressPrefixSteps = 0
	detailInput.ProgressGrandTotal = 0
	_, detailedMeta, _, err := assignClusterEntitiesToAtMostOneAppByRefsImpl(detailInput)
	if err != nil {
		return nil, ClusterEntityAssignmentMetadata{}, err
	}
	mergedMeta := cloneClusterEntityAssignmentMetadata(meta)
	for _, id := range ambiguousIDs {
		if apps, ok := detailedMeta.AmbiguousAppIDsByClusterEntity[id]; ok {
			mergedMeta.AmbiguousAppIDsByClusterEntity[id] = append([]types.AppId{}, apps...)
		}
		if byApp, ok := detailedMeta.AmbiguousAppReasonsByClusterEntity[id]; ok {
			copiedByApp := make(map[types.AppId][]AssignmentReason, len(byApp))
			for app, reasons := range byApp {
				copiedByApp[app] = append([]AssignmentReason{}, reasons...)
			}
			mergedMeta.AmbiguousAppReasonsByClusterEntity[id] = copiedByApp
		}
	}
	return assignment, mergedMeta, nil
}

func (m *ResourceModel) InventoryForGraph() *Inventory {
	if m == nil {
		return nil
	}
	return m.inventory
}

func (m *ResourceModel) Close() {
	if m == nil || m.inventory == nil {
		return
	}
	m.inventory.Close()
}

func (m *ResourceModel) applyTemplateAssignments(
	perApp map[types.AppId]entity.Entities,
	progress log.Progress,
	task log.ProgressTask,
) error {
	if m == nil {
		return nil
	}
	total := 0
	for _, ents := range perApp {
		total += len(ents.Items)
	}
	done := 0
	for app, ents := range perApp {
		for _, ent := range ents.Items {
			id, err := ent.Id()
			if err != nil {
				return err
			}
			row := m.rows[id]
			row.ID = id
			row.AssignedApp = app
			row.HasAssignedApp = true
			row.Unassigned = false
			row.Reasons = append(row.Reasons, AssignmentReason{Kind: AssignmentReasonKindAssignedViaTemplateID})
			m.rows[id] = row
			done++
			if progress != nil && total > 0 {
				progress.Advance(done, total)
			}
			if task != nil && total > 0 &&
				(done%resourceModelAssignmentProgressEvery == 0 || done == total) {
				task.SetDetail(k8s.TruncateFooterDetail(fmt.Sprintf(
					"assign template ownership - %d / %d", done, total)))
			}
		}
	}
	m.orderedIDs = sortedResourceRowIDs(m.rows)
	return nil
}

func emptyEntities() entity.Entities {
	l := log.Default()
	ents, err := entity.NewEntities(nil)
	if err != nil {
		l.Warn(logIdCommands, "failed to create empty entity list: {err}", log.Err(err))
	}
	return ents
}

func clonePerAppEntities(in map[types.AppId]entity.Entities) map[types.AppId]entity.Entities {
	if in == nil {
		return nil
	}
	out := make(map[types.AppId]entity.Entities, len(in))
	for app, ents := range in {
		out[app] = ents
	}
	return out
}

func cloneAppIDSet(in sets.Set[types.AppId]) sets.Set[types.AppId] {
	if in == nil {
		return nil
	}
	out := sets.New[types.AppId]()
	for id := range in {
		out.Insert(id)
	}
	return out
}

func defaultPredicateAppIDs(predicateAppIDs, appIDs sets.Set[types.AppId]) sets.Set[types.AppId] {
	if predicateAppIDs != nil && predicateAppIDs.Len() > 0 {
		return predicateAppIDs
	}
	return appIDs
}

func validateUniqueTemplateOwnership(perApp map[types.AppId]entity.Entities) error {
	owners := map[types.Id]sets.Set[types.AppId]{}
	for app, ents := range perApp {
		for _, ent := range ents.Items {
			id, err := ent.Id()
			if err != nil {
				return err
			}
			if owners[id] == nil {
				owners[id] = sets.New[types.AppId]()
			}
			owners[id].Insert(app)
		}
	}
	conflicts := map[types.Id][]types.AppId{}
	for id, apps := range owners {
		if apps.Len() <= 1 {
			continue
		}
		list := apps.UnsortedList()
		slices.SortFunc(list, func(a, b types.AppId) int {
			return strings.Compare(string(a), string(b))
		})
		conflicts[id] = list
	}
	if len(conflicts) > 0 {
		return DuplicateTemplateResourceError{Conflicts: conflicts}
	}
	return nil
}

func rowsFromInventory(inv *Inventory) map[types.Id]ResourceModelRow {
	rows := map[types.Id]ResourceModelRow{}
	for _, item := range inv.UnifiedEntities() {
		rows[item.ID] = ResourceModelRow{
			ID:          item.ID,
			Template:    item.Template,
			HasTemplate: item.HasTemplate,
			Cluster:     item.Live,
			HasCluster:  item.HasLive,
		}
	}
	return rows
}

func sortedResourceRowIDs(rows map[types.Id]ResourceModelRow) []types.Id {
	ids := make([]types.Id, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b types.Id) int {
		return strings.Compare(string(a), string(b))
	})
	return ids
}

func (m *ResourceModel) applyAssignments(assignment map[types.Id]types.AppId, meta ClusterEntityAssignmentMetadata) {
	for id, app := range assignment {
		row := m.rows[id]
		row.ID = id
		row.AssignedApp = app
		row.HasAssignedApp = true
		row.Unassigned = false
		m.rows[id] = row
	}
	for id, apps := range meta.AmbiguousAppIDsByClusterEntity {
		row := m.rows[id]
		row.ID = id
		row.AmbiguousApps = append([]types.AppId{}, apps...)
		row.Unassigned = false
		if reasonsByApp := meta.AmbiguousAppReasonsByClusterEntity[id]; len(reasonsByApp) > 0 {
			for _, reasons := range reasonsByApp {
				row.Reasons = append(row.Reasons, reasons...)
			}
		}
		m.rows[id] = row
	}
	for id := range meta.UnassignedIDs {
		row := m.rows[id]
		row.ID = id
		if !row.HasAssignedApp && len(row.AmbiguousApps) == 0 {
			row.Unassigned = true
		}
		m.rows[id] = row
	}
	m.orderedIDs = sortedResourceRowIDs(m.rows)
}

func (m *ResourceModel) Rows() []ResourceModelRow {
	if m == nil {
		return nil
	}
	out := make([]ResourceModelRow, 0, len(m.orderedIDs))
	for _, id := range m.orderedIDs {
		out = append(out, m.rows[id])
	}
	return out
}

func (m *ResourceModel) Row(id types.Id) (ResourceModelRow, bool) {
	if m == nil {
		return ResourceModelRow{}, false
	}
	row, ok := m.rows[id]
	return row, ok
}

func (m *ResourceModel) TemplateEntities() entity.Entities {
	if m == nil {
		return emptyEntities()
	}
	return m.templateEntities
}

func (m *ResourceModel) ClusterEntities() entity.Entities {
	if m == nil {
		return emptyEntities()
	}
	return m.clusterEntities
}

func (m *ResourceModel) SetClusterEntities(entities entity.Entities) {
	if m == nil {
		return
	}
	m.clusterEntities = entities
	m.InvalidateClusterTopologyCache()
}

func (m *ResourceModel) InvalidateClusterTopologyCache() {
	if m == nil {
		return
	}
	m.clusterTopology = nil
}

func (m *ResourceModel) clusterOwnerRefChildIndex() (ownerRefChildIndex, error) {
	if err := m.ensureClusterTopology(); err != nil {
		return ownerRefChildIndex{}, err
	}
	return m.clusterTopology.ownerRefChildren, nil
}

func (m *ResourceModel) clusterRefOwnershipEntityIndexesRootFirst(
	assignment map[types.Id]types.AppId,
	ambiguousIDs sets.Set[types.Id],
	focusIDs sets.Set[types.Id],
	includeAssigned bool,
) ([]int, error) {
	if err := m.ensureClusterTopology(); err != nil {
		return nil, err
	}
	indexes := make([]int, 0, len(m.clusterTopology.rootFirstIndexes))
	for _, i := range m.clusterTopology.rootFirstIndexes {
		eid, err := m.clusterEntities.Items[i].Id()
		if err != nil {
			return nil, err
		}
		if len(focusIDs) > 0 && !focusIDs.Has(eid) {
			continue
		}
		if ambiguousIDs.Has(eid) {
			continue
		}
		if !includeAssigned {
			if _, has := assignment[eid]; has {
				continue
			}
		}
		indexes = append(indexes, i)
	}
	return indexes, nil
}

func (m *ResourceModel) ensureClusterTopology() error {
	if m == nil {
		return nil
	}
	if m.clusterTopology != nil {
		return nil
	}
	childIndex, err := buildOwnerRefChildIndex(m.clusterEntities, types.KeyClusterEntity)
	if err != nil {
		return err
	}
	indexes := make([]int, m.clusterEntities.Len())
	for i := range m.clusterEntities.Items {
		indexes[i] = i
	}
	rootFirstIndexes, err := sortRefOwnershipEntityIndexesRootFirst(indexes, m.clusterEntities, nil, nil, childIndex)
	if err != nil {
		return err
	}
	m.clusterTopology = &resourceModelClusterTopology{
		ownerRefChildren: childIndex,
		rootFirstIndexes: rootFirstIndexes,
	}
	return nil
}

func (m *ResourceModel) Assignment() map[types.Id]types.AppId {
	if m == nil || !m.hasAssignment {
		return nil
	}
	out := map[types.Id]types.AppId{}
	for id, row := range m.rows {
		if row.HasAssignedApp {
			out[id] = row.AssignedApp
		}
	}
	return out
}

func (m *ResourceModel) AssignmentMetadata() ClusterEntityAssignmentMetadata {
	if m == nil || !m.hasAssignment {
		return ClusterEntityAssignmentMetadata{}
	}
	return cloneClusterEntityAssignmentMetadata(m.assignmentMeta)
}

func (m *ResourceModel) HasAppAssignment() bool {
	return m != nil && m.hasAssignment
}

func (m *ResourceModel) AssignedApp(id types.Id) (types.AppId, bool) {
	if m == nil || !m.hasAssignment {
		return "", false
	}
	row, ok := m.rows[id]
	if !ok || !row.HasAssignedApp {
		return "", false
	}
	return row.AssignedApp, true
}

func (m *ResourceModel) IdsForApp(app types.AppId) sets.Set[types.Id] {
	out := sets.New[types.Id]()
	if m == nil {
		return out
	}
	for id, row := range m.rows {
		if row.HasAssignedApp && row.AssignedApp == app {
			out.Insert(id)
		}
	}
	return out
}

func (m *ResourceModel) RowsForIDs(ids sets.Set[types.Id]) []ResourceModelRow {
	if m == nil || ids == nil {
		return nil
	}
	out := make([]ResourceModelRow, 0, ids.Len())
	for id := range ids {
		if row, ok := m.rows[id]; ok {
			out = append(out, row)
		}
	}
	slices.SortFunc(out, func(a, b ResourceModelRow) int {
		return strings.Compare(string(a.ID), string(b.ID))
	})
	return out
}

func (m *ResourceModel) UntrackedRootClusterEntities() (entity.Entities, error) {
	if m == nil {
		return entity.NewEntities(nil)
	}
	rootEntities, err := m.clusterEntities.ClusterInventoryRootEntities(types.KeyClusterEntity)
	if err != nil {
		return entity.Entities{}, err
	}
	var out []entity.Entity
	for _, ent := range rootEntities.Items {
		id, err := ent.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		row, ok := m.rows[id]
		if ok && (row.HasAssignedApp || len(row.AmbiguousApps) > 0) {
			continue
		}
		out = append(out, ent)
	}
	return entity.NewEntities(out)
}
