package entity

import (
	"testing"
)

// Regression: ScopeInfoMapFromCluster / VisitResources build entities for each
// APIResource with GVK + resource + namespaced only; instance name is added only
// when visiting individual list items.
func TestEntityBuilder_Build_APIResourceTypeWithoutInstanceName(t *testing.T) {
	t.Parallel()

	e, err := NewEntityBuilder().
		WithGroup("apps").
		WithVersion("v1").
		WithResource("deployments").
		WithKind("Deployment").
		WithNamespaced(true).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	idVal, err := e.Id()
	if err != nil {
		t.Fatalf("Id: %v", err)
	}
	id := string(idVal)
	want := "apps/v1/Deployment//"
	if id != want {
		t.Fatalf("Id: got %q, want %q", id, want)
	}
	if _, err := e.Name(); err == nil {
		t.Fatal("Name: expected error when instance name was not set")
	}
}

func TestEntityBuilder_Build_ClusterScopedAPIResourceTypeWithoutInstanceName(t *testing.T) {
	t.Parallel()

	e, err := NewEntityBuilder().
		WithGroup("rbac.authorization.k8s.io").
		WithVersion("v1").
		WithResource("clusterroles").
		WithKind("ClusterRole").
		WithNamespaced(false).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	idVal, err := e.Id()
	if err != nil {
		t.Fatalf("Id: %v", err)
	}
	id := string(idVal)
	want := "rbac.authorization.k8s.io/v1/ClusterRole//"
	if id != want {
		t.Fatalf("Id: got %q, want %q", id, want)
	}
}
