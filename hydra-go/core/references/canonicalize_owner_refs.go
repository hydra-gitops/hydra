package references

import (
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// gknnKey identifies an API object independent of API version (same as Kubernetes object identity).
type gknnKey struct {
	group     string
	kind      string
	namespace string
	name      string
}

// CanonicalizeOwnerRefTargetsToClusterIDs rewrites Ref.To on edges derived from
// metadata.ownerReferences when the target id uses a different API version than
// the live cluster object (same group, kind, namespace, name). Kubernetes keeps
// the controller-revision apiVersion in ownerReferences while the apiserver may
// persist another version; Hydra entity ids include the version, so edges would
// otherwise point at non-existent ids and UIs that require both endpoints to
// exist drop the edge.
func CanonicalizeOwnerRefTargetsToClusterIDs(refs []types.Ref, clusterEnts entity.Entities) []types.Ref {
	clusterIds := sets.New[types.Id]()
	gknnToID := make(map[gknnKey]types.Id)
	for _, e := range clusterEnts.Items {
		id, err := e.Id()
		if err != nil {
			continue
		}
		clusterIds.Insert(id)
		g, errG := e.Group()
		k, errK := e.Kind()
		ns, errN := e.Namespace()
		n, errName := e.Name()
		if errG != nil || errK != nil || errN != nil || errName != nil || n == "" {
			continue
		}
		key := gknnKey{group: string(g), kind: string(k), namespace: string(ns), name: string(n)}
		if _, ok := gknnToID[key]; !ok {
			gknnToID[key] = id
		}
	}

	out := make([]types.Ref, len(refs))
	copy(out, refs)
	for i := range out {
		if !refHasOriginOwner(out[i]) {
			continue
		}
		if clusterIds.Has(out[i].To) {
			continue
		}
		group, _, kind, ns, name, err := out[i].To.Components()
		if err != nil {
			continue
		}
		key := gknnKey{group: string(group), kind: string(kind), namespace: string(ns), name: string(name)}
		if canonical, ok := gknnToID[key]; ok && canonical != out[i].To {
			out[i].To = canonical
		}
	}
	return out
}

func refHasOriginOwner(r types.Ref) bool {
	for _, a := range r.Attributes {
		if a.Type == types.RefAttributeOriginOwner {
			return true
		}
	}
	return false
}
