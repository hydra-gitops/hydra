package cel

import (
	goocel "github.com/google/cel-go/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
)

// NewEnvWithEntityInventory builds a CEL environment with ClusterInventorySupport for ents
// (managedNamespaces(), templateEntities(), clusterEntities(), entities(), involvedObjectEvents(...)),
// merged with optional extra options (for example SetSupport). extras are included in the temporary
// environment used to build entity snapshots so declarations stay consistent.
func NewEnvWithEntityInventory(ents entity.Entities, extras ...goocel.EnvOption) (Env, error) {
	return NewEnvWithEntityInventoryOverlay(ents, entity.Entities{}, extras...)
}

func NewEnvWithEntityInventoryOverlay(ents entity.Entities, clusterInventoryOverlay entity.Entities, extras ...goocel.EnvOption) (Env, error) {
	tmp, err := NewEnv(extras...)
	if err != nil {
		return Env{}, err
	}
	inv, err := ClusterInventorySupport(tmp, ents, entity.Entities{}, clusterInventoryOverlay)
	if err != nil {
		return Env{}, err
	}
	return NewEnv(append([]goocel.EnvOption{inv}, extras...)...)
}
