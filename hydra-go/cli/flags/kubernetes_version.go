package flags

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type WithKubernetesVersionFlag interface {
	WithKubernetesVersionFlag() *KubernetesVersionFlag
}

type KubernetesVersionFlag struct {
	KubernetesVersion types.KubernetesVersion
}

var _ Flags = (*KubernetesVersionFlag)(nil)
var _ WithKubernetesVersionFlag = (*KubernetesVersionFlag)(nil)

func (f *KubernetesVersionFlag) Flags() Flags {
	return f
}

func (f *KubernetesVersionFlag) WithKubernetesVersionFlag() *KubernetesVersionFlag {
	return f
}
