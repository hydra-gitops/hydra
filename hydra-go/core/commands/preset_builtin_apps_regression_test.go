package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// presetEntitiesForTest materializes preset template entities for the named presets only,
// avoiding cross-preset anchor conflicts that would arise from force-enabling all embedded
// presets (e.g. flannel/canal share anchors and are mutually exclusive in real cluster configs).
func presetEntitiesForTest(t *testing.T, ids ...string) map[types.AppId]entity.Entities {
	t.Helper()
	effective := effectivePresetsWithEnabled(t, ids...)
	per, err := PresetTemplateEntities(types.InCluster, effective, 99)
	require.NoError(t, err)
	return per
}

// TestPresetAnchorBecomesTemplateIdOwnerOfBuiltinApp pins the bug-fix mechanism:
// after [MergeBuiltinPresetAppsIntoRendered] the explicit preset anchor IDs (here the
// local-path-provisioner Deployment) are present in [templateResourceIDToApp] mapped to the
// synthetic builtin app. This is what allows the standard ownership pipeline (template-id win,
// then refs / workload closure) to attribute related cluster resources (Events, PodMetrics, …)
// to the builtin app via the existing refs without any preset-specific code path.
func TestPresetAnchorBecomesTemplateIdOwnerOfBuiltinApp(t *testing.T) {
	emptyPerApp := map[types.AppId]entity.Entities{}
	emptyAll, err := entity.NewEntities(nil)
	require.NoError(t, err)
	emptyAppIds := sets.New[types.AppId]()

	presetEntities := presetEntitiesForTest(t, "local-path-provisioner")
	mergedPerApp, mergedAll, mergedAppIds, err := MergeBuiltinPresetAppsIntoRendered(
		emptyPerApp, emptyAll, emptyAppIds, presetEntities)
	require.NoError(t, err)

	wantApp, err := types.NewPresetAppId(types.InCluster, "local-path-provisioner")
	require.NoError(t, err)
	require.Contains(t, mergedPerApp, wantApp)
	assert.True(t, mergedAppIds.Has(wantApp))

	idToApp := templateResourceIDToApp(mergedPerApp)
	deployID := types.Id("apps/v1/Deployment/kube-system/local-path-provisioner")
	assert.Equal(t, wantApp, idToApp[deployID],
		"preset deployment anchor must be a template entity of the builtin app so generic event/podMetrics ownership can attribute related cluster objects")

	require.NotEmpty(t, mergedAll.Items)
}

// TestPresetMergeRespectsTemplatePrimacyOnAnchorOverlap pins template primacy: when a real Hydra
// app already declares an anchor ID in its standalone render, that ID does not become a builtin
// app template entity, so the real app keeps ownership.
func TestPresetMergeRespectsTemplatePrimacyOnAnchorOverlap(t *testing.T) {
	realApp := types.AppId("real.app")
	deployID := types.Id("apps/v1/Deployment/kube-system/local-path-provisioner")
	deployEntity := mustBuild(entity.NewEntityBuilder().
		WithGroup("apps").
		WithVersion("v1").
		WithKind("Deployment").
		WithNamespace("kube-system").
		WithName("local-path-provisioner").
		WithNamespaced(types.NamespacedYes).
		WithAppIds([]types.AppId{realApp}).
		WithAppId(realApp))
	realEnts, err := entity.NewEntities([]entity.Entity{deployEntity})
	require.NoError(t, err)
	perApp := map[types.AppId]entity.Entities{realApp: realEnts}
	allEnts, err := entity.NewEntities([]entity.Entity{deployEntity})
	require.NoError(t, err)
	appIds := sets.New[types.AppId](realApp)

	presetEntities := presetEntitiesForTest(t, "local-path-provisioner")
	mergedPerApp, _, _, err := MergeBuiltinPresetAppsIntoRendered(perApp, allEnts, appIds, presetEntities)
	require.NoError(t, err)

	idToApp := templateResourceIDToApp(mergedPerApp)
	assert.Equal(t, realApp, idToApp[deployID],
		"real app must keep ownership when a preset declares the same anchor id")

	builtinApp, err := types.NewPresetAppId(types.InCluster, "local-path-provisioner")
	require.NoError(t, err)
	if ents, ok := mergedPerApp[builtinApp]; ok {
		for _, e := range ents.Items {
			id, err := e.Id()
			require.NoError(t, err)
			assert.NotEqual(t, deployID, id, "overlapping anchor must not appear in builtin app render")
		}
	}
}

// TestEventRegardingPresetDeploymentAssignsBuiltinApp pins the generic event ownership model:
// when an Event regards a preset Deployment anchor, the builtin app is selected through the
// normal ref-ownership closure without any workload-specific Event CEL.
func TestEventRegardingPresetDeploymentAssignsBuiltinApp(t *testing.T) {
	emptyPerApp := map[types.AppId]entity.Entities{}
	emptyAll, err := entity.NewEntities(nil)
	require.NoError(t, err)
	emptyAppIds := sets.New[types.AppId]()

	for _, presetID := range []string{"local-path-provisioner", "metrics-server"} {
		t.Run(presetID, func(t *testing.T) {
			presetEntities := presetEntitiesForTest(t, presetID)
			mergedPerApp, _, _, err := MergeBuiltinPresetAppsIntoRendered(
				emptyPerApp, emptyAll, emptyAppIds, presetEntities)
			require.NoError(t, err)
			renderedAll := mergedPerApp[mustBuiltinAppID(t, presetID)]

			event := testLiveEntity("events.k8s.io/v1", "events", "Event", "kube-system", presetID+"-event")
			event = withLiveObject(t, event, map[string]any{
				"regarding": map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       presetID,
					"namespace":  "kube-system",
				},
				"reason":              "Scheduled",
				"action":              "Schedule",
				"reportingController": "test",
				"reportingInstance":   "test-1",
				"type":                "Normal",
				"note":                "test",
			})
			liveEnts, err := entity.NewEntities([]entity.Entity{event})
			require.NoError(t, err)

			assignment, _, _, err := AssignClusterEntitiesToAtMostOneAppByRefs(AssignClusterEntitiesToAtMostOneAppByRefsInput{
				ClusterEntities: liveEnts,
				AllAppIDs:       sets.New[types.AppId](mustBuiltinAppID(t, presetID)),
				PerAppRendered:  mergedPerApp,
				RenderedAllApps: renderedAll,
				Parallel:        1,
				NetworkMode:     types.HelmNetworkModeOffline,
			})
			require.NoError(t, err)

			wantApp, err := types.NewPresetAppId(types.InCluster, presetID)
			require.NoError(t, err)
			eventID, err := event.Id()
			require.NoError(t, err)
			assert.Equal(t, wantApp, assignment[eventID],
				"Event must be attributed to the builtin app via generic Event.regarding ownership")
		})
	}
}

func TestAssignClusterEntitiesToAtMostOneAppByRefs_OrphanServiceLBEventAssignsIngressApp(t *testing.T) {
	ingressApp := types.AppId("in-cluster.cluster-infra.ingress-nginx")
	ingressService := testTemplateEntity("services", "Service", "ingress-nginx", "ingress-nginx-controller", ingressApp)
	renderedIngress := mustEntities(t, []entity.Entity{ingressService})
	renderedAll := renderedIngress

	event := testLiveEntity("events.k8s.io/v1", "events", "Event", "kube-system", "svclb-ingress-nginx-controller-e6e15d71.18afc93e76dc4aae")
	event = withLiveObject(t, event, map[string]any{
		"regarding": map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"name":       "svclb-ingress-nginx-controller-e6e15d71-49rdp",
			"namespace":  "kube-system",
		},
		"reason":              "Pulling",
		"action":              "Pulling",
		"reportingController": "kubelet",
		"reportingInstance":   "node-1",
		"type":                "Normal",
		"note":                "test",
	})
	liveEnts, err := entity.NewEntities([]entity.Entity{event})
	require.NoError(t, err)

	assignment, _, _, err := AssignClusterEntitiesToAtMostOneAppByRefs(AssignClusterEntitiesToAtMostOneAppByRefsInput{
		ClusterEntities: liveEnts,
		AllAppIDs:       sets.New[types.AppId](ingressApp),
		PerAppRendered:  map[types.AppId]entity.Entities{ingressApp: renderedIngress},
		RenderedAllApps: renderedAll,
		Parallel:        1,
		NetworkMode:     types.HelmNetworkModeOffline,
	})
	require.NoError(t, err)

	eventID, err := event.Id()
	require.NoError(t, err)
	assert.Equal(t, ingressApp, assignment[eventID],
		"orphan svclb Event must be attributed to ingress-nginx via the stable Service template anchor")
}

func TestAssignClusterEntitiesToAtMostOneAppByRefs_OrphanLocalPathHelperEventAssignsBuiltinApp(t *testing.T) {
	presetEntities := presetEntitiesForTest(t, "local-path-provisioner")
	emptyAll, err := entity.NewEntities(nil)
	require.NoError(t, err)
	mergedPerApp, _, _, err := MergeBuiltinPresetAppsIntoRendered(
		map[types.AppId]entity.Entities{},
		emptyAll,
		sets.New[types.AppId](),
		presetEntities,
	)
	require.NoError(t, err)

	builtinApp := mustBuiltinAppID(t, "local-path-provisioner")
	renderedAll := mergedPerApp[builtinApp]

	event := testLiveEntity("events.k8s.io/v1", "events", "Event", "kube-system", "helper-pod-create-pvc-5f68d777-e552-4644-acfb-dfad58a881af.18afcdc67252a81a")
	event = withLiveObject(t, event, map[string]any{
		"regarding": map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"name":       "helper-pod-create-pvc-5f68d777-e552-4644-acfb-dfad58a881af",
			"namespace":  "kube-system",
		},
		"reason":              "Pulling",
		"reportingController": "kubelet",
		"reportingInstance":   "k3d-argocd-server-0",
		"type":                "Normal",
		"note":                "Pulling image \"rancher/mirrored-library-busybox:1.37.0\"",
	})
	liveEnts, err := entity.NewEntities([]entity.Entity{event})
	require.NoError(t, err)

	assignment, _, _, err := AssignClusterEntitiesToAtMostOneAppByRefs(AssignClusterEntitiesToAtMostOneAppByRefsInput{
		ClusterEntities: liveEnts,
		AllAppIDs:       sets.New[types.AppId](builtinApp),
		PerAppRendered:  mergedPerApp,
		RenderedAllApps: renderedAll,
		Parallel:        1,
		NetworkMode:     types.HelmNetworkModeOffline,
	})
	require.NoError(t, err)

	eventID, err := event.Id()
	require.NoError(t, err)
	assert.Equal(t, builtinApp, assignment[eventID],
		"orphan local-path helper Event must be attributed to local-path-provisioner via the stable Deployment template anchor")
}

func TestAssignClusterEntitiesToAtMostOneAppByRefs_OrphanWorkloadPodEventAssignsBuiltinAppViaTemplateAnchor(t *testing.T) {
	testCases := []struct {
		presetID  string
		eventName string
		podName   string
	}{
		{
			presetID:  "coredns",
			eventName: "coredns-7566b5ff58-zpk45.18afbd497cea6738",
			podName:   "coredns-7566b5ff58-zpk45",
		},
		{
			presetID:  "local-path-provisioner",
			eventName: "local-path-provisioner-6bc6568469-rlr5q.18afbe8af39528ee",
			podName:   "local-path-provisioner-6bc6568469-rlr5q",
		},
		{
			presetID:  "metrics-server",
			eventName: "metrics-server-786d997795-vr8zc.18afbd49c1b4baed",
			podName:   "metrics-server-786d997795-vr8zc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.presetID, func(t *testing.T) {
			presetEntities := presetEntitiesForTest(t, tc.presetID)
			emptyAll, err := entity.NewEntities(nil)
			require.NoError(t, err)
			mergedPerApp, _, _, err := MergeBuiltinPresetAppsIntoRendered(
				map[types.AppId]entity.Entities{},
				emptyAll,
				sets.New[types.AppId](),
				presetEntities,
			)
			require.NoError(t, err)

			builtinApp := mustBuiltinAppID(t, tc.presetID)
			renderedAll := mergedPerApp[builtinApp]

			event := testLiveEntity("events.k8s.io/v1", "events", "Event", "kube-system", tc.eventName)
			event = withLiveObject(t, event, map[string]any{
				"regarding": map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"name":       tc.podName,
					"namespace":  "kube-system",
				},
				"reason":              "Pulling",
				"action":              "Pulling",
				"reportingController": "kubelet",
				"reportingInstance":   "node-1",
				"type":                "Normal",
				"note":                "test",
			})
			liveEnts, err := entity.NewEntities([]entity.Entity{event})
			require.NoError(t, err)

			assignment, _, _, err := AssignClusterEntitiesToAtMostOneAppByRefs(AssignClusterEntitiesToAtMostOneAppByRefsInput{
				ClusterEntities: liveEnts,
				AllAppIDs:       sets.New[types.AppId](builtinApp),
				PerAppRendered:  mergedPerApp,
				RenderedAllApps: renderedAll,
				Parallel:        1,
				NetworkMode:     types.HelmNetworkModeOffline,
			})
			require.NoError(t, err)

			eventID, err := event.Id()
			require.NoError(t, err)
			assert.Equal(t, builtinApp, assignment[eventID],
				"orphan workload Pod Event must be attributed via the stable template workload anchor")
		})
	}
}

func TestAssignClusterEntitiesToAtMostOneAppByRefs_AssignsPersistentVolumeFromAssignedClaim(t *testing.T) {
	promApp := types.AppId("in-cluster.cluster-infra.kube-prometheus-stack")
	pvcTemplate := testTemplateEntity(
		"persistentvolumeclaims",
		"PersistentVolumeClaim",
		"monitoring",
		"prometheus-kube-prometheus-stack-prometheus-db-prometheus-kube-prometheus-stack-prometheus-0",
		promApp,
	)
	renderedProm := mustEntities(t, []entity.Entity{pvcTemplate})
	renderedAll := renderedProm

	pvc := makeClusterInventoryEntity(
		"",
		"v1",
		"PersistentVolumeClaim",
		"monitoring",
		"prometheus-kube-prometheus-stack-prometheus-db-prometheus-kube-prometheus-stack-prometheus-0",
		"uid-pvc",
		nil,
	)
	pv := makeClusterInventoryEntity(
		"",
		"v1",
		"PersistentVolume",
		"",
		"pvc-5f68d777-e552-4644-acfb-dfad58a881af",
		"uid-pv",
		nil,
	)
	liveEnts, err := entity.NewEntities([]entity.Entity{pvc, pv})
	require.NoError(t, err)

	pvcID, err := pvc.Id()
	require.NoError(t, err)
	pvID, err := pv.Id()
	require.NoError(t, err)

	assignment, _, _, err := AssignClusterEntitiesToAtMostOneAppByRefs(AssignClusterEntitiesToAtMostOneAppByRefsInput{
		ClusterEntities:   liveEnts,
		AllAppIDs:         sets.New[types.AppId](promApp),
		PerAppRendered:    map[types.AppId]entity.Entities{promApp: renderedProm},
		RenderedAllApps:   renderedAll,
		MergedInspectRefs: []types.Ref{{From: pvcID, To: pvID}},
		Parallel:          1,
		NetworkMode:       types.HelmNetworkModeOffline,
	})
	require.NoError(t, err)

	assert.Equal(t, promApp, assignment[pvcID])
	assert.Equal(t, promApp, assignment[pvID],
		"PersistentVolume must inherit app ownership from its bound PersistentVolumeClaim")
}

func mustBuiltinAppID(t *testing.T, presetID string) types.AppId {
	t.Helper()
	appID, err := types.NewPresetAppId(types.InCluster, presetID)
	require.NoError(t, err)
	return appID
}
