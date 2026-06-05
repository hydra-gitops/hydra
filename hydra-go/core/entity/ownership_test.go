package entity

import (
	"testing"

	"hydra-gitops.org/hydra/hydra-go/core/types"
)

func TestEntityOwnership_DefaultsToUntracked(t *testing.T) {
	t.Parallel()

	e, err := NewEntityBuilder().
		WithGroup("").
		WithVersion("v1").
		WithKind("ConfigMap").
		WithNamespace("default").
		WithName("app-config").
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if e.AppOwned() {
		t.Fatal("AppOwned: expected false by default")
	}
	if e.BuiltIn() {
		t.Fatal("BuiltIn: expected false by default")
	}
	if got := e.Ownership(); got != types.EntityOwnershipUntracked {
		t.Fatalf("Ownership: got %q, want %q", got, types.EntityOwnershipUntracked)
	}
}

func TestEntityOwnership_AppOwnedWinsOverBuiltIn(t *testing.T) {
	t.Parallel()

	e, err := NewEntityBuilder().
		WithGroup("rbac.authorization.k8s.io").
		WithVersion("v1").
		WithKind("ClusterRole").
		WithName("view").
		WithBuiltIn().
		WithAppOwned().
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if !e.AppOwned() {
		t.Fatal("AppOwned: expected true")
	}
	if !e.BuiltIn() {
		t.Fatal("BuiltIn: expected true")
	}
	if got := e.Ownership(); got != types.EntityOwnershipAppOwned {
		t.Fatalf("Ownership: got %q, want %q", got, types.EntityOwnershipAppOwned)
	}
}

func TestEntityOwnership_BuiltInWhenOnlyBuiltInIsSet(t *testing.T) {
	t.Parallel()

	e, err := NewEntityBuilder().
		WithGroup("").
		WithVersion("v1").
		WithKind("Namespace").
		WithName("kube-system").
		WithBuiltIn().
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if got := e.Ownership(); got != types.EntityOwnershipBuiltIn {
		t.Fatalf("Ownership: got %q, want %q", got, types.EntityOwnershipBuiltIn)
	}
}

func TestEntityBuilder_WithAppIdsMarksEntityAsAppOwned(t *testing.T) {
	t.Parallel()

	e, err := NewEntityBuilder().
		WithGroup("").
		WithVersion("v1").
		WithKind("ConfigMap").
		WithNamespace("default").
		WithName("app-config").
		WithAppIds([]types.AppId{"cluster.app"}).
		Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if !e.AppOwned() {
		t.Fatal("AppOwned: expected true")
	}
	if got := e.Ownership(); got != types.EntityOwnershipAppOwned {
		t.Fatalf("Ownership: got %q, want %q", got, types.EntityOwnershipAppOwned)
	}
}
