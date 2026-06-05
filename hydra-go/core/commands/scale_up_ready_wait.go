package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// gvrForTransitiveReady returns the entity GVR, or if the REST resource name is not stored
// on the entity (common for some live/template merges), derives the plural resource from
// GVK using meta.UnsafeGuessKindToResource.
func gvrForTransitiveReady(e entity.Entity) (types.GVR, error) {
	gvr, gvrErr := e.GVR()
	if gvrErr == nil {
		return gvr, nil
	}
	gvk, gvkErr := e.GVK()
	if gvkErr != nil {
		return types.GVR{}, gvkErr
	}
	plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{
		Group:   string(gvk.Group),
		Version: string(gvk.Version),
		Kind:    string(gvk.Kind),
	})
	if plural.Resource == "" {
		return types.GVR{}, gvrErr
	}
	return types.NewGVR(types.Group(plural.Group), types.Version(plural.Version), types.Resource(plural.Resource)), nil
}

func transitiveReadyGatedIDs(
	entities entity.Entities,
	fullRefs []types.Ref,
	eval *ReadyEvaluator,
	anchorID types.Id,
	liveKey types.EntityKeyUnstructured,
) []types.Id {
	if eval == nil || len(fullRefs) == 0 {
		return nil
	}
	seen := map[types.Id]bool{}
	var out []types.Id
	add := func(id types.Id) {
		if id == "" || seen[id] {
			return
		}
		e, ok := entities.EntityMap[id]
		if !ok {
			return
		}
		if id != anchorID {
			depGVK, err := e.GVKString()
			if err == nil && scaleDependencyRefRole(anchorID, id, fullRefs, depGVK) == ScaleDependencyRefRoleDownstream {
				return
			}
		}
		if eval.RuleMatched(e, liveKey) {
			seen[id] = true
			out = append(out, id)
		}
	}
	add(anchorID)
	for _, id := range TransitiveOutgoingRefReach(fullRefs, anchorID) {
		add(id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func transitiveReadyDisplayName(e entity.Entity, fallback types.Id) string {
	name, err := e.Name()
	if err != nil || name == "" {
		return string(fallback)
	}
	gvk, err := e.GVK()
	if err != nil {
		return string(name)
	}
	ns, _ := e.Namespace()
	if ns == "" {
		return fmt.Sprintf("%s %s", gvk.Kind, name)
	}
	return fmt.Sprintf("%s %s/%s", gvk.Kind, ns, name)
}

func transitiveReadyPendingNames(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	entities entity.Entities,
	fullRefs []types.Ref,
	eval *ReadyEvaluator,
	liveKey types.EntityKeyUnstructured,
	anchorID types.Id,
) ([]string, error) {
	if eval == nil {
		return nil, nil
	}
	gatedIDs := transitiveReadyGatedIDs(entities, fullRefs, eval, anchorID, liveKey)
	pending := make([]string, 0, len(gatedIDs))
	for _, id := range gatedIDs {
		e, ok := entities.EntityMap[id]
		if !ok {
			pending = append(pending, string(id))
			continue
		}
		gvr, err := gvrForTransitiveReady(e)
		if err != nil {
			return nil, err
		}
		name, err := e.Name()
		if err != nil {
			return nil, err
		}
		ns, _ := e.Namespace()
		rc := resourceClient(dynamicClient, ns, gvr)
		obj, err := rc.Get(ctx, string(name), metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				pending = append(pending, transitiveReadyDisplayName(e, id))
				continue
			}
			return nil, err
		}
		matched, ready, _, err := eval.ReadyFromLiveObject(e, obj.Object, liveKey)
		if err != nil || !matched || !ready {
			if err != nil {
				return nil, err
			}
			pending = append(pending, transitiveReadyDisplayName(e, id))
		}
	}
	sort.Strings(pending)
	return pending, nil
}

func statusPathLooksReady(v any) bool {
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case string:
		return x != ""
	case int64:
		return x > 0
	case int:
		return x > 0
	case int32:
		return x > 0
	case float64:
		return x > 0
	case bool:
		return x
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

func customWorkloadLiveReadyAtPath(obj *unstructured.Unstructured, statusPath string) bool {
	if obj == nil || strings.TrimSpace(statusPath) == "" {
		return false
	}
	parts := strings.Split(statusPath, ".")
	v := values.Lookup(obj.Object, parts...)
	return statusPathLooksReady(v)
}

func scaleUpCustomWorkloadNeedsWait(target ScaleTarget, e entity.Entity, eval *ReadyEvaluator, liveKey types.EntityKeyUnstructured) bool {
	if target.StatusReadyPath != "" {
		return true
	}
	if eval != nil && eval.RuleMatched(e, liveKey) {
		return true
	}
	return false
}
