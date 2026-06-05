package commands

import (
	htypes "hydra-gitops.org/hydra/hydra-go/core/types"
)

// Values for [ClusterScaleWorkloadDependencyStatus.RefRole] (YAML / API); empty means unclassified.
const (
	ScaleDependencyRefRoleUnspecified  = ""
	ScaleDependencyRefRolePrerequisite = "prerequisite"
	// ScaleDependencyRefRoleProduces is from ref-parser label "source" (workload materializes target).
	ScaleDependencyRefRoleProduces = "produces"
	// ScaleDependencyRefRoleDownstream is controller-managed children (ReplicaSet, Pod) without a labeled dep→child ref.
	ScaleDependencyRefRoleDownstream = "downstream"
	scaleDependencyProducesRefLabel  = "source"
)

var scaleDependencyPrerequisiteRefLabels = map[string]struct{}{
	"imagePullSecret":       {},
	"env":                   {},
	"volume":                {},
	"envFrom":               {},
	"serviceAccount":        {},
	"initContainer envFrom": {},
	"initContainer env":     {},
}

// scaleDependencyRefRoleFromLabeledDirectEdges classifies the direct logical edge from→to using ref-parser
// labels only (reverse handled like workload dependency edges). Unspecified when no matching labeled ref.
func scaleDependencyRefRoleFromLabeledDirectEdges(from, to htypes.Id, refs []htypes.Ref) string {
	var seenSource, seenPrereq bool
	for _, ref := range refs {
		f := htypes.Id(ref.From)
		t := htypes.Id(ref.To)
		if ref.Reverse {
			f, t = t, f
		}
		if f != from || t != to {
			continue
		}
		for _, l := range ref.Labels {
			if l == scaleDependencyProducesRefLabel {
				seenSource = true
			}
			if _, ok := scaleDependencyPrerequisiteRefLabels[l]; ok {
				seenPrereq = true
			}
		}
	}
	if seenSource {
		return ScaleDependencyRefRoleProduces
	}
	if seenPrereq {
		return ScaleDependencyRefRolePrerequisite
	}
	return ScaleDependencyRefRoleUnspecified
}

// scaleDependencyRefRole classifies a scale-status dependency row: labeled direct edges first, then GVK
// fallbacks for refs that only exist transitively or without parser labels (e.g. SA-injected pull secrets,
// automounted CA bundle, ReplicaSet/Pod under a Deployment).
func scaleDependencyRefRole(from, to htypes.Id, refs []htypes.Ref, depGVK htypes.GVKString) string {
	if r := scaleDependencyRefRoleFromLabeledDirectEdges(from, to, refs); r != ScaleDependencyRefRoleUnspecified {
		return r
	}
	switch depGVK {
	case htypes.KubernetesGvkAppsV1ReplicaSet, htypes.KubernetesGvkV1Pod:
		return ScaleDependencyRefRoleDownstream
	case htypes.KubernetesGvkV1Secret, htypes.KubernetesGvkV1ConfigMap:
		return ScaleDependencyRefRolePrerequisite
	default:
		return ScaleDependencyRefRoleUnspecified
	}
}

// refRoleSortRank orders scale-status dependency rows: out-like roles first, then in (prerequisite), then unclassified.
func refRoleSortRank(role string) int {
	switch role {
	case ScaleDependencyRefRoleProduces, ScaleDependencyRefRoleDownstream:
		return 0
	case ScaleDependencyRefRolePrerequisite:
		return 1
	default:
		return 2
	}
}

// scaleDependencyRefFlowTag returns a short label for scale status text: prerequisite → "in",
// produces (source) or downstream (RS/Pod) → "out". Empty when unclassified.
func scaleDependencyRefFlowTag(role string) string {
	switch role {
	case ScaleDependencyRefRolePrerequisite:
		return "in"
	case ScaleDependencyRefRoleProduces, ScaleDependencyRefRoleDownstream:
		return "out"
	default:
		return ""
	}
}
