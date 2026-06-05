package commands

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// effectivePresetsWithEnabled loads the embedded effective preset set and force-enables the named
// preset ids; everything else is left at its declared defaultEnabled. This avoids depending on
// transitive activations from k3s/k3d/syseleven for focused tests.
func effectivePresetsWithEnabled(t *testing.T, ids ...string) []hydra.ClusterDefaultsPresetEffective {
	t.Helper()
	effs, err := hydra.EffectiveClusterDefaultsPresets(nil)
	require.NoError(t, err)
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	out := make([]hydra.ClusterDefaultsPresetEffective, len(effs))
	copy(out, effs)
	for i := range out {
		if _, ok := wanted[out[i].ID]; ok {
			out[i].Enabled = true
		} else {
			out[i].Enabled = false
		}
	}
	return out
}

func entityIDs(t *testing.T, ents entity.Entities) []string {
	t.Helper()
	out := make([]string, 0, ents.Len())
	for _, e := range ents.Items {
		id, err := e.Id()
		require.NoError(t, err)
		out = append(out, string(id))
	}
	sort.Strings(out)
	return out
}

func TestPresetTemplateEntities_LocalPathProvisionerAnchorsBecomeBuiltinAppEntities(t *testing.T) {
	effective := effectivePresetsWithEnabled(t, "local-path-provisioner")
	per, err := PresetTemplateEntities(types.InCluster, effective, 99)
	require.NoError(t, err)

	wantApp, err := types.NewPresetAppId(types.InCluster, "local-path-provisioner")
	require.NoError(t, err)
	require.Contains(t, per, wantApp)

	got := entityIDs(t, per[wantApp])
	want := []string{
		"apps/v1/Deployment/kube-system/local-path-provisioner",
		"rbac.authorization.k8s.io/v1/ClusterRole//local-path-provisioner-role",
		"rbac.authorization.k8s.io/v1/ClusterRoleBinding//local-path-provisioner-bind",
		"storage.k8s.io/v1/StorageClass//local-path",
		"v1/ConfigMap/kube-system/local-path-config",
		"v1/ServiceAccount/kube-system/local-path-provisioner-service-account",
	}
	assert.Equal(t, want, got)

	for _, e := range per[wantApp].Items {
		appId, err := e.AppId()
		require.NoError(t, err)
		assert.Equal(t, wantApp, appId)
	}
}

func TestPresetTemplateEntities_MetricsServerAnchorsBecomeBuiltinAppEntitiesAndOmitCelOnly(t *testing.T) {
	effective := effectivePresetsWithEnabled(t, "metrics-server")
	per, err := PresetTemplateEntities(types.InCluster, effective, 99)
	require.NoError(t, err)

	wantApp, err := types.NewPresetAppId(types.InCluster, "metrics-server")
	require.NoError(t, err)
	require.Contains(t, per, wantApp)

	got := entityIDs(t, per[wantApp])
	want := []string{
		"apiregistration.k8s.io/v1/APIService//v1beta1.metrics.k8s.io",
		"apps/v1/Deployment/kube-system/metrics-server",
		"rbac.authorization.k8s.io/v1/ClusterRole//system:aggregated-metrics-reader",
		"rbac.authorization.k8s.io/v1/ClusterRole//system:metrics-server",
		"rbac.authorization.k8s.io/v1/ClusterRoleBinding//metrics-server:system:auth-delegator",
		"rbac.authorization.k8s.io/v1/ClusterRoleBinding//system:metrics-server",
		"rbac.authorization.k8s.io/v1/RoleBinding/kube-system/metrics-server-auth-reader",
		"v1/Secret/kube-system/metrics-server-serving-cert",
		"v1/Service/kube-system/metrics-server",
		"v1/ServiceAccount/kube-system/metrics-server",
	}
	assert.Equal(t, want, got)

	// CEL-only preset predicates (PodMetrics regex, NodeMetrics, PodDisruptionBudget) must not
	// appear as synthetic template entities; they remain as predicate-only matchers.
	for _, id := range got {
		assert.NotContains(t, id, "PodMetrics")
		assert.NotContains(t, id, "NodeMetrics")
		assert.NotContains(t, id, "PodDisruptionBudget")
	}
}

func TestPresetTemplateEntities_DisabledPresetsProduceNoEntities(t *testing.T) {
	effective := effectivePresetsWithEnabled(t /* none */)
	per, err := PresetTemplateEntities(types.InCluster, effective, 99)
	require.NoError(t, err)
	assert.Empty(t, per)
}

func TestPresetTemplateEntities_ConflictingAnchorsAcrossPresetsErrors(t *testing.T) {
	dup := types.PresetIdItem{Id: "v1/ConfigMap/kube-system/shared-anchor"}
	effective := []hydra.ClusterDefaultsPresetEffective{
		{
			ID:      "alpha",
			Enabled: true,
			Predicates: map[string]hydra.ClusterDefaultsPredicateEffective{
				"workloads": {Enabled: true, Ids: []hydra.ClusterDefaultsIdLine{dup}},
			},
		},
		{
			ID:      "beta",
			Enabled: true,
			Predicates: map[string]hydra.ClusterDefaultsPredicateEffective{
				"workloads": {Enabled: true, Ids: []hydra.ClusterDefaultsIdLine{dup}},
			},
		},
	}
	_, err := PresetTemplateEntities(types.InCluster, effective, 99)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shared-anchor")
}
