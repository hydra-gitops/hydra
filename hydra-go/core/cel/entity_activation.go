package cel

import (
	goocel "github.com/google/cel-go/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type entityActivation struct {
	env    *Env
	entity entity.Entity
	cache  map[string]any
}

var _ goocel.Activation = (*entityActivation)(nil)

func newEntityActivation(env *Env, entity entity.Entity) goocel.Activation {
	return &entityActivation{
		env:    env,
		entity: entity,
		cache:  make(map[string]any, len(types.EntityKeys())),
	}
}

func (e *Env) EntityActivation(entity entity.Entity) goocel.Activation {
	return newEntityActivation(e, entity)
}

func (a *entityActivation) ResolveName(name string) (any, bool) {
	if value, ok := a.cache[name]; ok {
		return value, true
	}

	target, ok := a.env.keyByNameCache[name]
	if !ok {
		return nil, false
	}

	value := a.env.keyGetter(target)(a.entity)
	a.cache[name] = value
	return value, true
}

func (a *entityActivation) Parent() goocel.Activation {
	return nil
}

func (a *entityActivation) CachedMap() map[string]any {
	out := make(map[string]any, len(a.cache))
	for k, v := range a.cache {
		out[k] = v
	}
	return out
}
