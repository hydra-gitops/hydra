package hydra

import (
	"fmt"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ClusterDefaultResourceSpec identifies a bootstrap object for synthetic template merge and audit
// (same fields as legacy kubernetesCoreBuiltin in commands).
type ClusterDefaultResourceSpec struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

// KubernetesClusterDefaultExpectedIDSet returns bootstrap resource ids from the builtin **kubernetes**
// preset ids (bootstrap-audit* groups only, after minor gating). Prefer ClusterDefaultsPresetAuditExpectedIDs
// over all merged presets for full blanket audit coverage.
func KubernetesClusterDefaultExpectedIDSet(k8sMinor int) sets.Set[types.Id] {
	m, err := loadBuiltinClusterDefaultsPresets()
	if err != nil {
		return sets.New[types.Id]()
	}
	kube, ok := m[ClusterDefaultsPresetIDKubernetes]
	if !ok {
		return sets.New[types.Id]()
	}
	eff := effectiveClusterDefaultsPresetFromBuiltin(kube, nil)
	return ClusterDefaultsPresetAuditExpectedIDs(k8sMinor, []ClusterDefaultsPresetEffective{eff})
}

// KubernetesClusterDefaultBootstrapSpecs returns ordered bootstrap specs for synthetic template merge,
// derived from bootstrap-audit* ids in the builtin kubernetes preset.
func KubernetesClusterDefaultBootstrapSpecs(k8sMinor int) []ClusterDefaultResourceSpec {
	m, err := loadBuiltinClusterDefaultsPresets()
	if err != nil {
		return nil
	}
	kube, ok := m[ClusterDefaultsPresetIDKubernetes]
	if !ok {
		return nil
	}
	pnames := make([]string, 0, len(kube.Predicates))
	for n := range kube.Predicates {
		pnames = append(pnames, n)
	}
	slices.Sort(pnames)
	var out []ClusterDefaultResourceSpec
	for _, pname := range pnames {
		if !strings.HasPrefix(pname, "bootstrap-audit") {
			continue
		}
		be := kube.Predicates[pname]
		pe := ClusterDefaultsPredicateEffective{
			Enabled:            true,
			Ids:                append([]ClusterDefaultsIdLine(nil), be.Ids...),
			KubernetesMinorMin: be.KubernetesMinorMin,
			KubernetesMinorMax: be.KubernetesMinorMax,
		}
		if !ClusterDefaultsPredicateMinorApplies(k8sMinor, pe) {
			continue
		}
		ids := make([]string, 0, len(pe.Ids))
		for _, idl := range pe.Ids {
			ids = append(ids, idl.Id)
		}
		slices.Sort(ids)
		for _, idStr := range ids {
			spec, ok := clusterDefaultResourceSpecFromAuditID(types.Id(idStr))
			if ok {
				out = append(out, spec)
			}
		}
	}
	return out
}

func clusterDefaultResourceSpecFromAuditID(id types.Id) (ClusterDefaultResourceSpec, bool) {
	g, ver, kind, ns, name, err := id.Components()
	if err != nil {
		return ClusterDefaultResourceSpec{}, false
	}
	k := string(kind)
	switch k {
	case "ClusterRole", "ClusterRoleBinding":
		return ClusterDefaultResourceSpec{
			APIVersion: apiVersionFromGroupVersion(g, ver),
			Kind:       k,
			Namespace:  "",
			Name:       string(name),
		}, true
	case "Role", "ConfigMap", "Service":
		return ClusterDefaultResourceSpec{
			APIVersion: apiVersionFromGroupVersion(g, ver),
			Kind:       k,
			Namespace:  string(ns),
			Name:       string(name),
		}, true
	default:
		return ClusterDefaultResourceSpec{}, false
	}
}

func apiVersionFromGroupVersion(g types.Group, v types.Version) string {
	if g == "" {
		return string(v)
	}
	return fmt.Sprintf("%s/%s", g, v)
}
