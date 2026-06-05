package entity

import (
	"cmp"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	mergeSlotRightOnly = iota
	mergeSlotLeftOnly
	mergeSlotBoth
)

func truncateMergeFooterDetail(s string) string {
	const max = 96
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

type CompareResult struct {
	All       Entities
	LeftOnly  Entities
	RightOnly Entities
	Both      Entities
}

func newCompareResult(
	all []Entity,
	leftOnly []Entity,
	rightOnly []Entity,
	both []Entity,
) (CompareResult, error) {
	a, err := NewEntities(all)
	if err != nil {
		return CompareResult{}, err
	}
	l, err := NewEntities(leftOnly)
	if err != nil {
		return CompareResult{}, err
	}
	r, err := NewEntities(rightOnly)
	if err != nil {
		return CompareResult{}, err
	}
	b, err := NewEntities(both)
	if err != nil {
		return CompareResult{}, err
	}
	return CompareResult{
		All:       a,
		LeftOnly:  l,
		RightOnly: r,
		Both:      b,
	}, nil
}

func (entities Entities) Compare(
	leftKey types.EntityKeyUnstructured,
	rightKey types.EntityKeyUnstructured,
) (CompareResult, error) {
	_, left, err := entities.SelectByContainsEntityKey(leftKey)
	if err != nil {
		return CompareResult{}, err
	}

	_, right, err := entities.SelectByContainsEntityKey(rightKey)
	if err != nil {
		return CompareResult{}, err
	}

	return merge(left, right, []types.EntityKeyUnstructured{}, nil, 1)
}

type mergeSlotResult struct {
	kind int
	ent  Entity
}

func merge(
	leftEntities Entities,
	rightEntities Entities,
	rightKeys []types.EntityKeyUnstructured,
	progress log.Progress,
	parallel int,
) (CompareResult, error) {
	all := []Entity{}
	leftOnly := []Entity{}
	rightOnly := []Entity{}
	both := []Entity{}

	ids := EntityMapIds(leftEntities.EntityMap, rightEntities.EntityMap).UnsortedList()
	slices.SortFunc(ids, func(a, b types.Id) int {
		return cmp.Compare(string(a), string(b))
	})
	nIds := len(ids)

	par := parallel
	if par == 0 {
		par = runtime.GOMAXPROCS(0)
	}
	if par < 1 {
		par = 1
	}
	if par > 64 {
		par = 64
	}

	processIndex := func(i int) (mergeSlotResult, error) {
		id := ids[i]
		leftEntity, lok := leftEntities.EntityMap[id]
		rightEntity, rok := rightEntities.EntityMap[id]

		if !lok {
			return mergeSlotResult{kind: mergeSlotRightOnly, ent: rightEntity}, nil
		}
		if !rok {
			return mergeSlotResult{kind: mergeSlotLeftOnly, ent: leftEntity}, nil
		}
		if !compareMeta(leftEntity.data, rightEntity.data) {
			return mergeSlotResult{}, log.CreateError(
				errors.ErrMetadataError,
				"mismatched metadata for id {id}:\nleft :{left}\nright:{right}",
				log.String("id", string(id)),
				log.String("left", leftEntity.String()),
				log.String("right", rightEntity.String()))
		}
		builder := leftEntity.ToBuilder().MergeKeysWithoutUnstructured(rightEntity)
		for _, rightKey := range rightKeys {
			if u, ok := rightEntity.Unstructured(rightKey); ok {
				builder = builder.WithUnstructured(rightKey, u)
			}
		}
		e, err := builder.Build()
		if err != nil {
			return mergeSlotResult{}, err
		}
		return mergeSlotResult{kind: mergeSlotBoth, ent: e}, nil
	}

	useParallel := par > 1 && nIds > 0
	if !useParallel {
		var seqTask log.ProgressTask
		if progress != nil {
			seqTask = progress.NewTask("")
		}
		for i := range ids {
			if seqTask != nil {
				seqTask.SetDetail(truncateMergeFooterDetail(string(ids[i])))
			}
			slot, err := processIndex(i)
			if err != nil {
				return CompareResult{}, err
			}
			switch slot.kind {
			case mergeSlotRightOnly:
				all = append(all, slot.ent)
				rightOnly = append(rightOnly, slot.ent)
			case mergeSlotLeftOnly:
				all = append(all, slot.ent)
				leftOnly = append(leftOnly, slot.ent)
			case mergeSlotBoth:
				all = append(all, slot.ent)
				both = append(both, slot.ent)
			}
			if progress != nil {
				progress.Advance(i+1, nIds)
			}
		}
	} else {
		n := nIds
		workerN := par
		if workerN > n {
			workerN = n
		}
		workerTasks := make([]log.ProgressTask, workerN)
		if progress != nil {
			for w := 0; w < workerN; w++ {
				workerTasks[w] = progress.NewTask("")
			}
		}
		outcomes := make([]mergeSlotResult, n)
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
					if i >= n {
						return
					}
					id := ids[i]
					if wt != nil {
						wt.SetDetail(truncateMergeFooterDetail(string(id)))
					}
					slot, perr := processIndex(i)
					if perr != nil {
						stepErrMu.Lock()
						if stepErr == nil {
							stepErr = perr
						}
						stepErrMu.Unlock()
					} else {
						outcomes[i] = slot
					}
					c := completedN.Add(1)
					if progress != nil {
						progress.Advance(int(c), nIds)
					}
				}
			}()
		}
		wg.Wait()
		if stepErr != nil {
			return CompareResult{}, stepErr
		}
		for i := 0; i < n; i++ {
			switch outcomes[i].kind {
			case mergeSlotRightOnly:
				all = append(all, outcomes[i].ent)
				rightOnly = append(rightOnly, outcomes[i].ent)
			case mergeSlotLeftOnly:
				all = append(all, outcomes[i].ent)
				leftOnly = append(leftOnly, outcomes[i].ent)
			case mergeSlotBoth:
				all = append(all, outcomes[i].ent)
				both = append(both, outcomes[i].ent)
			}
		}
	}
	if progress != nil && nIds == 0 {
		progress.Advance(1, 1)
	}

	return newCompareResult(all, leftOnly, rightOnly, both)
}

func compareMeta(left, right entityData) bool {
	for key := range sets.KeySet(left).Union(sets.KeySet(right)) {
		if !key.CanCompare() {
			continue
		}
		l, ok := left[key]
		if !ok {
			continue
		}
		r, ok := right[key]
		if !ok {
			continue
		}
		if l != r {
			return false
		}
	}

	return true
}

func (entities Entities) Merge(
	other Entities,
	otherKeys ...types.EntityKeyUnstructured,
) (Entities, error) {
	return entities.MergeWithProgress(other, nil, 1, otherKeys...)
}

// MergeWithProgress is like [Entities.Merge] but advances the optional footer bar once per merged id.
// When parallel is 0, the worker count is [runtime.GOMAXPROCS](0) (clamped to [1, 64]). When parallel
// is greater than 1 and there is at least one id pair, work runs in concurrent workers.
// When progress is non-nil and the effective worker count is greater than 1, progress uses one
// [log.ProgressTask] line per worker.
func (entities Entities) MergeWithProgress(
	other Entities,
	progress log.Progress,
	parallel int,
	otherKeys ...types.EntityKeyUnstructured,
) (Entities, error) {
	compareResult, err := merge(entities, other, otherKeys, progress, parallel)
	if err != nil {
		return Entities{}, err
	}

	return compareResult.All, nil
}
