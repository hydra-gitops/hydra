package k8s

import "hydra-gitops.org/hydra/hydra-go/core/entity"

// mustBuild returns the built entity or panics (for tests only).
func mustBuild(b entity.EntityBuilder) entity.Entity {
	e, err := b.Build()
	if err != nil {
		panic(err)
	}
	return e
}
