package cel

import (
	"slices"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ManagedNamespaceNamesFromEntities returns the sorted unique set of Kubernetes namespace names from
// rendered entities: namespaced objects contribute metadata.namespace, and cluster-scoped v1/Namespace
// objects contribute metadata.name. Same derivation as the former HydraManagedNamespaces CEL binding.
func ManagedNamespaceNamesFromEntities(rendered entity.Entities) []string {
	seen := sets.New[string]()
	for _, e := range rendered.Items {
		gvk, err := e.GVKString()
		if err != nil {
			continue
		}
		if gvk == types.KubernetesGvkV1Namespace {
			name, err := e.Name()
			if err != nil {
				continue
			}
			if s := string(name); s != "" {
				seen.Insert(s)
			}
			continue
		}
		if !e.HasNamespace() {
			continue
		}
		ns, err := e.Namespace()
		if err != nil {
			continue
		}
		s := string(ns)
		if s != "" {
			seen.Insert(s)
		}
	}
	list := seen.UnsortedList()
	slices.Sort(list)
	return list
}
