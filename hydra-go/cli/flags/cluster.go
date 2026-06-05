package flags

import "hydra-gitops.org/hydra/hydra-go/core/types"

type WithClusterFlag interface {
	WithClusterFlag() *ClusterFlag
}

type ClusterFlag struct {
	Cluster types.ClusterName
}

var _ Flags = (*ClusterFlag)(nil)
var _ WithClusterFlag = (*ClusterFlag)(nil)

func (f *ClusterFlag) Flags() Flags {
	return f
}

func (f *ClusterFlag) WithClusterFlag() *ClusterFlag {
	return f
}
