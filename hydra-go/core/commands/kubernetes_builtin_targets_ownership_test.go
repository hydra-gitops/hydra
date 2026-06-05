package commands

import (
	"testing"

	"hydra-gitops.org/hydra/hydra-go/core/types"
)

func TestSyntheticKubernetesBuiltinEntity_SetsBuiltInOwnership(t *testing.T) {
	t.Parallel()

	e, err := syntheticKubernetesBuiltinEntity(types.KeyTemplateEntity, "v1", "Namespace", "", "kube-system")
	if err != nil {
		t.Fatalf("syntheticKubernetesBuiltinEntity: %v", err)
	}
	if !e.BuiltIn() {
		t.Fatal("BuiltIn: expected true")
	}
	if got := e.Ownership(); got != types.EntityOwnershipBuiltIn {
		t.Fatalf("Ownership: got %q, want %q", got, types.EntityOwnershipBuiltIn)
	}
}
