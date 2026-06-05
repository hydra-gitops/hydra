package flags

import "hydra-gitops.org/hydra/hydra-go/core/types"

type WithKubernetesConnectionAllowedFlag interface {
	WithKubernetesConnectionAllowedFlag() *KubernetesConnectionAllowedFlag
}

type KubernetesConnectionAllowedFlag struct {
	KubernetesConnectionAllowed types.KubernetesConnectionAllowed
}

var _ Flags = (*KubernetesConnectionAllowedFlag)(nil)
var _ WithKubernetesConnectionAllowedFlag = (*KubernetesConnectionAllowedFlag)(nil)

func (f *KubernetesConnectionAllowedFlag) Flags() Flags {
	return f
}

func (f *KubernetesConnectionAllowedFlag) WithKubernetesConnectionAllowedFlag() *KubernetesConnectionAllowedFlag {
	return f
}
