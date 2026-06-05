package commands

import (
	"errors"
	"testing"

	"hydra-gitops.org/hydra/hydra-go/base/cache"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

func TestCollectDiscoveryGVKJobs_SkipsCoreV1EventOnly(t *testing.T) {
	t.Parallel()
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Kind: "Pod", Namespaced: true, Verbs: metav1.Verbs{"list"}},
				{Name: "events", Kind: "Event", Namespaced: true, Verbs: metav1.Verbs{"list"}},
			},
		},
		{
			GroupVersion: "events.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "events", Kind: "Event", Namespaced: true, Verbs: metav1.Verbs{"list"}},
			},
		},
	}
	h := &VisitorHandlers{
		HandleApiList: func(*metav1.APIResourceList) (bool, error) { return true, nil },
	}
	jobs, err := collectDiscoveryGVKJobs(lists, h)
	if err != nil {
		t.Fatalf("collectDiscoveryGVKJobs: %v", err)
	}
	var got []types.GVKString
	for _, j := range jobs {
		got = append(got, j.gvk)
	}
	for _, gvk := range got {
		if gvk == types.KubernetesGvkV1Event {
			t.Fatalf("core v1/Event should be skipped, got jobs: %v", got)
		}
	}
	want := []types.GVKString{types.KubernetesGvkV1Pod, types.KubernetesGvkEventsK8sIoV1Event}
	if len(got) != len(want) {
		t.Fatalf("expected %d jobs, got %v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected jobs %v, got %v", want, got)
		}
	}
}

func TestDiscoveryCacheKey_String(t *testing.T) {
	tests := []struct {
		name     string
		key      discoveryCacheKey
		expected string
	}{
		{
			name:     "simple key",
			key:      discoveryCacheKey{contextPath: "/path/to/context", clusterName: "dev"},
			expected: "/path/to/context:dev",
		},
		{
			name:     "different cluster",
			key:      discoveryCacheKey{contextPath: "/path/to/context", clusterName: "prod"},
			expected: "/path/to/context:prod",
		},
		{
			name:     "empty values",
			key:      discoveryCacheKey{contextPath: "", clusterName: ""},
			expected: ":",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.key.String()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDiscoveryCache_FirstCallExecutesLoader(t *testing.T) {
	c := cache.NewCache[discoveryCacheKey, discoveryResult](log.Default(), "discovery-cache", true, nil)
	key := discoveryCacheKey{contextPath: "/ctx", clusterName: "cluster-a"}

	expectedResources := []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "pods", Kind: "Pod"}}},
	}

	loadCount := 0
	result, err := c.GetOrLoad(key, func() (discoveryResult, error) {
		loadCount++
		return discoveryResult{apiResourceLists: expectedResources}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loadCount != 1 {
		t.Errorf("expected loader to be called once, got %d", loadCount)
	}
	if len(result.apiResourceLists) != 1 {
		t.Fatalf("expected 1 API resource list, got %d", len(result.apiResourceLists))
	}
	if result.apiResourceLists[0].GroupVersion != "v1" {
		t.Errorf("expected GroupVersion 'v1', got %q", result.apiResourceLists[0].GroupVersion)
	}
	if result.partialErr != nil {
		t.Errorf("expected nil partialErr, got %v", result.partialErr)
	}
}

func TestDiscoveryCache_SecondCallReturnsCachedResult(t *testing.T) {
	c := cache.NewCache[discoveryCacheKey, discoveryResult](log.Default(), "discovery-cache", true, nil)
	key := discoveryCacheKey{contextPath: "/ctx", clusterName: "cluster-a"}

	expectedResources := []*metav1.APIResourceList{
		{GroupVersion: "apps/v1", APIResources: []metav1.APIResource{{Name: "deployments", Kind: "Deployment"}}},
	}

	loadCount := 0
	loader := func() (discoveryResult, error) {
		loadCount++
		return discoveryResult{apiResourceLists: expectedResources}, nil
	}

	_, _ = c.GetOrLoad(key, loader)

	result, err := c.GetOrLoad(key, loader)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loadCount != 1 {
		t.Errorf("expected loader to be called once, got %d", loadCount)
	}
	if len(result.apiResourceLists) != 1 || result.apiResourceLists[0].GroupVersion != "apps/v1" {
		t.Error("cached result does not match expected")
	}
}

func TestDiscoveryCache_DifferentClustersSeparateEntries(t *testing.T) {
	c := cache.NewCache[discoveryCacheKey, discoveryResult](log.Default(), "discovery-cache", true, nil)

	keyA := discoveryCacheKey{contextPath: "/ctx", clusterName: "cluster-a"}
	keyB := discoveryCacheKey{contextPath: "/ctx", clusterName: "cluster-b"}
	keyC := discoveryCacheKey{contextPath: "/other-ctx", clusterName: "cluster-a"}

	loadCounts := map[string]int{}

	loaderFor := func(gv string) func() (discoveryResult, error) {
		return func() (discoveryResult, error) {
			loadCounts[gv]++
			return discoveryResult{
				apiResourceLists: []*metav1.APIResourceList{{GroupVersion: gv}},
			}, nil
		}
	}

	resultA, _ := c.GetOrLoad(keyA, loaderFor("v1"))
	resultB, _ := c.GetOrLoad(keyB, loaderFor("apps/v1"))
	resultC, _ := c.GetOrLoad(keyC, loaderFor("batch/v1"))

	if loadCounts["v1"] != 1 || loadCounts["apps/v1"] != 1 || loadCounts["batch/v1"] != 1 {
		t.Errorf("expected each loader called once, got %v", loadCounts)
	}
	if resultA.apiResourceLists[0].GroupVersion != "v1" {
		t.Errorf("keyA: expected 'v1', got %q", resultA.apiResourceLists[0].GroupVersion)
	}
	if resultB.apiResourceLists[0].GroupVersion != "apps/v1" {
		t.Errorf("keyB: expected 'apps/v1', got %q", resultB.apiResourceLists[0].GroupVersion)
	}
	if resultC.apiResourceLists[0].GroupVersion != "batch/v1" {
		t.Errorf("keyC: expected 'batch/v1', got %q", resultC.apiResourceLists[0].GroupVersion)
	}
}

func TestDiscoveryCache_PartialErrGroupDiscoveryFailed(t *testing.T) {
	c := cache.NewCache[discoveryCacheKey, discoveryResult](log.Default(), "discovery-cache", true, nil)
	key := discoveryCacheKey{contextPath: "/ctx", clusterName: "cluster-a"}

	partialResources := []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "pods", Kind: "Pod"}}},
	}
	partialErr := &discovery.ErrGroupDiscoveryFailed{
		Groups: map[schema.GroupVersion]error{
			{Group: "metrics.k8s.io", Version: "v1beta1"}: errors.New("unavailable"),
		},
	}

	loadCount := 0
	loader := func() (discoveryResult, error) {
		loadCount++
		return discoveryResult{
			apiResourceLists: partialResources,
			partialErr:       partialErr,
		}, nil
	}

	result1, err := c.GetOrLoad(key, loader)
	if err != nil {
		t.Fatalf("partial errors should not be returned as loader error, got: %v", err)
	}
	if result1.partialErr == nil {
		t.Fatal("expected partialErr to be set")
	}
	if _, ok := result1.partialErr.(*discovery.ErrGroupDiscoveryFailed); !ok {
		t.Errorf("expected *discovery.ErrGroupDiscoveryFailed, got %T", result1.partialErr)
	}
	if len(result1.apiResourceLists) != 1 {
		t.Fatalf("expected 1 API resource list, got %d", len(result1.apiResourceLists))
	}

	result2, err := c.GetOrLoad(key, loader)
	if err != nil {
		t.Fatalf("unexpected error on cache hit: %v", err)
	}
	if loadCount != 1 {
		t.Errorf("expected loader called once, got %d", loadCount)
	}
	if result2.partialErr != partialErr {
		t.Error("cached partialErr does not match original")
	}
}

func TestDiscoveryCache_FatalErrorCached(t *testing.T) {
	c := cache.NewCache[discoveryCacheKey, discoveryResult](log.Default(), "discovery-cache", true, nil)
	key := discoveryCacheKey{contextPath: "/ctx", clusterName: "cluster-a"}

	fatalErr := errors.New("connection refused")

	loadCount := 0
	loader := func() (discoveryResult, error) {
		loadCount++
		return discoveryResult{}, fatalErr
	}

	_, err := c.GetOrLoad(key, loader)
	if err != fatalErr {
		t.Fatalf("expected fatal error %v, got %v", fatalErr, err)
	}

	_, err = c.GetOrLoad(key, loader)
	if err != fatalErr {
		t.Fatalf("expected cached fatal error %v, got %v", fatalErr, err)
	}
	if loadCount != 1 {
		t.Errorf("expected loader called once, got %d", loadCount)
	}
}
