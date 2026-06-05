package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeScaleDownEntity(group, version, kind, resource, namespace, name string, replicas int64) entity.Entity {
	gvk := types.NewGVK(types.Group(group), types.Version(version), types.Kind(kind))
	b := entity.NewEntityBuilder().
		WithGVK(gvk).
		WithResource(types.Resource(resource)).
		WithName(types.Name(name))
	if namespace != "" {
		b = b.WithNamespace(types.Namespace(namespace))
	}
	e := mustBuild(b)
	apiVersion := version
	if group != "" {
		apiVersion = group + "/" + version
	}
	u := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]any{
				"replicas": replicas,
			},
		},
	}
	return withUnstructured(e, types.KeyClusterEntity, u)
}

func TestCollectScaleDownTargets_NoWorkloads(t *testing.T) {
	cm := makeEntity("", "v1", "ConfigMap", "default", "app-config")
	secret := makeEntity("", "v1", "Secret", "default", "tls-cert")

	entities, err := entity.NewEntities([]entity.Entity{cm, secret})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	assert.Empty(t, targets)
}

func TestCollectScaleDownTargets_DeploymentWithReplicas(t *testing.T) {
	dep := makeScaleDownEntity("apps", "v1", "Deployment", "deployments", "default", "web-app", 3)

	entities, err := entity.NewEntities([]entity.Entity{dep})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, types.Id("apps/v1/Deployment/default/web-app"), targets[0].Id)
	assert.Equal(t, types.Name("web-app"), targets[0].Name)
	assert.Equal(t, types.Namespace("default"), targets[0].Ns)
	assert.Equal(t, types.GVKString("apps/v1/Deployment"), targets[0].GVK)
	assert.Equal(t, types.NewGVR("apps", "v1", "deployments"), targets[0].GVR)
	assert.Equal(t, int64(3), targets[0].Replicas)
}

func TestCollectScaleDownTargets_StatefulSetAlreadyScaled(t *testing.T) {
	sts := makeScaleDownEntity("apps", "v1", "StatefulSet", "statefulsets", "default", "db", 0)

	entities, err := entity.NewEntities([]entity.Entity{sts})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	assert.Empty(t, targets)
}

func TestCollectScaleDownTargets_MixedTypes(t *testing.T) {
	dep := makeScaleDownEntity("apps", "v1", "Deployment", "deployments", "default", "web-app", 3)
	sts := makeScaleDownEntity("apps", "v1", "StatefulSet", "statefulsets", "default", "db", 0)
	cm := makeEntity("", "v1", "ConfigMap", "default", "app-config")

	entities, err := entity.NewEntities([]entity.Entity{dep, sts, cm})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, types.Id("apps/v1/Deployment/default/web-app"), targets[0].Id)
	assert.Equal(t, int64(3), targets[0].Replicas)
}

func TestCollectScaleDownTargets_ReplicaSetWithReplicas(t *testing.T) {
	rs := makeScaleDownEntity("apps", "v1", "ReplicaSet", "replicasets", "default", "web-app-abc123", 2)

	entities, err := entity.NewEntities([]entity.Entity{rs})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, types.Id("apps/v1/ReplicaSet/default/web-app-abc123"), targets[0].Id)
	assert.Equal(t, types.Name("web-app-abc123"), targets[0].Name)
	assert.Equal(t, types.Namespace("default"), targets[0].Ns)
	assert.Equal(t, types.GVKString("apps/v1/ReplicaSet"), targets[0].GVK)
	assert.Equal(t, types.NewGVR("apps", "v1", "replicasets"), targets[0].GVR)
	assert.Equal(t, int64(2), targets[0].Replicas)
}

func TestCollectScaleDownTargets_EntityWithoutUnstructuredData(t *testing.T) {
	dep := withResource(makeEntity("apps", "v1", "Deployment", "default", "no-data"), types.Resource("deployments"))

	entities, err := entity.NewEntities([]entity.Entity{dep})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	assert.Empty(t, targets)
}

func TestCollectScaleDownTargets_MultipleWorkloads(t *testing.T) {
	dep1 := makeScaleDownEntity("apps", "v1", "Deployment", "deployments", "default", "frontend", 2)
	dep2 := makeScaleDownEntity("apps", "v1", "Deployment", "deployments", "default", "backend", 5)
	sts := makeScaleDownEntity("apps", "v1", "StatefulSet", "statefulsets", "default", "db", 1)

	entities, err := entity.NewEntities([]entity.Entity{dep1, dep2, sts})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	require.Len(t, targets, 3)

	targetIds := make([]types.Id, len(targets))
	for i, tgt := range targets {
		targetIds[i] = tgt.Id
	}
	assert.Contains(t, targetIds, types.Id("apps/v1/Deployment/default/frontend"))
	assert.Contains(t, targetIds, types.Id("apps/v1/Deployment/default/backend"))
	assert.Contains(t, targetIds, types.Id("apps/v1/StatefulSet/default/db"))

	for _, tgt := range targets {
		switch tgt.Id {
		case "apps/v1/Deployment/default/frontend":
			assert.Equal(t, int64(2), tgt.Replicas)
			assert.Equal(t, types.Name("frontend"), tgt.Name)
			assert.Equal(t, types.Namespace("default"), tgt.Ns)
			assert.Equal(t, types.GVKString("apps/v1/Deployment"), tgt.GVK)
			assert.Equal(t, types.NewGVR("apps", "v1", "deployments"), tgt.GVR)
		case "apps/v1/Deployment/default/backend":
			assert.Equal(t, int64(5), tgt.Replicas)
			assert.Equal(t, types.Name("backend"), tgt.Name)
			assert.Equal(t, types.Namespace("default"), tgt.Ns)
			assert.Equal(t, types.GVKString("apps/v1/Deployment"), tgt.GVK)
			assert.Equal(t, types.NewGVR("apps", "v1", "deployments"), tgt.GVR)
		case "apps/v1/StatefulSet/default/db":
			assert.Equal(t, int64(1), tgt.Replicas)
			assert.Equal(t, types.Name("db"), tgt.Name)
			assert.Equal(t, types.Namespace("default"), tgt.Ns)
			assert.Equal(t, types.GVKString("apps/v1/StatefulSet"), tgt.GVK)
			assert.Equal(t, types.NewGVR("apps", "v1", "statefulsets"), tgt.GVR)
		}
	}
}

// --- Phase Logging Helper Tests ---

func TestPhaseMessage(t *testing.T) {
	tests := []struct {
		name        string
		current     int
		total       int
		description string
		skipped     bool
		want        string
	}{
		{
			name:        "standard mode scale down before deletion",
			current:     5,
			total:       6,
			description: "scaling down workloads before deletion",
			skipped:     false,
			want:        "phase 5/6: scaling down workloads before deletion",
		},
		{
			name:        "bootstrap mode scale down before deletion",
			current:     8,
			total:       9,
			description: "scaling down workloads before deletion",
			skipped:     false,
			want:        "phase 8/9: scaling down workloads before deletion",
		},
		{
			name:        "standard mode scale down before deletion skipped",
			current:     5,
			total:       6,
			description: "scaling down workloads before deletion",
			skipped:     true,
			want:        "phase 5/6: scaling down workloads before deletion (skipped)",
		},
		{
			name:        "applying CRDs skipped",
			current:     1,
			total:       6,
			description: "applying CRDs",
			skipped:     true,
			want:        "phase 1/6: applying CRDs (skipped)",
		},
		{
			name:        "bootstrap mode applying CRDs",
			current:     1,
			total:       9,
			description: "applying CRDs",
			skipped:     false,
			want:        "phase 1/9: applying CRDs",
		},
		{
			name:        "uninstall webhook delete",
			current:     1,
			total:       3,
			description: "deleting webhook configurations",
			skipped:     false,
			want:        "phase 1/3: deleting webhook configurations",
		},
		{
			name:        "uninstall webhook delete skipped",
			current:     1,
			total:       3,
			description: "deleting webhook configurations",
			skipped:     true,
			want:        "phase 1/3: deleting webhook configurations (skipped)",
		},
		{
			name:        "uninstall scale down",
			current:     2,
			total:       3,
			description: "scaling down workloads before deletion",
			skipped:     false,
			want:        "phase 2/3: scaling down workloads before deletion",
		},
		{
			name:        "uninstall delete",
			current:     3,
			total:       3,
			description: "deleting 42 resources",
			skipped:     false,
			want:        "phase 3/3: deleting 42 resources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PhaseMessage(tt.current, tt.total, tt.description, tt.skipped)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPhaseMessageWithID(t *testing.T) {
	got := PhaseMessageWithID(2, 5, "applying namespaces", false, "apply-namespaces")
	assert.Equal(t, "phase 2/5: applying namespaces (apply-namespaces)", got)
	gotSkipped := PhaseMessageWithID(2, 5, "applying namespaces", true, "apply-namespaces")
	assert.Equal(t, "phase 2/5: applying namespaces (skipped) (apply-namespaces)", gotSkipped)
}

func TestDryRunPrefix(t *testing.T) {
	tests := []struct {
		name   string
		dryRun bool
		want   string
	}{
		{
			name:   "dry-run active",
			dryRun: true,
			want:   "[dry-run] ",
		},
		{
			name:   "dry-run inactive",
			dryRun: false,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DryRunPrefix(tt.dryRun)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDryRunPrefix_CombinedWithScaleDownMessage(t *testing.T) {
	entityId := "apps/v1/Deployment/demo/my-deployment"
	prefix := DryRunPrefix(true)
	msg := fmt.Sprintf("%sscaling down %s from %d to %d replicas", prefix, entityId, 3, 0)

	assert.Equal(t, "[dry-run] scaling down apps/v1/Deployment/demo/my-deployment from 3 to 0 replicas", msg)
}

// --- computeNamespaceRefs tests ---

func TestComputeNamespaceRefs_NamespacedEntities(t *testing.T) {
	secret := makeEntity("", "v1", "Secret", "demo", "my-secret")
	cm := makeEntity("", "v1", "ConfigMap", "demo", "my-config")
	ns := makeEntity("", "v1", "Namespace", "", "demo")

	entities, err := entity.NewEntities([]entity.Entity{secret, cm, ns})
	require.NoError(t, err)

	refs, err := computeNamespaceRefs(entities)
	require.NoError(t, err)
	require.Len(t, refs, 2)

	refFromIds := map[types.Id]types.Id{}
	for _, ref := range refs {
		refFromIds[ref.From] = ref.To
		assert.Equal(t, []string{types.RefLabelNamespace}, ref.Labels)
	}

	assert.Equal(t, types.Id("v1/Namespace//demo"), refFromIds["v1/Secret/demo/my-secret"])
	assert.Equal(t, types.Id("v1/Namespace//demo"), refFromIds["v1/ConfigMap/demo/my-config"])
}

func TestComputeNamespaceRefs_ClusterScopedOnly(t *testing.T) {
	ns := makeEntity("", "v1", "Namespace", "", "demo")
	cr := makeEntity("rbac.authorization.k8s.io", "v1", "ClusterRole", "", "admin")

	entities, err := entity.NewEntities([]entity.Entity{ns, cr})
	require.NoError(t, err)

	refs, err := computeNamespaceRefs(entities)
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestComputeNamespaceRefs_MultipleNamespaces(t *testing.T) {
	s1 := makeEntity("", "v1", "Secret", "demo", "secret-a")
	s2 := makeEntity("", "v1", "Secret", "monitoring", "secret-b")

	entities, err := entity.NewEntities([]entity.Entity{s1, s2})
	require.NoError(t, err)

	refs, err := computeNamespaceRefs(entities)
	require.NoError(t, err)
	require.Len(t, refs, 2)

	refTargets := map[types.Id]types.Id{}
	for _, ref := range refs {
		refTargets[ref.From] = ref.To
	}
	assert.Equal(t, types.Id("v1/Namespace//demo"), refTargets["v1/Secret/demo/secret-a"])
	assert.Equal(t, types.Id("v1/Namespace//monitoring"), refTargets["v1/Secret/monitoring/secret-b"])
}

// --- computeDeleteOrder tests ---

func TestComputeDeleteOrder_NamespaceIsLast(t *testing.T) {
	ns := makeEntity("", "v1", "Namespace", "", "demo")
	secret := makeEntity("", "v1", "Secret", "demo", "my-secret")
	cm := makeEntity("", "v1", "ConfigMap", "demo", "my-config")
	sa := makeEntity("", "v1", "ServiceAccount", "demo", "default")

	entities, err := entity.NewEntities([]entity.Entity{ns, secret, cm, sa})
	require.NoError(t, err)

	ordered, err := computeDeleteOrder(entities)
	require.NoError(t, err)
	require.Len(t, ordered, 4)

	ids := make([]types.Id, len(ordered))
	for i, e := range ordered {
		id, idErr := e.Id()
		require.NoError(t, idErr)
		ids[i] = id
	}

	assert.Equal(t, types.Id("v1/Namespace//demo"), ids[len(ids)-1],
		"namespace must be the last entity deleted")

	for _, id := range ids[:len(ids)-1] {
		assert.NotEqual(t, types.Id("v1/Namespace//demo"), id)
	}
}

func TestComputeDeleteOrder_MultipleNamespaces(t *testing.T) {
	nsDemo := makeEntity("", "v1", "Namespace", "", "demo")
	nsMonitoring := makeEntity("", "v1", "Namespace", "", "monitoring")
	secretDemo := makeEntity("", "v1", "Secret", "demo", "s1")
	secretMon := makeEntity("", "v1", "Secret", "monitoring", "s2")

	entities, err := entity.NewEntities([]entity.Entity{nsDemo, nsMonitoring, secretDemo, secretMon})
	require.NoError(t, err)

	ordered, err := computeDeleteOrder(entities)
	require.NoError(t, err)
	require.Len(t, ordered, 4)

	ids := make([]types.Id, len(ordered))
	for i, e := range ordered {
		id, idErr := e.Id()
		require.NoError(t, idErr)
		ids[i] = id
	}

	idxSecretDemo := -1
	idxNsDemo := -1
	idxSecretMon := -1
	idxNsMon := -1
	for i, id := range ids {
		switch id {
		case "v1/Secret/demo/s1":
			idxSecretDemo = i
		case "v1/Namespace//demo":
			idxNsDemo = i
		case "v1/Secret/monitoring/s2":
			idxSecretMon = i
		case "v1/Namespace//monitoring":
			idxNsMon = i
		}
	}

	assert.Less(t, idxSecretDemo, idxNsDemo,
		"demo secret must be deleted before demo namespace")
	assert.Less(t, idxSecretMon, idxNsMon,
		"monitoring secret must be deleted before monitoring namespace")
}

func TestComputeDeleteOrder_NoNamespaceEntity(t *testing.T) {
	secret := makeEntity("", "v1", "Secret", "demo", "my-secret")
	cm := makeEntity("", "v1", "ConfigMap", "demo", "my-config")

	entities, err := entity.NewEntities([]entity.Entity{secret, cm})
	require.NoError(t, err)

	ordered, err := computeDeleteOrder(entities)
	require.NoError(t, err)
	assert.Len(t, ordered, 2, "all entities should be present even without namespace entity")
}

func TestComputeDeleteOrder_ClusterScopedOnly(t *testing.T) {
	ns1 := makeEntity("", "v1", "Namespace", "", "a")
	ns2 := makeEntity("", "v1", "Namespace", "", "b")

	entities, err := entity.NewEntities([]entity.Entity{ns1, ns2})
	require.NoError(t, err)

	ordered, err := computeDeleteOrder(entities)
	require.NoError(t, err)
	assert.Len(t, ordered, 2)
}

// --- Owned ReplicaSet filtering tests ---

func makeOwnedReplicaSetEntity(namespace, name, ownerKind, ownerName, ownerUID string, replicas int64) entity.Entity {
	gvk := types.NewGVK(types.Group("apps"), types.Version("v1"), types.Kind("ReplicaSet"))
	b := entity.NewEntityBuilder().
		WithGVK(gvk).
		WithResource(types.Resource("replicasets")).
		WithName(types.Name(name))
	if namespace != "" {
		b = b.WithNamespace(types.Namespace(namespace))
	}
	e := mustBuild(b)
	u := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "ReplicaSet",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
				"ownerReferences": []any{
					map[string]any{
						"apiVersion": "apps/v1",
						"kind":       ownerKind,
						"name":       ownerName,
						"uid":        ownerUID,
					},
				},
			},
			"spec": map[string]any{
				"replicas": replicas,
			},
		},
	}
	return withUnstructured(e, types.KeyClusterEntity, u)
}

func TestCollectScaleDownTargets_ReplicaSetOwnedByDeployment(t *testing.T) {
	rs := makeOwnedReplicaSetEntity("default", "web-app-abc123", "Deployment", "web-app", "deploy-uid-123", 2)
	dep := makeScaleDownEntity("apps", "v1", "Deployment", "deployments", "default", "web-app", 3)

	entities, err := entity.NewEntities([]entity.Entity{rs, dep})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, types.Id("apps/v1/Deployment/default/web-app"), targets[0].Id)
}

func TestCollectScaleDownTargets_ReplicaSetOwnedByStatefulSet(t *testing.T) {
	rs := makeOwnedReplicaSetEntity("default", "db-rs-xyz789", "StatefulSet", "db", "sts-uid-456", 2)
	sts := makeScaleDownEntity("apps", "v1", "StatefulSet", "statefulsets", "default", "db", 3)

	entities, err := entity.NewEntities([]entity.Entity{rs, sts})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, types.Id("apps/v1/StatefulSet/default/db"), targets[0].Id)
}

func TestCollectScaleDownTargets_ReplicaSetOwnedByCustomController(t *testing.T) {
	rs := makeOwnedReplicaSetEntity("default", "custom-rs-abc", "MyCustomController", "my-ctrl", "ctrl-uid-789", 2)

	entities, err := entity.NewEntities([]entity.Entity{rs})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, types.Id("apps/v1/ReplicaSet/default/custom-rs-abc"), targets[0].Id)
	assert.Equal(t, int64(2), targets[0].Replicas)
}

func TestCollectScaleDownTargets_BareReplicaSet(t *testing.T) {
	rs := makeScaleDownEntity("apps", "v1", "ReplicaSet", "replicasets", "default", "standalone-rs", 2)

	entities, err := entity.NewEntities([]entity.Entity{rs})
	require.NoError(t, err)

	targets, err := collectScaleDownTargets(entities, types.KeyClusterEntity)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assert.Equal(t, types.Id("apps/v1/ReplicaSet/default/standalone-rs"), targets[0].Id)
	assert.Equal(t, int64(2), targets[0].Replicas)
}

// --- SplitWebhooks tests ---

func TestSplitWebhooks_MixedEntities(t *testing.T) {
	vwc := makeEntity("admissionregistration.k8s.io", "v1", "ValidatingWebhookConfiguration", "", "validate-policy")
	mwc := makeEntity("admissionregistration.k8s.io", "v1", "MutatingWebhookConfiguration", "", "inject-sidecar")
	deploy := makeEntity("apps", "v1", "Deployment", "default", "my-app")
	svc := makeEntity("", "v1", "Service", "default", "my-svc")

	all, err := entity.NewEntities([]entity.Entity{vwc, mwc, deploy, svc})
	require.NoError(t, err)

	webhooks, rest, err := SplitWebhooks(all)
	require.NoError(t, err)

	assert.Equal(t, 2, webhooks.Len(), "should extract both webhook configurations")
	assert.Equal(t, 2, rest.Len(), "Deployment and Service should remain in rest")

	for _, e := range webhooks.Items {
		gvk, gErr := e.GVKString()
		require.NoError(t, gErr)
		assert.True(t,
			gvk == types.KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration ||
				gvk == types.KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration,
			"expected webhook GVK, got %s", gvk)
	}

	for _, e := range rest.Items {
		gvk, gErr := e.GVKString()
		require.NoError(t, gErr)
		assert.NotEqual(t, types.KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration, gvk)
		assert.NotEqual(t, types.KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration, gvk)
	}
}

func TestSplitWebhooks_NoWebhooks(t *testing.T) {
	deploy := makeEntity("apps", "v1", "Deployment", "default", "my-app")
	svc := makeEntity("", "v1", "Service", "default", "my-svc")

	all, err := entity.NewEntities([]entity.Entity{deploy, svc})
	require.NoError(t, err)

	webhooks, rest, err := SplitWebhooks(all)
	require.NoError(t, err)

	assert.Equal(t, 0, webhooks.Len(), "no webhook entities expected")
	assert.Equal(t, 2, rest.Len(), "all entities should be in rest")
}

func TestSplitWebhooks_OnlyWebhooks(t *testing.T) {
	vwc := makeEntity("admissionregistration.k8s.io", "v1", "ValidatingWebhookConfiguration", "", "validate-policy")
	mwc := makeEntity("admissionregistration.k8s.io", "v1", "MutatingWebhookConfiguration", "", "inject-sidecar")

	all, err := entity.NewEntities([]entity.Entity{vwc, mwc})
	require.NoError(t, err)

	webhooks, rest, err := SplitWebhooks(all)
	require.NoError(t, err)

	assert.Equal(t, 2, webhooks.Len(), "all entities should be webhooks")
	assert.Equal(t, 0, rest.Len(), "rest should be empty")
}

// TestSplitWebhooks_RestExcludesAdmissionWebhooks documents the cluster uninstall invariant: after splitting
// out admission webhook configurations, the remaining entity set must not re-classify any as webhooks so
// DeleteResources can skip the webhook phase when those objects were deleted before RemoveUninstallFinalizers.
func TestSplitWebhooks_RestExcludesAdmissionWebhooks(t *testing.T) {
	vwc := makeEntity("admissionregistration.k8s.io", "v1", "ValidatingWebhookConfiguration", "", "validate-policy")
	deploy := makeEntity("apps", "v1", "Deployment", "default", "my-app")

	all, err := entity.NewEntities([]entity.Entity{vwc, deploy})
	require.NoError(t, err)

	_, rest, err := SplitWebhooks(all)
	require.NoError(t, err)

	webhooksAgain, _, err := SplitWebhooks(rest)
	require.NoError(t, err)
	assert.Equal(t, 0, webhooksAgain.Len(), "rest must contain no Validating/MutatingWebhookConfiguration entities")
	assert.Equal(t, 1, rest.Len())
}
