package flags

import "time"

type WithClusterWorkloadTimeoutFlag interface {
	WithClusterWorkloadTimeoutFlag() *ClusterWorkloadTimeoutFlag
}

type ClusterWorkloadTimeoutFlag struct {
	ClusterWorkloadTimeout time.Duration
}

var _ Flags = (*ClusterWorkloadTimeoutFlag)(nil)
var _ WithClusterWorkloadTimeoutFlag = (*ClusterWorkloadTimeoutFlag)(nil)

func (f *ClusterWorkloadTimeoutFlag) Flags() Flags {
	return f
}

func (f *ClusterWorkloadTimeoutFlag) WithClusterWorkloadTimeoutFlag() *ClusterWorkloadTimeoutFlag {
	return f
}
