package action

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ApplyOperations holds classified template vs cluster entities for hydra gitops apply.
// New/Update/Replace/Unchanged cover resources present in rendered templates; Delete lists
// cluster orphans to remove in the final phase.
type ApplyOperations struct {
	New       entity.Entities
	Update    entity.Entities
	Replace   entity.Entities
	Unchanged entity.Entities
	Delete    entity.Entities

	// PatchFailures lists recoverable SSA dry-run failures that led to Replace entries.
	PatchFailures []commands.DryRunPatchFailure
}

// ClassifyApplyOperations builds ApplyOperations from rendered templates, live inventory, and orphans.
// It applies the same ArgoCD AppProject syncWindows adjustment as the workload apply path (see
// commands.ApplyClusterApplySyncWindowToEntities) before the server-side apply dry-run so the plan
// matches what will actually be applied for --sync.
// It runs a single server-side apply dry-run pass over all existing (template∩cluster) entities.
func ClassifyApplyOperations(
	cluster *hydra.Cluster,
	rendered entity.Entities,
	clusterEntities entity.Entities,
	orphans entity.Entities,
	diffIgnore *commands.DiffIgnorePipeline,
	syncWindow types.ClusterApplySyncWindow,
	parallel int,
	useFooter bool,
) (*ApplyOperations, error) {
	newEnt, existingEnt, err := splitNewAndExisting(rendered, clusterEntities)
	if err != nil {
		return nil, err
	}

	l := cluster.L()
	newEnt, _, err = commands.ApplyClusterApplySyncWindowToEntities(
		l, newEnt, clusterEntities, types.KeyTemplateEntity, types.KeyClusterEntity, syncWindow, true)
	if err != nil {
		return nil, err
	}
	existingEnt, _, err = commands.ApplyClusterApplySyncWindowToEntities(
		l, existingEnt, clusterEntities, types.KeyTemplateEntity, types.KeyClusterEntity, syncWindow, false)
	if err != nil {
		return nil, err
	}

	ops := &ApplyOperations{
		New:    newEnt,
		Delete: orphans,
	}

	if existingEnt.Len() == 0 {
		empty, err := entity.NewEntities(nil)
		if err != nil {
			return nil, err
		}
		ops.Update = empty
		ops.Replace = empty
		ops.Unchanged = empty
		return ops, nil
	}

	dryRunOpts := commands.ServerSideDryRunApplyOptions{
		ReplaceNonImmutableDryRunFailures: true,
		Parallel:                          parallel,
	}
	dryRunEntities, patchFailures, dErr := commands.ServerSideDryRunApplyEntities(
		cluster, existingEnt, types.KeyTemplateEntity, types.KeyDryRunEntity, dryRunOpts, useFooter)
	if dErr != nil {
		return nil, dErr
	}
	ops.PatchFailures = patchFailures

	changedEnts, err := findChangedEntities(cluster.L(), dryRunEntities, diffIgnore)
	if err != nil {
		return nil, err
	}

	changedIds := sets.New[types.Id]()
	for _, e := range changedEnts.Items {
		id, err := e.Id()
		if err != nil {
			return nil, err
		}
		changedIds.Insert(id)
	}

	patchById := make(map[types.Id]commands.DryRunPatchFailure, len(patchFailures))
	for _, pf := range patchFailures {
		patchById[pf.Id] = pf
	}

	var updateItems, replaceItems, unchangedItems []entity.Entity
	for _, e := range dryRunEntities.Items {
		id, err := e.Id()
		if err != nil {
			return nil, err
		}
		if !changedIds.Has(id) {
			unchangedItems = append(unchangedItems, e)
			continue
		}
		if _, ok := patchById[id]; ok {
			replaceItems = append(replaceItems, e)
			continue
		}
		updateItems = append(updateItems, e)
	}

	ops.Update, err = entity.NewEntities(updateItems)
	if err != nil {
		return nil, err
	}
	ops.Replace, err = entity.NewEntities(replaceItems)
	if err != nil {
		return nil, err
	}
	ops.Unchanged, err = entity.NewEntities(unchangedItems)
	if err != nil {
		return nil, err
	}
	return ops, nil
}

// FilterApplyOperationsByResourcePredicate keeps only template-side operations (new/update/replace/unchanged)
// whose entities match pred, clears orphan deletes, and trims PatchFailures to the filtered Replace set.
// Orphan deletes must be omitted whenever predicates narrow the apply set: the cluster may still hold
// valid objects that were simply filtered out of this run.
func FilterApplyOperationsByResourcePredicate(ops *ApplyOperations, pred cel.Predicate) (*ApplyOperations, error) {
	if ops == nil || pred == nil {
		return ops, nil
	}
	newItems, err := filterDiffEntitiesByPredicate(ops.New.Items, pred)
	if err != nil {
		return nil, err
	}
	updateItems, err := filterDiffEntitiesByPredicate(ops.Update.Items, pred)
	if err != nil {
		return nil, err
	}
	replaceItems, err := filterDiffEntitiesByPredicate(ops.Replace.Items, pred)
	if err != nil {
		return nil, err
	}
	unchangedItems, err := filterDiffEntitiesByPredicate(ops.Unchanged.Items, pred)
	if err != nil {
		return nil, err
	}
	newEnt, err := entity.NewEntities(newItems)
	if err != nil {
		return nil, err
	}
	updateEnt, err := entity.NewEntities(updateItems)
	if err != nil {
		return nil, err
	}
	replaceEnt, err := entity.NewEntities(replaceItems)
	if err != nil {
		return nil, err
	}
	unchangedEnt, err := entity.NewEntities(unchangedItems)
	if err != nil {
		return nil, err
	}
	emptyDel, err := entity.NewEntities(nil)
	if err != nil {
		return nil, err
	}
	return &ApplyOperations{
		New:           newEnt,
		Update:        updateEnt,
		Replace:       replaceEnt,
		Unchanged:     unchangedEnt,
		Delete:        emptyDel,
		PatchFailures: filterPatchFailures(ops.PatchFailures, replaceEnt),
	}, nil
}

// NonImmutableReplaceCount returns how many replace operations require --replace (non-immutable SSA failures).
func (o *ApplyOperations) NonImmutableReplaceCount() int {
	if o == nil {
		return 0
	}
	n := 0
	for _, pf := range o.PatchFailures {
		if !pf.Immutable {
			n++
		}
	}
	return n
}

// ReplaceCount is the total number of delete-before-apply operations (immutable + non-immutable).
func (o *ApplyOperations) ReplaceCount() int {
	if o == nil {
		return 0
	}
	return o.Replace.Len()
}

// TotalRendered returns the number of rendered resources (excluding orphans).
func (o *ApplyOperations) TotalRendered() int {
	if o == nil {
		return 0
	}
	return o.New.Len() + o.Update.Len() + o.Replace.Len() + o.Unchanged.Len()
}

// ValidateReplaceFlag returns an error when non-immutable replace operations require --replace.
func ValidateReplaceFlag(replaceFlag bool, ops *ApplyOperations) error {
	if ops == nil {
		return nil
	}
	if replaceFlag {
		return nil
	}
	if ops.NonImmutableReplaceCount() == 0 {
		return nil
	}
	return log.CreateError(errors.ErrAborted,
		"apply aborted: {count} resource(s) require delete-before-apply for reasons other than immutable fields; re-run with --replace or fix manifests",
		log.Int("count", ops.NonImmutableReplaceCount()))
}

// logApplyPlanSummary logs aggregate counts for the classified apply plan.
func logApplyPlanSummary(l log.Logger, ops *ApplyOperations) {
	if ops == nil {
		return
	}
	l.Info(logIdAction, "apply plan: new={new} update={update} replace={replace} replace_non_immutable={replaceNI} unchanged={unchanged} delete={delete} rendered_total={rendered}",
		log.Int("new", ops.New.Len()),
		log.Int("update", ops.Update.Len()),
		log.Int("replace", ops.Replace.Len()),
		log.Int("replaceNI", ops.NonImmutableReplaceCount()),
		log.Int("unchanged", ops.Unchanged.Len()),
		log.Int("delete", ops.Delete.Len()),
		log.Int("rendered", ops.TotalRendered()))
}

func filterEntitiesByGVK(ents entity.Entities, match func(types.GVKString) bool) (entity.Entities, error) {
	if ents.Len() == 0 {
		return entity.NewEntities(nil)
	}
	var out []entity.Entity
	for _, e := range ents.Items {
		gvk, err := e.GVKString()
		if err != nil {
			return entity.Entities{}, err
		}
		if match(gvk) {
			out = append(out, e)
		}
	}
	return entity.NewEntities(out)
}

// FilterCRDs returns only CustomResourceDefinition entities from each bucket.
func (o ApplyOperations) FilterCRDs() (ApplyOperations, error) {
	return o.filterByGVK(func(gvk types.GVKString) bool {
		return gvk == types.KubernetesGvkApiextensionsK8sIoV1CustomResourceDefinition
	})
}

// FilterNamespaces returns only Namespace entities from each bucket.
func (o ApplyOperations) FilterNamespaces() (ApplyOperations, error) {
	return o.filterByGVK(func(gvk types.GVKString) bool {
		return gvk == types.KubernetesGvkV1Namespace
	})
}

// FilterWebhooks returns admission webhook configuration entities from each bucket.
func (o ApplyOperations) FilterWebhooks() (ApplyOperations, error) {
	return o.filterByGVK(func(gvk types.GVKString) bool {
		return gvk == types.KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration ||
			gvk == types.KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration
	})
}

// FilterNonWebhooks excludes webhook configurations from each bucket (CRDs and namespaces remain).
func (o ApplyOperations) FilterNonWebhooks() (ApplyOperations, error) {
	return o.filterByGVK(func(gvk types.GVKString) bool {
		return gvk != types.KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration &&
			gvk != types.KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration
	})
}

// FilterMainWorkload returns entities that are not CRDs, namespaces, or webhook configurations.
func (o ApplyOperations) FilterMainWorkload() (ApplyOperations, error) {
	return o.filterByGVK(func(gvk types.GVKString) bool {
		if gvk == types.KubernetesGvkApiextensionsK8sIoV1CustomResourceDefinition {
			return false
		}
		if gvk == types.KubernetesGvkV1Namespace {
			return false
		}
		if gvk == types.KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration ||
			gvk == types.KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration {
			return false
		}
		return true
	})
}

// FilterPostNamespaceApply returns entities that should participate in the main
// apply phase after CRDs and namespaces: workloads, jobs, services, and
// optionally webhook configurations when bootstrap-style webhook downscaling is
// enabled.
func (o ApplyOperations) FilterPostNamespaceApply() (ApplyOperations, error) {
	return o.filterByGVK(func(gvk types.GVKString) bool {
		return gvk != types.KubernetesGvkApiextensionsK8sIoV1CustomResourceDefinition &&
			gvk != types.KubernetesGvkV1Namespace
	})
}

func (o ApplyOperations) filterByGVK(match func(types.GVKString) bool) (ApplyOperations, error) {
	newE, err := filterEntitiesByGVK(o.New, match)
	if err != nil {
		return ApplyOperations{}, err
	}
	upd, err := filterEntitiesByGVK(o.Update, match)
	if err != nil {
		return ApplyOperations{}, err
	}
	rep, err := filterEntitiesByGVK(o.Replace, match)
	if err != nil {
		return ApplyOperations{}, err
	}
	unch, err := filterEntitiesByGVK(o.Unchanged, match)
	if err != nil {
		return ApplyOperations{}, err
	}
	del, err := filterEntitiesByGVK(o.Delete, match)
	if err != nil {
		return ApplyOperations{}, err
	}
	// PatchFailures: keep only those whose id appears in filtered replace or update paths still needing delete — actually replace ids only
	pf := filterPatchFailures(o.PatchFailures, rep)
	return ApplyOperations{
		New:           newE,
		Update:        upd,
		Replace:       rep,
		Unchanged:     unch,
		Delete:        del,
		PatchFailures: pf,
	}, nil
}

func filterPatchFailures(pfs []commands.DryRunPatchFailure, replace entity.Entities) []commands.DryRunPatchFailure {
	if len(pfs) == 0 || replace.Len() == 0 {
		return nil
	}
	allowed := sets.New[types.Id]()
	for _, e := range replace.Items {
		id, err := e.Id()
		if err != nil {
			continue
		}
		allowed.Insert(id)
	}
	var out []commands.DryRunPatchFailure
	for _, pf := range pfs {
		if allowed.Has(pf.Id) {
			out = append(out, pf)
		}
	}
	return out
}

// MergeMutating concatenates New, Update, and Replace for apply paths that need a single entity list.
func (o ApplyOperations) MergeMutating() (entity.Entities, error) {
	n := o.New.Len() + o.Update.Len() + o.Replace.Len()
	items := make([]entity.Entity, 0, n)
	items = append(items, o.New.Items...)
	items = append(items, o.Update.Items...)
	items = append(items, o.Replace.Items...)
	return entity.NewEntities(items)
}

// MergeUpdateReplace concatenates Update and Replace for existing-object apply paths.
func (o ApplyOperations) MergeUpdateReplace() (entity.Entities, error) {
	n := o.Update.Len() + o.Replace.Len()
	items := make([]entity.Entity, 0, n)
	items = append(items, o.Update.Items...)
	items = append(items, o.Replace.Items...)
	return entity.NewEntities(items)
}

// HasMutatingWork is true when any create or patch apply work remains.
func (o ApplyOperations) HasMutatingWork() bool {
	return o.New.Len()+o.Update.Len()+o.Replace.Len() > 0
}

// DeleteReplaceIds returns ids that require delete-before-apply for this operations slice.
func (o ApplyOperations) DeleteReplaceIds() sets.Set[types.Id] {
	if o.Replace.Len() == 0 {
		return nil
	}
	ids := make([]types.Id, 0, o.Replace.Len())
	for _, e := range o.Replace.Items {
		id, err := e.Id()
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return sets.New[types.Id](ids...)
}
