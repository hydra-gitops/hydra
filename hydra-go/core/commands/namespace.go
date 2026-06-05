package commands

import (
	"maps"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

var systemNamespaces = sets.New(
	types.Namespace("kube-system"),
	types.Namespace("kube-public"),
	types.Namespace("kube-node-lease"),
)

// isKubernetesSystemNamespace reports namespaces reserved for or managed by the cluster control plane.
// They must not be treated as exclusively owned by a Hydra app for namespace→app resolution (e.g. clone targets).
func isKubernetesSystemNamespace(ns types.Namespace) bool {
	if systemNamespaces.Has(ns) {
		return true
	}
	return strings.HasPrefix(string(ns), "kube-")
}

func WithoutSystemNamespaces(namespaces sets.Set[types.Namespace]) sets.Set[types.Namespace] {
	out := sets.New[types.Namespace]()
	for ns := range namespaces {
		if !isKubernetesSystemNamespace(ns) {
			out.Insert(ns)
		}
	}
	return out
}

func ExclusiveNamespaces(l log.Logger, entities entity.Entities, selectedAppIds sets.Set[types.AppId]) (sets.Set[types.Namespace], error) {
	namespaces := sets.New[types.Namespace]()
	for ns, nsAppIds := range GroupNamespacesByApp(entities) {
		if isKubernetesSystemNamespace(ns) {
			continue
		}
		appIds := nsAppIds.UnsortedList()
		slices.Sort(appIds)
		for _, id := range appIds {
			l.DebugLog(logIdCommands, "Namespace '{namespace}' is used by app '{appId}'",
				log.String("namespace", string(ns)),
				log.String("appId", string(id)))
		}
		if selectedAppIds.IsSuperset(nsAppIds) {
			l.DebugLog(logIdCommands, "{namespace} is exclusive used by selected app ids",
				log.String("namespace", string(ns)))
			namespaces.Insert(ns)
		}
	}

	return namespaces, nil
}

// UninstallLeftoverNamespaces returns namespaces used when scanning for uninstall leftovers
// (handleLeftovers, handleForceLeftovers) and for uninstall-safe CEL predicates (ns in namespaces).
// It is the union of ExclusiveNamespaces (namespaces used only by the selected apps) and every
// namespace that appears in the selected apps' rendered templates (see GroupNamespacesByApp).
//
// Shared team namespaces (multiple Hydra apps deploy there) are not exclusive, so ExclusiveNamespaces
// alone omits them — but operator- or runtime-created objects (for example PVCs) for an app still live
// in those namespaces. Unioning template namespaces ensures uninstall-force and leftover passes see them.
func UninstallLeftoverNamespaces(
	exclusive sets.Set[types.Namespace],
	renderedSelectedApps entity.Entities,
) sets.Set[types.Namespace] {
	fromTemplates := sets.New[types.Namespace]()
	for ns := range GroupNamespacesByApp(renderedSelectedApps) {
		fromTemplates.Insert(ns)
	}
	return exclusive.Union(fromTemplates)
}

func GroupNamespacesByApp(entities entity.Entities) map[types.Namespace]sets.Set[types.AppId] {
	result := map[types.Namespace]sets.Set[types.AppId]{}

	for _, e := range entities.Items {
		ns, err := e.Namespace()
		if err != nil || ns == "" {
			gvk, _ := e.GVKString()
			if gvk != types.KubernetesGvkV1Namespace {
				continue
			}
			name, err := e.Name()
			if err != nil {
				continue
			}
			ns = types.Namespace(name)
		}

		appIds, err := e.AppIds()
		if err != nil {
			continue
		}

		for _, appId := range appIds {
			appIds, ok := result[ns]
			if !ok {
				appIds = sets.New[types.AppId]()
			}
			appIds.Insert(appId)
			result[ns] = appIds
		}
	}

	return result
}

// InferredOwnerNamespacesForApp returns namespaces where exactly one app deploys resources
// (per GroupNamespacesByApp) and that app is appId. Kubernetes system namespaces are excluded.
// The result is sorted and suitable for merging into displayed or effective owner namespace lists.
func InferredOwnerNamespacesForApp(appId types.AppId, entities entity.Entities) []string {
	byNs := GroupNamespacesByApp(entities)
	var out []string
	for ns, apps := range byNs {
		if isKubernetesSystemNamespace(ns) {
			continue
		}
		if apps.Len() != 1 {
			continue
		}
		sole := apps.UnsortedList()[0]
		if sole != appId {
			continue
		}
		out = append(out, string(ns))
	}
	slices.Sort(out)
	return out
}

// MergeInferredOwnerNamespacesIntoHydraMap returns a copy of merged with ownerNamespaces set to the
// sorted union of the existing ownerNamespaces slice and InferredOwnerNamespacesForApp.
func MergeInferredOwnerNamespacesIntoHydraMap(merged types.ValuesMap, appId types.AppId, entities entity.Entities) types.ValuesMap {
	inferred := InferredOwnerNamespacesForApp(appId, entities)
	if len(inferred) == 0 {
		return merged
	}
	existing := stringSliceFromValuesMap(merged, "ownerNamespaces")
	combined := unionSortedUniqueStrings(existing, inferred)
	out := maps.Clone(merged)
	out["ownerNamespaces"] = combined
	return out
}

func stringSliceFromValuesMap(m types.ValuesMap, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return append([]string(nil), s...)
	case []any:
		var out []string
		for _, x := range s {
			if str, ok := x.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

func unionSortedUniqueStrings(a, b []string) []string {
	set := sets.New[string]()
	for _, x := range a {
		if x != "" {
			set.Insert(x)
		}
	}
	for _, x := range b {
		if x != "" {
			set.Insert(x)
		}
	}
	out := set.UnsortedList()
	slices.Sort(out)
	return out
}

func createNamespaceEntities(namespace types.Namespace) types.YamlString {
	return types.YamlString(`
# Source: kubernetes-defaults-` + string(namespace) + `.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: ` + string(namespace) + `
  name: kube-root-ca.crt
data:
  ca.crt: ""
---
# Source: kubernetes-defaults-` + string(namespace) + `.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: ` + string(namespace) + `
  name: default
---
# Source: kubernetes-defaults-` + string(namespace) + `.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ` + string(namespace))
}

func CreateNamespaceEntities(
	namespaces sets.Set[types.Namespace],
	key types.EntityKeyUnstructured,
) (entity.Entities, error) {
	result := []entity.Entity{}
	for ns := range namespaces {
		entities, err := entity.NewEntitiesFromYaml(log.Default(), createNamespaceEntities(ns), key)
		if err != nil {
			return entity.Entities{}, err
		}
		result = append(result, entities.Items...)
	}

	entities, err := entity.NewEntities(result)
	if err != nil {
		return entity.Entities{}, err
	}

	return ApplyScopeInfoMap(types.CrdModeIgnore, entities, DefaultScopeInfoMap(), key)
}

// WithoutDuplicateSyntheticKubernetesDefaults removes synthetic namespace-bundle entities (from
// CreateNamespaceEntities / kubernetes-defaults-<ns>.yaml) whose IDs already exist in renderedTemplate.
// This avoids duplicate Namespace, default ServiceAccount, and kube-root-ca ConfigMap when charts already render them.
func WithoutDuplicateSyntheticKubernetesDefaults(
	l log.Logger,
	renderedTemplate entity.Entities,
	synthetic entity.Entities,
) (entity.Entities, error) {
	existing := sets.New[types.Id]()
	for _, e := range renderedTemplate.Items {
		id, err := e.Id()
		if err != nil {
			continue
		}
		existing.Insert(id)
	}
	var kept []entity.Entity
	for _, e := range synthetic.Items {
		id, err := e.Id()
		if err != nil {
			continue
		}
		if existing.Has(id) {
			l.DebugLog(logIdCommands,
				"skip synthetic kubernetes-defaults entity (already in rendered templates): {id}",
				log.String("id", string(id)))
			continue
		}
		kept = append(kept, e)
	}
	return entity.NewEntities(kept)
}

// CollectNamespacesFromEntities returns namespaces referenced by namespaced template or cluster
// entities plus metadata.name of v1/Namespace objects.
// NamespaceNamePresence classifies a namespace name against template inventory and live Namespace objects.
type NamespaceNamePresence int

const (
	NamespaceNameNeither NamespaceNamePresence = iota
	NamespaceNameTemplateOnly
	NamespaceNameClusterOnly
	NamespaceNameTemplateAndCluster
)

// NamespaceNamePresenceLabel returns a stable English label for NamespaceNamePresence.
func NamespaceNamePresenceLabel(p NamespaceNamePresence) string {
	switch p {
	case NamespaceNameTemplateOnly:
		return "template only"
	case NamespaceNameClusterOnly:
		return "cluster only"
	case NamespaceNameTemplateAndCluster:
		return "template and cluster"
	default:
		return "neither"
	}
}

// ClassifyNamespaceNamePresence classifies ns against namespaces referenced by rendered templates
// and namespaces that exist as live v1/Namespace objects.
func ClassifyNamespaceNamePresence(
	ns types.Namespace,
	templateNamespaces, liveNamespaceNames sets.Set[types.Namespace],
) NamespaceNamePresence {
	inT := templateNamespaces.Has(ns)
	inC := liveNamespaceNames.Has(ns)
	switch {
	case inT && inC:
		return NamespaceNameTemplateAndCluster
	case inT && !inC:
		return NamespaceNameTemplateOnly
	case !inT && inC:
		return NamespaceNameClusterOnly
	default:
		return NamespaceNameNeither
	}
}

// CollectLiveClusterNamespaceNames returns metadata.name for each v1/Namespace in live inventory.
func CollectLiveClusterNamespaceNames(liveCluster entity.Entities) sets.Set[types.Namespace] {
	out := sets.New[types.Namespace]()
	for _, e := range liveCluster.Items {
		gvk, err := e.GVKString()
		if err != nil || gvk != types.KubernetesGvkV1Namespace {
			continue
		}
		name, err := e.Name()
		if err != nil || name == "" {
			continue
		}
		out.Insert(types.Namespace(name))
	}
	return out
}

func CollectNamespacesFromEntities(entities entity.Entities) sets.Set[types.Namespace] {
	out := sets.New[types.Namespace]()
	for _, e := range entities.Items {
		gvk, err := e.GVKString()
		if err != nil {
			continue
		}
		if gvk == types.KubernetesGvkV1Namespace {
			name, err := e.Name()
			if err != nil || name == "" {
				continue
			}
			out.Insert(types.Namespace(name))
			continue
		}
		if !e.HasNamespace() {
			continue
		}
		ns, err := e.Namespace()
		if err != nil || ns == "" {
			continue
		}
		out.Insert(ns)
	}
	return out
}

// BuildSyntheticNamespaceDefaultTargets returns kubernetes-defaults bundle entities (Namespace,
// ServiceAccount/default, ConfigMap/kube-root-ca.crt with data.ca.crt) for every namespace seen
// in rendered, dropping any document whose ID already exists in rendered.
func BuildSyntheticNamespaceDefaultTargets(
	l log.Logger,
	rendered entity.Entities,
	key types.EntityKeyUnstructured,
) (entity.Entities, error) {
	nsSet := CollectNamespacesFromEntities(rendered)
	if len(nsSet) == 0 {
		return entity.Entities{}, nil
	}
	synthetic, err := CreateNamespaceEntities(nsSet, key)
	if err != nil {
		return entity.Entities{}, err
	}
	return WithoutDuplicateSyntheticKubernetesDefaults(l, rendered, synthetic)
}
