package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"hydra-gitops.org/hydra/hydra-go/base/cache"
	herrors "hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	htypes "hydra-gitops.org/hydra/hydra-go/core/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

const fieldManagerHydra = "hydra"

// msgNonImmutableSSADryRunFailure is the ErrorLog template when SSA dry-run fails for a reason other
// than an immutable-field conflict while ReplaceNonImmutableDryRunFailures allows planning delete-before-apply.
const msgNonImmutableSSADryRunFailure = "server-side apply dry-run failed for {entity} (dry-run rejected patch; not an immutable-field conflict); delete-before-apply is planned for this resource; re-run with --replace or fix the manifest"

// DryRunPatchFailure records a server-side apply dry-run patch failure that is
// recoverable via delete-before-apply.
type DryRunPatchFailure struct {
	Id        htypes.Id
	Immutable bool
}

// ServerSideDryRunApplyOptions configures ServerSideDryRunApplyEntities.
type ServerSideDryRunApplyOptions struct {
	// ReplaceNonImmutableDryRunFailures (CLI --replace) when true: any patch dry-run
	// failure is treated as recoverable via delete-before-apply.
	ReplaceNonImmutableDryRunFailures bool
	// FailOnAnyPatchError when true: any patch dry-run failure aborts with an error
	// (e.g. cluster diff — no automatic recovery).
	FailOnAnyPatchError bool
	// APIWarningSource labels Kubernetes API Warning headers in logs (RFC 7234).
	// If empty, [k8s.KubernetesAPIWarningSourceClusterApplyPlan] is used.
	APIWarningSource string
	// Parallel is the number of concurrent SSA dry-run patch calls (0 = GOMAXPROCS, capped at 64).
	// Values below 2 keep
	// sequential behavior (footer advances before each patch). When 2 or more, workers
	// run in parallel; the footer shows one status line per worker via [log.ProgressTask].
	Parallel int
}

// ssaDryRunStepOutcome is the result of processing one entity index in [ServerSideDryRunApplyEntities].
type ssaDryRunStepOutcome struct {
	Out            entity.Entity
	PatchFailure   *DryRunPatchFailure
	HardSSAFailure *htypes.Id
	Err            error
}

// ServerSideDryRunApplyEntities applies each entity via server-side apply with
// dry-run enabled. The API server fills in defaults (clusterIP, sessionAffinity,
// etc.) and returns the expected object state. The returned entities carry the
// enriched unstructured data under targetKey, so a subsequent comparison
// against the live cluster state produces clean diffs without false positives
// from server-side defaults. When sourceKey == targetKey, the original data is
// replaced in-place.
//
// Patch dry-run failures are recoverable when the Kubernetes API reports an
// immutable-field conflict (delete+recreate is valid) or when
// opts.ReplaceNonImmutableDryRunFailures is set (--replace). Otherwise the run
// fails after processing all entities. Recoverable failures are returned in the
// second result for delete-before-apply. Entities that cannot resolve GVR still
// fall back to raw comparison with a warning only (does not trigger patch failure).
func ServerSideDryRunApplyEntities(
	cluster *hydra.Cluster,
	entities entity.Entities,
	sourceKey htypes.EntityKeyUnstructured,
	targetKey htypes.EntityKeyUnstructured,
	opts ServerSideDryRunApplyOptions,
	useFooter bool,
) (out entity.Entities, patchFailures []DryRunPatchFailure, err error) {
	restConfig, err := RestConfigForHydra(cluster)
	if err != nil {
		return entity.Entities{}, nil, err
	}
	// Replace client-go's default (klog "Warning: …" without code) so API warnings
	// include the HTTP warning code and call-site label.
	src := opts.APIWarningSource
	if src == "" {
		src = k8s.KubernetesAPIWarningSourceClusterApplyPlan
	}
	restConfig.WarningHandlerWithContext = k8s.KubernetesAPICtxWarningHandler{
		Logger: cluster.L(),
		LogID:  logIdCommands,
		Source: src,
		Debug:  false,
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return entity.Entities{}, nil, err
	}

	l := cluster.L()
	total := entities.Len()
	l.Info(logIdCommands, "running server-side apply dry-run for {count} entities",
		log.Int("count", total))

	var bar log.Progress
	var detailTask log.ProgressTask
	if useFooter && total > 0 {
		k8s.FlushProgressLogBeforeFooter()
		bar, err = l.NewProgress("ssa dry-run (planning)", total)
		if err != nil {
			return entity.Entities{}, nil, err
		}
		defer func() {
			if bar == nil {
				return
			}
			_ = bar.Close()
			summary := fmt.Sprintf("ssa dry-run (planning): completed %d resource(s)", total)
			if err != nil {
				summary = fmt.Sprintf("ssa dry-run (planning): failed: %v", err)
			}
			l.Info(logIdCommands, summary)
		}()
	}

	clientCache := cache.NewCache[htypes.GVRNString, dynamic.ResourceInterface](
		l, "ssa-client-cache", true, nil)

	resourceClient := func(ns htypes.Namespace, gvr htypes.GVR) (dynamic.ResourceInterface, error) {
		return clientCache.GetOrLoad(htypes.NewGVRNString(gvr, ns), func() (dynamic.ResourceInterface, error) {
			rc := dynamicClient.Resource(gvr.K8s())
			if ns == "" {
				return rc, nil
			}
			return rc.Namespace(string(ns)), nil
		})
	}

	parallel := EffectiveClusterWorkerParallelism(opts.Parallel)
	if bar != nil && total > 0 && parallel <= 1 {
		detailTask = bar.NewTask("")
	}

	force := true
	step := func(i int, e entity.Entity) ssaDryRunStepOutcome {
		u, ok := e.Unstructured(sourceKey)
		if !ok {
			return ssaDryRunStepOutcome{Out: e}
		}

		gvr, gvrErr := e.GVR()
		if gvrErr != nil {
			id, _ := e.Id()
			l.Warn(logIdCommands, "cannot resolve GVR for {entity}, using raw comparison",
				log.String("entity", string(id)), log.Err(gvrErr))
			return ssaDryRunStepOutcome{Out: e}
		}

		ns, _ := e.Namespace()
		rc, rcErr := resourceClient(ns, gvr)
		if rcErr != nil {
			return ssaDryRunStepOutcome{Err: rcErr}
		}

		data, mErr := json.Marshal(u.Object)
		if mErr != nil {
			id, _ := e.Id()
			return ssaDryRunStepOutcome{Err: log.CreateError(herrors.ErrServerSideApplyFailed,
				"failed to marshal {entity}", log.String("entity", string(id)), log.Err(mErr))}
		}

		name, nErr := e.Name()
		if nErr != nil {
			return ssaDryRunStepOutcome{Err: nErr}
		}

		entityID, idErr := e.Id()
		if idErr != nil {
			return ssaDryRunStepOutcome{Err: idErr}
		}
		patchCtx := k8s.ContextWithResourceID(context.Background(), string(entityID))

		applied, pErr := rc.Patch(
			patchCtx,
			string(name),
			types.ApplyPatchType,
			data,
			metav1.PatchOptions{
				DryRun:       []string{metav1.DryRunAll},
				FieldManager: fieldManagerHydra,
				Force:        &force,
			},
		)
		if pErr != nil {
			id, idErr2 := e.Id()
			if idErr2 != nil {
				return ssaDryRunStepOutcome{Err: idErr2}
			}

			immutable := IsImmutableFieldKubernetesError(pErr)
			var recoverable bool
			switch {
			case opts.FailOnAnyPatchError:
				recoverable = false
			case immutable:
				recoverable = true
			default:
				recoverable = opts.ReplaceNonImmutableDryRunFailures
			}

			if recoverable {
				pf := DryRunPatchFailure{Id: id, Immutable: immutable}
				if immutable {
					l.Warn(logIdCommands, "server-side apply dry-run failed for {entity} (API reports immutable field; delete-before-apply is scheduled)",
						log.String("entity", string(id)), log.Err(pErr))
				} else {
					l.Error(logIdCommands, msgNonImmutableSSADryRunFailure,
						log.String("entity", string(id)), log.Err(pErr))
				}
				return ssaDryRunStepOutcome{Out: e, PatchFailure: &pf}
			}

			hardID := id
			l.Warn(logIdCommands, "server-side apply dry-run failed for {entity}",
				log.String("entity", string(id)), log.Err(pErr))
			return ssaDryRunStepOutcome{Out: e, HardSSAFailure: &hardID}
		}

		modified, modErr := e.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
			return b.WithUnstructured(targetKey, *applied)
		})
		if modErr != nil {
			return ssaDryRunStepOutcome{Err: modErr}
		}
		return ssaDryRunStepOutcome{Out: modified}
	}

	var (
		result                 = make([]entity.Entity, total)
		ssaDryRunPatchFailures int
		ssaDryRunFailureIds    []htypes.Id
	)

	if parallel <= 1 {
		for i, e := range entities.Items {
			if bar != nil && detailTask != nil && total > 0 {
				detail := ""
				if id, idErr := e.Id(); idErr == nil {
					detail = k8s.TruncateFooterDetail(string(id))
				}
				detailTask.SetDetail(detail)
				bar.Advance(i+1, total)
			}
			o := step(i, e)
			if o.Err != nil {
				return entity.Entities{}, nil, o.Err
			}
			result[i] = o.Out
			if o.PatchFailure != nil {
				patchFailures = append(patchFailures, *o.PatchFailure)
			}
			if o.HardSSAFailure != nil {
				ssaDryRunPatchFailures++
				ssaDryRunFailureIds = append(ssaDryRunFailureIds, *o.HardSSAFailure)
			}
		}
	} else {
		workerN := parallel
		if workerN > total {
			workerN = total
		}
		outcomes := make([]ssaDryRunStepOutcome, total)
		workerTasks := make([]log.ProgressTask, workerN)
		if bar != nil {
			for w := 0; w < workerN; w++ {
				workerTasks[w] = bar.NewTask("")
			}
		}
		var nextIdx atomic.Uint64
		var completedN atomic.Uint64
		var stepErrMu sync.Mutex
		var stepErr error
		var wg sync.WaitGroup
		for w := 0; w < workerN; w++ {
			wg.Add(1)
			wid := w
			go func() {
				defer wg.Done()
				var wt log.ProgressTask
				if wid < len(workerTasks) {
					wt = workerTasks[wid]
				}
				for {
					i := int(nextIdx.Add(1)) - 1
					if i >= total {
						return
					}
					e := entities.Items[i]
					if wt != nil {
						detail := ""
						if id, idErr := e.Id(); idErr == nil {
							detail = k8s.TruncateFooterDetail(string(id))
						}
						wt.SetDetail(detail)
					}
					o := step(i, e)
					outcomes[i] = o
					if o.Err != nil {
						stepErrMu.Lock()
						if stepErr == nil {
							stepErr = o.Err
						}
						stepErrMu.Unlock()
					}
					c := completedN.Add(1)
					if bar != nil {
						bar.Advance(int(c), total)
					}
				}
			}()
		}
		wg.Wait()
		if stepErr != nil {
			return entity.Entities{}, nil, stepErr
		}
		for i := 0; i < total; i++ {
			o := outcomes[i]
			result[i] = o.Out
			if o.PatchFailure != nil {
				patchFailures = append(patchFailures, *o.PatchFailure)
			}
			if o.HardSSAFailure != nil {
				ssaDryRunPatchFailures++
				ssaDryRunFailureIds = append(ssaDryRunFailureIds, *o.HardSSAFailure)
			}
		}
	}

	if ssaDryRunPatchFailures > 0 {
		err = log.CreateError(herrors.ErrServerSideApplyFailed,
			"server-side apply dry-run failed for {count} resource(s); fix manifests or re-run with --replace to delete+recreate when the API rejects the patch for reasons other than immutable fields",
			log.Int("count", ssaDryRunPatchFailures))
		for _, id := range ssaDryRunFailureIds {
			l.Info(logIdCommands, "* {id}", log.String("id", string(id)))
		}
		return entity.Entities{}, nil, err
	}

	if bar == nil {
		l.Info(logIdCommands, "server-side apply dry-run completed")
	}
	out, err = entity.NewEntities(result)
	if err != nil {
		return entity.Entities{}, nil, err
	}
	return out, patchFailures, nil
}
