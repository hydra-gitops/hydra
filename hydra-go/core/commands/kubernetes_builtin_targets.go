package commands

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

const missingClusterDefaultResourceFinding = "missing cluster default resource"

func kubernetesBuiltinCoreID(c kubernetesCoreBuiltin) types.Id {
	return types.Id(fmt.Sprintf("%s/%s/%s/%s", c.Version, c.Kind, c.Namespace, c.Name))
}

func kubernetesBuiltinClusterRoleID(name string) types.Id {
	return types.Id("rbac.authorization.k8s.io/v1/ClusterRole//" + name)
}

// IsKubernetesStandardRefOwnershipExempt returns true for objects that Kubernetes creates and
// maintains outside Hydra templates (minimal namespaces/default SAs in kubernetes_builtin_catalog.go,
// builtin cluster-defaults preset ids (kubernetes / coredns / flannel / canal / kubermatic / syseleven / metakube / syseleven-node-problem-detector / quobyte / cloudinit / cinder / talos), plus per-namespace ServiceAccount/default
// and ConfigMap/kube-root-ca.crt from createNamespaceEntities / BuildSyntheticNamespaceDefaultTargets).
// Cluster review must not report
// "ref ownership: cluster-only resource has no Hydra app assignment" for these ids.
//
// kubernetesMinor should be the live apiserver minor when known; pass 99 when the minor cannot be
// determined so version-gated RBAC bootstrap names are all treated as standard.
func IsKubernetesStandardRefOwnershipExempt(id types.Id, kubernetesMinor int) bool {
	minor := kubernetesMinor
	if minor <= 0 {
		minor = 99
	}
	if KubernetesBuiltinExpectedIDSet(minor, nil).Has(id) {
		return true
	}
	return isKubernetesInjectedNamespaceDefaultID(id)
}

// isKubernetesInjectedNamespaceDefaultID matches the root CA bundle and default service account the
// apiserver injects into every namespace (same pairing as qualifiesForSyntheticNamespaceDefaultRef).
func isKubernetesInjectedNamespaceDefaultID(id types.Id) bool {
	_, ver, kind, ns, name, err := id.Components()
	if err != nil {
		return false
	}
	if ver != types.KubernetesVersionV1 {
		return false
	}
	if ns == "" {
		return false
	}
	switch kind {
	case types.KubernetesKindConfigMap:
		return name == types.Name("kube-root-ca.crt")
	case types.KubernetesKindServiceAccount:
		return name == types.Name("default")
	default:
		return false
	}
}

// KubernetesBuiltinExpectedIDSet returns minimal cluster builtins plus the union of preset ids
// (ClusterDefaultsPresetAuditExpectedIDs). Pass nil effective to load builtins-only merged presets;
// pass non-nil from HydraMergedClusterDefaultsPresetsSection + EffectiveClusterDefaultsPresets for merged Helm/ConfigMap.
func KubernetesBuiltinExpectedIDSet(k8sMinor int, effective []hydra.ClusterDefaultsPresetEffective) sets.Set[types.Id] {
	eff := effective
	if len(eff) == 0 {
		var err error
		eff, err = hydra.EffectiveClusterDefaultsPresetsForKubernetesMinor(nil, k8sMinor)
		if err != nil {
			out := sets.New[types.Id]()
			for _, c := range kubernetesClusterMinimalBuiltins {
				out.Insert(kubernetesBuiltinCoreID(c))
			}
			return out.Union(hydra.KubernetesClusterDefaultExpectedIDSet(k8sMinor))
		}
	}
	out := sets.New[types.Id]()
	for _, c := range kubernetesClusterMinimalBuiltins {
		out.Insert(kubernetesBuiltinCoreID(c))
	}
	return out.Union(hydra.ClusterDefaultsPresetAuditExpectedIDs(k8sMinor, eff))
}

func kubernetesBuiltinSyntheticEntities(
	key types.EntityKeyUnstructured,
	k8sMinor int,
) (entity.Entities, error) {
	bootstrap := hydra.KubernetesClusterDefaultBootstrapSpecs(k8sMinor)
	capacity := len(kubernetesClusterMinimalBuiltins) + len(bootstrap)
	built := make([]entity.Entity, 0, capacity)
	for _, c := range kubernetesClusterMinimalBuiltins {
		e, err := syntheticKubernetesBuiltinEntity(key, c.Version, c.Kind, c.Namespace, c.Name)
		if err != nil {
			return entity.Entities{}, err
		}
		built = append(built, e)
	}
	for _, s := range bootstrap {
		e, err := syntheticKubernetesBuiltinEntity(key, s.APIVersion, s.Kind, s.Namespace, s.Name)
		if err != nil {
			return entity.Entities{}, err
		}
		built = append(built, e)
	}
	return entity.NewEntities(built)
}

func syntheticKubernetesBuiltinEntity(
	key types.EntityKeyUnstructured,
	apiVersion, kind, namespace, name string,
) (entity.Entity, error) {
	u := unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetName(name)
	if namespace != "" {
		u.SetNamespace(namespace)
	}
	gvk := types.NewGVKFromK8s(u.GroupVersionKind())
	b := entity.NewEntityBuilder().
		WithGVK(gvk).
		WithName(types.Name(name)).
		WithBuiltIn().
		WithTemplatePath("hydra-kubernetes-builtin.yaml").
		WithTemplateIndex(1)
	if namespace != "" {
		b = b.WithNamespace(types.Namespace(namespace)).WithNamespaced(types.NamespacedNo)
	}
	return b.WithUnstructured(key, u).Build()
}

func mergeKubernetesBuiltinTemplateTargets(
	targets entity.Entities,
	key types.EntityKeyUnstructured,
	k8sMinor int,
) (entity.Entities, error) {
	builtins, err := kubernetesBuiltinSyntheticEntities(key, k8sMinor)
	if err != nil {
		return entity.Entities{}, err
	}
	var extra []entity.Entity
	for _, e := range builtins.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if targets.IdSet.Has(id) {
			continue
		}
		extra = append(extra, e)
	}
	if len(extra) == 0 {
		return targets, nil
	}
	return targets.Append(entity.Entities{Items: extra})
}
